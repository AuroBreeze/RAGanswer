"""解析包读取与校验（第 2 节）。任一项不通过即抛 ARTIFACT_INVALID。"""
from __future__ import annotations

import hashlib
import json
from dataclasses import dataclass
from pathlib import Path, PurePosixPath

from pydantic import ValidationError

from . import adapters
from .config import get_settings
from .errors import ARTIFACT_INVALID, ProtocolError
from .models import Block, DocumentHeader


@dataclass
class Package:
    header: DocumentHeader
    blocks: list[Block]


def _bad(msg: str) -> ProtocolError:
    return ProtocolError(ARTIFACT_INVALID, msg)


def read_artifact(artifact_ref: str) -> bytes:
    """只允许从受控存储根目录读取，拒绝绝对路径与 .. 穿越。"""
    s = get_settings()
    p = PurePosixPath(artifact_ref)
    if "\\" in artifact_ref or p.is_absolute() or ".." in p.parts:
        raise _bad("artifact_ref 不在允许的存储范围内。")
    root = s.artifact_root.resolve()
    full = (root / Path(*p.parts)).resolve()
    if root not in full.parents or not full.is_file():
        raise _bad("解析包不存在。")
    if full.stat().st_size > s.max_artifact_bytes:
        raise _bad("解析包超过大小上限。")
    return full.read_bytes()


def parse_package(
    data: bytes,
    expected_sha256: str,
    *,
    fallback_document_id: str | None = None,
    fallback_source_id: str | None = None,
    fallback_parse_id: str | None = None,
) -> Package:
    if hashlib.sha256(data).hexdigest() != expected_sha256:
        raise _bad("解析包摘要不匹配。")
    try:
        text = data.decode("utf-8")
    except UnicodeDecodeError:
        raise _bad("解析包不是 UTF-8 编码。")
    # 兼容单个 JSON 展示样例（selected_web_extraction_for_display）
    stripped = text.strip()
    if stripped.startswith("{"):
        try:
            whole = json.loads(stripped)
        except ValueError:
            whole = None
        if adapters.is_display_sample(whole):
            try:
                header, blocks = adapters.parse_display_sample(
                    whole,
                    document_id=fallback_document_id,
                    source_id=fallback_source_id,
                    parse_id=fallback_parse_id,
                )
            except ValidationError as e:
                raise _bad(f"展示样例无效: {e}")
            return Package(header, blocks)

    lines = [ln for ln in text.splitlines() if ln.strip()]
    if not lines:
        raise _bad("解析包为空。")
    try:
        header = DocumentHeader.model_validate(json.loads(lines[0]))
    except (ValueError, ValidationError) as e:
        raise _bad(f"文档头无效: {e}")

    blocks: list[Block] = []
    seen: set[str] = set()
    for i, ln in enumerate(lines[1:], start=1):
        try:
            b = Block.model_validate(json.loads(ln))
        except (ValueError, ValidationError) as e:
            raise _bad(f"第 {i + 1} 行内容块无效: {e}")
        if b.sequence != i:
            raise _bad(f"sequence 必须从 1 连续递增，第 {i + 1} 行为 {b.sequence}。")
        if b.block_id in seen:
            raise _bad(f"block_id 重复: {b.block_id}")
        seen.add(b.block_id)
        if b.locator.type != header.source_type:
            raise _bad(f"{b.block_id} 的定位类型与来源类型不符。")
        if header.source_type == "web" and b.extraction_method != "html":
            raise _bad(f"{b.block_id} 网页块 extraction_method 必须为 html。")
        if header.source_type == "pdf":
            if b.extraction_method not in ("pdf_text", "ocr"):
                raise _bad(f"{b.block_id} PDF 块 extraction_method 必须为 pdf_text 或 ocr。")
            if b.locator.page > header.page_count:
                raise _bad(f"{b.block_id} 页码超出 page_count。")
        blocks.append(b)
    if len(blocks) != header.block_count:
        raise _bad(f"block_count={header.block_count} 与实际内容块数 {len(blocks)} 不符。")
    return Package(header, blocks)
