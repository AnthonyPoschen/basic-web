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
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2"
	jsparser "github.com/tdewolff/parse/v2/js"
)

var elementManifest []byte
var elementDefinitionPattern = regexp.MustCompile(`customElements\.define\(\s*['"]([a-z0-9]+(?:-[a-z0-9]+)+)['"]`)
var elementTemplatePattern = regexp.MustCompile(`(?is)<template\b[^>]*>(.*?)</template>`)
var elementTagPattern = regexp.MustCompile(`<([a-z0-9]+(?:-[a-z0-9]+)+)(?:\s|/?>)`)
var elementCreatePattern = regexp.MustCompile(`(?i)\bdocument\.createElement\(\s*['"]([a-z0-9]+(?:-[a-z0-9]+)+)['"]\s*\)`)
var elementScriptPattern = regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
var moduleScriptTypePattern = regexp.MustCompile(`(?i)(?:^|\s)type\s*=\s*(?:"module"|'module'|module)(?:\s|$)`)
var files fs.FS

//go:embed js/loader.js
var js_loader []byte

//go:embed js/router.js
var js_router []byte

//go:embed js/utils.js
var js_utils []byte

// One runtime file avoids three render-blocking classic scripts. See docs/performance.md.
var js_basic_web = bytes.Join([][]byte{
	js_utils,
	[]byte("\n;\n"),
	js_loader,
	[]byte("\n;\n"),
	js_router,
}, nil)

// SetupHttpMuxOptions configures cache behavior for framework resources.
type SetupHttpMuxOptions struct {
	WebVersion string
}

func framework(webVersion string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if IsDev() {
			w.Header().Set("Cache-Control", "no-cache")
		}
		var err error
		switch r.URL.Path {
		case "/framework/element-manifest.json":
			if webVersion != "" && webVersion != "dev" && r.URL.Query().Get("v") == webVersion {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.Header().Set("Content-Type", "application/json")
			_, err = w.Write(elementManifest)
		case "/framework/basic-web.js":
			// Versioned URLs are content-addressed by WebVersion. A 1-day default
			// cache here revalidates bytes the query already busts. See docs/performance.md.
			if webVersion != "" && webVersion != "dev" && r.URL.Query().Get("v") == webVersion {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
			_, err = w.Write(js_basic_web)
		case "/framework/styles.css":
			var data []byte
			data, err = fs.ReadFile(files, bundledStylesheetPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					http.NotFound(w, r)
					return
				}
				break
			}
			if webVersion != "" && webVersion != "dev" && r.URL.Query().Get("v") == webVersion {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
			_, err = w.Write(data)
		default:
			http.NotFound(w, r)
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			slog.Error("Failed to fetching framework resource", "err", err.Error())
		}
	}
}

func SetupHttpMux(mux *http.ServeMux, filesystem fs.FS) {
	SetupHttpMuxWithOptions(mux, filesystem, SetupHttpMuxOptions{})
}

// SetupHttpMuxWithOptions registers framework and static-file handlers.
func SetupHttpMuxWithOptions(mux *http.ServeMux, filesystem fs.FS, options SetupHttpMuxOptions) {
	// Combine local index.html stylesheets for the browser; keep source files split.
	files = WithBundledStylesheets(filesystem)
	// build initial manifest once we know the filesystem
	var err error
	elementManifest, err = buildElementManifest()
	if err != nil {
		panic(err)
	}
	// add hot reloading if dev
	if IsDev() {
		mux.Handle("/dev/reload", SSEFunc(HotReloadHandler))
	}
	// add element manifest
	mux.Handle("/framework/", Middleware(CompressHandler(framework(options.WebVersion))))
	// mux.Handle("/element-manifest.json", Middleware(CompressFunc(componentManifestHandler)))

	//add default http file server
	mux.Handle("/", Middleware(CompressFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			serveRobotsTXT(w, r, files)
			return
		}

		ok, err := shouldServeIndex(r.URL.Path, files)
		if err != nil {
			if isAssetRequest(r.URL.Path) {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/", http.StatusPermanentRedirect)
			return
		}
		if ok {
			manifestURL := "/framework/element-manifest.json"
			if options.WebVersion != "" && options.WebVersion != "dev" {
				manifestURL += "?v=" + url.QueryEscape(options.WebVersion)
			}
			// Start the manifest before basic-web.js finishes. The URL must match
			// the loader request, including ?v=. See docs/performance.md.
			w.Header().Set("Link", "<"+manifestURL+">; rel=preload; as=fetch; crossorigin")
			http.ServeFileFS(w, r, files, "index.html")
			return
		}
		http.ServeFileFS(w, r, files, r.URL.Path)
	})))
}

func elementManifestHandler(w http.ResponseWriter, r *http.Request) {
}

func getelementManifest() ([]byte, error) {
	return elementManifest, nil
}

