# 网页与 PDF 解析和 RAG 入库对接协议

| 项目 | 约定 |
| --- | --- |
| 版本 | 1.0 草案 |
| 协议对象 | 来源登记、解析产物、索引状态和问答引用 |
| 首版输入 | 单个已登记的 HTML 页面、上传的 PDF 文件 |
| 首版交付 | 网页正文与章节、PDF 文本与页码、失败状态、可核验出处 |
| 首版对接方式 | UTF-8 NDJSON 解析包。解析与 RAG 可在同一后端进程内交接 |
| 分服务对接 | 使用第 5 节的内部异步导入接口，复用同一种解析包 |

## 1. 分工与数据流

| 模块 | 负责内容 | 交付给下一环节 |
| --- | --- | --- |
| 来源登记 | 登记网页或 PDF、存储原件或快照、配置数据权限与生效状态 | `source_id`、不可变文档版本和存储引用 |
| 网页与 PDF 解析 | 提取可读正文、保留标题层级和阅读顺序、定位原文、报告解析质量 | 统一的文档头和内容块 |
| RAG 入库 | 校验解析包、切分检索片段、生成向量、建索引、登记每段的来源映射 | 入库状态及错误信息 |
| RAG 问答 | 按权限与有效版本检索，生成答案并组装出处 | 前端问答协议中的 `citations` |

**协作边界**。解析模块交付接近原文结构的内容块，例如一个段落、一张表或一个列表。RAG 模块决定检索片段的长度、重叠和向量化策略，并保留每段与原始内容块的对应关系。来源登记可由现有后端实现，双方复用同一份文档与权限记录。

```text
登记网页或 PDF → 保存原件或快照 → 解析文档 → 校验并交付解析包
                                                       ↓
打开授权原文 ← 回答携带出处 ← 权限过滤后检索 ← 切分并建立索引
```

### 1.1 提交给解析模块的任务

来源登记后，由服务端生成任务。下面的对象可作为本地函数参数或队列消息，字段均必填。网页抓取与 PDF 上传都先把原件写入受控存储，再发解析任务。

```json
{
  "parse_id": "parse_pdf_001",
  "source_id": "src_inspection_pdf",
  "document_id": "doc_inspection_v3",
  "source_type": "pdf",
  "snapshot_ref": "snapshot/pdf/inspection-v3.pdf"
}
```

解析模块通过 `document_id` 获取登记标题、原始 URL、版本及原件摘要。产物准备好时，返回或持久化 `parse_id`、`status: parsed`、`artifact_ref` 和 `artifact_sha256`；任务失败时返回 `status: failed` 与含 `code`、`message`、`retryable` 的 `error` 对象。重投同一任务可按 `parse_id` 去重；需要改变解析配置时分配新 `parse_id`。

## 2. 统一解析包

### 2.1 编码与记录顺序

- 每个包对应一份文档的一个解析产物。文件以 UTF-8 编码，每行一个 JSON 对象，第一行为文档头，其余为内容块，按原文阅读顺序写出。
- `schema_version` 首版固定为 `"1.0"`。读取方可忽略新增的可选字段。改变既有字段含义、类型或定位方式时，须升主版本并提前兼容。
- 产物写完并校验后再发布。RAG 可以逐行解析，但须确认记录数量、格式、来源和文件摘要均正确后才能发布索引。空文档以解析失败回报。
- 配置联调环境时约定单个原件和解析包的大小上限、抓取时限、OCR 时限。超限应返回明确状态。

### 2.2 文档头

