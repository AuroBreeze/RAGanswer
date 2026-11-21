# RAG 入库与问答（最简实现）

实现《网页与 PDF 解析和 RAG 入库对接协议 v1.0》中 **RAG 侧** 的职责：校验解析包 → 切分 → 向量化 → 建索引 → 按权限与有效版本检索 → 生成带 `citations` 的回答。
网页抓取和 PDF 解析由 [Go 解析模块](parsers.md) 提供。来源登记与授权由调用方同步。

## 运行

```powershell
python -m venv .venv
.\.venv\Scripts\pip install -r requirements.txt
copy .env.example .env   # 填写 OpenAI 兼容接口的地址与 Key
.\.venv\Scripts\uvicorn app.main:app --port 8080
.\.venv\Scripts\python -m pytest -q tests   # 离线测试（hash 向量 + 摘录式回答）
```

## 对接方式

| 接口 | 调用方 | 说明 |
| --- | --- | --- |
| `POST /internal/v1/documents` | 来源登记 | 同步文档版本与授权：`document_id, source_id, source_type, title, version, source_sha256, effective, factories, workshops, roles` |
| `PATCH /internal/v1/sources/{source_id}` | 来源登记 | 修改授权或 `status: revoked`，检索立即生效 |
| `POST /internal/v1/documents/{document_id}/revoke` | 来源登记 | 废止某版本 |
| `POST /internal/v1/parses/{parse_id}/approve` | 来源登记 | `quality: partial` 复核通过，自动重跑入库 |
| `POST /internal/v1/ingestions` | 解析 | 协议 5.1，返回 202 |
| `GET /internal/v1/ingestions/{job_id}` | 解析 | 协议 5.2 |
| `POST /api/v1/chat` | 后端/前端 | `{"question"}` → `{"answer","citations"}`（第 6 节） |

- 内部接口使用 `Authorization: Bearer <RAG_SERVICE_TOKENS>`。
- `/api/v1/chat` 的用户身份由后端鉴权后通过请求头传入：`X-User-Id`、`X-User-Roles`（逗号分隔）、`X-User-Factory`、`X-User-Workshop`。授权列表为空表示该维度不限制。
- 没有受控预览路由时，保留 `RAG_PREVIEW_URL_TEMPLATE` 为空，答案仍会返回引用标题、章节、页码和摘录。
- 同进程部署时可直接调用 `app.ingest.submit(IngestionRequest(...))`，无需走 HTTP。
- 顺序要求：**先登记文档，再提交入库**（否则 404 `DOCUMENT_NOT_FOUND`）。

本地管理页 `/admin` 使用服务令牌连接同一个知识库，显示真实登记与索引状态。可上传最大 10 MiB 的 NDJSON 解析包。上传后，服务端校验并保存产物，再登记文档并提交入库。部分解析结果须核对原文后点击「复核通过」。停用资料会将其从问答中排除。

## 实现要点

- **校验**（`app/package.py`）：SHA-256、UTF-8、文档头/内容块字段、`sequence` 连续、`block_id` 唯一、`block_count`、定位类型与 `extraction_method` 匹配来源类型、PDF 页码不超过 `page_count`；并与登记记录核对 `source_id / source_type / source_sha256 / version`。不通过 → `ARTIFACT_INVALID`。
- **幂等**：相同 `parse_id` + 摘要返回原任务；文档或摘要不同 → `409 PARSE_CONFLICT`。片段写入先删后插，重试不重复。
- **切分**（`app/chunker.py`）：同章节相邻块合并到 `RAG_CHUNK_MAX_CHARS`，超长块滑窗切分；每个片段保存 `[{block_id, locator, text}]`。
- **发布切换**（`app/registry.py: try_publish`）：只有已生效、未废止、且不旧于当前检索版本的产物才会切换，晚到的旧任务不会覆盖新索引；新版本失败时旧索引继续可用。
- **引用**（`app/retrieval.py`）：跨页片段按页拆成独立引用单元；PDF 的 `url` 带 `#page=N`，网页带 `#anchor`。
- **向量存储**：最简版直接存 SQLite 并暴力计算余弦相似度，适合几千个片段以内。数据量变大时把 `ingest.run` 的写入和 `retrieval.search` 换成 Chroma/Milvus 即可，其他代码不用动。
