package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"
)

//go:embed dist/*
var webDistFS embed.FS

// registerWebConsole mounts the React SPA web console on the router.
// It serves static assets with proper MIME types, and falls back to index.html
// for client-side routes (e.g. /dashboard, /devices, /login, /audit).
// Any path beginning with /api/ or /healthz is left to API handlers.
func registerWebConsole(r *chi.Mux) {
	subFS, err := fs.Sub(webDistFS, "dist")
	if err != nil {
		log.Warn().Err(err).Msg("failed to create sub-filesystem for web console")
		return
	}

	fileServer := http.FileServer(http.FS(subFS))

	r.Get("/*", func(w http.ResponseWriter, req *http.Request) {
		trimmed := strings.TrimPrefix(req.URL.Path, "/")
		if trimmed == "" {
			trimmed = "index.html"
		}

		// If requested file exists in embedded assets, serve it directly
		f, err := subFS.Open(trimmed)
		if err == nil {
			_ = f.Close()
			fileServer.ServeHTTP(w, req)
			return
		}

		// Fallback to index.html for SPA client routes
		if !strings.HasPrefix(req.URL.Path, "/api") && req.URL.Path != "/healthz" {
			indexData, err := fs.ReadFile(subFS, "index.html")
			if err != nil {
				http.NotFound(w, req)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(indexData)
			return
		}

		http.NotFound(w, req)
	})

	log.Info().Msg("web console mounted (embedded single-binary SPA)")
}
