"""对应协议第 7 节“首次联调验收”中 RAG 侧的验收项。"""
from conftest import SVC, header, ingest, pdf_block, register, user, web_block, write_package


def ask(client, q, h=None):
    r = client.post("/api/v1/chat", json={"question": q}, headers=h or user())
    assert r.status_code == 200, r.text
    return r.json()


def test_web_page_citation(client, tmp_path):
    head = header(source_id="src_web", document_id="doc_web_r1", source_type="web", title="新员工培训须知",
                  version=None, original_url="https://example.com/training", page_count=None)
    register(client, head)
    st, _, _ = ingest(client, tmp_path, head, [web_block(1, "新员工培训签到表由培训室前台发放。", "training-sign-in")],
                      parse_id="parse_web_1")
    assert st["status"] == "ready" and st["indexed_chunk_count"] == 1
    res = ask(client, "培训签到表在哪里领？")
    c = res["citations"][0]
    assert c["page"] is None and c["document_id"] == "doc_web_r1" and c["section"] == "培训签到"
    assert c["url"].endswith("#training-sign-in")


def test_pdf_physical_page(client, tmp_path):
    head = header()
    register(client, head)
    blocks = [pdf_block(1, "本手册适用于全部车间。", 3, section=("1 总则",)),
              pdf_block(2, "点检记录表可在车间资料柜领取。", 4, section=("3 记录管理", "3.2 表单领取"), label="2")]
    st, _, _ = ingest(client, tmp_path, head, blocks)
    assert st["status"] == "ready"
    c = ask(client, "点检记录表可以在哪里领取？")["citations"][0]
    assert c["page"] == 4 and "资料柜" in c["excerpt"] and c["url"].endswith("#page=4")
    assert c["section"] == "3 记录管理 > 3.2 表单领取" and c["version"] == "V3"


def test_cross_page_chunk_cites_pages_separately(client, tmp_path):
    head = header()
    register(client, head)
    blocks = [pdf_block(1, "点检表每日填写一次。", 5), pdf_block(2, "点检表由班长每周签字确认。", 6)]
    ingest(client, tmp_path, head, blocks)
    res = client.post("/api/v1/chat", json={"question": "点检表", "top_k": 1}, headers=user())
    # 一个片段跨 5、6 两页，引用单元按页拆分
    from app import retrieval
    from app.registry import User
    units = retrieval._units(retrieval.search(User("u", "F1", None, []), "点检表", 1))
    assert sorted(retrieval._citation(u).page for u in units) == [5, 6]
    assert res.status_code == 200


def test_partial_requires_review_then_ocr_page_locates(client, tmp_path):
    head = header(quality="partial", warnings=[{"code": "OCR_PAGE_FAILED", "message": "第 3 页 OCR 失败", "page": 3}])
    register(client, head)
    blocks = [pdf_block(1, "文本页内容。", 1), pdf_block(2, "扫描页：润滑周期为 500 小时。", 2, method="ocr")]
    st, _, _ = ingest(client, tmp_path, head, blocks)
    assert st["status"] == "failed" and st["error"]["code"] == "SOURCE_NOT_APPROVED"
    assert ask(client, "润滑周期")["citations"] == []
    assert client.post("/internal/v1/parses/parse_1/approve", headers=SVC).status_code == 204
    st = client.get(f"/internal/v1/ingestions/{st['job_id']}", headers=SVC).json()
    assert st["status"] == "ready"
    assert ask(client, "润滑周期是多少？")["citations"][0]["page"] == 2


def test_duplicate_submit_is_idempotent_and_conflict(client, tmp_path):
    head = header()
    register(client, head)
    st, ref, sha = ingest(client, tmp_path, head, [pdf_block(1, "内容甲。", 1)])
    body = {"parse_id": "parse_1", "document_id": "doc_v3", "artifact_ref": ref, "artifact_sha256": sha}
    r = client.post("/internal/v1/ingestions", json=body, headers=SVC)
    assert r.status_code == 202 and r.json()["job_id"] == st["job_id"] and r.json()["status"] == "ready"
    from app.db import query_one
    assert query_one("SELECT COUNT(*) n FROM chunks")["n"] == 1
    r = client.post("/internal/v1/ingestions", json={**body, "artifact_sha256": "c" * 64}, headers=SVC)
    assert r.status_code == 409 and r.json()["error"]["code"] == "PARSE_CONFLICT"


