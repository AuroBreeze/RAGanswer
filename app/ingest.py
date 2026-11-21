"""入库：受理 → 校验解析包 → 切分 → 向量化 → 写索引 → 发布（第 4、5 节）。"""
from __future__ import annotations

import array
import logging
import time
import uuid
from concurrent.futures import ThreadPoolExecutor

from . import llm, registry
from .chunker import chunk_blocks
from .config import get_settings
from .db import jdump, now_iso, query_one, tx
from .errors import (ARTIFACT_INVALID, INDEX_UNAVAILABLE, PARSE_CONFLICT, SOURCE_NOT_APPROVED, ProtocolError)
from .models import IngestionRequest
from .package import parse_package, read_artifact

log = logging.getLogger(__name__)
_executor = ThreadPoolExecutor(max_workers=2, thread_name_prefix="ingest")
SYNC = False  # 测试时置 True，同步执行


def get_job(job_id: str):
    return query_one("SELECT * FROM ingestion_jobs WHERE job_id=?", (job_id,))


def submit(req: IngestionRequest):
    """受理导入任务。返回 job 行。按 parse_id 幂等。"""
    registry.require_document(req.document_id)  # 404 DOCUMENT_NOT_FOUND
    with tx() as c:
        old = c.execute("SELECT * FROM ingestion_jobs WHERE parse_id=?", (req.parse_id,)).fetchone()
        if old is not None:
            if old["document_id"] != req.document_id or old["artifact_sha256"] != req.artifact_sha256:
                raise ProtocolError(PARSE_CONFLICT, "相同 parse_id 对应了不同的文档或产物摘要。", http_status=409)
            return old
        job_id = f"ing_{uuid.uuid4().hex[:16]}"
        c.execute("INSERT INTO ingestion_jobs(job_id, parse_id, document_id, artifact_ref, artifact_sha256, status, "
                  "updated_at) VALUES (?,?,?,?,?,'queued',?)",
                  (job_id, req.parse_id, req.document_id, req.artifact_ref, req.artifact_sha256, now_iso()))
    if SYNC:
        run(job_id)
    else:
        _executor.submit(run, job_id)
    return get_job(job_id)


def _set(job_id: str, status: str, count=None, error: dict | None = None):
    with tx() as c:
        c.execute("UPDATE ingestion_jobs SET status=?, indexed_chunk_count=?, error=?, updated_at=? WHERE job_id=?",
                  (status, count, jdump(error) if error else None, now_iso(), job_id))


def _validate_against_registry(job, pkg) -> None:
    h = pkg.header
    doc = registry.require_document(job["document_id"])
    if h.parse_id != job["parse_id"] or h.document_id != job["document_id"]:
        raise ProtocolError(ARTIFACT_INVALID, "解析包的 parse_id 或 document_id 与请求不一致。")
    if (h.source_id, h.source_type, h.source_sha256, h.version) != \
            (doc["source_id"], doc["source_type"], doc["source_sha256"], doc["version"]):
        raise ProtocolError(ARTIFACT_INVALID, "解析包的来源、类型、原件摘要或版本与登记记录不一致。")
    if doc["status"] != "active":
        raise ProtocolError(SOURCE_NOT_APPROVED, "该文档版本已废止。")
    if h.quality == "partial" and not registry.is_approved(h.parse_id):
        raise ProtocolError(SOURCE_NOT_APPROVED, "quality=partial 的产物需人工复核通过后才能入库。")


def run(job_id: str) -> None:
    s = get_settings()
    job = get_job(job_id)
    if job is None or job["status"] in ("ready", "indexing"):
        return
    _set(job_id, "indexing")
    try:
        pkg = parse_package(read_artifact(job["artifact_ref"]), job["artifact_sha256"])
        _validate_against_registry(job, pkg)
        chunks = chunk_blocks(pkg.blocks, s.chunk_max_chars, s.chunk_overlap_chars)
        # 向量化文本带上标题与章节，提高召回；存储的 text 仍为原文
        inputs = [" / ".join([pkg.header.title, *c.section_path]) + "\n" + c.text for c in chunks]
        vectors = None
        for attempt in range(s.index_max_retries):
            try:
                vectors = llm.embed(inputs)
                break
            except Exception as e:  # 依赖故障：限次重试
                log.warning("embedding 失败（第 %d 次）：%s", attempt + 1, e)
                time.sleep(min(2 ** attempt, 10))
        if vectors is None:
            raise ProtocolError(INDEX_UNAVAILABLE, "索引服务暂时不可用。", retryable=True)
        with tx() as c:  # 先删后写，重试写入去重
            c.execute("DELETE FROM chunks WHERE parse_id=?", (job["parse_id"],))
            c.executemany(
                "INSERT INTO chunks(chunk_id, parse_id, document_id, text, section_path, sources, embedding) "
                "VALUES (?,?,?,?,?,?,?)",
                [(f"{job['parse_id']}:{i:05d}", job["parse_id"], job["document_id"], ch.text,
                  jdump(ch.section_path), jdump(ch.sources), array.array("f", v).tobytes())
                 for i, (ch, v) in enumerate(zip(chunks, vectors))])
        _set(job_id, "ready", count=len(chunks))
        registry.try_publish(job["parse_id"])
    except ProtocolError as e:
        _set(job_id, "failed", error=e.to_dict())
    except Exception as e:
        log.exception("入库失败")
        _set(job_id, "failed", error={"code": "INTERNAL_ERROR", "message": str(e)[:200], "retryable": True})


def retry(parse_id: str) -> None:
    """复核通过或依赖恢复后，对同一任务重跑（同一 parse_id，不新建任务）。"""
    job = query_one("SELECT * FROM ingestion_jobs WHERE parse_id=?", (parse_id,))
    if job and job["status"] == "failed":
        _set(job["job_id"], "queued")
        (run if SYNC else lambda j: _executor.submit(run, j))(job["job_id"])
