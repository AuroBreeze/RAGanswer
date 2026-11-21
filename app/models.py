"""协议中的数据结构（第 1.1、2、3、5、6 节）。"""
from __future__ import annotations

import re
from datetime import datetime
from typing import Annotated, Any, Literal, Optional, Union

from pydantic import BaseModel, ConfigDict, Field, field_validator, model_validator

SCHEMA_VERSION = "1.0"
SHA256_RE = re.compile(r"^[0-9a-f]{64}$")


def _check_sha(v: str) -> str:
    if not SHA256_RE.match(v):
        raise ValueError("必须是 64 位小写十六进制 SHA-256")
    return v


class ErrorInfo(BaseModel):
    code: str
    message: str
    retryable: bool


# ---------- 来源登记同步（后端 → RAG） ----------
class DocumentRegistration(BaseModel):
    model_config = ConfigDict(extra="forbid")
    document_id: str = Field(min_length=1)
    source_id: str = Field(min_length=1)
    source_type: Literal["web", "pdf"]
    title: str
    version: Optional[str] = None
    source_sha256: str
    effective: bool = True
    factories: Optional[list[str]] = None
    workshops: Optional[list[str]] = None
    roles: Optional[list[str]] = None

    _sha = field_validator("source_sha256")(_check_sha)


class SourceUpdate(BaseModel):
    model_config = ConfigDict(extra="forbid")
    status: Optional[Literal["active", "revoked"]] = None
    factories: Optional[list[str]] = None
    workshops: Optional[list[str]] = None
    roles: Optional[list[str]] = None


# ---------- 2.2 文档头 ----------
class Warning_(BaseModel):
    model_config = ConfigDict(extra="allow")
    code: str
    message: str
    page: Optional[int] = None


class DocumentHeader(BaseModel):
    model_config = ConfigDict(extra="allow")  # 读取方可忽略新增可选字段
    record_type: Literal["document"]
    schema_version: Literal["1.0"]
    source_id: str = Field(min_length=1)
    document_id: str = Field(min_length=1)
    parse_id: str = Field(min_length=1)
    source_type: Literal["web", "pdf"]
    title: str
    version: Optional[str]
    original_url: Optional[str]
    snapshot_ref: str = Field(min_length=1)
    source_sha256: str
    parsed_at: datetime
    quality: Literal["complete", "partial"]
    block_count: int = Field(gt=0)
    page_count: Optional[int]
    warnings: list[Warning_]

    _sha = field_validator("source_sha256")(_check_sha)

    @field_validator("parsed_at")
    @classmethod
    def _tz(cls, v: datetime) -> datetime:
        if v.tzinfo is None:
            raise ValueError("parsed_at 必须带时区")
        return v

    @model_validator(mode="after")
    def _page_count(self):
        if self.source_type == "web" and self.page_count is not None:
            raise ValueError("网页 page_count 必须为 null")
        if self.source_type == "pdf" and (self.page_count is None or self.page_count < 1):
            raise ValueError("PDF page_count 必须为正整数")
        return self


# ---------- 3 定位 ----------
class WebLocator(BaseModel):
    model_config = ConfigDict(extra="allow")
    type: Literal["web"]
    anchor: Optional[str]


class PdfLocator(BaseModel):
    model_config = ConfigDict(extra="allow")
    type: Literal["pdf"]
    page: int = Field(ge=1)
    printed_page_label: Optional[str] = None
    bbox: Optional[list[float]] = None

    @field_validator("bbox")
    @classmethod
    def _bbox(cls, v):
        if v is None:
            return v
        if len(v) != 4 or any(not (0.0 <= x <= 1.0) for x in v):
            raise ValueError("bbox 必须为 4 个 0–1 的归一化坐标")
        l, t, r, b = v
        if l > r or t > b:
            raise ValueError("bbox 须满足 left<=right, top<=bottom")
        return v


Locator = Annotated[Union[WebLocator, PdfLocator], Field(discriminator="type")]


class TableData(BaseModel):
    model_config = ConfigDict(extra="allow")
    headers: Optional[list[str]] = None
    rows: Optional[list[list[str]]] = None


# ---------- 2.3 内容块 ----------
class Block(BaseModel):
    model_config = ConfigDict(extra="allow")
    record_type: Literal["block"]
    block_id: str = Field(min_length=1)
    sequence: int = Field(ge=1)
    kind: Literal["heading", "paragraph", "list", "table", "caption"]
    text: str
    section_path: list[str]
    locator: Locator
    extraction_method: Literal["html", "pdf_text", "ocr"]
    table: Optional[TableData] = None

    @field_validator("text")
    @classmethod
    def _non_empty(cls, v: str) -> str:
        if not v.strip():
            raise ValueError("text 不能为空")
        return v

    @model_validator(mode="after")
    def _table_only(self):
        if self.table is not None and self.kind != "table":
            raise ValueError("table 字段仅用于 kind=table")
        return self


# ---------- 5 内部接口 ----------
class IngestionRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")
    parse_id: str = Field(min_length=1)
    document_id: str = Field(min_length=1)
    artifact_ref: str = Field(min_length=1)
    artifact_sha256: str

    _sha = field_validator("artifact_sha256")(_check_sha)


class IngestionAccepted(BaseModel):
    job_id: str
    parse_id: str
    status: str


class IngestionStatus(BaseModel):
    job_id: str
    document_id: str
    parse_id: str
    status: Literal["queued", "indexing", "ready", "failed"]
    indexed_chunk_count: Optional[int]
    updated_at: datetime
    error: Optional[ErrorInfo]


# ---------- 6 问答引用 ----------
class Citation(BaseModel):
    document_id: str
    title: str
    version: Optional[str]
    section: Optional[str]
    page: Optional[int]
    excerpt: str
    url: Optional[str]


class ChatRequest(BaseModel):
    question: str = Field(min_length=1, max_length=4000)
    top_k: Optional[int] = Field(default=None, ge=1, le=50)


class ChatResponse(BaseModel):
    answer: str
    citations: list[Citation]
