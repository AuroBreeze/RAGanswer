package webparse

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
)

const sample = `<!doctype html><html><head><title>网页标题</title></head><body>
<nav>无关导航</nav><main>&#65279;<h1 id="manual">维护手册</h1><h2 id="check">点检</h2>
<p>压力须为 <b>8 MPa</b>。<br>每班点检。</p><ol start="3"><li>关阀<ul><li>记录数值</li></ul></li></ol>
<table><caption>点检阈值</caption><tr><th>项目</th><th>单位</th></tr><tr><td>压力</td><td>MPa</td></tr></table>
<script>doNotIncludeThisScript()</script><p hidden>隐藏文字</p><p style="display: none !important">隐藏内容</p></main></body></html>`

func setupTask(t *testing.T, source string) (*Store, Task, SourceMetadata) {
	t.Helper()
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	task := Task{"parse_web_001", "src_manual", "doc_r1", "web", "snapshot/web/manual-r1/page.html"}
	meta := SourceMetadata{Title: "登记标题", OriginalURL: "https://example.com/manual", SourceSHA256: checksum([]byte(source)), ContentType: "text/html; charset=utf-8"}
	if err := store.putImmutable(task.SnapshotRef, []byte(source)); err != nil {
		t.Fatal(err)
	}
	return store, task, meta
}

func TestParseSnapshotProducesProtocolRecords(t *testing.T) {
	store, task, meta := setupTask(t, sample)
	result := ParseSnapshot(store, task, meta)
	if result.Status != "parsed" || result.ArtifactRef != "parsed/parse_web_001/document.ndjson" || result.Error != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	data, err := store.read(result.ArtifactRef, MaxArtifactBytes)
	if err != nil {
		t.Fatal(err)
	}
	if result.ArtifactSHA256 != checksum(data) || bytes.Contains(data, []byte("doNotIncludeThisScript")) || bytes.Contains(data, []byte("隐藏")) || bytes.Contains(data, []byte("无关导航")) {
		t.Fatalf("invalid digest or excluded content: %s", data)
	}
	lines := bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n"))
	var head Document
	if err := json.Unmarshal(lines[0], &head); err != nil {
		t.Fatal(err)
	}
	if head.RecordType != "document" || head.SchemaVersion != "1.0" || head.SourceType != "web" || head.SourceID != task.SourceID || head.DocumentID != task.DocumentID || head.ParseID != task.ParseID || head.SnapshotRef != task.SnapshotRef || head.SourceSHA256 != meta.SourceSHA256 || head.Title != meta.Title || head.OriginalURL != meta.OriginalURL || head.ParsedAt.IsZero() || head.Version != nil || head.PageCount != nil || head.Quality != "complete" || head.Warnings == nil || len(head.Warnings) != 0 || head.BlockCount != len(lines)-1 {
		t.Fatalf("unexpected document header: %+v", head)
	}
	wantKinds := []string{"heading", "heading", "paragraph", "list", "caption", "table"}
	if head.BlockCount != len(wantKinds) {
		t.Fatalf("unexpected block count %d", head.BlockCount)
	}
	for i, line := range lines[1:] {
		var b Block
		if err := json.Unmarshal(line, &b); err != nil {
			t.Fatal(err)
		}
		if b.RecordType != "block" || b.BlockID == "" || b.Sequence != i+1 || b.Kind != wantKinds[i] || b.Locator.Type != "web" || b.ExtractionMethod != "html" || b.Locator.Anchor == nil || b.Text == "" {
			t.Fatalf("unexpected block %+v", b)
		}
		if i > 0 && (*b.Locator.Anchor != "check" || strings.Join(b.SectionPath, "/") != "维护手册/点检") {
			t.Fatalf("wrong section or anchor in block %+v", b)
		}
		switch b.Kind {
		case "paragraph":
			if b.Text != "压力须为 8 MPa。\n每班点检。" {
				t.Fatalf("wrong paragraph %q", b.Text)
			}
		case "list":
			if b.Text != "3. 关阀\n  - 记录数值" {
				t.Fatalf("wrong list %q", b.Text)
			}
		case "table":
			if b.Table == nil || strings.Join(b.Table.Headers, ",") != "项目,单位" || len(b.Table.Rows) != 1 || strings.Join(b.Table.Rows[0], ",") != "压力,MPa" {
				t.Fatalf("wrong table %+v", b.Table)
			}
		}
	}
}

func TestParseSnapshotDeduplicatesConcurrentRetries(t *testing.T) {
	store, task, meta := setupTask(t, sample)
	const count = 6
	results := make(chan Result, count)
	var wg sync.WaitGroup
	for range count {
		wg.Go(func() { results <- ParseSnapshot(store, task, meta) })
	}
	wg.Wait()
	close(results)
	first := <-results
	if first.Status != "parsed" {
		t.Fatalf("first result failed: %+v", first.Error)
	}
	for result := range results {
		if result != first {
			t.Fatalf("retry got %+v, want %+v", result, first)
		}
	}
	task.DocumentID = "changed_version"
	if result := ParseSnapshot(store, task, meta); result.Error == nil || result.Error.Code != "PARSE_CONFLICT" {
		t.Fatalf("unexpected rebind: %+v", result)
	}
}

