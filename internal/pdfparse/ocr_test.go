package pdfparse

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func ocrFixture(text string) *OCRResult {
	return &OCRResult{
		PageCount: 1,
		Pages: []OCRPage{{
			Index:        0,
			Text:         text,
			FinishReason: "stop",
		}},
	}
}

func TestOCRBlocksFromMarkdown(t *testing.T) {
	blocks, warnings := ocrBlocks(ocrFixture(strings.Join([]string{
		"# 测试文档",
		"",
		"## 技能",
		"",
		"* C",
		"* Rust",
		"",
		"| 项目 | 角色 |",
		"| --- | --- |",
		"| 示例项目 | 独立开发 |",
		"",
		"正文段落。",
	}, "\n")))

	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}

	want := []struct {
		kind string
		text string
	}{
		{"heading", "测试文档"},
		{"heading", "技能"},
		{"list", "C"},
		{"list", "Rust"},
		{"table", "项目 | 角色\n示例项目 | 独立开发"},
		{"paragraph", "正文段落。"},
	}
	if len(blocks) != len(want) {
		t.Fatalf("blocks = %d, want %d: %+v", len(blocks), len(want), blocks)
	}
	for i, expected := range want {
		block := blocks[i]
		if block.Kind != expected.kind || block.Text != expected.text {
			t.Fatalf("block %d = %s/%q, want %s/%q", i, block.Kind, block.Text, expected.kind, expected.text)
		}
		if block.RecordType != "block" || block.Sequence != i+1 || block.BlockID != blockID(i+1) {
			t.Fatalf("block %d identity = %+v", i, block)
		}
		if block.ExtractionMethod != "ocr" {
			t.Fatalf("block %d extraction_method = %q", i, block.ExtractionMethod)
		}
		if block.Locator.Type != "pdf" || block.Locator.Page != 1 {
			t.Fatalf("block %d locator = %+v", i, block.Locator)
		}
		if block.SectionPath == nil {
			t.Fatalf("block %d section_path must be []", i)
		}
	}

	if got := strings.Join(blocks[3].SectionPath, "/"); got != "测试文档/技能" {
		t.Fatalf("list section_path = %q", got)
	}
	table := blocks[4].Table
	if table == nil || len(table.Headers) != 2 || len(table.Rows) != 1 || table.Rows[0][0] != "示例项目" {
		t.Fatalf("table = %+v", table)
	}
}

func TestOCRKeepsWrappedLinesInOneParagraph(t *testing.T) {
	blocks, warnings := ocrBlocks(ocrFixture("2024-09 ~ 2028-06\n示例大学\n软件工程 · 本科 (2028届)\n\n第二段"))

	if len(warnings) != 0 {
		t.Fatalf("warnings = %+v", warnings)
	}
	if len(blocks) != 2 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if want := "2024-09 ~ 2028-06\n示例大学\n软件工程 · 本科 (2028届)"; blocks[0].Text != want {
		t.Fatalf("paragraph = %q, want %q", blocks[0].Text, want)
	}
	if blocks[1].Text != "第二段" {
		t.Fatalf("second paragraph = %q", blocks[1].Text)
	}
}

