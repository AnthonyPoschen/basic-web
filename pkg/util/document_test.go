package util

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestSetupHttpMuxServesUniqueFirstHTMLPerRoute(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMuxWithOptions(mux, fstest.MapFS{
		"index.html": {Data: []byte(`<!DOCTYPE html><html><head><title data-basic-web-title>Shell</title><meta name="description" content="shell"></head><body><route-view not-found="page-home"></route-view></body></html>`)},
		"elements/pages/home.html": {Data: []byte(`
<template id="page-home" data-route="/" data-title="Garden home" data-description="Home gardens" data-index>
  <h1>Garden home</h1>
  <p>Welcome to the garden.</p>
  <a href="/about">About</a>
</template>
<script>customElements.define("page-home", class extends ShadowHTMLElement { constructor() { super("page-home") } })</script>`)},
		"elements/pages/about.html": {Data: []byte(`
<template id="page-about" data-route="/about" data-title="About the garden" data-description="Our gardeners" data-index>
  <h1>About the garden</h1>
  <p>We garden in Sydney.</p>
  <a href="/">Home</a>
</template>
<script>customElements.define("page-about", class extends ShadowHTMLElement { constructor() { super("page-about") } })</script>`)},
	}, SetupHttpMuxOptions{SiteOrigin: "https://example.test"})

	home := httptest.NewRecorder()
	mux.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	about := httptest.NewRecorder()
	mux.ServeHTTP(about, httptest.NewRequest(http.MethodGet, "/about", nil))

	if home.Code != http.StatusOK || about.Code != http.StatusOK {
		t.Fatalf("status home=%d about=%d", home.Code, about.Code)
	}
	homeBody := home.Body.String()
	aboutBody := about.Body.String()
	if !strings.Contains(homeBody, "<title>Garden home</title>") || strings.Contains(homeBody, "<title>Shell</title>") {
		t.Fatalf("home title missing or duplicate shell title: %s", homeBody)
	}
	if !strings.Contains(aboutBody, "<title>About the garden</title>") {
		t.Fatalf("about title missing: %s", aboutBody)
	}
	if strings.Contains(homeBody, "<title>About the garden</title>") || homeBody == aboutBody {
		t.Fatal("routes returned the same first HTML")
	}
	if !strings.Contains(homeBody, "Welcome to the garden.") || !strings.Contains(aboutBody, "We garden in Sydney.") {
		t.Fatal("route bodies missing unique copy")
	}
	if !strings.Contains(homeBody, `href="https://example.test/"`) && !strings.Contains(homeBody, `href="https://example.test/"`) {
		if !strings.Contains(homeBody, `rel="canonical"`) {
			t.Fatal("home canonical missing")
		}
	}
	if !strings.Contains(aboutBody, "https://example.test/about") {
		t.Fatalf("about canonical missing: %s", aboutBody)
	}
	if !strings.Contains(homeBody, `shadowrootmode="open"`) || !strings.Contains(homeBody, "<page-home") {
		t.Fatal("home missing declarative page tree")
	}
	if home.Header().Get("Cache-Control") != DefaultPublicHTMLCacheControl {
		t.Fatalf("home cache = %q", home.Header().Get("Cache-Control"))
	}

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown path status = %d", missing.Code)
	}
}

func TestSetupHttpMuxResolvesIndexedParamRoutes(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMuxWithOptions(mux, fstest.MapFS{
		"index.html": {Data: []byte(`<!DOCTYPE html><html><head><title>Shell</title></head><body><route-view></route-view></body></html>`)},
		"elements/pages/item.html": {Data: []byte(`
<template id="page-item" data-route="/items/:id" data-title="Item" data-index>
  <h1 data-fill="heading">Item</h1>
  <p data-fill="lede">Details</p>
</template>
<script>customElements.define("page-item", class extends ShadowHTMLElement { constructor() { super("page-item") } })</script>`)},
	}, SetupHttpMuxOptions{
		SiteOrigin: "https://example.test",
		Resolve: func(r *http.Request, route Route, params map[string]string) (Document, bool) {
			if params["id"] != "oak" {
				return Document{Status: http.StatusNotFound}, true
			}
			return Document{
				Title:       "Oak trees",
				Description: "An oak in the garden",
				Canonical:   r.URL.Path,
				Index:       true,
				Fills:       map[string]string{"heading": "Oak trees", "lede": "A long-lived oak."},
			}, true
		},
	})

	ok := httptest.NewRecorder()
	mux.ServeHTTP(ok, httptest.NewRequest(http.MethodGet, "/items/oak", nil))
	if ok.Code != http.StatusOK || !strings.Contains(ok.Body.String(), "<title>Oak trees</title>") {
		t.Fatalf("oak status=%d body=%s", ok.Code, ok.Body.String())
	}
	if !strings.Contains(ok.Body.String(), "A long-lived oak.") {
		t.Fatalf("oak fill missing: %s", ok.Body.String())
	}

	missing := httptest.NewRecorder()
	mux.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/items/missing", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing item status = %d", missing.Code)
	}
}
