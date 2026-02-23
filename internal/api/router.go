package api

import (
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/atakanatali/contextify/internal/memory"
	"github.com/atakanatali/contextify/internal/steward"
)

func NewRouter(svc *memory.Service, stewardMgr *steward.Manager) *chi.Mux {
	h := NewHandlers(svc, stewardMgr)

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(requestIDMiddleware)
	r.Use(loggingMiddleware)

	r.Get("/health", h.Health)

	r.Route("/api/v1", func(r chi.Router) {
		// Memories CRUD
		r.Post("/memories", h.StoreMemory)
		r.Get("/memories/{id}", h.GetMemory)
		r.Put("/memories/{id}", h.UpdateMemory)
		r.Delete("/memories/{id}", h.DeleteMemory)

		// Search
		r.Post("/memories/search", h.SearchMemories)
		r.Post("/memories/recall", h.RecallMemories)

		// Promote
		r.Post("/memories/{id}/promote", h.PromoteMemory)

		// Merge
		r.Post("/memories/{id}/merge", h.MergeMemories)

		// Related
		r.Get("/memories/{id}/related", h.GetRelatedMemories)

		// Duplicates & Consolidation
		r.Get("/memories/duplicates", h.GetDuplicates)
		r.Post("/memories/consolidate", h.BatchConsolidate)

		// Relationships
		r.Post("/relationships", h.CreateRelationship)

		// Stats & Analytics
		r.Get("/stats", h.GetStats)
		r.Get("/analytics", h.GetAnalytics)
		r.Get("/analytics/funnel", h.GetFunnelAnalytics)

		// Context
		r.Post("/context/{project}", h.GetContext)

		// Consolidation
		r.Get("/consolidation/suggestions", h.GetSuggestions)
		r.Put("/consolidation/suggestions/{id}", h.UpdateSuggestion)
		r.Get("/consolidation/log", h.GetConsolidationLog)

		// Admin
		r.Post("/admin/normalize-projects", h.NormalizeProjects)

		// Steward
		r.Get("/steward/status", h.GetStewardStatus)
		r.Get("/steward/runs", h.GetStewardRuns)
		r.Get("/steward/jobs/{id}/events", h.GetStewardJobEvents)
		r.Get("/steward/metrics", h.GetStewardMetrics)
		r.Get("/steward/policies/history", h.GetStewardPolicyHistory)
		r.Post("/steward/run-once", h.StewardRunOnce)
		r.Put("/steward/mode", h.UpdateStewardMode)
		r.Post("/steward/jobs/{id}/retry", h.RetryStewardJob)
		r.Post("/steward/jobs/{id}/cancel", h.CancelStewardJob)
		r.Post("/steward/policies/rollback", h.RollbackStewardPolicy)
	})

	// Serve embedded Web UI static files (SPA with fallback to index.html)
	webDir := "/usr/share/contextify/web"
	if _, err := os.Stat(webDir); err == nil {
		spaHandler := spaFileServer(os.DirFS(webDir))
		r.NotFound(spaHandler.ServeHTTP)
	}

	return r
}

// spaFileServer serves static files and falls back to index.html for SPA routing.
func spaFileServer(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		// Try to open the file
		f, err := fsys.Open(path)
		if err != nil {
			// File not found — serve index.html for SPA routing
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}
		f.Close()

		fileServer.ServeHTTP(w, r)
	})
}
