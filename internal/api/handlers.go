package api

import (
	"encoding/json"
	"net/http"
	"strconv"

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

	result, err := h.svc.Store(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, result)
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

// Health check
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// POST /api/v1/memories/{id}/merge
func (h *Handlers) MergeMemories(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid memory id")
		return
	}

	var req memory.MergeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.SourceIDs) == 0 {
		writeError(w, http.StatusBadRequest, "source_ids are required")
		return
	}

	strategy := memory.MergeStrategy(req.Strategy)
	result, err := h.svc.ConsolidateMemories(r.Context(), id, req.SourceIDs, strategy, "api")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// GET /api/v1/memories/duplicates
func (h *Handlers) GetDuplicates(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	var projectPtr *string
	if projectID != "" {
		projectPtr = &projectID
	}

	suggestions, total, err := h.svc.GetSuggestions(r.Context(), projectPtr, "pending", limit, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"suggestions": suggestions,
		"total":       total,
	})
}

// POST /api/v1/memories/consolidate
func (h *Handlers) BatchConsolidate(w http.ResponseWriter, r *http.Request) {
	var req memory.BatchConsolidateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if len(req.Operations) == 0 {
		writeError(w, http.StatusBadRequest, "operations are required")
		return
	}

	var results []map[string]any
	for _, op := range req.Operations {
		strategy := memory.MergeStrategy(req.Strategy)
		merged, err := h.svc.ConsolidateMemories(r.Context(), op.TargetID, op.SourceIDs, strategy, "api")
		if err != nil {
			results = append(results, map[string]any{
				"target_id": op.TargetID,
				"error":     err.Error(),
			})
			continue
		}
		results = append(results, map[string]any{
			"target_id": op.TargetID,
			"memory":    merged,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

// GET /api/v1/consolidation/suggestions
func (h *Handlers) GetSuggestions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "pending"
	}
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var projectPtr *string
	if projectID != "" {
		projectPtr = &projectID
	}

	suggestions, total, err := h.svc.GetSuggestions(r.Context(), projectPtr, status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"suggestions": suggestions,
		"total":       total,
	})
}

// PUT /api/v1/consolidation/suggestions/{id}
func (h *Handlers) UpdateSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid suggestion id")
		return
	}

	var req memory.SuggestionStatusUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.Status != "accepted" && req.Status != "dismissed" {
		writeError(w, http.StatusBadRequest, "status must be 'accepted' or 'dismissed'")
		return
	}

	if err := h.svc.UpdateSuggestionStatus(r.Context(), id, req.Status); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": req.Status, "id": id.String()})
}

// GET /api/v1/consolidation/log
func (h *Handlers) GetConsolidationLog(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var targetID *uuid.UUID
	if v := r.URL.Query().Get("target_id"); v != "" {
		if id, err := uuid.Parse(v); err == nil {
			targetID = &id
		}
	}

	logs, err := h.svc.GetConsolidationLog(r.Context(), targetID, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, logs)
}

// POST /api/v1/admin/normalize-projects
func (h *Handlers) NormalizeProjects(w http.ResponseWriter, r *http.Request) {
	updated, err := h.svc.NormalizeAllProjectIDs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"updated": updated,
		"message": "project_id normalization complete",
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
