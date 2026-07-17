package nodelist

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultUserAgent identifies NodeGUI to file hosts that require a browser-like agent.
const DefaultUserAgent = "NodeGUI/0.2 (+https://github.com/JohnDovey/NodeGUI; FidoNet nodelist client)"

// ArchiveName returns the Z1DAILY.Zjj filename for a julian day (1–366).
// Extension uses the last two digits of the day number (Fido 8.3 convention).
func ArchiveName(dayOfYear int) string {
	if dayOfYear < 1 {
		dayOfYear = 1
	}
	jj := dayOfYear % 100
	return fmt.Sprintf("Z1DAILY.Z%02d", jj)
}

// ExtractedName returns the typical uncompressed filename (Z1DAILY.ddd).
func ExtractedName(dayOfYear int) string {
	return fmt.Sprintf("Z1DAILY.%d", dayOfYear)
}

// JoinURL joins a base directory URL with a filename, preserving special path
// segments such as "$webfile.send.ZC1." (no path.Clean — that would mangle them).
func JoinURL(base, name string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("empty base URL")
	}
	if _, err := url.Parse(base); err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	name = strings.TrimLeft(name, "/")
	if name == "" {
		return "", fmt.Errorf("empty filename")
	}
	return base + name, nil
}

// DownloadResult is the outcome of fetching a remote daily archive.
type DownloadResult struct {
	URL      string
	Filename string
	Data     []byte
	Day      int
}

// Client downloads nodelist archives over HTTP(S).
type Client struct {
	HTTP      *http.Client
	UserAgent string
	Referer   string
}

// NewClient returns a client with sensible timeouts and browser-like headers.
func NewClient() *Client {
	return &Client{
		HTTP: &http.Client{
			Timeout: 120 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				// Re-apply UA on redirects.
				if req.Header.Get("User-Agent") == "" {
					req.Header.Set("User-Agent", DefaultUserAgent)
				}
				return nil
			},
		},
		UserAgent: DefaultUserAgent,
	}
}

// FetchDay downloads Z1DAILY for the given julian day from baseURL.
func (c *Client) FetchDay(baseURL string, dayOfYear int) (*DownloadResult, error) {
	name := ArchiveName(dayOfYear)
	full, err := JoinURL(baseURL, name)
	if err != nil {
		return nil, err
	}
	data, err := c.get(full, baseURL)
	if err != nil {
		return nil, err
	}
	return &DownloadResult{URL: full, Filename: name, Data: data, Day: dayOfYear}, nil
}

// FetchLatest tries today, then walks backward up to maxBack days for an available archive.
func (c *Client) FetchLatest(baseURL string, now time.Time, maxBack int) (*DownloadResult, error) {
	if maxBack < 1 {
		maxBack = 14
	}
	if now.IsZero() {
		now = time.Now()
	}
	var lastErr error
	for i := 0; i < maxBack; i++ {
		t := now.AddDate(0, 0, -i)
		day := t.YearDay()
		res, err := c.FetchDay(baseURL, day)
		if err == nil {
			return res, nil
		}
		lastErr = err
		// Only continue on 404-like failures.
		if !isNotFound(err) && i == 0 {
			// still try a few more days in case of day-boundary lag
			continue
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no archive found")
	}
	return nil, fmt.Errorf("fetch daily nodelist (tried %d days): %w", maxBack, lastErr)
}

func (c *Client) get(fullURL, referer string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	ua := c.UserAgent
	if ua == "" {
		ua = DefaultUserAgent
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "*/*")
	if c.Referer != "" {
		req.Header.Set("Referer", c.Referer)
	} else if referer != "" {
		// Prefer site root as referer (helps hosts that reject bare clients).
		if u, err := url.Parse(referer); err == nil {
			req.Header.Set("Referer", u.Scheme+"://"+u.Host+"/")
		}
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", fullURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20)) // 64 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, &httpError{code: resp.StatusCode, url: fullURL, msg: "not found"}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &httpError{code: resp.StatusCode, url: fullURL, msg: string(body)}
	}
	if len(body) < 4 {
		return nil, fmt.Errorf("response too small from %s", fullURL)
	}
	return body, nil
}

type httpError struct {
	code int
	url  string
	msg  string
}

func (e *httpError) Error() string {
	return fmt.Sprintf("HTTP %d for %s: %s", e.code, e.url, truncate(e.msg, 120))
}

func isNotFound(err error) bool {
	if he, ok := err.(*httpError); ok {
		return he.code == http.StatusNotFound
	}
	return false
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// UnzipNodelist extracts the first plausible nodelist text file from a ZIP archive.
func UnzipNodelist(zipData []byte) (filename string, content []byte, err error) {
	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		// Not a zip? Return as plain text if it looks like a nodelist.
		if looksLikeNodelist(zipData) {
			return "NODELIST.TXT", zipData, nil
		}
		return "", nil, fmt.Errorf("open zip: %w", err)
	}
	var candidates []*zip.File
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := filepath.Base(f.Name)
		upper := strings.ToUpper(base)
		if strings.HasPrefix(upper, "Z1DAILY.") ||
			strings.HasPrefix(upper, "NODELIST.") ||
			strings.HasSuffix(upper, ".TXT") ||
			!strings.Contains(base, ".") {
			candidates = append(candidates, f)
		}
	}
	if len(candidates) == 0 {
		// fall back to first file
		for _, f := range r.File {
			if !f.FileInfo().IsDir() {
				candidates = append(candidates, f)
				break
			}
		}
	}
	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("zip contains no files")
	}
	// Prefer Z1DAILY.* then NODELIST.* then first
	pick := candidates[0]
	for _, f := range candidates {
		u := strings.ToUpper(filepath.Base(f.Name))
		if strings.HasPrefix(u, "Z1DAILY.") {
			pick = f
			break
		}
		if strings.HasPrefix(u, "NODELIST.") {
			pick = f
		}
	}
	rc, err := pick.Open()
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, 64<<20))
	if err != nil {
		return "", nil, err
	}
	if !looksLikeNodelist(data) {
		return "", nil, fmt.Errorf("extracted %s does not look like a nodelist", pick.Name)
	}
	return filepath.Base(pick.Name), data, nil
}

func looksLikeNodelist(data []byte) bool {
	// Sample first few KB for comment header or Zone, line.
	n := len(data)
	if n > 4096 {
		n = 4096
	}
	sample := string(data[:n])
	if strings.Contains(sample, "Fido") || strings.Contains(sample, "NodeList") ||
		strings.Contains(sample, "Nodelist") || strings.Contains(sample, "Zone,") {
		return true
	}
	// Leading semicolon comments are typical.
	return strings.HasPrefix(strings.TrimSpace(sample), ";")
}

// ReadLocalFile loads a local nodelist (plain text or zip) from disk.
func ReadLocalFile(path string) (filename string, content []byte, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, err
	}
	base := filepath.Base(path)
	upper := strings.ToUpper(base)
	if strings.HasSuffix(upper, ".ZIP") ||
		(len(data) >= 2 && data[0] == 'P' && data[1] == 'K') {
		return UnzipNodelist(data)
	}
	if !looksLikeNodelist(data) {
		return "", nil, fmt.Errorf("%s does not look like a nodelist", path)
	}
	return base, data, nil
}