func serveRobotsTXT(w http.ResponseWriter, r *http.Request, files fs.FS) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if _, err := fs.Stat(files, "robots.txt"); err == nil {
		http.ServeFileFS(w, r, files, "robots.txt")
		return
	} else if !errors.Is(err, fs.ErrNotExist) {
		http.Error(w, "failed to serve robots.txt", http.StatusInternalServerError)
		return
	}

	_, err := w.Write([]byte(defaultRobotsTXT(r, files)))
	if err != nil {
		slog.Error("Failed to serve default robots.txt", "err", err.Error())
	}
}

func defaultRobotsTXT(r *http.Request, files fs.FS) string {
	robots := "User-agent: *\nAllow: /\n"
	if _, err := fs.Stat(files, "sitemap.xml"); err == nil {
		robots += "\nSitemap: " + requestOrigin(r) + "/sitemap.xml\n"
	}
	return robots
}

func requestOrigin(r *http.Request) string {
	scheme := r.Header.Get("X-Forwarded-Proto")
	if scheme != "http" && scheme != "https" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	return scheme + "://" + r.Host
}

func buildElementManifest() ([]byte, error) {
	if _, err := fs.Stat(files, "elements"); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []byte("{}"), nil
		}
		return nil, err
	}

	manifest := map[string]string{}
	dependencies := map[string][]string{}
	moduleImports := map[string][]string{}

	err := fs.WalkDir(files, "elements", func(filePath string, entry fs.DirEntry, err error) error {
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

		matches := elementDefinitionPattern.FindAllSubmatch(contents, -1)
		if len(matches) == 0 {
			return nil
		}

		relativePath := strings.TrimPrefix(filePath, "elements/")
		fileDependencies := map[string]struct{}{}
		for _, templateMatch := range elementTemplatePattern.FindAllSubmatch(contents, -1) {
			for _, tagMatch := range elementTagPattern.FindAllSubmatch(templateMatch[1], -1) {
				fileDependencies[string(tagMatch[1])] = struct{}{}
			}
		}
		for _, createMatch := range elementCreatePattern.FindAllSubmatch(contents, -1) {
			fileDependencies[string(createMatch[1])] = struct{}{}
		}
		fileModuleImports := staticModuleImports(contents)

		for _, match := range matches {
			name := string(match[1])
			if existingPath, ok := manifest[name]; ok && existingPath != relativePath {
				return fmt.Errorf("element %q defined in both %q and %q", name, existingPath, relativePath)
			}
			manifest[name] = relativePath

			dependencyNames := make([]string, 0, len(fileDependencies))
			for dependency := range fileDependencies {
				if dependency != name {
					dependencyNames = append(dependencyNames, dependency)
				}
			}
			sort.Strings(dependencyNames)
			dependencies[name] = dependencyNames
			if len(fileModuleImports) > 0 {
				moduleImports[name] = fileModuleImports
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	manifestDocument := struct {
		Elements      map[string]string   `json:"elements"`
		Dependencies  map[string][]string `json:"dependencies"`
		ModuleImports map[string][]string `json:"moduleImports"`
	}{
		Elements:      manifest,
		Dependencies:  dependencies,
		ModuleImports: moduleImports,
	}

	return json.Marshal(manifestDocument)
}

func staticModuleImports(contents []byte) []string {
	imports := map[string]struct{}{}
	for _, scriptMatch := range elementScriptPattern.FindAllSubmatch(contents, -1) {
		if moduleScriptTypePattern.Match(scriptMatch[1]) == false {
			continue
		}

		ast, err := jsparser.Parse(parse.NewInputBytes(scriptMatch[2]), jsparser.Options{})
		if err != nil {
			continue
		}
		for _, statement := range ast.List {
			importStatement, ok := statement.(*jsparser.ImportStmt)
			if ok == false {
				continue
			}
			modulePath, err := strconv.Unquote(string(importStatement.Module))
			if err != nil || isLocalModuleImport(modulePath) == false {
				continue
			}
			imports[modulePath] = struct{}{}
		}
	}

	modulePaths := make([]string, 0, len(imports))
	for modulePath := range imports {
		modulePaths = append(modulePaths, modulePath)
	}
	sort.Strings(modulePaths)
	return modulePaths
}

func isLocalModuleImport(modulePath string) bool {
	return (strings.HasPrefix(modulePath, "/") && strings.HasPrefix(modulePath, "//") == false) ||
		strings.HasPrefix(modulePath, "./") ||
		strings.HasPrefix(modulePath, "../")
}
func shouldServeIndex(requestPath string, files fs.FS) (bool, error) {
	if requestPath == "/" {
		return true, nil
	}

	cleanPath := path.Clean(strings.TrimPrefix(requestPath, "/"))
	if cleanPath == "." || cleanPath == "" {
		return true, nil
	}

	if _, err := fs.Stat(files, cleanPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) && !isAssetRequest(requestPath) {
			return true, nil
		}
		return false, err
	}

	return !strings.Contains(path.Base(cleanPath), "."), nil
}

func isAssetRequest(requestPath string) bool {
	return strings.Contains(path.Base(path.Clean(requestPath)), ".")
}
