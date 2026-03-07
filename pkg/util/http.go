package util

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/CAFxX/httpcompression"
)

var componentManifest []byte
var componentDefinitionPattern = regexp.MustCompile(`customElements\.define\(\s*['"]([a-z0-9]+(?:-[a-z0-9]+)+)['"]`)
var files fs.FS
var CompressHandler func(http.Handler) http.Handler

//go:embed js/loader.js
var js_loader []byte

//go:embed js/router.js
var js_router []byte

//go:embed js/utils.js
var js_utils []byte

func framework(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if IsDev() {
		w.Header().Set("Cache-Control", "no-cache")
	}
	w.Header().Set("Content-Type", "application/json")
	var err error
	switch r.URL.Path {
	case "/framework/component-manifest.json":
		_, err = w.Write(componentManifest)
	case "/framework/loader.js":
		_, err = w.Write(js_loader)
	case "/framework/router.js":
		_, err = w.Write(js_router)
	case "/framework/utils.js":
		_, err = w.Write(js_utils)
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("Failed to fetching framework resource", "err", err.Error())
	}

}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if IsDev() {
			w.Header().Set("Cache-Control", "no-cache")
		} else {
			// 1 day cache expiry
			w.Header().Set("Cache-Control", "max-age=86400")
		}
		next.ServeHTTP(w, r)
	})
}

func CompressFunc(f func(w http.ResponseWriter, r *http.Request)) http.Handler {
	return CompressHandler(http.HandlerFunc(f))
}
func init() {
	CompressHandler, _ = httpcompression.DefaultAdapter()
}

func SetupHttpMux(mux *http.ServeMux, filesystem fs.FS) {
	files = filesystem
	// build initial manifest once we know the filesystem
	var err error
	componentManifest, err = buildComponentManifest()
	if err != nil {
		panic(err)
	}
	// add hot reloading if dev
	if IsDev() {
		mux.HandleFunc("/dev/reload", HotReloadHandler)
	}
	// add component manifest
	mux.Handle("/framework/", Middleware(CompressFunc(framework)))
	// mux.Handle("/component-manifest.json", Middleware(CompressFunc(componentManifestHandler)))

	//add default http file server
	mux.Handle("/", CompressFunc(func(w http.ResponseWriter, r *http.Request) {
		if shouldServeIndex(r.URL.Path, files) {
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		http.ServeFileFS(w, r, files, r.URL.Path)
	}))
}

func componentManifestHandler(w http.ResponseWriter, r *http.Request) {
}

func getComponentManifest() ([]byte, error) {
	return componentManifest, nil
}

func buildComponentManifest() ([]byte, error) {
	if _, err := fs.Stat(files, "component"); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []byte("{}"), nil
		}
		return nil, err
	}

	manifest := map[string]string{}

	err := fs.WalkDir(files, "component", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(filePath, ".html") {
			return nil
		}

		contents, err := fs.ReadFile(files, filePath)
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
func shouldServeIndex(requestPath string, files fs.FS) bool {
	if requestPath == "/" {
		return true
	}

	cleanPath := path.Clean(strings.TrimPrefix(requestPath, "/"))
	if cleanPath == "." || cleanPath == "" {
		return true
	}

	if _, err := fs.Stat(files, cleanPath); err == nil {
		return false
	}

	return !strings.Contains(path.Base(cleanPath), ".")
}
