# RAGanswer

面向工厂制度和设备知识的问答应用。从网页或 PDF 提取文本和原文定位，建立带权限的知识索引，再通过网页界面提问并查看来源。

| 层 | 目录 | 职责 |
| --- | --- | --- |
| 解析 | `cmd/webparse/`、`cmd/pdfparse/`、`webparse/`、`internal/pdfparse/` | 读取受控快照，输出带原文定位和摘要校验的 NDJSON 1.0。 |
| RAG 服务 | `app/`、`tests/` | 核对来源登记和摘要，切分并索引文档，按权限检索并返回答案及引用。 |
| 界面 | `ui/` | 提问、展示答案和引用。 |
| 协议 | `docs/` | 定义解析产物、入库任务和问答响应。 |

- [网页和 PDF 解析运行说明](docs/parsers.md)
- [RAG 服务安装、配置与接口](docs/rag-service.md)
- [解析与入库协议](docs/网页与PDF解析-RAG入库对接协议.md)
- [前后端问答接口协议](docs/工厂知识问答机器人-前后端接口协议.md)

## 本地联调

准备 Go 1.26、Python 3.10+ 和符合前端依赖要求的 Node.js（已验证 Node.js 22.19）。以下命令在 Windows PowerShell 中执行。

1. 从仓库根目录安装 Python 依赖并启动 RAG 服务。离线模式使用哈希向量和摘录式回答。

   ```powershell
   python -m venv .venv
   .\.venv\Scripts\python -m pip install -r requirements.txt
   New-Item -ItemType Directory -Force data/storage | Out-Null
   $env:RAG_EMBEDDING_PROVIDER = 'hash'
   $env:RAG_LLM_PROVIDER = 'extractive'
   $env:RAG_SERVICE_TOKENS = [Guid]::NewGuid().ToString('N')
   .\.venv\Scripts\python -m uvicorn app.main:app --host 127.0.0.1 --port 8080
   ```

2. 解析网页或 PDF，将 NDJSON 放在 `data/storage` 下。来源登记和入库调用使用解析包头中的 ID、原件摘要及相对于 `data/storage` 的 `artifact_ref`。按 [RAG 服务的接口顺序](docs/rag-service.md) 登记、提交和查询状态。`quality: partial` 需要先经来源侧复核。
3. 在另一个 PowerShell 中启动界面。`RAG_DEV_USER_ID` 仅用于本地演示，开发代理只转发问答路由并移除浏览器提交的用户身份头。

   ```powershell
   cd ui
   npm ci
   $env:RAG_DEV_USER_ID = 'local-demo'
   npm run dev
   ```

浏览器访问终端输出的本地地址。文本提问适用于所有受支持浏览器，语音输入取决于浏览器的语音识别实现和麦克风权限。部署时，由同源受信网关完成用户登录并为 `/api/v1/chat` 填入真实身份头，RAG 服务应在网关后提供服务。是否显示原文链接取决于网关是否提供授权预览。