def test_new_version_and_permission_revoke(client, tmp_path):
    v3 = header()
    register(client, v3, factories=["F1"])
    ingest(client, tmp_path, v3, [pdf_block(1, "点检周期为每周一次。", 2)])
    assert ask(client, "点检周期")["citations"][0]["version"] == "V3"

    v4 = header(document_id="doc_v4", version="V4", source_sha256="d" * 64)
    register(client, v4)
    ingest(client, tmp_path, v4, [pdf_block(1, "点检周期调整为每日一次。", 2)], parse_id="parse_2")
    c = ask(client, "点检周期")["citations"][0]
    assert c["version"] == "V4" and c["document_id"] == "doc_v4"

    # 旧版本的晚到任务不会覆盖新索引
    ingest(client, tmp_path, v3, [pdf_block(1, "点检周期为每周一次。", 2)], parse_id="parse_3")
    assert ask(client, "点检周期")["citations"][0]["version"] == "V4"

    # 其他厂区账号无权命中
    assert ask(client, "点检周期", user("u2", factory="F2"))["citations"] == []
    # 撤销权限后立即排除
    assert client.patch("/internal/v1/sources/src_pdf", json={"status": "revoked"}, headers=SVC).status_code == 204
    assert ask(client, "点检周期")["citations"] == []


def test_invalid_artifacts(client, tmp_path):
    head = header()
    register(client, head)
    # sequence 不连续
    bad = [pdf_block(1, "a", 1), {**pdf_block(2, "b", 1), "sequence": 3}]
    st, _, _ = ingest(client, tmp_path, head, bad, parse_id="p_bad1")
    assert st["status"] == "failed" and st["error"]["code"] == "ARTIFACT_INVALID"
    # 版本与登记不一致
    st, _, _ = ingest(client, tmp_path, {**head, "version": "V9"}, [pdf_block(1, "a", 1)], parse_id="p_bad2")
    assert st["error"]["code"] == "ARTIFACT_INVALID"
    # 页码超出 page_count
    st, _, _ = ingest(client, tmp_path, head, [pdf_block(1, "a", 99)], parse_id="p_bad3")
    assert st["error"]["code"] == "ARTIFACT_INVALID"
    # 摘要不匹配（读取后发现）
    ref, _ = write_package(tmp_path, "p_bad4", head, [pdf_block(1, "a", 1)])
    r = client.post("/internal/v1/ingestions", headers=SVC, json={
        "parse_id": "p_bad4", "document_id": "doc_v3", "artifact_ref": ref, "artifact_sha256": "e" * 64})
    st = client.get(f"/internal/v1/ingestions/{r.json()['job_id']}", headers=SVC).json()
    assert st["error"]["code"] == "ARTIFACT_INVALID"


def test_request_errors(client):
    body = {"parse_id": "p", "document_id": "nope", "artifact_ref": "parsed/p.ndjson", "artifact_sha256": "a" * 64}
    assert client.post("/internal/v1/ingestions", json=body).status_code == 401
    assert client.post("/internal/v1/ingestions", json=body, headers={"Authorization": "Bearer x"}).status_code == 403
    r = client.post("/internal/v1/ingestions", json=body, headers=SVC)
    assert r.status_code == 404 and r.json()["error"]["code"] == "DOCUMENT_NOT_FOUND"
    r = client.post("/internal/v1/ingestions", json={**body, "artifact_sha256": "XYZ"}, headers=SVC)
    assert r.status_code == 400 and r.json()["error"]["code"] == "INVALID_REQUEST"
