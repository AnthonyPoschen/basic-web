package util

import (
	"encoding/xml"
	"fmt"
	"html"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const DefaultPublicHTMLCacheControl = "public, max-age=3600"

var (
	titlePattern       = regexp.MustCompile(`(?is)<title\b[^>]*>[^<]*</title>`)
	descriptionPattern = regexp.MustCompile(`(?is)<meta\s+name=(?:"description"|'description'|description)[^>]*>`)
	canonicalPattern   = regexp.MustCompile(`(?is)<link\s+[^>]*rel=(?:"canonical"|'canonical'|canonical)[^>]*>`)
	headEndPattern     = regexp.MustCompile(`(?is)</head>`)
	routeViewPattern   = regexp.MustCompile(`(?is)<route-view\b[^>]*>.*?</route-view>`)
)

type Document struct {
	Title       string
	Description string
	Canonical   string
	Status      int
	Redirect    string
	Index       bool
	Fills       map[string]string
	HTMLFills   map[string]string
}

func (options SetupHttpMuxOptions) publicCacheControl() string {
	if options.PublicHTMLCacheControl != "" {
		return options.PublicHTMLCacheControl
	}
	return DefaultPublicHTMLCacheControl
}

func (options SetupHttpMuxOptions) origin() string {
	return strings.TrimRight(strings.TrimSpace(options.SiteOrigin), "/")
}

func absoluteURL(origin string, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if path == "" {
		path = "/"
	}
	return strings.TrimRight(origin, "/") + path
}

func applyDocumentHead(index []byte, doc Document, origin string) []byte {
	htmlDoc := string(index)
	if doc.Title != "" {
		htmlDoc = replaceTagContent(htmlDoc, titlePattern, "<title>"+html.EscapeString(doc.Title)+"</title>")
	}
	if doc.Description != "" {
		tag := `<meta name="description" content="` + html.EscapeString(doc.Description) + `">`
		if descriptionPattern.MatchString(htmlDoc) {
			htmlDoc = descriptionPattern.ReplaceAllString(htmlDoc, tag)
		} else {
			htmlDoc = insertHead(htmlDoc, tag)
		}
	}
	canonical := doc.Canonical
	if canonical != "" && origin != "" {
		canonical = absoluteURL(origin, canonical)
		tag := `<link rel="canonical" href="` + html.EscapeString(canonical) + `">`
		if canonicalPattern.MatchString(htmlDoc) {
			htmlDoc = canonicalPattern.ReplaceAllString(htmlDoc, tag)
		} else {
			htmlDoc = insertHead(htmlDoc, tag)
		}
	}
	htmlDoc = insertHead(htmlDoc, `<style>[data-basic-web-server-rendered]:not(:defined){visibility:visible}</style>`)
	return []byte(htmlDoc)
}

func replaceTagContent(document string, pattern *regexp.Regexp, replacement string) string {
	if pattern.MatchString(document) {
		return pattern.ReplaceAllString(document, replacement)
	}
	return insertHead(document, replacement)
}

func insertHead(document string, tag string) string {
	if loc := headEndPattern.FindStringIndex(document); loc != nil {
		return document[:loc[0]] + tag + document[loc[0]:]
	}
	if loc := titlePattern.FindStringIndex(document); loc != nil {
		return document[:loc[1]] + tag + document[loc[1]:]
	}
	return tag + document
}

func applyFills(input string, fills map[string]string, htmlFills map[string]string) string {
	for name, value := range fills {
		input = replaceFill(input, "data-fill", name, html.EscapeString(value))
	}
	for name, value := range htmlFills {
		input = replaceFill(input, "data-fill-html", name, value)
	}
	return input
}

func replaceFill(input string, attr string, name string, value string) string {
	re := regexp.MustCompile(`(?s)(` + regexp.QuoteMeta(attr) + `="` + regexp.QuoteMeta(name) + `"[^>]*>)(.*?)(</[a-zA-Z0-9-]+)`)
	input = re.ReplaceAllString(input, `${1}`+value+`${3}`)
	hidden := regexp.MustCompile(`(` + regexp.QuoteMeta(attr) + `="` + regexp.QuoteMeta(name) + `"[^>]*?)\s+hidden\b`)
	return hidden.ReplaceAllString(input, `$1`)
}

func renderElementTree(site *siteModel, elementName string, fills map[string]string, htmlFills map[string]string, depth int) string {
	if depth > 8 {
		return ""
	}
	inner, ok := site.templates[elementName]
	if ok == false {
		return ""
	}
	inner = applyFills(inner, fills, htmlFills)
	inner = expandNestedElements(site, inner, fills, htmlFills, depth+1)
	return inner
}

func expandNestedElements(site *siteModel, inner string, fills map[string]string, htmlFills map[string]string, depth int) string {
	if depth > 8 {
		return inner
	}
	return elementHostPattern.ReplaceAllStringFunc(inner, func(tag string) string {
		matches := elementHostPattern.FindStringSubmatch(tag)
		if len(matches) < 2 {
			return tag
		}
		name := matches[1]
		if name == "route-view" {
			return tag
		}
		if _, ok := site.templates[name]; ok == false {
			return tag
		}
		body := renderElementTree(site, name, nil, nil, depth)
		return `<` + name + ` data-basic-web-server-rendered><template shadowrootmode="open">` + body + `</template></` + name + `>`
	})
}

var elementHostPattern = regexp.MustCompile(`<([a-z0-9]+(?:-[a-z0-9]+)+)(?:\s[^>]*)?></[a-z0-9]+(?:-[a-z0-9]+)+>`)

func insertRenderedRoute(index []byte, path string, pattern string, elementName string, renderedInner string, fills map[string]string) []byte {
	host := `<` + elementName + ` data-basic-web-server-rendered data-route-path="` + html.EscapeString(path) + `"`
	if pattern != "" {
		host += ` data-route-pattern="` + html.EscapeString(pattern) + `"`
	}
	// Unslotted light-DOM copy for crawlers that do not flatten declarative shadow DOM.
	fallback := ""
	if heading := fills["heading"]; heading != "" {
		fallback = "<h1>" + html.EscapeString(heading) + "</h1>"
		if lede := fills["lede"]; lede != "" {
			fallback += "<p>" + html.EscapeString(lede) + "</p>"
		}
	}
	host += `><template shadowrootmode="open">` + renderedInner + `</template>` + fallback + `</` + elementName + `>`
	replacement := `<route-view not-found="page-home" data-basic-web-server-rendered>` + host + `</route-view>`
	if routeViewPattern.Match(index) {
		return routeViewPattern.ReplaceAll(index, []byte(replacement))
	}
	return append(index, []byte(replacement)...)
}

type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []sitemapURL
}

