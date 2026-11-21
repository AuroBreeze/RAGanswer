package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalPDF builds a one-page, one-line PDF used to exercise the CLI.
func minimalPDF(text string) []byte {
	content := fmt.Sprintf("BT\n/F1 12 Tf\n1 0 0 1 72 700 Tm\n(%s) Tj\nET", text)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}

	var buf strings.Builder
	buf.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for i, body := range objects {
		offsets[i+1] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i+1, body)
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return []byte(buf.String())
}

func TestRunPDFCommandWritesNDJSON(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "manual.pdf")
	if err := os.WriteFile(input, minimalPDF("Protocol line"), 0o644); err != nil {
		t.Fatalf("write pdf: %v", err)
	}
	output := filepath.Join(dir, "artifact.ndjson")

	code := runPDFCommand([]string{
		"--in", input,
		"--out", output,
		"--parse-id", "parse_1",
		"--source-id", "src_1",
		"--document-id", "doc_1",
	})
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}

	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) < 2 {
		t.Fatalf("artifact lines = %d", len(lines))
	}

	var header map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if header["record_type"] != "document" || header["parse_id"] != "parse_1" {
		t.Fatalf("unexpected header: %v", header)
	}

	var block map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &block); err != nil {
		t.Fatalf("decode block: %v", err)
	}
	if block["record_type"] != "block" || block["block_id"] != "b000001" {
		t.Fatalf("unexpected block: %v", block)
	}
}

func TestRunPDFCommandRejectsMissingInput(t *testing.T) {
	if code := runPDFCommand(nil); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}
