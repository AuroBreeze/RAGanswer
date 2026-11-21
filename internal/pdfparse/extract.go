package pdfparse

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/ledongthuc/pdf"
)

// line is one horizontal run of text extracted from a page.
type line struct {
	Text     string
	X        float64
	Y        float64
	W        float64
	FontSize float64
}

var (
	numberedHeading = regexp.MustCompile(`^(\d+(?:\.\d+){0,3})[\s、.]+`)
	listMarker      = regexp.MustCompile(`^([-–—•·*]|\d{1,3}[.)、]|[a-zA-Z][.)])\s+`)
	captionMarker   = regexp.MustCompile(`^(图|表|Figure|Fig\.|Table)\s*[\d一二三四五六七八九十]+`)
	columnSplit     = regexp.MustCompile(`\s{2,}|\t+`)
)

const (
	headingSizeRatio  = 1.18
	paragraphGapRatio = 1.9
)

type heading struct {
	level int
	text  string
}

type builder struct {
	blocks   []Block
	stack    []heading
	page     int
	bodySize float64
	method   string
}

// lineTolerance is how far apart two glyph baselines may be while still
// belonging to the same visual line, in points.
const lineTolerance = 2.0

// buildLines groups per-glyph text runs into visual lines, top to bottom, and
// keeps the largest font size of each line so headings can be detected.
// GetTextByRow does not report font metrics in this library, so the caller
// passes Page.Content().Text, which does.
func buildLines(texts []pdf.Text) []line {
	type group struct {
		y     float64
		items []pdf.Text
	}

	groups := make([]*group, 0, len(texts))
	for _, text := range texts {
		if isIgnorableGlyph(text.S) {
			continue
		}
		var current *group
		for _, candidate := range groups {
			if math.Abs(candidate.y-text.Y) <= lineTolerance {
				current = candidate
				break
			}
		}
		if current == nil {
			current = &group{y: text.Y}
			groups = append(groups, current)
		}
		current.items = append(current.items, text)
	}

	sort.SliceStable(groups, func(i, j int) bool { return groups[i].y > groups[j].y })

	lines := make([]line, 0, len(groups))
	for _, g := range groups {
		items := g.items
		sort.SliceStable(items, func(i, j int) bool { return items[i].X < items[j].X })

		var sb strings.Builder
		prevEnd := 0.0
		maxSize := 0.0
		minX := math.MaxFloat64
		maxX := 0.0
		lastIsSpace := true

		for _, item := range items {
			if item.S == "\n" || item.S == "\r" || item.S == "\t" {
				continue
			}
			if item.S == " " {
				if !lastIsSpace {
					sb.WriteString(" ")
					lastIsSpace = true
				}
				prevEnd = item.X + item.W
				continue
			}
			if !lastIsSpace && prevEnd > 0 && item.X-prevEnd > 1.2 {
				sb.WriteString(" ")
			}
			sb.WriteString(item.S)
			lastIsSpace = false
			prevEnd = item.X + item.W
			if item.FontSize > maxSize {
				maxSize = item.FontSize
			}
			if item.X < minX {
				minX = item.X
			}
			if item.X+item.W > maxX {
				maxX = item.X + item.W
			}
		}

		text := strings.TrimSpace(sb.String())
		if text == "" {
			continue
		}
		if minX == math.MaxFloat64 {
			minX = 0
		}
		lines = append(lines, line{Text: text, X: minX, Y: g.y, W: maxX - minX, FontSize: maxSize})
	}
	return lines
}

// isIgnorableGlyph reports glyphs that carry no horizontal position, such as
// the synthetic newline the library appends after TJ arrays. Ordinary spaces
// are kept because they preserve word and column boundaries.
func isIgnorableGlyph(text string) bool {
	switch text {
	case "", "\n", "\r", "\t":
		return true
	}
	return false
}

// bodyFontSize estimates the dominant font size of a page, weighted by the
// amount of text drawn at each size.
func bodyFontSize(lines []line) float64 {
	weight := make(map[float64]int)
	for _, l := range lines {
		size := roundSize(l.FontSize)
		weight[size] += len([]rune(l.Text))
	}

	best, bestWeight := 0.0, -1
	for size, w := range weight {
		if w > bestWeight || (w == bestWeight && size > best) {
			best, bestWeight = size, w
		}
	}
	if best <= 0 {
		return 12
	}
	return best
}

func roundSize(size float64) float64 {
	if size <= 0 {
		return 0
	}
	return math.Round(size*10) / 10
}

// assemble converts the lines of one page into ordered, typed blocks.
func (b *builder) assemble(lines []line) {
	var paragraph string

	flush := func() {
		if paragraph == "" {
			return
		}
		b.emit("paragraph", paragraph)
		paragraph = ""
	}

	for i := 0; i < len(lines); i++ {
		current := lines[i]

		if rows, next, ok := tableRun(lines, i); ok {
			flush()
			b.emitTable(rows)
			i = next - 1
			continue
		}

		if level, ok := headingLevel(current, b.bodySize); ok {
			flush()
			text := normalizeSpace(current.Text)
			b.pushHeading(level, text)
			b.emit("heading", text)
			continue
		}

		if listMarker.MatchString(current.Text) {
			flush()
			b.emit("list", normalizeSpace(current.Text))
			continue
		}

		if captionMarker.MatchString(current.Text) {
			flush()
			b.emit("caption", normalizeSpace(current.Text))
			continue
		}

		if paragraph != "" && paragraphBreak(lines[i-1], current, b.bodySize) {
			flush()
		}
		paragraph = appendText(paragraph, normalizeSpace(current.Text))
	}
	flush()
}

