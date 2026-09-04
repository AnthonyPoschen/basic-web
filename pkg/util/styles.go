package util

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"
)

const bundledStylesheetPath = "framework/styles.css"
const bundledStylesheetURL = "/framework/styles.css"

var htmlLinkTagPattern = regexp.MustCompile(`(?is)<link\b[^>]*>`)
var htmlHrefPattern = regexp.MustCompile(`(?i)\bhref\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
var htmlRelPattern = regexp.MustCompile(`(?i)\brel\s*=\s*("([^"]*)"|'([^']*)'|([^\s>]+))`)
var cssURLPattern = regexp.MustCompile(`url\(\s*(("([^"]*)")|('([^']*)')|((?:\\.|[^"'()\\])+))\s*\)`)

type bundledStylesFS struct {
	base fs.FS
}

// WithBundledStylesheets keeps source CSS files separate and rewrites index.html
// to a single /framework/styles.css response when the document links more than
// one local stylesheet. The bundle is rebuilt from the current files on each
// request so development edits still apply.
//
// This exists because each extra stylesheet is render-blocking and, on HTTP/1.1,
// queues LCP work behind extra round trips. Do not require apps to merge CSS in
// source. See docs/performance.md.
func WithBundledStylesheets(base fs.FS) fs.FS {
	if base == nil {
		return base
	}
	return bundledStylesFS{base: base}
}

func (f bundledStylesFS) Open(name string) (fs.File, error) {
	clean := path.Clean(strings.TrimPrefix(name, "/"))
	if clean == bundledStylesheetPath {
		_, css, ok, err := bundleIndexStylesheets(f.base)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fs.ErrNotExist
		}
		return newBytesFile(path.Base(clean), css), nil
	}

	file, err := f.base.Open(name)
	if err != nil {
		return nil, err
	}
	if clean != "index.html" {
		return file, nil
	}

	data, info, err := readFSFile(file)
	if err != nil {
		return nil, err
	}
	rewritten, _, ok, err := bundleIndexHTML(data, f.base)
	if err != nil {
		return nil, err
	}
	if !ok {
		return newBytesFile(info.Name(), data), nil
	}
	return newBytesFile(info.Name(), rewritten), nil
}

func bundleIndexStylesheets(files fs.FS) ([]byte, []byte, bool, error) {
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, false, nil
		}
		return nil, nil, false, err
	}
	return bundleIndexHTML(index, files)
}

func bundleIndexHTML(index []byte, files fs.FS) ([]byte, []byte, bool, error) {
	type styleLink struct {
		start int
		end   int
		path  string
		query string
	}

	html := string(index)
	var links []styleLink
	for _, match := range htmlLinkTagPattern.FindAllStringIndex(html, -1) {
		tag := html[match[0]:match[1]]
		if isStylesheetRel(quotedAttr(htmlRelPattern, tag)) == false {
			continue
		}
		href := quotedAttr(htmlHrefPattern, tag)
		fsPath, query, ok := localStylesheetPath(href)
		if !ok {
			continue
		}
		links = append(links, styleLink{start: match[0], end: match[1], path: fsPath, query: query})
	}
	if len(links) < 2 {
		return index, nil, false, nil
	}

	var css bytes.Buffer
	for i, link := range links {
		data, err := fs.ReadFile(files, link.path)
		if err != nil {
			return index, nil, false, nil
		}
		if i > 0 {
			css.WriteByte('\n')
		}
		css.WriteString(rewriteCSSURLs(string(data), link.path))
	}

	bundleHref := bundledStylesheetURL
	if links[0].query != "" {
		bundleHref += "?" + links[0].query
	}

	var rewritten strings.Builder
	rewritten.Grow(len(html) + len(bundleHref))
	cursor := 0
	inserted := false
	for _, link := range links {
		rewritten.WriteString(html[cursor:link.start])
		if inserted == false {
			rewritten.WriteString(`<link rel="stylesheet" href="` + bundleHref + `">`)
			inserted = true
		}
		cursor = link.end
	}
	rewritten.WriteString(html[cursor:])
	return []byte(rewritten.String()), css.Bytes(), true, nil
}

