package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/AnthonyPoschen/basic-web.git/pkg/memfs"
	"github.com/AnthonyPoschen/basic-web.git/pkg/util"
	"github.com/CAFxX/httpcompression"
)

//go:embed web/*
var embeddedFS embed.FS
var webFS fs.FS

func init() {
	if util.IsDev() {
		webFS = os.DirFS("./web")
	} else {
		tmpfs, _ := fs.Sub(embeddedFS, "web")
		webFS = memfs.CreateMinifiedFS(tmpfs)
	}
	util.SetupLogger()
}

func main() {
	if util.IsDev() {
		http.HandleFunc("/dev/reload", util.HotReloadHandler)
	}
	compress, _ := httpcompression.DefaultAdapter()

	// include API's / other websites above this. this is a fallback catch all
	http.Handle("/", compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if util.IsDev() {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, webFS, "index.html")
			return
		}
		http.FileServer(http.FS(webFS)).ServeHTTP(w, r)
	})))
	port := "42069"
	slog.Info("Server listening", "port", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		slog.Error("server failed to listen and serve", "error", err.Error())
	}
}