// appendText joins wrapped lines, omitting the space between CJK characters.
func appendText(current, next string) string {
	if current == "" {
		return next
	}
	last := []rune(current)
	first := []rune(next)
	if len(last) == 0 || len(first) == 0 {
		return current + next
	}
	if isCJK(last[len(last)-1]) || isCJK(first[0]) {
		return current + next
	}
	return current + " " + next
}

func (b *builder) pushHeading(level int, text string) {
	for len(b.stack) > 0 && b.stack[len(b.stack)-1].level >= level {
		b.stack = b.stack[:len(b.stack)-1]
	}
	b.stack = append(b.stack, heading{level: level, text: text})
}

func (b *builder) sectionPath() []string {
	path := make([]string, len(b.stack))
	for i, h := range b.stack {
		path[i] = h.text
	}
	return path
}

func (b *builder) emit(kind, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	seq := len(b.blocks) + 1
	b.blocks = append(b.blocks, Block{
		RecordType:       "block",
		BlockID:          fmt.Sprintf("b%06d", seq),
		Sequence:         seq,
		Kind:             kind,
		Text:             text,
		SectionPath:      b.sectionPath(),
		Locator:          Locator{Type: "pdf", Page: b.page},
		ExtractionMethod: b.extractionMethod(),
	})
}

func (b *builder) extractionMethod() string {
	if b.method == "" {
		return "pdf_text"
	}
	return b.method
}

func (b *builder) emitTable(rows [][]string) {
	if len(rows) < 2 || len(rows[0]) < 2 {
		return
	}
	textRows := make([]string, 0, len(rows))
	for _, row := range rows {
		textRows = append(textRows, strings.Join(row, " | "))
	}
	seq := len(b.blocks) + 1
	b.blocks = append(b.blocks, Block{
		RecordType:       "block",
		BlockID:          fmt.Sprintf("b%06d", seq),
		Sequence:         seq,
		Kind:             "table",
		Text:             strings.Join(textRows, "\n"),
		SectionPath:      b.sectionPath(),
		Locator:          Locator{Type: "pdf", Page: b.page},
		ExtractionMethod: b.extractionMethod(),
		Table:            &Table{Headers: rows[0], Rows: rows[1:]},
	})
}

func headingLevel(l line, body float64) (int, bool) {
	text := strings.TrimSpace(l.Text)
	runes := []rune(text)
	if len(runes) == 0 || len(runes) > 80 {
		return 0, false
	}
	if listMarker.MatchString(text) || captionMarker.MatchString(text) {
		return 0, false
	}

	// Numbered sections carry their outline level in the text itself, so they
	// only need to be at least body size to qualify as headings.
	if match := numberedHeading.FindStringSubmatch(text); match != nil &&
		len(runes) <= 40 &&
		!strings.HasSuffix(text, "。") && !strings.HasSuffix(text, ".") &&
		l.FontSize >= body-0.1 {
		level := strings.Count(match[1], ".") + 1
		if level > 3 {
			level = 3
		}
		return level, true
	}

	if body > 0 && l.FontSize >= body*headingSizeRatio {
		switch ratio := l.FontSize / body; {
		case ratio >= 1.6:
			return 1, true
		case ratio >= 1.35:
			return 2, true
		default:
			return 3, true
		}
	}

	return 0, false
}

func paragraphBreak(prev, current line, body float64) bool {
	gap := prev.Y - current.Y
	size := math.Max(prev.FontSize, current.FontSize)
	if size <= 0 {
		size = body
	}
	return gap > paragraphGapRatio*size
}

// tableRun detects three or more consecutive lines that share the same column
// layout. The threshold keeps ordinary prose containing double spaces out.
func tableRun(lines []line, start int) ([][]string, int, bool) {
	first := splitColumns(lines[start].Text)
	if len(first) < 2 {
		return nil, 0, false
	}

	rows := [][]string{first}
	next := start + 1
	for next < len(lines) {
		cols := splitColumns(lines[next].Text)
		if len(cols) != len(first) {
			break
		}
		rows = append(rows, cols)
		next++
	}
	if len(rows) < 3 {
		return nil, 0, false
	}
	return rows, next, true
}

func splitColumns(text string) []string {
	cols := columnSplit.Split(strings.TrimSpace(text), -1)
	out := make([]string, 0, len(cols))
	for _, col := range cols {
		col = strings.TrimSpace(col)
		if col == "" {
			return nil
		}
		out = append(out, col)
	}
	if len(out) < 2 {
		return nil
	}
	return out
}

func normalizeSpace(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func isCJK(r rune) bool {
	return (r >= 0x3000 && r <= 0x303F) ||
		(r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0xFF00 && r <= 0xFFEF)
}