| 字段 | 类型 | 规则 |
| --- | --- | --- |
| `record_type` | string | 固定为 `document` |
| `schema_version` | string | 固定为 `1.0` |
| `source_id` | string | 同一来源的稳定 ID。网页更新、手册换版时沿用 |
| `document_id` | string | 原件或快照的不可变版本 ID。原件内容改变时生成新 ID |
| `parse_id` | string | 本次发布的解析产物 ID。原件相同但重跑 OCR 或更换规则时可生成新 ID |
| `source_type` | enum | `web` 或 `pdf` |
| `title` | string | 文档标题。优先采用来源登记标题 |
| `version` | string 或 null | 原文中的版本，如 `V3`。网页无公开版本时传 null |
| `original_url` | string 或 null | 网页保存抓取后的最终 URL。上传 PDF 可为 null |
| `snapshot_ref` | string | 受控存储引用。网页对应保存的快照，PDF 对应原件 |
| `source_sha256` | string | 原件或快照文件字节的 SHA-256，小写十六进制 |
| `parsed_at` | string | 带时区的解析完成时间 |
| `quality` | enum | `complete` 或 `partial`。含义见第 4 节 |
| `block_count` | integer | 后续内容块行数，必须大于零 |
| `page_count` | integer 或 null | PDF 原件总页数，网页传 null |
| `warnings` | array | 不影响产物发布的质量提示，元素含 `code`、`message`，可附 `page`。无警告时传空数组 |

`source_id` 与 `document_id` 由来源登记或导入层分配，解析器回传原值。原件发生变更时，由来源管理方登记新版本。网页每次抓取形成独立快照时，若内容未变化，可复用已有版本。`source_sha256` 校验该版本绑定的字节，网页可另行使用正文指纹辅助识别导航栏和时间戳等非正文变化。旧版本继续保留其原件和可用定位。

### 2.3 内容块

| 字段 | 类型 | 规则 |
| --- | --- | --- |
| `record_type` | string | 固定为 `block` |
| `block_id` | string | 在同一 `parse_id` 内唯一，例如 `b000001` |
| `sequence` | integer | 从 1 开始，连续递增，等于正文阅读顺序 |
| `kind` | enum | `heading`、`paragraph`、`list`、`table` 或 `caption` |
| `text` | string | UTF-8 纯文本，非空。保留对检索有意义的换行、单位、步骤编号与表头 |
| `section_path` | string[] | 所属章节路径，例如 `["3 维护","3.2 点检记录"]`。无法确认时传 `[]` |
| `locator` | object | 与来源类型匹配的原文定位，见第 3 节 |
| `extraction_method` | enum | 网页用 `html`，PDF 按块用 `pdf_text` 或 `ocr` |
| `table` | object | 可选，仅用于表格。可包含 `headers` 和 `rows`，均保留原始列顺序。表格仍须提供可检索的 `text` |

解析器可清理重复空格、导航栏和页眉页脚，同时保留条款编号、表格列含义、数值和单位。表格跨页时按页输出，后续页保留可确认的列标题。RAG 在切分和存储索引时保留 `document_id`、`parse_id`、`block_id` 与定位。跨页片段可携带多个来源，回答时按支持结论的原文页分别引用。

## 3. 网页与 PDF 的定位

### 3.1 网页

- 首版解析已登记 URL 的正文，保留标题、段落、列表、表格和原文标题锚点。页面级筛选与站点爬取范围由来源登记配置。
- `locator` 固定含 `type: "web"` 和 `anchor`。若原始标题有稳定的 HTML ID，回传原值；解析服务也可在保存的预览快照中为块写入稳定锚点，并返回该值。无法定位时传 null。
- 原网页保留在 `original_url`。内部网页预览通过项目的授权文件接口返回快照，浏览器可用锚点定位。首版若未实现安全的快照预览，可只在出处卡片展示标题、章节与摘录。
- 页面需登录、正文依赖动态渲染、抓取到的是登录页或未能提取正文时，区分具体失败原因。首版可先约定静态 HTML 可用，需要动态渲染的页面作为扩展。

**网页产物示例**。内容与标识符为演示数据。

```ndjson
{"record_type":"document","schema_version":"1.0","source_id":"src_training_web","document_id":"doc_training_web_r1","parse_id":"parse_web_001","source_type":"web","title":"新员工培训须知","version":null,"original_url":"https://example.com/training","snapshot_ref":"snapshot/web/training-r1.html","source_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","parsed_at":"2026-11-21T09:30:00+08:00","quality":"complete","block_count":1,"page_count":null,"warnings":[]}
{"record_type":"block","block_id":"b000001","sequence":1,"kind":"paragraph","text":"新员工培训签到表由培训室前台发放。","section_path":["培训签到"],"locator":{"type":"web","anchor":"training-sign-in"},"extraction_method":"html"}
```

