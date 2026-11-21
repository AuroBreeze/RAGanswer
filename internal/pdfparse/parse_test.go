package pdfparse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempPDF(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "snapshot.pdf")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	return path
}

func sampleTask() Task {
	return Task{
		ParseID:     "parse_pdf_001",
		SourceID:    "src_inspection_pdf",
		DocumentID:  "doc_inspection_v3",
		SnapshotRef: "snapshot/pdf/inspection-v3.pdf",
		Title:       "设备点检记录管理须知",
		Version:     "V3",
	}
}

func TestParseProducesProtocolArtifact(t *testing.T) {
	pdfBytes := buildPDF([][]fixtureLine{
		{
			{size: 20, text: "Maintenance", y: 730},
			{size: 12, text: "Check the inspection record before each shift.", y: 700},
			{size: 12, text: "Report issues to the shift lead.", y: 684},
			{size: 16, text: "3.2 Inspection record", y: 650},
			{size: 12, text: "The form is kept in the workshop cabinet.", y: 620},
		},
		{
			{size: 12, text: "Replace the filter every 500 hours.", y: 730},
		},
	})
	path := writeTempPDF(t, pdfBytes)

	artifact, err := Parse(path, sampleTask())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	header := artifact.Header
	if header.RecordType != "document" || header.SchemaVersion != "1.0" {
		t.Fatalf("unexpected header: %+v", header)
	}
	if header.SourceID != "src_inspection_pdf" || header.DocumentID != "doc_inspection_v3" || header.ParseID != "parse_pdf_001" {
		t.Fatalf("identifier mismatch: %+v", header)
	}
	if header.SourceType != "pdf" || header.SnapshotRef != "snapshot/pdf/inspection-v3.pdf" {
		t.Fatalf("source fields mismatch: %+v", header)
	}
	if header.Title != "设备点检记录管理须知" || header.Version == nil || *header.Version != "V3" {
		t.Fatalf("title/version mismatch: %+v", header)
	}
	if header.OriginalURL != nil {
		t.Fatal("pdf original_url must be null")
	}
	if header.PageCount == nil || *header.PageCount != 2 {
		t.Fatalf("page_count = %v", header.PageCount)
	}
	if header.BlockCount != len(artifact.Blocks) || header.BlockCount == 0 {
		t.Fatalf("block_count = %d, blocks = %d", header.BlockCount, len(artifact.Blocks))
	}
	if header.Quality != "complete" || len(header.Warnings) != 0 {
		t.Fatalf("quality = %s warnings = %+v", header.Quality, header.Warnings)
	}
	if len(header.SourceSHA256) != 64 {
		t.Fatalf("source_sha256 = %q", header.SourceSHA256)
	}

	if artifact.Blocks[0].Kind != "heading" || artifact.Blocks[0].Text != "Maintenance" {
		t.Fatalf("first block = %+v", artifact.Blocks[0])
	}
	if got := strings.Join(artifact.Blocks[0].SectionPath, "/"); got != "Maintenance" {
		t.Fatalf("root section_path = %q", got)
	}
	if artifact.Blocks[1].Kind != "paragraph" || !strings.Contains(artifact.Blocks[1].Text, "inspection record") {
		t.Fatalf("paragraph block = %+v", artifact.Blocks[1])
	}

	last := artifact.Blocks[len(artifact.Blocks)-1]
	if last.Locator.Type != "pdf" || last.Locator.Page != 2 {
		t.Fatalf("last block locator = %+v", last.Locator)
	}
	if got := strings.Join(last.SectionPath, "/"); got != "Maintenance/3.2 Inspection record" {
		t.Fatalf("last section_path = %q", got)
	}

	for i, block := range artifact.Blocks {
		if block.RecordType != "block" {
			t.Fatalf("block %d record_type = %q", i, block.RecordType)
		}
		if block.Sequence != i+1 || block.BlockID != blockID(i+1) {
			t.Fatalf("block %d identity = %s/%d", i, block.BlockID, block.Sequence)
		}
		if block.ExtractionMethod != "pdf_text" {
			t.Fatalf("block %d extraction_method = %q", i, block.ExtractionMethod)
		}
		if block.SectionPath == nil {
			t.Fatalf("block %d section_path must be []", i)
		}
	}
}

func TestWriteNDJSON(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{
		{size: 12, text: "Only one paragraph.", y: 700},
	}}))

	artifact, err := Parse(path, sampleTask())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var buf bytes.Buffer
	if err := artifact.WriteNDJSON(&buf); err != nil {
		t.Fatalf("write ndjson: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != artifact.Header.BlockCount+1 {
		t.Fatalf("ndjson lines = %d, blocks = %d", len(lines), artifact.Header.BlockCount)
	}

	var header Header
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header.RecordType != "document" {
		t.Fatalf("first line record_type = %q", header.RecordType)
	}

	var block Block
	if err := json.Unmarshal([]byte(lines[1]), &block); err != nil {
		t.Fatalf("decode block: %v", err)
	}
	if block.RecordType != "block" || block.Sequence != 1 {
		t.Fatalf("unexpected block: %+v", block)
	}
}

func TestParsePartialWhenPageHasNoText(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{
		{{size: 12, text: "Page with text.", y: 700}},
		{},
	}))

	artifact, err := Parse(path, sampleTask())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if artifact.Header.Quality != "partial" {
		t.Fatalf("quality = %q", artifact.Header.Quality)
	}
	if len(artifact.Header.Warnings) != 1 {
		t.Fatalf("warnings = %+v", artifact.Header.Warnings)
	}
	warning := artifact.Header.Warnings[0]
	if warning.Page == nil || *warning.Page != 2 {
		t.Fatalf("warning page = %v", warning.Page)
	}
}

func TestParseNoUsableContent(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{}}))

	_, err := Parse(path, sampleTask())
	assertCode(t, err, CodeNoUsableContent)
}

func TestParseUnsupportedFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a.pdf")
	if err := os.WriteFile(path, []byte("plain text"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, err := Parse(path, sampleTask())
	assertCode(t, err, CodeUnsupportedFormat)
}

func TestParseRequiresIdentifiers(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{
		{size: 12, text: "Text.", y: 700},
	}}))

	task := sampleTask()
	task.ParseID = ""

	_, err := Parse(path, task)
	assertCode(t, err, CodeInvalidRequest)
}

func TestParseFallsBackToSnapshotRefAndTitle(t *testing.T) {
	path := writeTempPDF(t, buildPDF([][]fixtureLine{{
		{size: 12, text: "Text.", y: 700},
	}}))

	task := sampleTask()
	task.SnapshotRef = ""
	task.Title = ""

	artifact, err := Parse(path, task)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if artifact.Header.SnapshotRef != path {
		t.Fatalf("snapshot_ref = %q", artifact.Header.SnapshotRef)
	}
	if artifact.Header.Title != "snapshot" {
		t.Fatalf("title = %q", artifact.Header.Title)
	}
	if artifact.Header.Version == nil || *artifact.Header.Version != "V3" {
		t.Fatalf("version = %v", artifact.Header.Version)
	}
}

func assertCode(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %s", code)
	}
	parseErr, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("unexpected error type: %T %v", err, err)
	}
	if parseErr.Code != code {
		t.Fatalf("code = %s, want %s (%s)", parseErr.Code, code, parseErr.Message)
	}
}

func blockID(seq int) string {
	return fmt.Sprintf("b%06d", seq)
}
