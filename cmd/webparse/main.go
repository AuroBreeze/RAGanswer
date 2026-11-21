package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"raganswer/webparse"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, input io.Reader, output, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "capture" && args[0] != "parse" {
		fmt.Fprintln(stderr, "usage: webparse <capture|parse> -store <directory>")
		return 2
	}
	mode := args[0]
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(stderr)
	dir := flags.String("store", "", "existing private storage directory")
	var rawURL, hosts *string
	if mode == "capture" {
		rawURL = flags.String("url", "", "registered page URL")
		hosts = flags.String("allow-hosts", "", "comma-separated list of approved hosts")
	}
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *dir == "" || flags.NArg() != 0 || mode == "capture" && (*rawURL == "" || *hosts == "") {
		flags.Usage()
		return 2
	}
	store, err := webparse.OpenStore(*dir)
	if err != nil {
		fmt.Fprintln(stderr, "cannot open the private storage directory")
		return 1
	}
	defer store.Close()
	enc := json.NewEncoder(output)
	if mode == "capture" {
		captured, err := webparse.Capture(ctx, store, *rawURL, webparse.FetchConfig{AllowedHosts: strings.Split(*hosts, ",")})
		if err != nil {
			var failure *webparse.Problem
			if errors.As(err, &failure) {
				if err := enc.Encode(webparse.Result{Status: "failed", Error: failure}); err != nil {
					fmt.Fprintln(stderr, "cannot write capture result")
				}
			}
			return 1
		}
		if err := enc.Encode(captured); err != nil {
			return 1
		}
		return 0
	}
	var req struct {
		Task     webparse.Task           `json:"task"`
		Document webparse.SourceMetadata `json:"document"`
	}
	const inputLimit = 64 << 10
	data, err := io.ReadAll(io.LimitReader(input, inputLimit+1))
	if err != nil || len(data) > inputLimit || json.Unmarshal(data, &req) != nil {
		fmt.Fprintln(stderr, "expected one JSON object (max 64 KiB) with task and document fields")
		return 2
	}
	result := webparse.ParseSnapshot(store, req.Task, req.Document)
	if err := enc.Encode(result); err != nil || result.Status != "parsed" {
		return 1
	}
	return 0
}
