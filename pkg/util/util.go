package util

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

var isDev = false

func init() {
	if strings.Contains(os.Args[0], "go-build") {
		isDev = true
	}

}
func IsDev() bool {
	return isDev
}

func SetupLogger() {
	var handler slog.Handler
	if IsDev() {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	slog.SetDefault(slog.New(handler))

}

// HotReloadHandler enables client side hot reloading
func HotReloadHandler(w http.ResponseWriter, r *http.Request) {
	l := slog.With("Handler", "HotReload", "client", r.RemoteAddr)
	l.Debug("client connected")
	lastChanged := time.Now()
	for {
		select {
		case <-r.Context().Done():
			l.Debug("Closing connection")
			return
		case <-time.After(time.Millisecond * 50):
			var changed bool
			fs.WalkDir(os.DirFS("./web"), ".", func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				if info.ModTime().After(lastChanged) {
					changed = true
					return fs.SkipDir
				}
				return nil
			})
			if changed {
				var err error
				componentManifest, err = buildComponentManifest()
				if err != nil {
					slog.Error("Failed to update Components", "err", err.Error())
					return
				}
				fmt.Fprintf(w, "data: reload\n\n")
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
					l.Debug("sent hot reload event")
				}
				lastChanged = time.Now()
			}
		}
	}
}
