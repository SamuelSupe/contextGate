package ui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var assets embed.FS

func Handler() http.Handler {
	root, _ := fs.Sub(assets, "dist")
	files := http.FileServer(http.FS(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p == "" {
			p = "index.html"
		}
		if _, e := fs.Stat(root, p); e != nil {
			if strings.HasPrefix(p, "assets/") {
				w.Header().Set("Cache-Control", "no-store")
				http.NotFound(w, r)
				return
			}
			clone := r.Clone(r.Context())
			u := *r.URL
			u.Path = "/"
			clone.URL = &u
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, clone)
			return
		}
		if strings.HasPrefix(p, "assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
