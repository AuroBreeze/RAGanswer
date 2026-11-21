package webparse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
)

const MaxArtifactBytes = 16 << 20

var (
	ErrLoginRequired    = errors.New("web page requires login")
	ErrRenderRequired   = errors.New("web page requires JavaScript rendering")
	ErrNoContent        = errors.New("web page has no usable text")
	ErrArtifactTooLarge = errors.New("web parse artifact exceeds size limit")
)

type Document struct {
	RecordType    string    `json:"record_type"`
	SchemaVersion string    `json:"schema_version"`
	SourceID      string    `json:"source_id"`
	DocumentID    string    `json:"document_id"`
	ParseID       string    `json:"parse_id"`
	SourceType    string    `json:"source_type"`
	Title         string    `json:"title"`
	Version       *string   `json:"version"`
	OriginalURL   string    `json:"original_url"`
	SnapshotRef   string    `json:"snapshot_ref"`
	SourceSHA256  string    `json:"source_sha256"`
	ParsedAt      time.Time `json:"parsed_at"`
	Quality       string    `json:"quality"`
	BlockCount    int       `json:"block_count"`
	PageCount     *int      `json:"page_count"`
	Warnings      []Warning `json:"warnings"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

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

type Locator struct {
	Type   string  `json:"type"`
	Anchor *string `json:"anchor"`
}

type Table struct {
	Headers []string   `json:"headers,omitempty"`
	Rows    [][]string `json:"rows"`
}

// encode parses a decoded UTF-8 snapshot and serializes one NDJSON artifact.
func encode(r io.Reader, doc Document) ([]byte, error) {
	root, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	main := firstElement(root, "main", "")
	if main == nil {
		main = firstElement(root, "", "main")
	}
	if main == nil {
		main = firstElement(root, "article", "")
	}
	if main == nil {
		main = firstElement(root, "body", "")
	}
	if hasPasswordField(main) {
		return nil, ErrLoginRequired
	}
	var state extraction
	state.walk(main)
	if !state.hasBody {
		if firstElement(root, "script", "") != nil || firstElement(root, "noscript", "") != nil {
			return nil, ErrRenderRequired
		}
		return nil, ErrNoContent
	}
	if doc.Title == "" {
		doc.Title = textContent(firstElement(root, "title", ""))
	}
	if doc.Title == "" {
		for _, block := range state.blocks {
			if block.Kind == "heading" {
				doc.Title = block.Text
				break
			}
		}
	}
	if doc.Title == "" {
		doc.Title = doc.OriginalURL
	}
	doc.RecordType, doc.SchemaVersion, doc.SourceType = "document", "1.0", "web"
	doc.ParsedAt = time.Now().UTC()
	doc.BlockCount = len(state.blocks)
	doc.Quality = "complete"
	doc.Warnings = []Warning{}
	for _, block := range state.blocks {
		if block.Locator.Anchor == nil {
			doc.Quality = "partial"
			doc.Warnings = append(doc.Warnings, Warning{"WEB_ANCHOR_MISSING", "部分内容块缺少可用的原文锚点。"})
			break
		}
	}
	var artifact bytes.Buffer
	enc := json.NewEncoder(&artifact)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	for _, block := range state.blocks {
		if err := enc.Encode(block); err != nil {
			return nil, err
		}
		if artifact.Len() > MaxArtifactBytes {
			return nil, ErrArtifactTooLarge
		}
	}
	return artifact.Bytes(), nil
}

type section struct {
	level  int
	title  string
	anchor string
}

type extraction struct {
	sections []section
	blocks   []Block
	hasBody  bool
}

func (e *extraction) walk(node *html.Node) {
	if node == nil || skip(node) {
		return
	}
	if node.Type == html.ElementNode {
		if len(node.Data) == 2 && node.Data[0] == 'h' && node.Data[1] >= '1' && node.Data[1] <= '6' {
			if text := textContent(node); text != "" {
				level := int(node.Data[1] - '0')
				for len(e.sections) > 0 && e.sections[len(e.sections)-1].level >= level {
					e.sections = e.sections[:len(e.sections)-1]
				}
				e.sections = append(e.sections, section{level, text, attr(node, "id")})
				e.add(node, "heading", text, nil)
			}
			return
		}
		switch node.Data {
		case "p", "pre", "dt", "dd":
			e.add(node, "paragraph", textContent(node), nil)
			return
		case "figcaption", "caption":
			e.add(node, "caption", textContent(node), nil)
			return
		case "ul", "ol":
			e.add(node, "list", strings.Join(listLines(node, 0), "\n"), nil)
			return
		case "table":
			e.readTable(node)
			return
		}
	}

	var inline strings.Builder
	flush := func() {
		e.add(node, "paragraph", cleanText(inline.String()), nil)
		inline.Reset()
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if skip(child) {
			continue
		}
		if child.Type == html.TextNode {
			inline.WriteString(child.Data)
			continue
		}
		if child.Type != html.ElementNode {
			continue
		}
		switch child.Data {
		case "a", "abbr", "b", "br", "code", "em", "i", "label", "small", "span", "strong", "sub", "sup", "time":
			appendText(&inline, child, false)
		default:
			flush()
			e.walk(child)
		}
	}
	flush()
}

func (e *extraction) add(node *html.Node, kind, text string, table *Table) {
	if text == "" {
		return
	}
	path := make([]string, 0, len(e.sections))
	for _, section := range e.sections {
		path = append(path, section.title)
	}
	anchor := attr(node, "id")
	for i := len(e.sections) - 1; anchor == "" && i >= 0; i-- {
		anchor = e.sections[i].anchor
	}
	locator := Locator{Type: "web"}
	if anchor != "" {
		locator.Anchor = &anchor
	}
	sequence := len(e.blocks) + 1
	e.blocks = append(e.blocks, Block{
		RecordType: "block", BlockID: fmt.Sprintf("b%06d", sequence), Sequence: sequence,
		Kind: kind, Text: text, SectionPath: path, Locator: locator, ExtractionMethod: "html", Table: table,
	})
	e.hasBody = e.hasBody || kind != "heading"
}

func (e *extraction) readTable(node *html.Node) {
	table := &Table{Rows: [][]string{}}
	var lines []string
	var visit func(*html.Node)
	visit = func(row *html.Node) {
		if skip(row) {
			return
		}
		switch row.Data {
		case "caption":
			e.add(row, "caption", textContent(row), nil)
		case "tr":
			var cells []string
			allHeaders := true
			for cell := row.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type == html.ElementNode && !skip(cell) && (cell.Data == "th" || cell.Data == "td") {
					cells = append(cells, textContent(cell))
					allHeaders = allHeaders && cell.Data == "th"
				}
			}
			if len(cells) == 0 || strings.TrimSpace(strings.Join(cells, "")) == "" {
				return
			}
			if allHeaders && len(lines) == 0 {
				table.Headers = cells
			} else {
				table.Rows = append(table.Rows, cells)
			}
			lines = append(lines, strings.Join(cells, " | "))
		case "thead", "tbody", "tfoot":
			for child := row.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		visit(child)
	}
	e.add(node, "table", strings.Join(lines, "\n"), table)
}

func listLines(node *html.Node, depth int) []string {
	var lines []string
	index := 1
	if value, err := fmt.Sscan(attr(node, "start"), &index); err != nil || value != 1 {
		index = 1
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.Data != "li" || skip(child) {
			continue
		}
		var b strings.Builder
		appendText(&b, child, true)
		if text := cleanText(b.String()); text != "" {
			prefix := "- "
			if node.Data == "ol" {
				prefix = fmt.Sprintf("%d. ", index)
			}
			lines = append(lines, strings.Repeat("  ", depth)+prefix+text)
		}
		for nested := child.FirstChild; nested != nil; nested = nested.NextSibling {
			if nested.Type == html.ElementNode && (nested.Data == "ol" || nested.Data == "ul") && !skip(nested) {
				lines = append(lines, listLines(nested, depth+1)...)
			}
		}
		index++
	}
	return lines
}

func attr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

func firstElement(node *html.Node, tag, role string) *html.Node {
	if node == nil {
		return nil
	}
	if node.Type == html.ElementNode && (tag != "" && node.Data == tag || role != "" && attr(node, "role") == role) {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if result := firstElement(child, tag, role); result != nil {
			return result
		}
	}
	return nil
}

func hasPasswordField(node *html.Node) bool {
	if node == nil || skip(node) {
		return false
	}
	if node.Type == html.ElementNode && node.Data == "input" && strings.EqualFold(attr(node, "type"), "password") {
		return true
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if hasPasswordField(child) {
			return true
		}
	}
	return false
}

func skip(node *html.Node) bool {
	if node.Type != html.ElementNode {
		return false
	}
	switch node.Data {
	case "script", "style", "noscript", "template", "nav", "aside", "footer", "svg", "iframe", "select", "button", "textarea":
		return true
	}
	if attr(node, "role") == "navigation" || strings.EqualFold(attr(node, "aria-hidden"), "true") {
		return true
	}
	for _, attr := range node.Attr {
		if attr.Key == "hidden" {
			return true
		}
		if attr.Key == "style" {
			for _, rule := range strings.Split(attr.Val, ";") {
				name, value, _ := strings.Cut(rule, ":")
				value, _, _ = strings.Cut(value, "!")
				name, value = strings.ToLower(strings.TrimSpace(name)), strings.ToLower(strings.TrimSpace(value))
				if name == "display" && value == "none" || name == "visibility" && value == "hidden" {
					return true
				}
			}
		}
	}
	return false
}

func textContent(node *html.Node) string {
	var b strings.Builder
	appendText(&b, node, false)
	return cleanText(b.String())
}

func cleanText(text string) string {
	return strings.TrimFunc(strings.ReplaceAll(text, "\r", ""), func(r rune) bool {
		return r == '\uFEFF' || unicode.IsSpace(r)
	})
}

func appendText(b *strings.Builder, node *html.Node, omitLists bool) {
	if node == nil || skip(node) {
		return
	}
	if node.Type == html.TextNode {
		b.WriteString(node.Data)
		return
	}
	if node.Type != html.ElementNode || omitLists && (node.Data == "ul" || node.Data == "ol") {
		return
	}
	if node.Data == "br" || node.Data == "p" && b.Len() > 0 {
		b.WriteByte('\n')
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		appendText(b, child, omitLists)
	}
}
