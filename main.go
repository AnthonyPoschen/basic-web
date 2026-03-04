package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

//go:embed web/*
var embeddedFS embed.FS

func main() {
	var binFS fs.FS
	// if this is a go run isntance
	if strings.Contains(os.Args[0], "go-build") {
		fmt.Println("go run")
		binFS = os.DirFS("./web")
	} else {
		fmt.Println("built binary")
		binFS, _ = fs.Sub(embeddedFS, "web")
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFileFS(w, r, binFS, "index.html")
			return
		}
		http.FileServer(http.FS(binFS)).ServeHTTP(w, r)
	})
	http.ListenAndServe(":8080", nil)
}
