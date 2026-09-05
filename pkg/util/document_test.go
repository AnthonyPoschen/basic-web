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

	slash := httptest.NewRecorder()
	mux.ServeHTTP(slash, httptest.NewRequest(http.MethodGet, "/about/", nil))
	if slash.Code != http.StatusPermanentRedirect {
		t.Fatalf("/about/ status = %d", slash.Code)
	}
	if location := slash.Header().Get("Location"); location != "/about" {
		t.Fatalf("/about/ location = %q", location)
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

func TestApplyFillsMatchesMinifiedUnquotedAttributes(t *testing.T) {
	input := `<h1 id=heading data-fill=heading>Item</h1><nav data-fill-html=related hidden><span>none</span></nav>`
	got := applyFills(input, map[string]string{"heading": "Oak trees"}, map[string]string{"related": `<a href="/cedar">Cedar</a>`})
	if !strings.Contains(got, ">Oak trees</h1>") {
		t.Fatalf("unquoted text fill missing: %s", got)
	}
	if strings.Contains(got, " hidden") {
		t.Fatalf("related nav stayed hidden: %s", got)
	}
	if !strings.Contains(got, `href="/cedar"`) {
		t.Fatalf("unquoted html fill missing: %s", got)
	}
}

func TestApplyFillsPreservesDollarSigns(t *testing.T) {
	input := `<h1 data-fill="heading">Choose</h1><nav data-fill-html="related" hidden></nav><div id="plan-grid" data-fill-html="plans"></div>`
	got := applyFills(input, map[string]string{"heading": "Factorio"}, map[string]string{
		"related": `<a href="/z">Zomboid</a>`,
		"plans":   `<article><h3>Spark</h3><strong>$15</strong></article>`,
	})
	if !strings.Contains(got, "$15") {
		t.Fatalf("dollar amount was rewritten: %s", got)
	}
}

func TestSetupHttpMuxStampsOpenGraphFromDocument(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMuxWithOptions(mux, fstest.MapFS{
		"index.html": {Data: []byte(`<!DOCTYPE html><html><head><title>Shell</title></head><body><route-view></route-view></body></html>`)},
		"elements/pages/home.html": {Data: []byte(`
<template id="page-home" data-route="/" data-title="Garden home" data-description="Home gardens" data-index>
  <h1>Garden home</h1>
</template>
<script>customElements.define("page-home", class extends ShadowHTMLElement { constructor() { super("page-home") } })</script>`)},
	}, SetupHttpMuxOptions{
		SiteOrigin:        "https://example.test",
		SiteName:          "Garden",
		DefaultShareImage: "/images/share.jpg",
	})

	home := httptest.NewRecorder()
	mux.ServeHTTP(home, httptest.NewRequest(http.MethodGet, "/", nil))
	body := home.Body.String()
	for _, want := range []string{
		`property="og:title" content="Garden home"`,
		`property="og:description" content="Home gardens"`,
		`property="og:url" content="https://example.test/"`,
		`property="og:site_name" content="Garden"`,
		`property="og:image" content="https://example.test/images/share.jpg"`,
		`name="twitter:card" content="summary_large_image"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %s in %s", want, body)
		}
	}
}

func TestSetupHttpMuxExpandsParameterizedSitemapPaths(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMuxWithOptions(mux, fstest.MapFS{
		"index.html": {Data: []byte(`<!DOCTYPE html><html><head><title>Shell</title></head><body><route-view></route-view></body></html>`)},
		"elements/pages/plans.html": {Data: []byte(`
<template id="page-plans" data-route="/plans" data-routes="/plans/:game/:region" data-title="Plans" data-index>
  <h1>Plans</h1>
</template>
<script>customElements.define("page-plans", class extends ShadowHTMLElement { constructor() { super("page-plans") } })</script>`)},
	}, SetupHttpMuxOptions{
		SiteOrigin: "https://example.test",
		ExpandRoute: func(route Route) []map[string]string {
			if route.Pattern != "/plans/:game/:region" {
				return nil
			}
			return []map[string]string{
				{"game": "factorio", "region": "australia"},
				{"game": "project-zomboid", "region": "australia"},
			}
		},
	})

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("sitemap status = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"https://example.test/plans",
		"https://example.test/plans/factorio/australia",
		"https://example.test/plans/project-zomboid/australia",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("sitemap missing %s: %s", want, body)
		}
	}
	if strings.Contains(body, "/plans/:game") {
		t.Fatalf("sitemap listed a parameterized pattern: %s", body)
	}
}
