package pdfparse

import (
	"bytes"
	"fmt"
	"strings"
)

type fixtureLine struct {
	size float64
	text string
	y    float64
}

// buildPDF emits a minimal valid PDF with a Helvetica text layer, used to
// exercise the parser without binary fixtures.
func buildPDF(pages [][]fixtureLine) []byte {
	fontNum := 3 + len(pages)*2

	objects := make([]string, 3, fontNum)
	objects[1] = "<< /Type /Catalog /Pages 2 0 R >>"

	kids := make([]string, 0, len(pages))
	for _, lines := range pages {
		pageNum := len(objects)
		contentNum := pageNum + 1
		objects = append(objects,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 %d 0 R >> >> /Contents %d 0 R >>", fontNum, contentNum),
			fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(contentStream(lines)), contentStream(lines)),
		)
		kids = append(kids, fmt.Sprintf("%d 0 R", pageNum))
	}

	objects[2] = fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), len(pages))
	objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")

	var buf bytes.Buffer
	buf.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects))
	for i := 1; i < len(objects); i++ {
		offsets[i] = buf.Len()
		fmt.Fprintf(&buf, "%d 0 obj\n%s\nendobj\n", i, objects[i])
	}

	xref := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 %d\n", len(objects))
	buf.WriteString("0000000000 65535 f \n")
	for i := 1; i < len(objects); i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&buf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects), xref)

	return buf.Bytes()
}

func contentStream(lines []fixtureLine) string {
	var sb strings.Builder
	sb.WriteString("BT\n")
	for _, l := range lines {
		fmt.Fprintf(&sb, "/F1 %.1f Tf\n1 0 0 1 72 %.1f Tm\n(%s) Tj\n", l.size, l.y, escapePDF(l.text))
	}
	sb.WriteString("ET")
	return sb.String()
}

func escapePDF(text string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	return replacer.Replace(text)
}
