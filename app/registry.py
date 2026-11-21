"""RAG 侧持有的来源/版本/权限镜像，以及索引发布切换（第 4 节）。

来源登记由后端实现；后端在登记版本、修改权限、废止时调用 /internal/v1/documents 等接口同步到这里。
"""
from __future__ import annotations

import sqlite3
from dataclasses import dataclass, field
from typing import Optional

from .db import jdump, jload, now_iso, query, query_one, tx
from .errors import DOCUMENT_NOT_FOUND, ProtocolError


@dataclass
class User:
    user_id: str
    factory: Optional[str] = None
    workshop: Optional[str] = None
    roles: list[str] = field(default_factory=list)


def upsert_document(*, document_id: str, source_id: str, source_type: str, title: str, version: str | None,
                    source_sha256: str, effective: bool = True, factories=None, workshops=None, roles=None) -> None:
    """登记（或更新）一个不可变文档版本及其来源授权。版本顺序按首次登记先后。"""
    with tx() as c:
        src = c.execute("SELECT 1 FROM sources WHERE source_id=?", (source_id,)).fetchone()
        if src is None:
            c.execute("INSERT INTO sources(source_id, factories, workshops, roles) VALUES (?,?,?,?)",
                      (source_id, jdump(factories or []), jdump(workshops or []), jdump(roles or [])))
        elif factories is not None or workshops is not None or roles is not None:
            _set_acl(c, source_id, factories, workshops, roles)
        old = c.execute("SELECT seq FROM documents WHERE document_id=?", (document_id,)).fetchone()
        if old is None:
            seq = c.execute("SELECT COALESCE(MAX(seq),0)+1 FROM documents WHERE source_id=?", (source_id,)).fetchone()[0]
            c.execute("INSERT INTO documents(document_id, source_id, seq, source_type, title, version, source_sha256, "
                      "effective) VALUES (?,?,?,?,?,?,?,?)",
                      (document_id, source_id, seq, source_type, title, version, source_sha256, int(effective)))
        else:
            c.execute("UPDATE documents SET title=?, version=?, effective=? WHERE document_id=?",
                      (title, version, int(effective), document_id))
    if effective:
        _publish_latest_ready(document_id)


def _set_acl(c, source_id, factories, workshops, roles):
    for col, val in (("factories", factories), ("workshops", workshops), ("roles", roles)):
        if val is not None:
            c.execute(f"UPDATE sources SET {col}=? WHERE source_id=?", (jdump(val), source_id))


def update_source(source_id: str, *, status: str | None = None, factories=None, workshops=None, roles=None) -> None:
    with tx() as c:
        if c.execute("SELECT 1 FROM sources WHERE source_id=?", (source_id,)).fetchone() is None:
            raise ProtocolError("SOURCE_NOT_FOUND", "来源不存在。", http_status=404)
        if status:
            c.execute("UPDATE sources SET status=? WHERE source_id=?", (status, source_id))
        _set_acl(c, source_id, factories, workshops, roles)


def revoke_document(document_id: str) -> None:
    """显式废止版本：立即从检索中排除。"""
    with tx() as c:
        c.execute("UPDATE documents SET status='revoked', effective=0 WHERE document_id=?", (document_id,))
        c.execute("UPDATE sources SET active_document_id=NULL, active_parse_id=NULL, active_job_seq=NULL "
                  "WHERE active_document_id=?", (document_id,))


def approve_parse(parse_id: str) -> None:
    with tx() as c:
        c.execute("INSERT OR IGNORE INTO approvals(parse_id, approved_at) VALUES (?,?)", (parse_id, now_iso()))


def is_approved(parse_id: str) -> bool:
    return query_one("SELECT 1 FROM approvals WHERE parse_id=?", (parse_id,)) is not None


def require_document(document_id: str) -> sqlite3.Row:
    d = query_one("SELECT * FROM documents WHERE document_id=?", (document_id,))
    if d is None:
        raise ProtocolError(DOCUMENT_NOT_FOUND, "未知文档。", http_status=404)
    return d


# ---------------- 权限 ----------------
def can_access(user: User, src: sqlite3.Row) -> bool:
    """空列表表示该维度不限制。"""
    if src is None or src["status"] != "active":
        return False
    f, w, r = jload(src["factories"], []), jload(src["workshops"], []), jload(src["roles"], [])
    return (not f or user.factory in f) and (not w or user.workshop in w) and (not r or bool(set(r) & set(user.roles)))


def accessible_active_parses(user: User) -> list[str]:
    rows = query("SELECT s.* FROM sources s JOIN documents d ON d.document_id=s.active_document_id "
                 "WHERE s.status='active' AND d.status='active' AND s.active_parse_id IS NOT NULL")
    return [r["active_parse_id"] for r in rows if can_access(user, r)]


# ---------------- 发布切换 ----------------
def try_publish(parse_id: str) -> bool:
    """入库 ready 后核对来源记录：版本已生效且未废止，并且不比当前检索版本旧时才切换。

    “旧”的判定：文档版本序号更小；或同一版本但入库任务更早提交（避免晚完成的旧任务覆盖新索引）。
    """
    with tx() as c:
        row = c.execute(
            "SELECT j.seq AS jseq, d.document_id, d.source_id, d.seq AS dseq, d.effective, d.status "
            "FROM ingestion_jobs j JOIN documents d ON d.document_id=j.document_id "
            "WHERE j.parse_id=? AND j.status='ready'", (parse_id,)).fetchone()
        if row is None or not row["effective"] or row["status"] != "active":
            return False
        src = c.execute("SELECT * FROM sources WHERE source_id=?", (row["source_id"],)).fetchone()
        if src["active_document_id"]:
            cur = c.execute("SELECT seq FROM documents WHERE document_id=?", (src["active_document_id"],)).fetchone()
            if (cur["seq"], src["active_job_seq"] or 0) > (row["dseq"], row["jseq"]):
                return False
        c.execute("UPDATE sources SET active_document_id=?, active_parse_id=?, active_job_seq=? WHERE source_id=?",
                  (row["document_id"], parse_id, row["jseq"], row["source_id"]))
        return True


def _publish_latest_ready(document_id: str) -> None:
    r = query_one("SELECT parse_id FROM ingestion_jobs WHERE document_id=? AND status='ready' "
                  "ORDER BY seq DESC LIMIT 1", (document_id,))
    if r:
        try_publish(r["parse_id"])
