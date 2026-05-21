package util

import (
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
