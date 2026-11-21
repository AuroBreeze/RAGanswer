import hashlib
import json

import pytest
from fastapi.testclient import TestClient

TOKEN = "test-service-token"
SVC = {"Authorization": f"Bearer {TOKEN}"}


@pytest.fixture
def client(tmp_path, monkeypatch):
    monkeypatch.setenv("RAG_DATA_DIR", str(tmp_path))
    monkeypatch.setenv("RAG_ARTIFACT_ROOT", str(tmp_path / "storage"))
    monkeypatch.setenv("RAG_SERVICE_TOKENS", TOKEN)
    monkeypatch.setenv("RAG_EMBEDDING_PROVIDER", "hash")
    monkeypatch.setenv("RAG_LLM_PROVIDER", "extractive")
    from app import ingest
    from app.config import get_settings
    get_settings.cache_clear()
    monkeypatch.setattr(ingest, "SYNC", True)
    from app.main import app
    yield TestClient(app)
    get_settings.cache_clear()


def user(uid="u1", roles="worker", factory="F1", workshop=None):
    h = {"X-User-Id": uid, "X-User-Roles": roles, "X-User-Factory": factory}
    if workshop:
        h["X-User-Workshop"] = workshop
    return h


def header(**kw):
    h = {"record_type": "document", "schema_version": "1.0", "source_id": "src_pdf", "document_id": "doc_v3",
         "parse_id": "parse_1", "source_type": "pdf", "title": "设备点检记录管理须知", "version": "V3",
         "original_url": None, "snapshot_ref": "snapshot/pdf/v3.pdf", "source_sha256": "b" * 64,
         "parsed_at": "2026-11-21T09:35:00+08:00", "quality": "complete", "block_count": 0, "page_count": 8,
         "warnings": []}
    h.update(kw)
    return h


def pdf_block(i, text, page, section=("3 记录管理",), method="pdf_text", label=None):
    loc = {"type": "pdf", "page": page}
    if label:
        loc["printed_page_label"] = label
    return {"record_type": "block", "block_id": f"b{i:06d}", "sequence": i, "kind": "paragraph", "text": text,
            "section_path": list(section), "locator": loc, "extraction_method": method}


def web_block(i, text, anchor, section=("培训签到",)):
    return {"record_type": "block", "block_id": f"b{i:06d}", "sequence": i, "kind": "paragraph", "text": text,
            "section_path": list(section), "locator": {"type": "web", "anchor": anchor}, "extraction_method": "html"}


def write_package(tmp_path, parse_id, head, blocks):
    head = {**head, "parse_id": parse_id, "block_count": len(blocks)}
    data = "\n".join(json.dumps(x, ensure_ascii=False) for x in [head, *blocks]).encode("utf-8")
    ref = f"parsed/{parse_id}.ndjson"
    p = tmp_path / "storage" / "parsed"
    p.mkdir(parents=True, exist_ok=True)
    (p / f"{parse_id}.ndjson").write_bytes(data)
    return ref, hashlib.sha256(data).hexdigest()


def register(client, head, **acl):
    body = {"document_id": head["document_id"], "source_id": head["source_id"], "source_type": head["source_type"],
            "title": head["title"], "version": head["version"], "source_sha256": head["source_sha256"], **acl}
    r = client.post("/internal/v1/documents", json=body, headers=SVC)
    assert r.status_code == 204, r.text


def ingest(client, tmp_path, head, blocks, parse_id="parse_1"):
    ref, sha = write_package(tmp_path, parse_id, head, blocks)
    r = client.post("/internal/v1/ingestions", headers=SVC, json={
        "parse_id": parse_id, "document_id": head["document_id"], "artifact_ref": ref, "artifact_sha256": sha})
    assert r.status_code == 202, r.text
    job = r.json()
    return client.get(f"/internal/v1/ingestions/{job['job_id']}", headers=SVC).json(), ref, sha
