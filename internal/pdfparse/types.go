// Package pdfparse implements the PDF side of the parsing protocol defined in
// docs/网页与PDF解析-RAG入库对接协议.md.
package pdfparse

import (
	"encoding/json"
	"fmt"
	"io"
)

// SchemaVersion is the protocol version emitted in the document header.
const SchemaVersion = "1.0"

// Error codes defined by the protocol. Codes outside the protocol table are
// used only for parser-side task validation.
const (
	CodeInvalidRequest    = "INVALID_REQUEST"
	CodePDFEncrypted      = "PDF_ENCRYPTED"
	CodeUnsupportedFormat = "UNSUPPORTED_FORMAT"
	CodeNoUsableContent   = "NO_USABLE_CONTENT"
)

// Task is the parse job handed to the parser. Every field is required; the
// caller resolves title/version from the source registry via document_id.
type Task struct {
	ParseID     string
	SourceID    string
	DocumentID  string
	SnapshotRef string
	Title       string
	Version     string
}

// Warning is a non-fatal quality hint on the document header.
type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Page    *int   `json:"page,omitempty"`
}

// Header is the first NDJSON record of an artifact.
type Header struct {
	RecordType    string    `json:"record_type"`
	SchemaVersion string    `json:"schema_version"`
	SourceID      string    `json:"source_id"`
	DocumentID    string    `json:"document_id"`
	ParseID       string    `json:"parse_id"`
	SourceType    string    `json:"source_type"`
	Title         string    `json:"title"`
	Version       *string   `json:"version"`
	OriginalURL   *string   `json:"original_url"`
	SnapshotRef   string    `json:"snapshot_ref"`
	SourceSHA256  string    `json:"source_sha256"`
	ParsedAt      string    `json:"parsed_at"`
	Quality       string    `json:"quality"`
	BlockCount    int       `json:"block_count"`
	PageCount     *int      `json:"page_count"`
	Warnings      []Warning `json:"warnings"`
}

// Locator points a block back to its physical PDF page.
type Locator struct {
	Type             string `json:"type"`
	Page             int    `json:"page"`
	PrintedPageLabel string `json:"printed_page_label,omitempty"`
}

// Table holds the original column order for kind=table blocks.
type Table struct {
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// Block is one content block record.
type Block struct {
	RecordType       string   `json:"record_type"`
	BlockID          string   `json:"block_id"`
	Sequence         int      `json:"sequence"`
	Kind             string   `json:"kind"`
	Text             string   `json:"text"`
	SectionPath      []string `json:"section_path"`
	Locator          Locator  `json:"locator"`
	ExtractionMethod string   `json:"extraction_method"`
	Table            *Table   `json:"table,omitempty"`
}

// Artifact is a complete parse product ready to be written as NDJSON.
type Artifact struct {
	Header Header
	Blocks []Block
}

// ParseError carries the protocol error fields.
type ParseError struct {
	Code      string
	Message   string
	Retryable bool
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func newError(code, message string, retryable bool) *ParseError {
	return &ParseError{Code: code, Message: message, Retryable: retryable}
}

// WriteNDJSON writes the document header followed by every block, one JSON
// object per line.
func (a *Artifact) WriteNDJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(a.Header); err != nil {
		return err
	}
	for i := range a.Blocks {
		if err := encoder.Encode(a.Blocks[i]); err != nil {
			return err
		}
	}
	return nil
}

// Failure is the JSON object reported when a parse task fails.
type Failure struct {
	ParseID string      `json:"parse_id"`
	Status  string      `json:"status"`
	Error   FailureInfo `json:"error"`
}

// FailureInfo mirrors the protocol error object.
type FailureInfo struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// NewFailure builds a failure record from err.
func NewFailure(parseID string, err error) Failure {
	info := FailureInfo{Code: CodeNoUsableContent, Message: err.Error()}
	if pe, ok := err.(*ParseError); ok {
		info = FailureInfo{Code: pe.Code, Message: pe.Message, Retryable: pe.Retryable}
	}
	return Failure{ParseID: parseID, Status: "failed", Error: info}
}
