package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/atakanatali/contextify/internal/memory"
	"github.com/atakanatali/contextify/internal/steward"
)

type Handlers struct {
	svc        *memory.Service
	stewardMgr *steward.Manager
}

func NewHandlers(svc *memory.Service, stewardMgr *steward.Manager) *Handlers {
	return &Handlers{svc: svc, stewardMgr: stewardMgr}
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
		if errors.Is(err, memory.ErrMemoryNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
			return
		}
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
		if errors.Is(err, memory.ErrMemoryNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
			return
		}
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
		if errors.Is(err, memory.ErrMemoryNotFound) {
			writeError(w, http.StatusNotFound, "memory not found")
			return
		}
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

// GET /api/v1/analytics/funnel
func (h *Handlers) GetFunnelAnalytics(w http.ResponseWriter, r *http.Request) {
	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 365 {
			writeError(w, http.StatusBadRequest, "days must be an integer between 1 and 365")
			return
		}
		days = n
	}

	var req memory.FunnelAnalyticsRequest
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr != "" || toStr != "" {
		if fromStr == "" || toStr == "" {
			writeError(w, http.StatusBadRequest, "both from and to are required when using explicit date range")
			return
		}
		from, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "from must be in YYYY-MM-DD format")
			return
		}
		to, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "to must be in YYYY-MM-DD format")
			return
		}
		req.From = from
		req.To = to
	} else {
		now := time.Now().UTC()
		req.To = now
		req.From = now.AddDate(0, 0, -(days - 1))
	}

	if v := r.URL.Query().Get("agent_source"); v != "" {
		req.AgentSource = &v
	}
	if v := r.URL.Query().Get("project_id"); v != "" {
		req.ProjectID = &v
	}

	data, err := h.svc.GetFunnelAnalytics(r.Context(), req)
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

type stewardModeRequest struct {
	Paused bool `json:"paused"`
	DryRun bool `json:"dry_run"`
}

type stewardPolicyRollbackRequest struct {
	PolicyKey string `json:"policy_key"`
}

func (h *Handlers) requireSteward(w http.ResponseWriter) bool {
	if h.stewardMgr == nil {
		writeError(w, http.StatusNotFound, "steward not configured")
		return false
	}
	return true
}

func (h *Handlers) requireStewardAdmin(w http.ResponseWriter, r *http.Request) bool {
	if !h.requireSteward(w) {
		return false
	}
	expected := os.Getenv("STEWARD_ADMIN_TOKEN")
	if expected == "" {
		return true
	}
	if r.Header.Get("X-Steward-Admin-Token") != expected {
		writeError(w, http.StatusForbidden, "steward admin token required")
		return false
	}
	return true
}

func (h *Handlers) GetStewardStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireSteward(w) {
		return
	}
	writeJSON(w, http.StatusOK, h.stewardMgr.GetStatus())
}

func (h *Handlers) GetStewardRuns(w http.ResponseWriter, r *http.Request) {
	if !h.requireSteward(w) {
		return
	}
	var f steward.RunFilters
	if v := r.URL.Query().Get("status"); v != "" {
		f.Status = &v
	}
	if v := r.URL.Query().Get("job_type"); v != "" {
		f.JobType = &v
	}
	if v := r.URL.Query().Get("project_id"); v != "" {
		f.ProjectID = &v
	}
	if v := r.URL.Query().Get("model"); v != "" {
		f.Model = &v
	}
	f.Limit = 50
	f.Offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		f.Limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be >= 0")
			return
		}
		f.Offset = n
	}
	runs, err := h.stewardMgr.ListRuns(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "limit": f.Limit, "offset": f.Offset})
}

func (h *Handlers) GetStewardJobEvents(w http.ResponseWriter, r *http.Request) {
	if !h.requireSteward(w) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	limit, offset := 200, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 1000")
			return
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be >= 0")
			return
		}
		offset = n
	}
	events, err := h.stewardMgr.ListEventsByJob(r.Context(), id, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events, "limit": limit, "offset": offset})
}

func (h *Handlers) GetStewardMetrics(w http.ResponseWriter, r *http.Request) {
	if !h.requireSteward(w) {
		return
	}
	m, err := h.stewardMgr.Metrics(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (h *Handlers) GetStewardPolicyHistory(w http.ResponseWriter, r *http.Request) {
	if !h.requireSteward(w) {
		return
	}
	limit, offset := 50, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 || n > 500 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 500")
			return
		}
		limit = n
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "offset must be >= 0")
			return
		}
		offset = n
	}
	var key *string
	if v := r.URL.Query().Get("policy_key"); v != "" {
		key = &v
	}
	items, err := h.stewardMgr.ListPolicyChanges(r.Context(), key, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (h *Handlers) StewardRunOnce(w http.ResponseWriter, r *http.Request) {
	if !h.requireStewardAdmin(w, r) {
		return
	}
	if err := h.stewardMgr.RunOnce(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (h *Handlers) UpdateStewardMode(w http.ResponseWriter, r *http.Request) {
	if !h.requireStewardAdmin(w, r) {
		return
	}
	var req stewardModeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, h.stewardMgr.SetMode(req.Paused, req.DryRun))
}

func (h *Handlers) RetryStewardJob(w http.ResponseWriter, r *http.Request) {
	if !h.requireStewardAdmin(w, r) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if err := h.stewardMgr.RetryJob(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "queued", "id": id.String()})
}

func (h *Handlers) CancelStewardJob(w http.ResponseWriter, r *http.Request) {
	if !h.requireStewardAdmin(w, r) {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return
	}
	if err := h.stewardMgr.CancelJob(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled", "id": id.String()})
}

func (h *Handlers) RollbackStewardPolicy(w http.ResponseWriter, r *http.Request) {
	if !h.requireStewardAdmin(w, r) {
		return
	}
	var req stewardPolicyRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	if req.PolicyKey == "" {
		writeError(w, http.StatusBadRequest, "policy_key is required")
		return
	}
	change, err := h.stewardMgr.RollbackPolicy(r.Context(), req.PolicyKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, change)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
