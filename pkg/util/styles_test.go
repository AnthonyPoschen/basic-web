package util

import (
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestWithBundledStylesheetsCombinesLocalStylesheets(t *testing.T) {
	files := WithBundledStylesheets(fstest.MapFS{
		"index.html":   &fstest.MapFile{Data: []byte(`<!doctype html><link rel="icon" href="/favicon.ico"><link rel="stylesheet" href="/styles/a.css?v=rel"><link rel="stylesheet" href="./b.css"><link rel="stylesheet" href="https://cdn.example/x.css">`)},
		"styles/a.css": &fstest.MapFile{Data: []byte(`@font-face{src:url("../fonts/a.woff2")}`)},
		"b.css":        &fstest.MapFile{Data: []byte(`body{background:url("bg.png")}`)},
	})

	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	got := string(index)
	if strings.Count(got, `rel="stylesheet"`) != 2 {
		t.Fatalf("stylesheet links = %q", got)
	}
	if strings.Contains(got, `href="/framework/styles.css?v=rel"`) == false {
		t.Fatalf("missing bundled stylesheet, got %q", got)
	}
	if strings.Contains(got, `href="https://cdn.example/x.css"`) == false {
		t.Fatalf("external stylesheet was dropped, got %q", got)
	}
	if strings.Contains(got, "/styles/a.css") || strings.Contains(got, "./b.css") {
		t.Fatalf("source stylesheet links remained, got %q", got)
	}

	css, err := fs.ReadFile(files, "framework/styles.css")
	if err != nil {
		t.Fatal(err)
	}
	gotCSS := string(css)
	if strings.Contains(gotCSS, `url("/fonts/a.woff2")`) == false {
		t.Fatalf("relative font url was not rewritten, got %q", gotCSS)
	}
	if strings.Contains(gotCSS, `url("/bg.png")`) == false {
		t.Fatalf("expected /bg.png from b.css, got %q", gotCSS)
	}
}

func TestWithBundledStylesheetsLeavesSingleStylesheet(t *testing.T) {
	files := WithBundledStylesheets(fstest.MapFS{
		"index.html":          &fstest.MapFile{Data: []byte(`<link rel="stylesheet" href="/theme.css">`)},
		"theme.css":           &fstest.MapFile{Data: []byte(`body{}`)},
		"framework/other.txt": &fstest.MapFile{Data: []byte(`keep`)},
	})

	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if string(index) != `<link rel="stylesheet" href="/theme.css">` {
		t.Fatalf("single stylesheet rewritten: %q", index)
	}
	if _, err := fs.ReadFile(files, "framework/styles.css"); err == nil || errors.Is(err, fs.ErrNotExist) == false {
		t.Fatalf("bundle open error = %v", err)
	}
}

func TestSetupHttpMuxServesBundledStylesAndVersionedRuntime(t *testing.T) {
	mux := http.NewServeMux()
	SetupHttpMuxWithOptions(mux, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte(`<link rel="stylesheet" href="/a.css?v=rel-9"><link rel=stylesheet href=/b.css?v=rel-9>`)},
		"a.css":      &fstest.MapFile{Data: []byte(`a{color:red}`)},
		"b.css":      &fstest.MapFile{Data: []byte(`b{color:blue}`)},
	}, SetupHttpMuxOptions{WebVersion: "rel-9"})

	index := httptest.NewRecorder()
	mux.ServeHTTP(index, httptest.NewRequest(http.MethodGet, "/", nil))
	if index.Code != http.StatusOK {
		t.Fatalf("index status = %d", index.Code)
	}
	body, _ := io.ReadAll(index.Result().Body)
	if strings.Contains(string(body), `href="/framework/styles.css?v=rel-9"`) == false {
		t.Fatalf("index missing bundle link: %s", body)
	}

	css := httptest.NewRecorder()
	mux.ServeHTTP(css, httptest.NewRequest(http.MethodGet, "/framework/styles.css?v=rel-9", nil))
	if css.Code != http.StatusOK {
		t.Fatalf("bundle status = %d body=%s", css.Code, css.Body.String())
	}
	if css.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("bundle cache = %q", css.Header().Get("Cache-Control"))
	}
	if css.Body.String() != "a{color:red}\nb{color:blue}" {
		t.Fatalf("bundle css = %q", css.Body.String())
	}

	runtime := httptest.NewRecorder()
	mux.ServeHTTP(runtime, httptest.NewRequest(http.MethodGet, "/framework/basic-web.js?v=rel-9", nil))
	if runtime.Header().Get("Cache-Control") != "public, max-age=31536000, immutable" {
		t.Fatalf("runtime cache = %q", runtime.Header().Get("Cache-Control"))
	}

	stale := httptest.NewRecorder()
	mux.ServeHTTP(stale, httptest.NewRequest(http.MethodGet, "/framework/basic-web.js", nil))
	if stale.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("unversioned runtime cache = %q", stale.Header().Get("Cache-Control"))
	}
}