### 3.2 PDF

- 优先按页抽取文本。扫描页或文本层不可用的页面走 OCR。混合 PDF 可同时包含 `pdf_text` 和 `ocr` 内容块。
- `locator` 固定含 `type: "pdf"` 和 `page`。`page` 为从 1 开始的原件物理页序。目录罗马数字、封面页与印刷页码不影响此值。可选的 `printed_page_label` 仅用于辅助展示。
- 每个块只对应一页。可选的 `bbox` 表示该块在页面上的位置，格式为 `[left, top, right, bottom]`，坐标均归一化至 0–1，以浏览器所见的旋正页面左上角为原点。无法精确定位时可省略。
- OCR 不完整时在文档级警告中记录受影响页。看不清的数字、单位或条款不要补写。加密 PDF 或 OCR 后仍无可用内容时返回明确错误。

**PDF 产物示例**。物理第 4 页的印刷页码为 2。

```ndjson
{"record_type":"document","schema_version":"1.0","source_id":"src_inspection_pdf","document_id":"doc_inspection_v3","parse_id":"parse_pdf_001","source_type":"pdf","title":"设备点检记录管理须知","version":"V3","original_url":null,"snapshot_ref":"snapshot/pdf/inspection-v3.pdf","source_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","parsed_at":"2026-11-21T09:35:00+08:00","quality":"complete","block_count":1,"page_count":8,"warnings":[]}
{"record_type":"block","block_id":"b000001","sequence":1,"kind":"paragraph","text":"点检记录表可在车间资料柜领取。","section_path":["3 记录管理","3.2 表单领取"],"locator":{"type":"pdf","page":4,"printed_page_label":"2"},"extraction_method":"pdf_text"}
```

## 4. 状态、权限和版本发布

| 环节 | 状态 | 判定条件 |
| --- | --- | --- |
| 解析 | `queued → parsing → parsed` 或 `failed` | `parsed` 表示产物已写完、可读取。局部失败时可交付 `quality: partial` |
| 入库 | `queued → indexing → ready` 或 `failed` | `ready` 表示索引已完整发布，可用于检索 |

- `quality: complete` 指处理了全部目标范围且完成可用文本抽取。内含 OCR 时仍可能是 `complete`，OCR 的可疑区域可附警告。`partial` 指存在确定缺页、正文未完整提取或定位缺失。首版对 `partial` 的默认策略为待人工复核，复核通过才可提交入库。审批标记保存在服务端来源记录中。
- 同一来源的新版本成功入库且来源记录标记为生效后，再切换检索使用的版本。发布时核对来源记录指定的当前目标产物，避免较旧任务晚完成后覆盖新索引。新版本解析或建索引失败时，原有仍生效的索引可继续使用。文档显式废止或权限撤销时，检索立即按当前规则排除。
- 解析包只提供内容和出处。厂区、车间、角色、权限版本及生效状态由受控来源记录提供。RAG 在建立索引时载入授权元数据，在检索与打开引用时再按当前权限限制。
- 网页抓取仅通过获准的站点与访问配置执行，后端校验重定向后的目标仍符合网络访问策略。抓取服务使用受控凭证，解析包、日志和模型输入中不要包含 Cookie、访问令牌或页面脚本。网页预览以经过清理的静态内容交付。
- PDF 和网页按输入设定大小与处理时限。页面文字和 PDF 文本均作为外部数据处理，页面内的指令不会改变解析任务、权限范围或 RAG 的系统行为。

## 5. 分服务时的内部接口

首版同进程部署可直接交接解析包和 SHA-256。采用独立服务部署时，解析方写入受控对象存储，再调用以下导入接口。接口使用服务身份鉴权。接收方以 `artifact_ref` 从允许的存储读取文件。

### 5.1 提交索引任务 `POST /internal/v1/ingestions`

```json
{
  "parse_id": "parse_pdf_001",
  "document_id": "doc_inspection_v3",
  "artifact_ref": "parsed/parse_pdf_001.ndjson",
  "artifact_sha256": "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
}
```

