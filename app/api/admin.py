from __future__ import annotations

import hashlib
from pathlib import Path
from tempfile import NamedTemporaryFile

from fastapi import APIRouter, Depends, HTTPException, Request
from starlette.concurrency import run_in_threadpool

from .. import ingest, registry
from ..config import get_settings
from ..db import jload, query, query_one
from ..errors import ARTIFACT_INVALID, PARSE_CONFLICT, SOURCE_NOT_APPROVED, ProtocolError
from ..models import IngestionAccepted, IngestionRequest
from ..package import parse_package
from .auth import service_auth

router = APIRouter(prefix="/internal/v1/admin", dependencies=[Depends(service_auth)])


@router.get("/documents")
def list_documents():
    rows = query(
        """SELECT d.document_id, d.source_type, d.title, d.version, d.status AS document_status,
                  s.status AS source_status, s.active_document_id, j.parse_id, j.status AS job_status,
                  j.indexed_chunk_count, j.updated_at, j.error
           FROM documents d JOIN sources s ON d.source_id = s.source_id
           LEFT JOIN ingestion_jobs j ON j.seq = (
               SELECT MAX(latest.seq) FROM ingestion_jobs latest WHERE latest.document_id = d.document_id
           )
           ORDER BY j.seq DESC, d.document_id LIMIT 100"""
    )
    items = []
    for row in rows:
        item = dict(row)
        item["searchable"] = (item.pop("active_document_id") == item["document_id"]
                              and item["document_status"] == item["source_status"] == "active")
        item["error"] = jload(item["error"])
        items.append(item)
    return {"items": items}


def _store_artifact(ref: str, data: bytes) -> None:
    root = get_settings().artifact_root.resolve()
    dest = (root / ref).resolve()
    if root not in dest.parents:
        raise ProtocolError(ARTIFACT_INVALID, "解析包路径超出受控存储范围。")
    dest.parent.mkdir(parents=True, exist_ok=True)
    if dest.exists():
        if dest.read_bytes() != data:
            raise ProtocolError(ARTIFACT_INVALID, "已有解析包的内容与摘要不一致。")
        return
    with NamedTemporaryFile(dir=dest.parent, delete=False) as stream:
        temporary = Path(stream.name)
        try:
            stream.write(data)
            stream.close()
            temporary.replace(dest)
        finally:
            temporary.unlink(missing_ok=True)


def _import_artifact(data: bytes) -> IngestionAccepted:
    sha = hashlib.sha256(data).hexdigest()
    header = parse_package(data, sha).header
    doc = query_one("SELECT * FROM documents WHERE document_id=?", (header.document_id,))
    old_job = query_one("SELECT * FROM ingestion_jobs WHERE parse_id=?", (header.parse_id,))
    if doc and any(doc[key] != getattr(header, key) for key in ("source_id", "source_type", "source_sha256", "version")):
        raise ProtocolError(ARTIFACT_INVALID, "文档 ID 已绑定其他原件或版本。", http_status=409)
    if old_job and (old_job["document_id"] != header.document_id or old_job["artifact_sha256"] != sha):
        raise ProtocolError(PARSE_CONFLICT, "解析 ID 已绑定其他解析包。", http_status=409)
    if old_job:
        return IngestionAccepted(job_id=old_job["job_id"], parse_id=old_job["parse_id"], status=old_job["status"])

    ref = f"parsed/admin/{sha}.ndjson"
    _store_artifact(ref, data)
    if doc is None:
        registry.upsert_document(document_id=header.document_id, source_id=header.source_id,
                                 source_type=header.source_type, title=header.title, version=header.version,
                                 source_sha256=header.source_sha256)
    job = ingest.submit(IngestionRequest(parse_id=header.parse_id, document_id=header.document_id,
                                           artifact_ref=ref, artifact_sha256=sha))
    return IngestionAccepted(job_id=job["job_id"], parse_id=job["parse_id"], status=job["status"])


@router.post("/documents", status_code=202, response_model=IngestionAccepted)
async def import_artifact(request: Request):
    data = bytearray()
    limit = min(get_settings().max_artifact_bytes, 10 * 1024 * 1024)
    async for chunk in request.stream():
        if len(data) + len(chunk) > limit:
            raise ProtocolError(ARTIFACT_INVALID, "管理页上传上限为 10 MiB。", http_status=413)
        data.extend(chunk)
    return await run_in_threadpool(_import_artifact, bytes(data))


@router.post("/parses/{parse_id}/approve", status_code=204)
def approve_artifact(parse_id: str):
    row = query_one("SELECT status, error FROM ingestion_jobs WHERE parse_id=?", (parse_id,))
    if row is None:
        raise HTTPException(404, "解析任务不存在")
    if row["status"] != "failed" or jload(row["error"], {}).get("code") != SOURCE_NOT_APPROVED:
        raise HTTPException(409, "当前任务不处于待复核状态")
    registry.approve_parse(parse_id)
    ingest.retry(parse_id)


@router.post("/documents/{document_id}/revoke", status_code=204)
def revoke_document(document_id: str):
    registry.require_document(document_id)
    registry.revoke_document(document_id)
