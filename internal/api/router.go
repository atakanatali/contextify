package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"

	"github.com/atakanatali/contextify/internal/memory"
)

func NewRouter(svc *memory.Service) *chi.Mux {
	h := NewHandlers(svc)

	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
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

		// Related
		r.Get("/memories/{id}/related", h.GetRelatedMemories)

		// Relationships
		r.Post("/relationships", h.CreateRelationship)

		// Stats
		r.Get("/stats", h.GetStats)

		// Context
		r.Post("/context/{project}", h.GetContext)
	})

	return r
}