- 返回 `202` 和 `{"job_id":"ing_001","parse_id":"parse_pdf_001","status":"queued"}`，表示已受理。
- 相同 `parse_id` 和产物摘要重试时返回既有任务及当前状态。相同 `parse_id` 携带不同文档或摘要返回 `409 PARSE_CONFLICT`，防止同一解析产物 ID 对应不同内容。
- 请求字段错误返回 `400 INVALID_REQUEST`。未授权返回 `401` 或 `403`；未知文档返回 `404 DOCUMENT_NOT_FOUND`。读取存储后发现摘要、文档 ID 或版本不匹配时，导入任务进入 `failed`。

### 5.2 查询索引任务 `GET /internal/v1/ingestions/{job_id}`

```json
{
  "job_id": "ing_001",
  "document_id": "doc_inspection_v3",
  "parse_id": "parse_pdf_001",
  "status": "ready",
  "indexed_chunk_count": 1,
  "updated_at": "2026-11-21T09:36:00+08:00",
  "error": null
}
```

任务状态查询返回 `200`。处理中 `indexed_chunk_count` 为 null，完成后为实际发布的片段数。失败时 `error` 为 `{"code":"INDEX_UNAVAILABLE","message":"索引服务暂时不可用。","retryable":true}`。临时故障由入库服务对同一任务限次重试，重试写入须去重。修正源数据或解析规则后，使用新的 `parse_id` 提交。

| 阶段 | 示例错误码 | 处理方式 |
| --- | --- | --- |
| 获取网页 | `FETCH_TIMEOUT`、`WEB_ACCESS_DENIED`、`WEB_RENDER_REQUIRED` | 超时可限次重试。权限或渲染问题按配置处理 |
| 解析 PDF 或网页 | `PDF_ENCRYPTED`、`UNSUPPORTED_FORMAT`、`NO_USABLE_CONTENT` | 修复来源、调整解析配置或更换文件后重新解析 |
| 导入与索引 | `ARTIFACT_INVALID`、`SOURCE_NOT_APPROVED`、`INDEX_UNAVAILABLE` | 校验或审批问题回到来源侧；索引依赖故障由入库侧重试 |

## 6. 映射到现有问答接口

| 前端引用字段 | RAG 的来源 |
| --- | --- |
| `document_id`、`title`、`version` | 文档头中的同名字段 |
| `section` | 把 `section_path` 连成用户可读章节路径。空路径映射为 null |
| `page` | PDF 的 `locator.page`。网页传 null |
| `excerpt` | 能支撑结论的原文块中的摘录 |
| `url` | 按 `document_id` 生成受控预览路径。网页支持快照预览时可附锚点。无预览时传 null |

现有问答响应可直接展示网页出处和 PDF 出处。文件预览路由按账号重新检查权限。网页快照的内部存储地址和解析包地址保留在服务端。

## 7. 首次联调验收

| 样例 | 解析侧交付 | RAG 侧验收 |
| --- | --- | --- |
| 普通网页 | 正文、章节、表格与有条件可用的锚点 | 用一句自然语言命中正文，答案带网页出处，`page: null` |
| 有封面的 PDF | 正文、章节、物理页序；有印刷页码时保留标签 | 答案引用跳到 PDF 查看器的同一页，摘录能在该页核对 |
| 混合扫描 PDF | 文本页和 OCR 页的块，失败页附警告 | OCR 页能按物理页定位；明确缺页的产物按复核规则处理 |
| 同一文件重复提交 | 相同 `parse_id`、文件摘要和存储引用 | 返回原任务，索引不重复 |
| 更新或撤销文档权限 | 相同来源下登记新版本，或更新授权记录 | 生效后新问题使用新版本；权限撤销后无权账号无法命中内容或打开原文 |

首轮联调准备一个网页和一个 PDF。解析侧交付原件或快照及对应的 NDJSON，RAG 侧回传入库状态、命中内容块和最终引用。验收以同一条问题能追溯到原文的对应章节或页码为准。
