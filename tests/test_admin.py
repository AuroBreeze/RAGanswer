import json

from conftest import SVC, header, user, web_block


def test_admin_upload_review_and_revoke_reaches_chat(client):
    route = "/internal/v1/admin/documents"
    assert client.get(route).status_code == 401
    assert client.post(route, content=b"bad data").status_code == 401
    assert client.get(route, headers=SVC).json() == {"items": []}

    head = header(document_id="doc_admin", source_id="src_admin", parse_id="parse_admin", source_type="web",
                  title="阀门巡检手册", original_url="https://example.com/checks", page_count=None,
                  quality="partial", block_count=1)
    data = "\n".join(json.dumps(row, ensure_ascii=False) for row in (
        head, web_block(1, "阀门巡检在每周二进行。", None, section=("巡检计划",)),
    )).encode("utf-8")
    uploaded = client.post(route, headers=SVC, content=data)
    assert uploaded.status_code == 202, uploaded.text
    job = uploaded.json()
    assert job["status"] == "failed"
    row = client.get(route, headers=SVC).json()["items"][0]
    assert row["error"]["code"] == "SOURCE_NOT_APPROVED" and not row["searchable"]
    assert client.post(route, headers=SVC, content=data).json()["job_id"] == job["job_id"]
    assert client.post("/internal/v1/admin/parses/parse_admin/approve", headers=SVC).status_code == 204
    row = client.get(route, headers=SVC).json()["items"][0]
    assert row["job_status"] == "ready" and row["indexed_chunk_count"] == 1 and row["searchable"]
    question = {"question": "阀门巡检是什么时候？"}
    answer = client.post("/api/v1/chat", headers=user(), json=question).json()
    assert "每周二" in answer["answer"] and answer["citations"][0]["document_id"] == "doc_admin"

    assert client.post(f"{route}/doc_admin/revoke", headers=SVC).status_code == 204
    assert not client.get(route, headers=SVC).json()["items"][0]["searchable"]
    assert client.post("/api/v1/chat", headers=user(), json=question).json()["citations"] == []
