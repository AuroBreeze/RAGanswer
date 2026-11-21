"""检索与问答：按权限与有效版本检索，组装 citations（第 4、6 节）。"""
from __future__ import annotations

import array
import math
import re
from urllib.parse import quote

from . import llm, registry
from .config import get_settings
from .db import jload, query
from .models import ChatResponse, Citation

NOT_FOUND = "未在您有权限访问的资料中找到相关内容。"
SOURCE_LOOKUP = re.compile(
    r"(?:(?:我想(?:要)?|我希望|请问|请|帮我|麻烦|能否|可以|能不能)\s*)*"
    r"(?:(?:查看|查找|查询|搜索|搜一下|查一下|找一下|看一下|了解(?:一下)?|介绍(?:一下)?)\s*)?"
    r"(?P<subject>[^，,。！？?\n]{2,80}?)(?:的)?\s*(?:相关)?(?:资料|信息|简介|档案|报道|研究成果)"
)
HONORIFIC = re.compile(r"\s*(?:副教授|教授|老师|研究员|工程师|博士|先生|女士)$")


def _cos(a, b) -> float:
    dot = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a)) or 1.0
    nb = math.sqrt(sum(x * x for x in b)) or 1.0
    return dot / (na * nb)


def _accessible_chunks(user: registry.User):
    parses = registry.accessible_active_parses(user)  # 每次查询按当前权限与生效版本过滤
    if not parses:
        return []
    marks = ",".join("?" * len(parses))
    return query(
        f"SELECT c.*, d.title, d.version, d.source_type FROM chunks c JOIN documents d ON d.document_id=c.document_id "
        f"WHERE c.parse_id IN ({marks})", parses)


def search(user: registry.User, question: str, top_k: int):
    rows = _accessible_chunks(user)
    if not rows:
        return []
    qv = llm.embed([question])[0]
    scored = [(_cos(qv, array.array("f", r["embedding"])), r) for r in rows]
    scored.sort(key=lambda x: x[0], reverse=True)
    return scored[:top_k]


def _overlap(q: str, t: str) -> float:
    """问题与文本的字符二元组重合度，用于片段内各页单元排序。"""
    g = lambda s: {s[i:i + 2] for i in range(len(s) - 1)}
    qg = g(re.sub(r"\s+", "", q))
    return len(qg & g(re.sub(r"\s+", "", t))) / (len(qg) or 1)


def _units(rows, question: str = "") -> list[dict]:
    """把片段按原文页（网页按锚点）拆成引用单元：跨页片段按页分别引用。
    片段间保持检索得分顺序，片段内按与问题的相关度排序。"""
    units = []
    for _, r in rows:
        groups: dict = {}
        for src in jload(r["sources"], []):
            loc = src["locator"]
            key = loc.get("page") if loc["type"] == "pdf" else loc.get("anchor")
            groups.setdefault(key, []).append(src)
        part = [{"row": r, "srcs": srcs, "text": "\n".join(s["text"] for s in srcs)} for srcs in groups.values()]
        part.sort(key=lambda u: _overlap(question, u["text"]), reverse=True)
        units.extend(part)
    return units


def _citation(u: dict) -> Citation:
    r, first = u["row"], u["srcs"][0]["locator"]
    s = get_settings()
    path = jload(r["section_path"], [])
    url = None
    if s.preview_url_template:
        url = s.preview_url_template.format(document_id=quote(r["document_id"], safe=""))
    if first["type"] == "pdf":
        page = first["page"]
        if url:
            url = f"{url}#page={page}"
    else:
        page = None
        if url and first.get("anchor"):
            url = f"{url}#{quote(first['anchor'], safe='')}"
    excerpt = re.sub(r"\s+", " ", u["text"]).strip()
    return Citation(document_id=r["document_id"], title=r["title"], version=r["version"],
                    section=" > ".join(path) if path else None, page=page,
                    excerpt=excerpt[:200] + ("…" if len(excerpt) > 200 else ""), url=url)


def _lookup_sources(user: registry.User, question: str, limit: int) -> ChatResponse | None:
    """直接列出姓名或主题精确匹配的已收录资料。"""
    match = SOURCE_LOOKUP.fullmatch(question.strip(" \t。！？?!“”"))
    if not match or limit < 1:
        return None
    subject = HONORIFIC.sub("", match["subject"].strip(" \t《》“”\"'")).strip()
    if len(subject) < 2:
        return None

    term = subject.casefold()
    matches = []
    for row in _accessible_chunks(user):
        title_match = term in row["title"].casefold()
        text_match = term in row["text"].casefold()
        if title_match or text_match:
            matches.append(((title_match, text_match, len(row["text"])), row))
    matches.sort(key=lambda item: item[0], reverse=True)

    seen = set()
    citations = []
    for _, row in matches:
        document_id = row["document_id"]
        if document_id in seen:
            continue
        units = _units([(0, row)], subject)
        if units:
            citations.append(_citation(units[0]))
            seen.add(document_id)
            if len(citations) >= limit:
                break

    if not citations:
        return None
    titles = [f"- 《{citation.title}》[{i}]" for i, citation in enumerate(citations, 1)]
    return ChatResponse(answer=f"找到与「{subject}」相关的已收录资料：\n" + "\n".join(titles),
                        citations=citations)


def answer(user: registry.User, question: str, top_k: int | None = None) -> ChatResponse:
    limit = top_k or get_settings().top_k
    source_lookup = _lookup_sources(user, question, limit)
    if source_lookup is not None:
        return source_lookup
    hits = search(user, question, limit)
    if not hits:
        return ChatResponse(answer=NOT_FOUND, citations=[])
    units = _units(hits, question)
    text = llm.generate(question, [u["text"] for u in units])
    used = sorted({int(n) for n in re.findall(r"\[(\d+)\]", text) if 1 <= int(n) <= len(units)})
    if not used:  # 模型未标注时，引用最相关的一个单元
        used = [1]
    # 把正文中的编号重排为 citations 的下标（从 1 开始）
    remap = {old: new for new, old in enumerate(used, start=1)}
    text = re.sub(r"\[(\d+)\]", lambda m: f"[{remap[int(m.group(1))]}]" if int(m.group(1)) in remap else "", text)
    return ChatResponse(answer=text.strip(), citations=[_citation(units[i - 1]) for i in used])
