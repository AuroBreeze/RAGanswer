package pdfparse

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// OCR modes accepted by ParseWithOCR.
const (
	OCRModeAuto  = "auto"
	OCRModeOff   = "off"
	OCRModeForce = "force"
)

const (
	defaultOCREndpoint = "https://api.deepseek.com/v1/tools/ocr"
	defaultOCRPurpose  = "text_recognition"
	defaultOCRTimeout  = 180 * time.Second
	ocrResponseLimit   = 8 << 20
)

// OCRClient calls a multipart OCR endpoint that returns per-page text, such as
// POST /v1/tools/ocr. The request carries exactly two fields: file and purpose.
type OCRClient struct {
	Endpoint string
	APIKey   string
	Purpose  string
	HTTP     *http.Client
}

// OCRClientFromEnv builds a client from OCR_* variables and falls back to
// DEEPSEEK_API_KEY. It returns nil when no credential is configured, which
// keeps OCR opt-in.
func OCRClientFromEnv() *OCRClient {
	key := strings.TrimSpace(os.Getenv("OCR_API_KEY"))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	}
	if key == "" {
		return nil
	}

	endpoint := strings.TrimSpace(os.Getenv("OCR_ENDPOINT"))
	if endpoint == "" {
		endpoint = defaultOCREndpoint
	}
	purpose := strings.TrimSpace(os.Getenv("OCR_PURPOSE"))
	if purpose == "" {
		purpose = defaultOCRPurpose
	}

	timeout := defaultOCRTimeout
	if raw := strings.TrimSpace(os.Getenv("OCR_TIMEOUT_SECONDS")); raw != "" {
		if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	return &OCRClient{
		Endpoint: endpoint,
		APIKey:   key,
		Purpose:  purpose,
		HTTP:     &http.Client{Timeout: timeout},
	}
}

// OCRPage is one recognized page. Index is zero-based, as returned by the API.
type OCRPage struct {
	Index        int    `json:"index"`
	Text         string `json:"text"`
	FinishReason string `json:"finish_reason"`
}

// OCRResult is the recognized document.
type OCRResult struct {
	PageCount int
	Pages     []OCRPage
}

type ocrResponse struct {
	PageCount int       `json:"page_count"`
	Pages     []OCRPage `json:"pages"`
	Error     *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// Recognize uploads path and returns the per-page recognition result.
func (c *OCRClient) Recognize(ctx context.Context, path string) (*OCRResult, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" {
		return nil, newError(CodeInvalidRequest, "未配置 OCR 服务凭据", false)
	}

	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)

	go func() {
		defer pipeWriter.Close()

		if err := writer.WriteField("purpose", c.Purpose); err != nil {
			pipeWriter.CloseWithError(err)
			return
		}
		part, err := writer.CreateFormFile("file", filepath.Base(path))
		if err != nil {
			pipeWriter.CloseWithError(err)
			return
		}
		file, err := os.Open(path)
		if err != nil {
			pipeWriter.CloseWithError(err)
			return
		}
		defer file.Close()
		if _, err := io.Copy(part, file); err != nil {
			pipeWriter.CloseWithError(err)
			return
		}
		if err := writer.Close(); err != nil {
			pipeWriter.CloseWithError(err)
		}
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, pipeReader)
	if err != nil {
		pipeReader.CloseWithError(err)
		return nil, newError(CodeNoUsableContent, "构建 OCR 请求失败："+err.Error(), false)
	}
	request.Header.Set("Authorization", "Bearer "+c.APIKey)
	request.Header.Set("Content-Type", writer.FormDataContentType())

	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: defaultOCRTimeout}
	}

	response, err := client.Do(request)
	if err != nil {
		pipeReader.CloseWithError(err)
		return nil, newError(CodeNoUsableContent, "调用 OCR 服务失败："+err.Error(), true)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, ocrResponseLimit))
	if err != nil {
		return nil, newError(CodeNoUsableContent, "读取 OCR 响应失败："+err.Error(), true)
	}

	var payload ocrResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		if response.StatusCode != http.StatusOK {
			return nil, ocrError(response.StatusCode, strings.TrimSpace(string(body)))
		}
		return nil, newError(CodeNoUsableContent, "OCR 响应不是有效 JSON", true)
	}

	if payload.Error != nil {
		return nil, ocrError(response.StatusCode, payload.Error.Message)
	}
	if response.StatusCode != http.StatusOK {
		return nil, ocrError(response.StatusCode, "")
	}
	if len(payload.Pages) == 0 {
		return nil, newError(CodeNoUsableContent, "OCR 未返回任何页面结果", false)
	}

	return &OCRResult{PageCount: payload.PageCount, Pages: payload.Pages}, nil
}

