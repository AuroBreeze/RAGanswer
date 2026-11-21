"""把内容块切分为检索片段，并保留每段与原始块的对应关系（第 2.3 节）。

策略（最简）：同一章节内的相邻块合并到 chunk_max_chars；单个超长块按字符窗口切开并重叠。
每个片段记录 sources=[{block_id, locator, text}]，跨页片段自然携带多个来源。
"""
from __future__ import annotations

from dataclasses import dataclass, field

from .models import Block


@dataclass
class Chunk:
    text: str
    section_path: list[str]
    sources: list[dict] = field(default_factory=list)


def _split_long(text: str, size: int, overlap: int) -> list[str]:
    if len(text) <= size:
        return [text]
    step = max(1, size - overlap)
    return [text[i:i + size] for i in range(0, len(text), step) if text[i:i + size].strip()]


def chunk_blocks(blocks: list[Block], max_chars: int = 600, overlap: int = 80) -> list[Chunk]:
    chunks: list[Chunk] = []
    cur: Chunk | None = None

    def flush():
        nonlocal cur
        if cur and cur.sources:
            chunks.append(cur)
        cur = None

    for b in blocks:
        loc = b.locator.model_dump(exclude_none=True)
        pieces = _split_long(b.text, max_chars, overlap)
        for piece in pieces:
            src = {"block_id": b.block_id, "kind": b.kind, "locator": loc, "text": piece}
            if cur is None or cur.section_path != b.section_path or len(cur.text) + len(piece) + 1 > max_chars \
                    or len(pieces) > 1:
                flush()
                cur = Chunk(text=piece, section_path=list(b.section_path), sources=[src])
            else:
                cur.text += "\n" + piece
                cur.sources.append(src)
        if len(pieces) > 1:
            flush()
    flush()
    return chunks
