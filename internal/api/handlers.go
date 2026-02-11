package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atakanatali/contextify/internal/memory"
)

type Handlers struct {
	svc *memory.Service
}

func NewHandlers(svc *memory.Service) *Handlers {
	return &Handlers{svc: svc}
}

// POST /api/v1/memories
func (h *Handlers) StoreMemory(w http.ResponseWriter, r *http.Request) {
	var req memory.StoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Title == "" || req.Content == "" {
		writeError(w, http.StatusBadRequest, "title and content are required")
		return
	}

	mem, err := h.svc.Store(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, mem)
}

// GET /api/v1/memories/{id}
func (h *Handlers) GetMemory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	mem, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if mem == nil {
		writeError(w, http.StatusNotFound, "memory not found")
		return
	}

	writeJSON(w, http.StatusOK, mem)
}

// PUT /api/v1/memories/{id}
func (h *Handlers) UpdateMemory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	var req memory.UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	mem, err := h.svc.Update(r.Context(), id, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, mem)
}

// DELETE /api/v1/memories/{id}
func (h *Handlers) DeleteMemory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "id": id.String()})
}

// POST /api/v1/memories/search
func (h *Handlers) SearchMemories(w http.ResponseWriter, r *http.Request) {
	var req memory.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	results, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// POST /api/v1/memories/recall
func (h *Handlers) RecallMemories(w http.ResponseWriter, r *http.Request) {
	var req memory.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Query == "" {
		writeError(w, http.StatusBadRequest, "query is required")
		return
	}

	results, err := h.svc.Search(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, results)
}

// POST /api/v1/relationships
func (h *Handlers) CreateRelationship(w http.ResponseWriter, r *http.Request) {
	var req memory.RelationshipRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	rel, err := h.svc.CreateRelationship(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, rel)
}

// GET /api/v1/memories/{id}/related
func (h *Handlers) GetRelatedMemories(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	memories, relationships, err := h.svc.GetRelated(r.Context(), id, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"memories":      memories,
		"relationships": relationships,
	})
}

// GET /api/v1/stats
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.svc.GetStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// POST /api/v1/context/{project}
func (h *Handlers) GetContext(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project")
	if projectID == "" {
		writeError(w, http.StatusBadRequest, "project id is required")
		return
	}

	memories, err := h.svc.GetContext(r.Context(), projectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, memories)
}

// POST /api/v1/memories/{id}/promote
func (h *Handlers) PromoteMemory(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	if err := h.svc.Promote(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted", "id": id.String()})
}

// GET /api/v1/analytics
func (h *Handlers) GetAnalytics(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GetAnalytics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// Health check
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