// ocrError maps OCR service errors onto protocol error codes.
func ocrError(status int, message string) *ParseError {
	message = strings.TrimSpace(message)
	lower := strings.ToLower(message)

	switch {
	case strings.Contains(lower, "password"):
		return newError(CodePDFEncrypted, "PDF 已加密，需提供密码后再解析", false)
	case status == http.StatusRequestEntityTooLarge,
		status == http.StatusBadRequest,
		strings.Contains(lower, "too large"),
		strings.Contains(lower, "more than 1000 pages"),
		strings.Contains(lower, "could not be parsed"),
		strings.Contains(lower, "format is not supported"),
		strings.Contains(lower, "could not be decoded"):
		return newError(CodeUnsupportedFormat, "PDF 无法通过 OCR 解析："+describeOCRStatus(status, message), false)
	}

	retryable := status == http.StatusRequestTimeout ||
		status == http.StatusTooManyRequests ||
		status >= 500 ||
		strings.Contains(lower, "temporarily unavailable") ||
		strings.Contains(lower, "did not finish")

	return newError(CodeNoUsableContent, "OCR 识别失败："+describeOCRStatus(status, message), retryable)
}

func describeOCRStatus(status int, message string) string {
	if message == "" {
		return fmt.Sprintf("HTTP %d", status)
	}
	if status == 0 || status == http.StatusOK {
		return message
	}
	return fmt.Sprintf("HTTP %d：%s", status, message)
}

var (
	markdownHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	markdownTableSeparator = regexp.MustCompile(`^\|(\s*:?-{2,}:?\s*\|)+$`)
	markdownListItem       = regexp.MustCompile(`^([-*+•]|\d{1,3}[.)])\s+(.+)$`)
	ocrMarkerPattern       = regexp.MustCompile(`<｜/?ref｜>|<｜box｜>\s*\[\[[^\]]*\]\]\s*<｜/box｜>`)
)

// stripOCRMarkers removes the ref/box annotations that rich_layout_recognition
// adds, so the default and rich purposes produce the same block text.
func stripOCRMarkers(line string) string {
	return strings.TrimSpace(ocrMarkerPattern.ReplaceAllString(line, ""))
}

// ocrBlocks converts recognition pages into protocol blocks in reading order.
func ocrBlocks(result *OCRResult) ([]Block, []Warning) {
	b := &builder{method: "ocr"}
	warnings := make([]Warning, 0)

	for _, page := range result.Pages {
		pageNo := page.Index + 1
		if page.FinishReason == "length" {
			warnings = append(warnings, pageWarning("PAGE_TRUNCATED", "OCR 输出达到上限，文本可能不完整", pageNo))
		}
		if strings.TrimSpace(page.Text) == "" {
			warnings = append(warnings, pageWarning("PAGE_NO_TEXT", "OCR 未识别到文本", pageNo))
			continue
		}
		b.page = pageNo
		b.assembleMarkdown(page.Text)
	}

	return b.blocks, warnings
}

// assembleMarkdown turns the OCR markdown output of one page into blocks.
// Consecutive plain lines belong to the same paragraph and are flushed on a
// blank line or when a structural line starts.
func (b *builder) assembleMarkdown(text string) {
	lines := strings.Split(text, "\n")
	pending := make([]string, 0, 4)

	flush := func() {
		if len(pending) == 0 {
			return
		}
		// Keep the original line breaks: OCR lines are visual lines, and
		// joining them with spaces can fuse unrelated fields.
		merged := strings.Join(pending, "\n")
		pending = pending[:0]
		b.emit("paragraph", merged)
	}

	for i := 0; i < len(lines); i++ {
		line := stripOCRMarkers(lines[i])
		if line == "" {
			flush()
			continue
		}

		if match := markdownHeadingPattern.FindStringSubmatch(line); match != nil {
			flush()
			level := len(match[1])
			if level > 3 {
				level = 3
			}
			title := normalizeSpace(match[2])
			b.pushHeading(level, title)
			b.emit("heading", title)
			continue
		}

		if rows, next, ok := markdownTable(lines, i); ok {
			flush()
			b.emitTable(rows)
			i = next - 1
			continue
		}

		if match := markdownListItem.FindStringSubmatch(line); match != nil {
			flush()
			b.emit("list", normalizeSpace(match[2]))
			continue
		}

		if captionMarker.MatchString(line) {
			flush()
			b.emit("caption", normalizeSpace(line))
			continue
		}

		pending = append(pending, normalizeSpace(line))
	}

	flush()
}

func markdownTable(lines []string, start int) ([][]string, int, bool) {
	if !strings.HasPrefix(strings.TrimSpace(lines[start]), "|") {
		return nil, 0, false
	}

	var rows [][]string
	next := start
	for next < len(lines) {
		line := strings.TrimSpace(lines[next])
		if !strings.HasPrefix(line, "|") {
			break
		}
		next++
		if markdownTableSeparator.MatchString(line) {
			continue
		}
		cells := splitTableRow(line)
		if len(cells) == 0 {
			break
		}
		if len(rows) > 0 && len(cells) != len(rows[0]) {
			break
		}
		rows = append(rows, cells)
	}

	if len(rows) < 2 {
		return nil, 0, false
	}
	return rows, next, true
}

func splitTableRow(line string) []string {
	parts := strings.Split(strings.Trim(line, "|"), "|")
	if len(parts) < 2 {
		return nil
	}
	cells := make([]string, 0, len(parts))
	for _, part := range parts {
		cells = append(cells, stripOCRMarkers(part))
	}
	return cells
}
