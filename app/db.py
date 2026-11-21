from __future__ import annotations

import json
import sqlite3
import threading
from contextlib import contextmanager
from datetime import datetime, timezone
from typing import Any, Iterator

from .config import get_settings

_lock = threading.Lock()
_initialized: set[str] = set()

SCHEMA = """
-- 受控来源记录（由来源登记方同步过来）：授权元数据、生效状态、当前检索版本
CREATE TABLE IF NOT EXISTS sources (
    source_id          TEXT PRIMARY KEY,
    status             TEXT NOT NULL DEFAULT 'active',   -- active | revoked
    factories          TEXT NOT NULL DEFAULT '[]',
    workshops          TEXT NOT NULL DEFAULT '[]',
    roles              TEXT NOT NULL DEFAULT '[]',
    active_document_id TEXT,
    active_parse_id    TEXT,
    active_job_seq     INTEGER
);
CREATE TABLE IF NOT EXISTS documents (
    document_id   TEXT PRIMARY KEY,
    source_id     TEXT NOT NULL REFERENCES sources(source_id),
    seq           INTEGER NOT NULL,        -- 同一来源内的版本顺序
    source_type   TEXT NOT NULL,
    title         TEXT NOT NULL,
    version       TEXT,
    source_sha256 TEXT NOT NULL,
    effective     INTEGER NOT NULL DEFAULT 1,
    status        TEXT NOT NULL DEFAULT 'active'  -- active | revoked
);
-- quality=partial 的人工复核通过标记
CREATE TABLE IF NOT EXISTS approvals (parse_id TEXT PRIMARY KEY, approved_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS ingestion_jobs (
    seq                 INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id              TEXT UNIQUE NOT NULL,
    parse_id            TEXT UNIQUE NOT NULL,
    document_id         TEXT NOT NULL,
    artifact_ref        TEXT NOT NULL,
    artifact_sha256     TEXT NOT NULL,
    status              TEXT NOT NULL,     -- queued | indexing | ready | failed
    indexed_chunk_count INTEGER,
    error               TEXT,
    updated_at          TEXT NOT NULL
);
-- 检索片段：向量 + 与原始内容块的映射
CREATE TABLE IF NOT EXISTS chunks (
    chunk_id     TEXT PRIMARY KEY,
    parse_id     TEXT NOT NULL,
    document_id  TEXT NOT NULL,
    text         TEXT NOT NULL,
    section_path TEXT NOT NULL,
    sources      TEXT NOT NULL,   -- [{block_id, locator, text}]
    embedding    BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chunks_parse ON chunks(parse_id);
"""


def now_iso() -> str:
    return datetime.now(timezone.utc).astimezone().isoformat(timespec="seconds")


def connect() -> sqlite3.Connection:
    s = get_settings()
    s.data_dir.mkdir(parents=True, exist_ok=True)
    path = str(s.db_path.resolve())
    conn = sqlite3.connect(path, timeout=30, isolation_level=None, check_same_thread=False)
    conn.row_factory = sqlite3.Row
    if path not in _initialized:
        with _lock:
            if path not in _initialized:
                conn.execute("PRAGMA journal_mode=WAL")
                conn.executescript(SCHEMA)
                _initialized.add(path)
    return conn


@contextmanager
def tx() -> Iterator[sqlite3.Connection]:
    """立即获取写锁的事务，用于“读-判断-写”。"""
    conn = connect()
    try:
        conn.execute("BEGIN IMMEDIATE")
        yield conn
        conn.execute("COMMIT")
    except BaseException:
        conn.execute("ROLLBACK")
        raise
    finally:
        conn.close()


def query(sql: str, params: tuple | list = ()) -> list[sqlite3.Row]:
    conn = connect()
    try:
        return conn.execute(sql, params).fetchall()
    finally:
        conn.close()


def query_one(sql: str, params: tuple | list = ()) -> sqlite3.Row | None:
    rows = query(sql, params)
    return rows[0] if rows else None


def jdump(v: Any) -> str:
    return json.dumps(v, ensure_ascii=False)


def jload(v: str | None, default: Any = None) -> Any:
    return default if v is None else json.loads(v)
