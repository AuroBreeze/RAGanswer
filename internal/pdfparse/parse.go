package pdfparse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

// Parse extracts one PDF using only its text layer. The snapshot is read from
// path; title and version fall back to PDF metadata and file name.
func Parse(path string, task Task) (*Artifact, error) {
	return parse(path, task, OCRModeOff, nil)
}

// ParseWithOCR parses a PDF snapshot with optional OCR support. In auto mode
// the OCR service is called only when the text layer yields no blocks; force
// always uses OCR; off disables it.
func ParseWithOCR(path string, task Task, client *OCRClient, mode string) (*Artifact, error) {
	switch mode {
	case "", OCRModeAuto:
		mode = OCRModeAuto
	case OCRModeOff, OCRModeForce:
	default:
		return nil, newError(CodeInvalidRequest, "未知的 OCR 模式："+mode, false)
	}
	if mode == OCRModeForce && client == nil {
		return nil, newError(CodeInvalidRequest, "OCR 模式为 force，但未配置 OCR 服务凭据", false)
	}
	return parse(path, task, mode, client)
}

func parse(path string, task Task, mode string, client *OCRClient) (*Artifact, error) {
	if err := normalizeTask(&task, path); err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("无法读取 PDF：%v", err), false)
	}
	defer file.Close()

	signature := make([]byte, 5)
	if _, err := io.ReadFull(file, signature); err != nil || string(signature) != "%PDF-" {
		return nil, newError(CodeUnsupportedFormat, "文件不是有效的 PDF", false)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("无法定位 PDF 内容：%v", err), false)
	}

	sourceSHA, err := hashReader(file)
	if err != nil {
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("计算文件摘要失败：%v", err), false)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("无法定位 PDF 内容：%v", err), false)
	}

	info, err := file.Stat()
	if err != nil {
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("读取文件信息失败：%v", err), false)
	}

	reader, err := pdf.NewReader(file, info.Size())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "encrypt") {
			return nil, newError(CodePDFEncrypted, "PDF 已加密，需提供密码后再解析", false)
		}
		return nil, newError(CodeUnsupportedFormat, fmt.Sprintf("PDF 结构无法解析：%v", err), false)
	}

	pageCount := reader.NumPage()
	if pageCount <= 0 {
		return nil, newError(CodeNoUsableContent, "PDF 没有可解析的页面", false)
	}

	var blocks []Block
	warnings := make([]Warning, 0)

	if mode != OCRModeForce {
		blocks, warnings = textBlocks(reader)
	}

	if len(blocks) == 0 && mode != OCRModeOff && client != nil {
		result, ocrErr := client.Recognize(context.Background(), path)
		if ocrErr != nil {
			return nil, ocrErr
		}
		blocks, warnings = ocrBlocks(result)
		if result.PageCount > 0 {
			pageCount = result.PageCount
		}
	}

	if len(blocks) == 0 {
		return nil, newError(CodeNoUsableContent, "未提取到可用正文，请检查是否为扫描件或加密 PDF", false)
	}

	quality := "complete"
	if len(warnings) > 0 {
		quality = "partial"
	}

	header := Header{
		RecordType:    "document",
		SchemaVersion: SchemaVersion,
		SourceID:      task.SourceID,
		DocumentID:    task.DocumentID,
		ParseID:       task.ParseID,
		SourceType:    "pdf",
		Title:         documentTitle(task, reader, path),
		Version:       optionalString(task.Version),
		OriginalURL:   nil,
		SnapshotRef:   task.SnapshotRef,
		SourceSHA256:  sourceSHA,
		ParsedAt:      time.Now().Format(time.RFC3339),
		Quality:       quality,
		BlockCount:    len(blocks),
		PageCount:     &pageCount,
		Warnings:      warnings,
	}

	return &Artifact{Header: header, Blocks: blocks}, nil
}

// textBlocks extracts the embedded text layer page by page.
func textBlocks(reader *pdf.Reader) ([]Block, []Warning) {
	b := &builder{method: "pdf_text"}
	warnings := make([]Warning, 0)

	for page := 1; page <= reader.NumPage(); page++ {
		texts, err := pageText(reader.Page(page))
		if err != nil {
			warnings = append(warnings, pageWarning("PAGE_PARSE_FAILED", "文本层读取失败，可能需要 OCR", page))
			continue
		}
		lines := buildLines(texts)
		if len(lines) == 0 {
			warnings = append(warnings, pageWarning("PAGE_NO_TEXT", "未提取到文本层，可能需要 OCR", page))
			continue
		}

		b.page = page
		b.bodySize = bodyFontSize(lines)
		b.assemble(lines)
	}

	return b.blocks, warnings
}

// pageText extracts positioned glyphs and converts interpreter panics on
// malformed content streams into an error.
func pageText(page pdf.Page) (texts []pdf.Text, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("解析页面文本失败：%v", r)
		}
	}()
	return page.Content().Text, nil
}

func normalizeTask(task *Task, path string) error {
	var missing []string
	if strings.TrimSpace(task.ParseID) == "" {
		missing = append(missing, "parse_id")
	}
	if strings.TrimSpace(task.SourceID) == "" {
		missing = append(missing, "source_id")
	}
	if strings.TrimSpace(task.DocumentID) == "" {
		missing = append(missing, "document_id")
	}
	if len(missing) > 0 {
		return newError(CodeInvalidRequest, "缺少必填字段："+strings.Join(missing, "、"), false)
	}
	if strings.TrimSpace(task.SnapshotRef) == "" {
		task.SnapshotRef = path
	}
	return nil
}

func documentTitle(task Task, reader *pdf.Reader, path string) string {
	if title := strings.TrimSpace(task.Title); title != "" {
		return title
	}
	if info := reader.Trailer().Key("Info"); !info.IsNull() {
		if title := strings.TrimSpace(info.Key("Title").Text()); title != "" {
			return title
		}
	}
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func optionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	v := strings.TrimSpace(value)
	return &v
}

func pageWarning(code, message string, page int) Warning {
	p := page
	return Warning{Code: code, Message: fmt.Sprintf("第 %d 页%s。", page, message), Page: &p}
}

func hashReader(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
