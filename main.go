package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/AnthonyPoschen/basic-web.git/pkg/memfs"
	"github.com/AnthonyPoschen/basic-web.git/pkg/util"
	"github.com/CAFxX/httpcompression"
)

//go:embed web/*
var embeddedFS embed.FS
var webFS fs.FS
var componentManifest []byte

const componentManifestPath = "/component-manifest.json"

var componentDefinitionPattern = regexp.MustCompile(`customElements\.define\(\s*['"]([a-z0-9]+(?:-[a-z0-9]+)+)['"]`)

func init() {
	if util.IsDev() {
		webFS = os.DirFS("./web")
	} else {
		tmpfs, _ := fs.Sub(embeddedFS, "web")
		webFS = memfs.CreateMinifiedFS(tmpfs)
	}
	util.SetupLogger()
	if !util.IsDev() {
		var err error
		componentManifest, err = buildComponentManifest(webFS)
		if err != nil {
			panic(fmt.Errorf("build component manifest: %w", err))
		}
	}
}

func main() {
	if util.IsDev() {
		http.HandleFunc("/dev/reload", util.HotReloadHandler)
	}
	http.HandleFunc(componentManifestPath, componentManifestHandler)
	compress, _ := httpcompression.DefaultAdapter()
	fileServer := http.FileServer(http.FS(webFS))

	// include API's / other websites above this. this is a fallback catch all
	http.Handle("/", compress(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if util.IsDev() {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if shouldServeIndex(r.URL.Path) {
			http.ServeFileFS(w, r, webFS, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})))
	port := "42069"
	slog.Info("Server listening", "port", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		slog.Error("server failed to listen and serve", "error", err.Error())
	}
}

func shouldServeIndex(requestPath string) bool {
	if requestPath == "/" {
		return true
	}

	cleanPath := path.Clean(strings.TrimPrefix(requestPath, "/"))
	if cleanPath == "." || cleanPath == "" {
		return true
	}

	if _, err := fs.Stat(webFS, cleanPath); err == nil {
		return false
	}

	return !strings.Contains(path.Base(cleanPath), ".")
}

func componentManifestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	manifest, err := getComponentManifest()
	if err != nil {
		slog.Error("failed to build component manifest", "error", err.Error())
		http.Error(w, "failed to build component manifest", http.StatusInternalServerError)
		return
	}

	if util.IsDev() {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(manifest)
}

func getComponentManifest() ([]byte, error) {
	if util.IsDev() {
		return buildComponentManifest(webFS)
	}
	return componentManifest, nil
}

func buildComponentManifest(filesystem fs.FS) ([]byte, error) {
	if _, err := fs.Stat(filesystem, "component"); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []byte("{}"), nil
		}
		return nil, err
	}

	manifest := map[string]string{}

	err := fs.WalkDir(filesystem, "component", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(filePath, ".html") {
			return nil
		}

		contents, err := fs.ReadFile(filesystem, filePath)
		if err != nil {
			return err
		}

		matches := componentDefinitionPattern.FindAllSubmatch(contents, -1)
		if len(matches) == 0 {
			return nil
		}

		relativePath := strings.TrimPrefix(filePath, "component/")
		for _, match := range matches {
			name := string(match[1])
			if existingPath, ok := manifest[name]; ok && existingPath != relativePath {
				return fmt.Errorf("component %q defined in both %q and %q", name, existingPath, relativePath)
			}
			manifest[name] = relativePath
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(manifest))
	for key := range manifest {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	buffer := bytes.NewBufferString("{")
	for index, key := range keys {
		if index > 0 {
			buffer.WriteByte(',')
		}

		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		encodedPath, err := json.Marshal(manifest[key])
		if err != nil {
			return nil, err
		}

		buffer.Write(encodedKey)
		buffer.WriteByte(':')
		buffer.Write(encodedPath)
	}
	buffer.WriteByte('}')

	return buffer.Bytes(), nil
}
