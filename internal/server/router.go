package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"codex-register/internal/config"
	"codex-register/web"
)

type Router struct {
	api    http.Handler
	static http.Handler
	distFS fs.FS
}

func NewRouter(cfg config.Settings) http.Handler {
	distFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}

	return &Router{
		api:    apiMux(cfg),
		static: http.FileServer(http.FS(distFS)),
		distFS: distFS,
	}
}

func NewRouterWithAPI(cfg config.Settings, api http.Handler) http.Handler {
	distFS, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		panic(err)
	}

	return &Router{
		api:    api,
		static: http.FileServer(http.FS(distFS)),
		distFS: distFS,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	cleanPath := path.Clean("/" + req.URL.Path)

	switch {
	case strings.HasPrefix(cleanPath, "/api/"):
		r.api.ServeHTTP(w, req)
		return
	case strings.HasPrefix(cleanPath, "/ws/"):
		r.api.ServeHTTP(w, req)
		return
	case cleanPath == "/api":
		http.Redirect(w, req, "/api/", http.StatusPermanentRedirect)
		return
	}

	assetPath := strings.TrimPrefix(cleanPath, "/")
	if assetPath == "" || assetPath == "." {
		r.serveIndex(w, req)
		return
	}

	if entry, err := fs.Stat(r.distFS, assetPath); err == nil && !entry.IsDir() {
		r.static.ServeHTTP(w, req)
		return
	}

	r.serveIndex(w, req)
}

func (r *Router) serveIndex(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeFileFS(w, req, r.distFS, "index.html")
}
