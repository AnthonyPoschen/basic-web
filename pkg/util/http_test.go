package util

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestDefaultRobotsTXTAllowsAll(t *testing.T) {
	request := httptest.NewRequest("GET", "https://example.test/robots.txt", nil)

	got := defaultRobotsTXT(request, fstest.MapFS{})
	want := "User-agent: *\nAllow: /\n"
	if got != want {
		t.Fatalf("defaultRobotsTXT() = %q, want %q", got, want)
	}
}

func TestDefaultRobotsTXTIncludesExistingSitemap(t *testing.T) {
	request := httptest.NewRequest("GET", "https://example.test/robots.txt", nil)
	files := fstest.MapFS{
		"sitemap.xml": {},
	}

	got := defaultRobotsTXT(request, files)
	want := "User-agent: *\nAllow: /\n\nSitemap: https://example.test/sitemap.xml\n"
	if got != want {
		t.Fatalf("defaultRobotsTXT() = %q, want %q", got, want)
	}
}

func TestElementManifestIncludesLocalStaticModuleImports(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMux(mux, fstest.MapFS{
		"elements/pages/example.html": &fstest.MapFile{Data: []byte(`
<template id="page-example"></template>
<script type="module">
  import "/scripts/shared.js"
  import helper from "./helper.js"
  import "package-name"
  import("/scripts/dynamic.js")
  const card = document.createElement('game-card')
  customElements.define("page-example", class extends ShadowHTMLElement {})
</script>`)},
	})

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/framework/element-manifest.json", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("manifest response status = %d, want %d", response.Code, http.StatusOK)
	}

	var manifest struct {
		Dependencies  map[string][]string `json:"dependencies"`
		ModuleImports map[string][]string `json:"moduleImports"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	want := []string{"./helper.js", "/scripts/shared.js"}
	got := manifest.ModuleImports["page-example"]
	if len(got) != len(want) {
		t.Fatalf("module imports = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("module imports = %v, want %v", got, want)
		}
	}

	dependencies := manifest.Dependencies["page-example"]
	if len(dependencies) != 1 || dependencies[0] != "game-card" {
		t.Fatalf("dependencies = %v, want [game-card]", dependencies)
	}
}
