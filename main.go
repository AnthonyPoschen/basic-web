package main

import (
	"embed"
	"fmt"
	"html"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/AnthonyPoschen/basic-web/pkg/memfs"
	"github.com/AnthonyPoschen/basic-web/pkg/util"
)

//go:embed web/*
var embeddedFS embed.FS

func init() {
	var webFS fs.FS
	if util.IsDev() {
		webFS = os.DirFS("./web")
	} else {
		tmpfs, _ := fs.Sub(embeddedFS, "web")
		webFS = memfs.CreateMinifiedFS(tmpfs)
	}
	util.SetupLogger()
	// this returns a pre configured handler for future requests
	// automatically handling all filesystem requests
	// dev hot reloading, lazy loading etc
	util.SetupHttpMux(http.DefaultServeMux, webFS)
}

func main() {
	// include API's / other websites above this. this is a fallback catch all
	http.Handle("/api/items", util.Middleware(util.CompressFunc(itemsHandler)))

	port := "42069"
	slog.Info("Server listening", "port", port)
	err := http.ListenAndServe(":"+port, http.DefaultServeMux)
	if err != nil {
		slog.Error("server failed to listen and serve", "error", err.Error())
	}
}

func itemsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	items := []struct {
		ID          string
		Description string
	}{
		{ID: "abc", Description: "First sample item"},
		{ID: "def", Description: "Second sample item"},
		{ID: "ghi", Description: "Third sample item"},
	}

	var buffer strings.Builder
	for _, item := range items {
		tmpl := `<li><a href="/items/%s">%s</a></li>`
		escapedID := html.EscapeString(item.ID)
		escapedDescription := html.EscapeString(item.Description)
		buffer.WriteString(fmt.Sprintf(tmpl, escapedID, escapedDescription))
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(buffer.String()))
}

