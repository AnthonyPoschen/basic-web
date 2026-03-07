package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/AnthonyPoschen/basic-web.git/pkg/memfs"
	"github.com/AnthonyPoschen/basic-web.git/pkg/util"
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
	http.Handle("/foo", util.CompressFunc(foo))
	port := "42069"
	slog.Info("Server listening", "port", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		slog.Error("server failed to listen and serve", "error", err.Error())
	}
}

func foo(w http.ResponseWriter, r *http.Request) {}
