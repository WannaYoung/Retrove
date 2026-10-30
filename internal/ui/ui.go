package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distRoot embed.FS

// Dist returns the built admin UI filesystem when index.html is present.
func Dist() (fs.FS, bool) {
	sub, err := fs.Sub(distRoot, "dist")
	if err != nil {
		return nil, false
	}
	if _, err := sub.Open("index.html"); err != nil {
		return nil, false
	}
	return sub, true
}

// Handler serves the Vue SPA. Unknown paths fall back to index.html.
func Handler(uiFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(uiFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			serveIndex(w, r, uiFS)
			return
		}
		if f, err := uiFS.Open(path); err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, r)
			return
		}
		// history-mode SPA fallback
		if strings.Contains(path, ".") {
			http.NotFound(w, r)
			return
		}
		serveIndex(w, r, uiFS)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, uiFS fs.FS) {
	b, err := fs.ReadFile(uiFS, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(b)
}