func localStylesheetPath(href string) (string, string, bool) {
	href = strings.TrimSpace(href)
	if href == "" {
		return "", "", false
	}
	parsed, err := url.Parse(href)
	if err != nil || parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(href, "//") {
		return "", "", false
	}
	clean := path.Clean("/" + parsed.Path)
	fsPath := strings.TrimPrefix(clean, "/")
	if fsPath == "" || fsPath == "." {
		return "", "", false
	}
	return fsPath, parsed.RawQuery, true
}

func rewriteCSSURLs(css string, cssFile string) string {
	dir := path.Dir(cssFile)
	return cssURLPattern.ReplaceAllStringFunc(css, func(match string) string {
		sub := cssURLPattern.FindStringSubmatch(match)
		if len(sub) == 0 {
			return match
		}
		ref := firstNonEmpty(sub[3], sub[5], strings.TrimSpace(sub[6]))
		resolved := resolveCSSURL(dir, ref)
		if resolved == ref {
			return match
		}
		quote := ""
		if sub[3] != "" {
			quote = `"`
		} else if sub[5] != "" {
			quote = `'`
		}
		return "url(" + quote + resolved + quote + ")"
	})
}

func resolveCSSURL(dir, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || hasUnrewrittenCSSURLPrefix(ref) {
		return ref
	}
	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if parsed.Scheme != "" || parsed.Host != "" || strings.HasPrefix(parsed.Path, "/") {
		return ref
	}
	resolved := path.Clean("/" + path.Join(dir, parsed.Path))
	if parsed.RawQuery != "" {
		resolved += "?" + parsed.RawQuery
	}
	if parsed.Fragment != "" {
		resolved += "#" + parsed.Fragment
	}
	return resolved
}

func hasUnrewrittenCSSURLPrefix(ref string) bool {
	switch {
	case strings.HasPrefix(ref, "data:"):
		return true
	case strings.HasPrefix(ref, "http:"):
		return true
	case strings.HasPrefix(ref, "https:"):
		return true
	case strings.HasPrefix(ref, "//"):
		return true
	case strings.HasPrefix(ref, "#"):
		return true
	case strings.HasPrefix(ref, "/"):
		return true
	default:
		return false
	}
}

func isStylesheetRel(rel string) bool {
	for _, token := range strings.Fields(strings.ToLower(rel)) {
		if token == "stylesheet" {
			return true
		}
	}
	return false
}

func quotedAttr(pattern *regexp.Regexp, tag string) string {
	match := pattern.FindStringSubmatch(tag)
	if len(match) == 0 {
		return ""
	}
	return firstNonEmpty(match[2], match[3], match[4])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func readFSFile(file fs.File) ([]byte, fs.FileInfo, error) {
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	data, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	return data, info, nil
}

type bytesFile struct {
	reader *bytes.Reader
	name   string
	closed bool
}

func newBytesFile(name string, data []byte) *bytesFile {
	return &bytesFile{reader: bytes.NewReader(data), name: name}
}

func (f *bytesFile) Stat() (fs.FileInfo, error) {
	return bytesFileInfo{name: f.name, size: int64(f.reader.Size())}, nil
}

func (f *bytesFile) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	return f.reader.Read(p)
}

func (f *bytesFile) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	return f.reader.Seek(offset, whence)
}

func (f *bytesFile) Close() error {
	f.closed = true
	return nil
}

type bytesFileInfo struct {
	name string
	size int64
}

func (info bytesFileInfo) Name() string       { return info.name }
func (info bytesFileInfo) Size() int64        { return info.size }
func (info bytesFileInfo) Mode() fs.FileMode  { return 0o444 }
func (info bytesFileInfo) ModTime() time.Time { return time.Time{} }
func (info bytesFileInfo) IsDir() bool        { return false }
func (info bytesFileInfo) Sys() any           { return nil }