type sitemapURL struct {
	XMLName xml.Name `xml:"url"`
	Loc     string   `xml:"loc"`
}

func sitemapXML(origin string, paths []string) []byte {
	document := urlset{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	seen := map[string]struct{}{}
	for _, path := range paths {
		path = normalizeRoutePattern(path)
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		document.URLs = append(document.URLs, sitemapURL{Loc: absoluteURL(origin, path)})
	}
	body, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return []byte(xml.Header + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`)
	}
	return append([]byte(xml.Header), body...)
}

func serveSitemap(w http.ResponseWriter, options SetupHttpMuxOptions) {
	origin := options.origin()
	if origin == "" {
		origin = "http://localhost"
	}
	paths := site.staticIndexPaths()
	if options.SitemapPaths != nil {
		paths = append(paths, options.SitemapPaths()...)
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", options.publicCacheControl())
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(sitemapXML(origin, paths))
}

func serveRouteDocument(w http.ResponseWriter, r *http.Request, files fs.FS, options SetupHttpMuxOptions) bool {
	if len(site.routes) == 0 {
		return false
	}
	route, params, matched := site.match(r.URL.Path)
	if matched == false {
		writeNotFound(w)
		return true
	}
	doc := Document{
		Title:       route.Title,
		Description: route.Description,
		Canonical:   normalizeRoutePattern(r.URL.Path),
		Index:       route.Index,
	}
	if options.Resolve != nil {
		if resolved, ok := options.Resolve(r, route, params); ok {
			doc = mergeDocument(doc, resolved)
		} else if route.Index && strings.Contains(route.Pattern, ":") {
			writeNotFound(w)
			return true
		}
	} else if route.Index && strings.Contains(route.Pattern, ":") {
		writeNotFound(w)
		return true
	}
	if doc.Status == http.StatusNotFound {
		writeNotFound(w)
		return true
	}
	if doc.Redirect != "" {
		status := doc.Status
		if status == 0 {
			status = http.StatusPermanentRedirect
		}
		http.Redirect(w, r, doc.Redirect, status)
		return true
	}
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		return false
	}
	inner := renderElementTree(&site, route.Element, doc.Fills, doc.HTMLFills, 0)
	index = insertRenderedRoute(index, normalizeRoutePattern(r.URL.Path), route.Pattern, route.Element, inner, doc.Fills)
	index = applyDocumentHead(index, doc, options.origin())

	manifestURL := "/framework/element-manifest.json"
	if options.WebVersion != "" && options.WebVersion != "dev" {
		manifestURL += "?v=" + url.QueryEscape(options.WebVersion)
	}
	w.Header().Set("Link", "<"+manifestURL+">; rel=preload; as=fetch; crossorigin")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if doc.Index {
		w.Header().Set("Cache-Control", options.publicCacheControl())
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(index)
	return true
}

func mergeDocument(base Document, overlay Document) Document {
	if overlay.Title != "" {
		base.Title = overlay.Title
	}
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if overlay.Canonical != "" {
		base.Canonical = overlay.Canonical
	}
	if overlay.Status != 0 {
		base.Status = overlay.Status
	}
	if overlay.Redirect != "" {
		base.Redirect = overlay.Redirect
	}
	if overlay.Fills != nil {
		base.Fills = overlay.Fills
	}
	if overlay.HTMLFills != nil {
		base.HTMLFills = overlay.HTMLFills
	}
	if overlay.Index {
		base.Index = true
	}
	if overlay.Status == http.StatusNotFound {
		base.Index = false
	}
	return base
}

func writeNotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprint(w, `<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"><title>Page not found</title><meta name="robots" content="noindex"></head><body><h1>Page not found</h1></body></html>`)
}
