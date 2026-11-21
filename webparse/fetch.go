package webparse

import (
	"context"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const MaxSnapshotBytes = 4 << 20

type FetchConfig struct {
	AllowedHosts []string `json:"allowed_hosts"`
}

type CapturedPage struct {
	SnapshotRef  string `json:"snapshot_ref"`
	SourceSHA256 string `json:"source_sha256"`
	OriginalURL  string `json:"original_url"`
	ContentType  string `json:"content_type"`
}

// Capture saves the original bytes before returning their reference to the caller.
func Capture(ctx context.Context, storage *Store, rawURL string, cfg FetchConfig) (CapturedPage, error) {
	transport := &http.Transport{
		DialContext:            dialPublic,
		TLSHandshakeTimeout:    5 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		DisableKeepAlives:      true,
	}
	return capture(ctx, storage, rawURL, cfg, transport)
}

func capture(ctx context.Context, storage *Store, rawURL string, cfg FetchConfig, transport http.RoundTripper) (CapturedPage, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	allowed := make(map[string]bool, len(cfg.AllowedHosts))
	for _, host := range cfg.AllowedHosts {
		allowed[canonicalHost(strings.TrimSpace(host))] = true
	}
	target, err := url.Parse(rawURL)
	if err != nil || !allowedURL(target, allowed) {
		return CapturedPage{}, problem("WEB_ACCESS_DENIED", "网页地址不符合抓取策略。", false)
	}
	target.Fragment, target.RawFragment = "", ""
	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 || !allowedURL(req.URL, allowed) || via[len(via)-1].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return problem("WEB_ACCESS_DENIED", "网页重定向目标不符合抓取策略。", false)
			}
			// The page URL does not need to be sent to the redirect destination.
			req.Header.Del("Referer")
			return nil
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return CapturedPage{}, problem("INVALID_REQUEST", "网页地址无效。", false)
	}
	req.Header.Set("Accept", "text/html, application/xhtml+xml")
	req.Header.Set("User-Agent", "RAGanswer-WebParser/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return CapturedPage{}, fetchFailure(err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusProxyAuthRequired:
		return CapturedPage{}, problem("WEB_LOGIN_REQUIRED", "网页需要登录。", false)
	case http.StatusForbidden:
		return CapturedPage{}, problem("WEB_ACCESS_DENIED", "网页拒绝访问。", false)
	case http.StatusOK:
	default:
		return CapturedPage{}, problem("WEB_FETCH_FAILED", "网页没有返回可解析的成功响应。", resp.StatusCode >= 500 || resp.StatusCode == http.StatusTooManyRequests)
	}
	contentType := resp.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "text/html" && mediaType != "application/xhtml+xml" {
		return CapturedPage{}, problem("UNSUPPORTED_FORMAT", "只支持 HTML 网页。", false)
	}
	// Keep only the media type and character encoding from the response headers.
	contentType = mediaType
	if charset := params["charset"]; charset != "" && len(charset) <= 64 {
		contentType = mime.FormatMediaType(mediaType, map[string]string{"charset": charset})
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxSnapshotBytes+1))
	if err != nil {
		return CapturedPage{}, fetchFailure(err)
	}
	if len(data) > MaxSnapshotBytes {
		return CapturedPage{}, problem("SNAPSHOT_TOO_LARGE", "网页快照超过 4 MiB。", false)
	}
	if len(data) == 0 {
		return CapturedPage{}, problem("NO_USABLE_CONTENT", "网页响应为空。", false)
	}
	finalURL := *resp.Request.URL
	finalURL.Fragment, finalURL.RawFragment = "", ""
	finalURL.Host = canonicalHost(finalURL.Hostname())
	digest := checksum(data)
	ref := "snapshot/web/" + digest + "/page.html"
	if err := storage.putImmutable(ref, data); err != nil {
		return CapturedPage{}, problem("STORAGE_UNAVAILABLE", "无法保存网页快照。", true)
	}
	return CapturedPage{SnapshotRef: ref, SourceSHA256: digest, OriginalURL: finalURL.String(), ContentType: contentType}, nil
}

func allowedURL(target *url.URL, allowed map[string]bool) bool {
	if target == nil || target.User != nil || target.Opaque != "" || target.RawQuery != "" || target.ForceQuery {
		return false
	}
	host := canonicalHost(target.Hostname())
	if host == "" || !allowed[host] || !strings.Contains(host, ".") {
		return false
	}
	if _, err := netip.ParseAddr(host); err == nil {
		return false
	}
	port := target.Port()
	return target.Scheme == "http" && (port == "" || port == "80") || target.Scheme == "https" && (port == "" || port == "443")
}

func canonicalHost(host string) string {
	return strings.ToLower(strings.TrimSuffix(host, "."))
}

var deniedNetworks = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"), netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("192.0.0.0/24"), netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.88.99.0/24"), netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"), netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("240.0.0.0/4"), netip.MustParsePrefix("2001::/23"),
	netip.MustParsePrefix("2001:db8::/32"), netip.MustParsePrefix("2002::/16"),
	netip.MustParsePrefix("3fff::/20"),
}

var publicIPv6 = netip.MustParsePrefix("2000::/3")

func isPublicIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.Is6() && !publicIPv6.Contains(addr) {
		return false
	}
	for _, network := range deniedNetworks {
		if network.Contains(addr) {
			return false
		}
	}
	return true
}

func dialPublic(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	for _, addr := range addresses {
		if !isPublicIP(addr) {
			return nil, problem("WEB_ACCESS_DENIED", "网页的网络地址不符合抓取策略。", false)
		}
	}
	// Connect to the inspected addresses directly, with no second DNS lookup.
	dialer := net.Dialer{Timeout: 2 * time.Second}
	lastErr := errors.New("no public address found")
	for _, addr := range addresses {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func fetchFailure(err error) error {
	var failure *Problem
	if errors.As(err, &failure) {
		return failure
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &netErr) && netErr.Timeout() {
		return problem("FETCH_TIMEOUT", "网页抓取超时。", true)
	}
	return problem("WEB_FETCH_FAILED", "无法获取网页。", true)
}