func TestPartialAndFailures(t *testing.T) {
	t.Run("missing anchor", func(t *testing.T) {
		store, task, meta := setupTask(t, `<main><p>有正文。</p></main>`)
		result := ParseSnapshot(store, task, meta)
		data, err := store.read(result.ArtifactRef, MaxArtifactBytes)
		if err != nil {
			t.Fatal(err, result.Error)
		}
		var doc Document
		var block Block
		dec := json.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&doc); err != nil || doc.Quality != "partial" || len(doc.Warnings) != 1 {
			t.Fatalf("expected partial with warning: %+v, %v", doc, err)
		}
		if err := dec.Decode(&block); err != nil || block.Locator.Anchor != nil || block.SectionPath == nil || len(block.SectionPath) != 0 {
			t.Fatalf("wrong locator or section: %+v, %v", block, err)
		}
		if err := dec.Decode(&block); !errors.Is(err, io.EOF) {
			t.Fatalf("extra records: %v", err)
		}
	})
	for _, tc := range []struct{ name, body, code string }{
		{"login", `<main><form><input type="password"></form></main>`, "WEB_LOGIN_REQUIRED"},
		{"JavaScript shell", `<body><div id="app"></div><script src="/app.js"></script></body>`, "WEB_RENDER_REQUIRED"},
		{"no content", `<main><nav>导航</nav><h1>标题</h1></main>`, "NO_USABLE_CONTENT"},
		{"wrong snapshot digest", sample, "SNAPSHOT_DIGEST_MISMATCH"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, task, meta := setupTask(t, tc.body)
			if tc.code == "SNAPSHOT_DIGEST_MISMATCH" {
				meta.SourceSHA256 = strings.Repeat("1", 64)
			}
			out := ParseSnapshot(store, task, meta)
			if out.Status != "failed" || out.Error == nil || out.Error.Code != tc.code || out.ArtifactRef != "" {
				t.Fatalf("unexpected failure: %+v (%v)", out, out.Error)
			}
			if _, err := store.read("parsed/"+task.ParseID+"/document.ndjson", MaxArtifactBytes); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed task published an artifact: %v", err)
			}
		})
	}
}

func TestCapturePolicy(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cfg := FetchConfig{AllowedHosts: []string{"example.com", "127.0.0.1"}}
	for _, raw := range []string{
		"https://elsewhere.example.com/", "https://example.com:8080/", "https://example.com/?token=private",
		"https://user:password@example.com/", "http://127.0.0.1/", "file:///example.com",
	} {
		_, err := Capture(context.Background(), store, raw, cfg)
		var failure *Problem
		if !errors.As(err, &failure) || failure.Code != "WEB_ACCESS_DENIED" {
			t.Fatalf("expected access denied for %s, got %v", raw, err)
		}
	}
	approved, _ := url.Parse("https://Example.COM.:443/manual#source")
	if !allowedURL(approved, map[string]bool{"example.com": true}) {
		t.Fatal("approved URL rejected")
	}
	for _, addr := range []string{"127.0.0.1", "10.1.2.3", "100.64.0.1", "169.254.169.254", "::1", "::ffff:192.168.1.1", "64:ff9b::a00:1", "2001:db8::1"} {
		if isPublicIP(netip.MustParseAddr(addr)) {
			t.Fatalf("non-public address accepted: %s", addr)
		}
	}
	for _, addr := range []string{"93.184.216.34", "2606:4700:4700::1111"} {
		if !isPublicIP(netip.MustParseAddr(addr)) {
			t.Fatalf("public address rejected: %s", addr)
		}
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func TestCapturePersistsFinalResponseAndChecksRedirects(t *testing.T) {
	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	const start = "https://example.com/start"
	const final = "https://docs.example.com/manual"
	for _, tc := range []struct{ name, redirect, wantError string }{
		{"permitted", final, ""},
		{"unapproved host", "https://elsewhere.example.com/manual", "WEB_ACCESS_DENIED"},
		{"downgrade", "http://example.com/manual", "WEB_ACCESS_DENIED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				switch req.URL.String() {
				case start:
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {tc.redirect}}, Body: http.NoBody, Request: req}, nil
				case final:
					if req.Header.Get("Referer") != "" || req.Header.Get("Cookie") != "" {
						t.Fatal("unexpected forwarded headers")
					}
					return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}}, Body: io.NopCloser(strings.NewReader(sample)), Request: req}, nil
				default:
					t.Fatalf("attempted unexpected request to %s", req.URL)
					return nil, nil
				}
			})
			result, err := capture(context.Background(), store, start, FetchConfig{AllowedHosts: []string{"example.com", "docs.example.com"}}, transport)
			if tc.wantError != "" {
				var failure *Problem
				if !errors.As(err, &failure) || failure.Code != tc.wantError {
					t.Fatalf("wrong error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := store.read(result.SnapshotRef, MaxSnapshotBytes)
			if err != nil || string(data) != sample || result.OriginalURL != final || result.SourceSHA256 != checksum(data) {
				t.Fatalf("unexpected captured page %+v, %v", result, err)
			}
		})
	}
}
