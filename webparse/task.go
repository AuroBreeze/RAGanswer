package webparse

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/url"
	"os"
	"regexp"
	"strings"

	"golang.org/x/net/html/charset"
)

type Task struct {
	ParseID     string `json:"parse_id"`
	SourceID    string `json:"source_id"`
	DocumentID  string `json:"document_id"`
	SourceType  string `json:"source_type"`
	SnapshotRef string `json:"snapshot_ref"`
}

type SourceMetadata struct {
	Title        string  `json:"title"`
	Version      *string `json:"version"`
	OriginalURL  string  `json:"original_url"`
	SourceSHA256 string  `json:"source_sha256"`
	ContentType  string  `json:"content_type"`
}

type Result struct {
	ParseID        string   `json:"parse_id"`
	Status         string   `json:"status"`
	ArtifactRef    string   `json:"artifact_ref,omitempty"`
	ArtifactSHA256 string   `json:"artifact_sha256,omitempty"`
	Error          *Problem `json:"error,omitempty"`
}

type Problem struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

func (p *Problem) Error() string { return p.Message }

func problem(code, message string, retryable bool) *Problem {
	return &Problem{Code: code, Message: message, Retryable: retryable}
}

var (
	identifier         = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)
	snapshotRefPattern = regexp.MustCompile(`^snapshot/web/(?:[A-Za-z0-9_-]{1,128}/)?[A-Za-z0-9_-]{1,128}\.html$`)
	digestPattern      = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// ParseSnapshot consumes a task and trusted registration metadata. It publishes the
// artifact only after checking the snapshot digest and encoding all records.
func ParseSnapshot(storage *Store, task Task, meta SourceMetadata) Result {
	fail := func(code, message string, retryable bool) Result {
		return Result{ParseID: task.ParseID, Status: "failed", Error: problem(code, message, retryable)}
	}
	if task.SourceType != "web" || !identifier.MatchString(task.ParseID) || !identifier.MatchString(task.SourceID) || !identifier.MatchString(task.DocumentID) || !snapshotRefPattern.MatchString(task.SnapshotRef) || !digestPattern.MatchString(meta.SourceSHA256) {
		return fail("INVALID_REQUEST", "网页解析任务或来源摘要无效。", false)
	}
	target, err := url.Parse(meta.OriginalURL)
	if err != nil || !allowedURL(target, map[string]bool{canonicalHost(target.Hostname()): true}) || target.Fragment != "" {
		return fail("INVALID_REQUEST", "登记的网页地址无效。", false)
	}
	mediaType, _, err := mime.ParseMediaType(meta.ContentType)
	if err != nil || mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return fail("UNSUPPORTED_FORMAT", "只支持 HTML 网页。", false)
	}
	data, err := storage.read(task.SnapshotRef, MaxSnapshotBytes)
	if errors.Is(err, os.ErrNotExist) {
		return fail("SNAPSHOT_NOT_FOUND", "未找到网页快照。", false)
	}
	if err != nil {
		return fail("SNAPSHOT_UNAVAILABLE", "无法读取网页快照或快照超限。", false)
	}
	if checksum(data) != meta.SourceSHA256 {
		return fail("SNAPSHOT_DIGEST_MISMATCH", "网页快照与登记摘要不一致。", false)
	}
	ref := "parsed/" + task.ParseID + "/document.ndjson"
	success := func(artifact []byte) Result {
		return Result{ParseID: task.ParseID, Status: "parsed", ArtifactRef: ref, ArtifactSHA256: checksum(artifact)}
	}
	readExisting := func() (Result, error) {
		artifact, err := storage.read(ref, MaxArtifactBytes)
		if err != nil {
			return Result{}, err
		}
		var first Document
		dec := json.NewDecoder(bytes.NewReader(artifact))
		if err := dec.Decode(&first); err != nil || first.RecordType != "document" || first.SchemaVersion != "1.0" || first.SourceType != "web" || first.ParseID != task.ParseID || first.SourceID != task.SourceID || first.DocumentID != task.DocumentID || first.SnapshotRef != task.SnapshotRef || first.SourceSHA256 != meta.SourceSHA256 || first.OriginalURL != meta.OriginalURL || !equalVersion(first.Version, meta.Version) || strings.TrimSpace(meta.Title) != "" && first.Title != strings.TrimSpace(meta.Title) {
			return Result{}, ErrImmutableConflict
		}
		if first.BlockCount < 1 || !validStoredBlocks(dec, first.BlockCount) {
			return Result{}, errors.New("published artifact has invalid content blocks")
		}
		return success(artifact), nil
	}
	if existing, err := readExisting(); err == nil {
		return existing
	} else if errors.Is(err, ErrImmutableConflict) {
		return fail("PARSE_CONFLICT", "解析 ID 已绑定其他来源、版本或快照。", false)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fail("STORAGE_UNAVAILABLE", "无法读取已有解析产物。", true)
	}
	reader, err := charset.NewReader(bytes.NewReader(data), meta.ContentType)
	if err != nil {
		return fail("UNSUPPORTED_FORMAT", "无法识别网页字符编码。", false)
	}
	artifact, err := encode(reader, Document{
		ParseID: task.ParseID, SourceID: task.SourceID, DocumentID: task.DocumentID,
		Title: strings.TrimSpace(meta.Title), Version: meta.Version, OriginalURL: meta.OriginalURL,
		SnapshotRef: task.SnapshotRef, SourceSHA256: meta.SourceSHA256,
	})
	switch {
	case errors.Is(err, ErrLoginRequired):
		return fail("WEB_LOGIN_REQUIRED", "网页正文需要登录。", false)
	case errors.Is(err, ErrRenderRequired):
		return fail("WEB_RENDER_REQUIRED", "静态网页没有正文，可能需要动态渲染。", false)
	case errors.Is(err, ErrNoContent):
		return fail("NO_USABLE_CONTENT", "网页未提取到可用正文。", false)
	case errors.Is(err, ErrArtifactTooLarge):
		return fail("ARTIFACT_TOO_LARGE", "解析包超过 16 MiB。", false)
	case err != nil:
		return fail("WEB_PARSE_FAILED", "网页无法解析。", false)
	}
	if err := storage.putImmutable(ref, artifact); err != nil {
		if errors.Is(err, ErrImmutableConflict) {
			if existing, err := readExisting(); err == nil {
				return existing
			} else if !errors.Is(err, ErrImmutableConflict) {
				return fail("STORAGE_UNAVAILABLE", "无法读取已有解析产物。", true)
			}
			return fail("PARSE_CONFLICT", "解析 ID 已绑定其他来源、版本或快照。", false)
		}
		return fail("STORAGE_UNAVAILABLE", "无法发布网页解析产物。", true)
	}
	return success(artifact)
}

func validStoredBlocks(dec *json.Decoder, count int) bool {
	for i := 1; i <= count; i++ {
		var block Block
		if err := dec.Decode(&block); err != nil || block.RecordType != "block" || block.Sequence != i || block.BlockID == "" || block.Text == "" {
			return false
		}
	}
	var extra json.RawMessage
	return errors.Is(dec.Decode(&extra), io.EOF)
}

func checksum(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func equalVersion(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
