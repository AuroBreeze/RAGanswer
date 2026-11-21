# RAGanswer 网页解析

按 [网页与 PDF 解析和 RAG 入库对接协议](docs/网页与PDF解析-RAG入库对接协议.md) 1.0 生成网页 NDJSON。解析端接收来源登记方分配的 `source_id`、`document_id`、`parse_id` 和网页快照，输出 `parsed` 或带 `code`、`message`、`retryable` 的 `failed`。网页产物可交给 RAG 入库方。

## 运行

需要 Go 1.26 和一个现有的私有存储目录。存储目录应仅供授权服务访问，里面的原始 HTML 快照不适合直接对外提供预览。

```sh
go run ./cmd/webparse capture -store ./private-data -url https://example.com/manual -allow-hosts example.com
go run ./cmd/webparse parse -store ./private-data < request.json
```

`capture` 使用来源登记方批准的 URL 和主机列表，返回快照引用、最终 URL、原件 SHA-256 和响应字符编码。首版支持公开可访问的静态 HTML、HTTP/HTTPS 默认端口和精确主机名单。URL 不接受查询字符串或访问凭证。每次跳转重新检查主机，连接仅使用已校验的公网 IP。网页抓取时限为 20 秒，原件上限为 4 MiB，解析包上限为 16 MiB。

`request.json` 的 `task` 对应协议第 1.1 节，`document` 由来源登记信息和抓取结果组成。下面用占位值演示字段，实际调用填入 `capture` 返回的 `snapshot_ref`、`original_url`、`source_sha256` 和 `content_type`。

```json
{
  "task": {
    "parse_id": "parse_web_001",
    "source_id": "src_manual",
    "document_id": "doc_manual_r1",
    "source_type": "web",
    "snapshot_ref": "snapshot/web/manual-r1.html"
  },
  "document": {
    "title": "操作手册",
    "version": null,
    "original_url": "https://example.com/manual",
    "source_sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "content_type": "text/html; charset=utf-8"
  }
}
```

服务端也可直接调用 `webparse.Capture` 和 `webparse.ParseSnapshot`。每份解析包的首行是文档头，后续行按阅读顺序保存标题、段落、列表、表格和题注。解析器使用原有锚点，缺失时输出 `anchor: null` 并标记 `quality: partial`。`partial` 由协议规定的来源侧复核后交给 RAG。

成功产物写入 `parsed/<parse_id>/document.ndjson`。任务重投时复用已发布的产物和摘要。原件内容变更时登记新 `document_id`，调整解析配置时使用新 `parse_id`。分服务联调时，由调用方将 `document_id`、`parse_id`、`artifact_ref` 和 `artifact_sha256` 交给协议第 5 节的入库接口，并为入库服务配置同一受控存储。