func TestOCRStripsRichLayoutMarkers(t *testing.T) {
	text := "# <｜ref｜>标题<｜/ref｜><｜box｜>[[52, 40, 948, 96]]<｜/box｜>\n" +
		"<｜ref｜>正文内容<｜/ref｜><｜box｜>[[52, 120, 948, 160]]<｜/box｜>"

	blocks, _ := ocrBlocks(ocrFixture(text))
	if len(blocks) != 2 {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[0].Kind != "heading" || blocks[0].Text != "标题" {
		t.Fatalf("heading = %+v", blocks[0])
	}
	if blocks[1].Text != "正文内容" {
		t.Fatalf("paragraph = %q", blocks[1].Text)
	}
}

func TestOCRWarnsOnTruncatedPage(t *testing.T) {
	result := ocrFixture("部分内容")
	result.Pages[0].FinishReason = "length"

	blocks, warnings := ocrBlocks(result)
	if len(blocks) != 1 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	if len(warnings) != 1 || warnings[0].Code != "PAGE_TRUNCATED" {
		t.Fatalf("warnings = %+v", warnings)
	}
	if warnings[0].Page == nil || *warnings[0].Page != 1 {
		t.Fatalf("warning page = %v", warnings[0].Page)
	}
}

type ocrRequest struct {
	Purpose  string
	FileName string
	FileSize int64
}

func newOCRServer(t *testing.T, status int, body string) (*httptest.Server, *ocrRequest) {
	t.Helper()
	captured := &ocrRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			t.Errorf("parse multipart: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		captured.Purpose = r.FormValue("purpose")
		if file, header, err := r.FormFile("file"); err == nil {
			defer file.Close()
			captured.FileName = header.Filename
			if size, err := io.Copy(io.Discard, file); err == nil {
				captured.FileSize = size
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)

	return server, captured
}

func testOCRClient(server *httptest.Server) *OCRClient {
	return &OCRClient{
		Endpoint: server.URL,
		APIKey:   "test-key",
		Purpose:  "text_recognition",
		HTTP:     server.Client(),
	}
}

func TestParseWithOCRFallsBackWhenTextLayerMissing(t *testing.T) {
	// A page without any text operators has no text layer, like an outlined or
	// scanned PDF.
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{}}))

	body := `{"id":"ocr-1","object":"ocr.result","created":1,"purpose":"text_recognition","page_count":1,"pages":[{"index":0,"text":"# 简历\n\n正文段落。","finish_reason":"stop"}],"usage":{}}`
	server, captured := newOCRServer(t, http.StatusOK, body)

	artifact, err := ParseWithOCR(path, sampleTask(), testOCRClient(server), OCRModeAuto)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if captured.Purpose != "text_recognition" {
		t.Fatalf("purpose = %q", captured.Purpose)
	}
	if captured.FileName != "snapshot.pdf" || captured.FileSize == 0 {
		t.Fatalf("uploaded file = %q (%d bytes)", captured.FileName, captured.FileSize)
	}

	if artifact.Header.BlockCount != 2 || artifact.Header.Quality != "complete" {
		t.Fatalf("header = %+v", artifact.Header)
	}
	if artifact.Header.PageCount == nil || *artifact.Header.PageCount != 1 {
		t.Fatalf("page_count = %v", artifact.Header.PageCount)
	}
	if artifact.Blocks[0].Kind != "heading" || artifact.Blocks[0].ExtractionMethod != "ocr" {
		t.Fatalf("first block = %+v", artifact.Blocks[0])
	}
	if got := strings.Join(artifact.Blocks[1].SectionPath, "/"); got != "简历" {
		t.Fatalf("section_path = %q", got)
	}
}

func TestParseWithOCRForceSkipsTextLayer(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{
		{size: 12, text: "Embedded text.", y: 700},
	}}))

	body := `{"id":"ocr-1","object":"ocr.result","created":1,"purpose":"text_recognition","page_count":1,"pages":[{"index":0,"text":"OCR text.","finish_reason":"stop"}],"usage":{}}`
	server, _ := newOCRServer(t, http.StatusOK, body)

	artifact, err := ParseWithOCR(path, sampleTask(), testOCRClient(server), OCRModeForce)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(artifact.Blocks) != 1 || artifact.Blocks[0].Text != "OCR text." {
		t.Fatalf("blocks = %+v", artifact.Blocks)
	}
	if artifact.Blocks[0].ExtractionMethod != "ocr" {
		t.Fatalf("extraction_method = %q", artifact.Blocks[0].ExtractionMethod)
	}
}

func TestParseWithOCRForceRequiresClient(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{}}))

	_, err := ParseWithOCR(path, sampleTask(), nil, OCRModeForce)
	assertCode(t, err, CodeInvalidRequest)
}

func TestParseWithOCRRejectsUnknownMode(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{}}))

	_, err := ParseWithOCR(path, sampleTask(), nil, "sometimes")
	assertCode(t, err, CodeInvalidRequest)
}

func TestParseWithOCRMapsServiceErrors(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{}}))

	cases := []struct {
		name      string
		status    int
		body      string
		code      string
		retryable bool
	}{
		{
			name:   "encrypted",
			status: http.StatusOK,
			body:   `{"error":{"message":"The file is password-protected and cannot be processed.","type":"invalid_request_error","code":"invalid_request_error"}}`,
			code:   CodePDFEncrypted,
		},
		{
			name:   "unsupported",
			status: http.StatusBadRequest,
			body:   `{"error":{"message":"Failed to recognize the file: the file format is not supported","type":"invalid_request_error"}}`,
			code:   CodeUnsupportedFormat,
		},
		{
			name:      "busy",
			status:    http.StatusServiceUnavailable,
			body:      `{"error":{"message":"The OCR service is temporarily unavailable. Please try again later.","code":"service_unavailable"}}`,
			code:      CodeNoUsableContent,
			retryable: true,
		},
		{
			name:   "unauthorized",
			status: http.StatusUnauthorized,
			body:   `{"error":{"message":"Invalid API key"}}`,
			code:   CodeNoUsableContent,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server, _ := newOCRServer(t, tc.status, tc.body)

			_, err := ParseWithOCR(path, sampleTask(), testOCRClient(server), OCRModeAuto)
			assertCode(t, err, tc.code)

			parseErr, ok := err.(*ParseError)
			if !ok {
				t.Fatalf("unexpected error type: %T", err)
			}
			if parseErr.Retryable != tc.retryable {
				t.Fatalf("retryable = %v, want %v (%s)", parseErr.Retryable, tc.retryable, parseErr.Message)
			}
		})
	}
}
