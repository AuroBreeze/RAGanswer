package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"raganswer/internal/pdfparse"
)

// runPDFCommand implements the "parse-pdf" subcommand, which turns a PDF
// snapshot into a protocol NDJSON artifact.
func runPDFCommand(args []string) int {
	flags := flag.NewFlagSet("parse-pdf", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	input := flags.String("in", "", "输入 PDF 路径")
	output := flags.String("out", "-", "输出 NDJSON 路径，- 表示标准输出")
	parseID := flags.String("parse-id", "", "本次解析产物 ID")
	sourceID := flags.String("source-id", "", "来源稳定 ID")
	documentID := flags.String("document-id", "", "不可变版本 ID")
	snapshotRef := flags.String("snapshot-ref", "", "受控存储引用，默认使用 --in")
	title := flags.String("title", "", "文档标题，默认取 PDF 元数据或文件名")
	version := flags.String("version", "", "原文版本，例如 V3")
	ocrMode := flags.String("ocr", pdfparse.OCRModeAuto, "OCR 模式：auto（无文本层时启用）、off、force")

	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *input == "" {
		fmt.Fprintln(os.Stderr, "缺少 --in")
		return 2
	}

	artifact, err := pdfparse.ParseWithOCR(*input, pdfparse.Task{
		ParseID:     *parseID,
		SourceID:    *sourceID,
		DocumentID:  *documentID,
		SnapshotRef: *snapshotRef,
		Title:       *title,
		Version:     *version,
	}, pdfparse.OCRClientFromEnv(), *ocrMode)
	if err != nil {
		_ = json.NewEncoder(os.Stderr).Encode(pdfparse.NewFailure(*parseID, err))
		return 1
	}

	if *output == "" || *output == "-" {
		if err := artifact.WriteNDJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "写出解析包失败：%v\n", err)
			return 1
		}
		return 0
	}

	file, err := os.Create(*output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "创建解析包失败：%v\n", err)
		return 1
	}
	defer file.Close()

	if err := artifact.WriteNDJSON(io.Writer(file)); err != nil {
		fmt.Fprintf(os.Stderr, "写出解析包失败：%v\n", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(runPDFCommand(os.Args[1:]))
}
