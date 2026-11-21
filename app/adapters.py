"""兼容适配器：将非标准展示样例（如 selected_web_extraction_for_display）转为协议标准 Package。"""
from __future__ import annotations

import hashlib
import json
from datetime import datetime, timezone
from typing import Any

from .errors import ARTIFACT_INVALID, ProtocolError
from .models import Block, DocumentHeader, Warning_

DISPLAY_SAMPLE_TYPE = "selected_web_extraction_for_display"

_FACT_LABELS = {
    "paper_title": "论文标题",
    "journal": "发表期刊",
    "page_date": "发布日期",
    "first_author": "第一作者",
    "corresponding_author": "通讯作者",
    "doi": "DOI",
    "research_summary": "研究摘要",
}


def is_display_sample(obj: Any) -> bool:
    return isinstance(obj, dict) and obj.get("sample_type") == DISPLAY_SAMPLE_TYPE


def _format_facts(facts: dict[str, Any]) -> str:
    lines: list[str] = []
    for k, label in _FACT_LABELS.items():
        v = facts.get(k)
        if v:
            lines.append(f"{label}：{v}")
    for k, v in facts.items():
        if k in _FACT_LABELS or k == "basis_block_ids" or not v:
            continue
        lines.append(f"{k}：{v}")
    return "\n".join(lines)


def parse_display_sample(
    raw: dict[str, Any],
    *,
    document_id: str | None = None,
    source_id: str | None = None,
    parse_id: str | None = None,
) -> tuple[DocumentHeader, list[Block]]:
    """把 selected_web_extraction_for_display JSON 转为标准 (DocumentHeader, list[Block])。"""
    test_meta = raw.get("test") if isinstance(raw.get("test"), dict) else {}
    raw_blocks = raw.get("selected_original_blocks") or []
    facts = raw.get("displayed_facts") if isinstance(raw.get("displayed_facts"), dict) else {}

    blocks: list[Block] = []
    seen: set[str] = set()
    first_section: list[str] = []

    for idx, item in enumerate(raw_blocks, start=1):
        if not isinstance(item, dict):
            raise ProtocolError(ARTIFACT_INVALID, f"selected_original_blocks[{idx - 1}] 必须是对象。")
        bid = str(item.get("block_id") or f"b{idx:06d}")
        if bid in seen:
            bid = f"{bid}_{idx}"
        seen.add(bid)
        sec = list(item.get("section_path") or [])
        if sec and not first_section:
            first_section = sec
        b = Block.model_validate({
            "record_type": "block",
            "block_id": bid,
            "sequence": idx,
            "kind": item.get("kind", "paragraph"),
            "text": item.get("text", ""),
            "section_path": sec,
            "locator": item.get("locator") or {"type": "web", "anchor": None},
            "extraction_method": item.get("extraction_method", "html"),
            "table": item.get("table"),
        })
        blocks.append(b)

    facts_text = _format_facts(facts) if facts else ""
    if facts_text:
        fid = "b_displayed_facts"
        while fid in seen:
            fid += "_1"
        blocks.append(Block.model_validate({
            "record_type": "block",
            "block_id": fid,
            "sequence": len(blocks) + 1,
            "kind": "paragraph",
            "text": facts_text,
            "section_path": first_section,
            "locator": {"type": "web", "anchor": None},
            "extraction_method": "html",
        }))

    if not blocks:
        raise ProtocolError(ARTIFACT_INVALID, "展示样例中没有可用内容块。")

    final_parse_id = parse_id or test_meta.get("parse_id") or "parse_display_sample"
    final_doc_id = document_id or f"doc_{final_parse_id}"
    final_src_id = source_id or f"src_{final_parse_id}"
    source_sha = test_meta.get("snapshot_sha256") or hashlib.sha256(
        json.dumps(raw, sort_keys=True, ensure_ascii=False).encode("utf-8")
    ).hexdigest()

    warnings = [Warning_.model_validate(w) for w in (test_meta.get("warnings") or [])]
    quality = test_meta.get("quality") if test_meta.get("quality") in ("complete", "partial") else "complete"

    header = DocumentHeader(
        record_type="document",
        schema_version="1.0",
        source_id=final_src_id,
        document_id=final_doc_id,
        parse_id=final_parse_id,
        source_type="web",
        title=raw.get("document_title") or final_doc_id,
        version=None,
        original_url=raw.get("source_url"),
        snapshot_ref=f"snapshot/web/{final_doc_id}.html",
        source_sha256=source_sha,
        parsed_at=datetime.now(timezone.utc).astimezone(),
        quality=quality,
        block_count=len(blocks),
        page_count=None,
        warnings=warnings,
    )
    return header, blocks


def to_ndjson_bytes(header: DocumentHeader, blocks: list[Block]) -> bytes:
    """导出为标准协议 UTF-8 NDJSON 字节流。"""
    lines = [header.model_dump_json()] + [b.model_dump_json() for b in blocks]
    return ("\n".join(lines) + "\n").encode("utf-8")
