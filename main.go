package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/AnthonyPoschen/basic-web.git/pkg/memfs"
	"github.com/CAFxX/httpcompression"
)

//go:embed web/*
var embeddedFS embed.FS

func main() {
	var binFS fs.FS
	// if this is a go run instance
	if strings.Contains(os.Args[0], "go-build") {
		fmt.Println("go run")
		binFS = os.DirFS("./web")
	} else {
		fmt.Println("built binary")
		webFS, _ := fs.Sub(embeddedFS, "web")
		binFS = memfs.CreateMinifiedFS(webFS)
	}
	compress, _ := httpcompression.DefaultAdapter()
	http.Handle("/", compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, binFS, "index.html")
			return
		}
		http.FileServer(http.FS(binFS)).ServeHTTP(w, r)
	})))
	http.ListenAndServe(":8080", nil)
}
