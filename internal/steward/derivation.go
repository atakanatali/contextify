package steward

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/atakanatali/contextify/internal/config"
	"github.com/atakanatali/contextify/internal/memory"
)

type DerivationExecutor struct {
	repo *Repository
	svc  *memory.Service
	cfg  *config.StewardDerivation
}

func NewDerivationExecutor(repo *Repository, svc *memory.Service, cfg config.StewardDerivation) *DerivationExecutor {
	cfgCopy := cfg
	return &DerivationExecutor{repo: repo, svc: svc, cfg: &cfgCopy}
}

func NewDerivationExecutorFromPtr(repo *Repository, svc *memory.Service, cfg *config.StewardDerivation) *DerivationExecutor {
	if cfg == nil {
		empty := config.StewardDerivation{}
		cfg = &empty
	}
	return &DerivationExecutor{repo: repo, svc: svc, cfg: cfg}
}

type derivePayload struct {
	SourceMemoryIDs []uuid.UUID `json:"source_memory_ids"`
	Trigger         string      `json:"trigger"`
}

func (e *DerivationExecutor) Execute(ctx context.Context, job Job) (*ExecutionResult, error) {
	if !e.cfg.Enabled {
		return &ExecutionResult{Status: JobSucceeded, Decision: "derive_disabled", Output: map[string]any{"enabled": false}}, nil
	}
	p, err := parseDerivePayload(job.Payload, job.SourceMemoryIDs)
	if err != nil {
		return nil, err
	}
	if len(p.SourceMemoryIDs) == 0 {
		return &ExecutionResult{Status: JobSucceeded, Decision: "derive_no_sources", Output: map[string]any{"created": 0}}, nil
	}

	var sourceMems []*memory.Memory
	for _, id := range p.SourceMemoryIDs {
		mem, err := e.svc.Get(ctx, id)
		if err != nil || mem == nil {
			continue
		}
		sourceMems = append(sourceMems, mem)
	}
	if len(sourceMems) == 0 {
		return &ExecutionResult{Status: JobSucceeded, Decision: "derive_missing_sources", Output: map[string]any{"created": 0}}, nil
	}

	maxCandidates := e.cfg.MaxCandidates
	if maxCandidates <= 0 {
		maxCandidates = 1
	}
	candidates := deriveCandidates(sourceMems, maxCandidates)

	createdIDs := []uuid.UUID{}
	sideEffects := []map[string]any{}
	for _, c := range candidates {
		if c.Confidence < e.cfg.MinConfidence || c.Novelty < e.cfg.MinNovelty {
			_ = e.repo.StoreDerivationRecord(ctx, Derivation{
				SourceMemoryIDs: p.SourceMemoryIDs,
				DerivationType:  string(c.Type),
				Confidence:      float32Ptr(float32(c.Confidence)),
				Novelty:         float32Ptr(float32(c.Novelty)),
				Status:          "skipped",
				Model:           strPtr("deterministic"),
				Payload:         map[string]any{"reason": "threshold", "title": c.Title},
				CreatedAt:       time.Now().UTC(),
				UpdatedAt:       time.Now().UTC(),
			})
			sideEffects = append(sideEffects, map[string]any{"type": "derivation_skipped", "reason": "threshold", "title": c.Title})
			continue
		}

		storeReq := memory.StoreRequest{
			Title:       c.Title,
			Content:     c.Content,
			Type:        c.Type,
			Scope:       memory.ScopeProject,
			ProjectID:   sourceMems[0].ProjectID,
			Tags:        append([]string{"derived", "steward"}, c.Tags...),
			Importance:  0.7,
			TTLSeconds:  nil,
			AgentSource: strPtr("steward-derive"),
		}
		res, err := e.svc.Store(ctx, storeReq) // Smart store acts as dedup gate
		if err != nil || res == nil || res.Memory == nil {
			sideEffects = append(sideEffects, map[string]any{"type": "derivation_failed", "title": c.Title})
			continue
		}
		derivedID := res.Memory.ID
		createdIDs = append(createdIDs, derivedID)
		_ = e.repo.StoreDerivationRecord(ctx, Derivation{
			SourceMemoryIDs: p.SourceMemoryIDs,
			DerivedMemoryID: &derivedID,
			DerivationType:  string(c.Type),
			Confidence:      float32Ptr(float32(c.Confidence)),
			Novelty:         float32Ptr(float32(c.Novelty)),
			Status:          "accepted",
			Model:           strPtr("deterministic"),
			Payload:         map[string]any{"title": c.Title, "store_action": res.Action},
			CreatedAt:       time.Now().UTC(),
			UpdatedAt:       time.Now().UTC(),
		})
		for _, src := range p.SourceMemoryIDs {
			_, _ = e.svc.CreateRelationship(ctx, memory.RelationshipRequest{
				FromMemoryID: derivedID,
				ToMemoryID:   src,
				Relationship: "DERIVED_FROM",
				Strength:     0.9,
			})
		}
		sideEffects = append(sideEffects, map[string]any{"type": "derivation_created", "derived_memory_id": derivedID})
	}

	return &ExecutionResult{
		Status:      JobSucceeded,
		Decision:    "derived",
		Retryable:   false,
		Output:      map[string]any{"created_memory_ids": createdIDs, "source_memory_ids": p.SourceMemoryIDs},
		SideEffects: sideEffects,
	}, nil
}

type derivedCandidate struct {
	Title      string
	Content    string
	Type       memory.MemoryType
	Tags       []string
	Confidence float64
	Novelty    float64
}

func deriveCandidates(sourceMems []*memory.Memory, maxCandidates int) []derivedCandidate {
	if len(sourceMems) == 0 || maxCandidates <= 0 {
		return nil
	}
	base := sourceMems[0]
	summary := strings.TrimSpace(base.Content)
	if len(summary) > 400 {
		summary = summary[:400]
	}
	if summary == "" {
		summary = base.Title
	}
	out := []derivedCandidate{{
		Title:      "Derived decision: " + base.Title,
		Content:    "Derived from steward synthesis.\n\nSource summary:\n" + summary,
		Type:       memory.TypeDecision,
		Tags:       []string{"derivation", "decision"},
		Confidence: 0.85,
		Novelty:    0.30,
	}}
	if maxCandidates == 1 {
		return out
	}
	out = append(out, derivedCandidate{
		Title:      "Derived workflow: " + base.Title,
		Content:    "Workflow candidate derived from source memory content.\n\n" + summary,
		Type:       memory.TypeWorkflow,
		Tags:       []string{"derivation", "workflow"},
		Confidence: 0.80,
		Novelty:    0.25,
	})
	if len(out) > maxCandidates {
		out = out[:maxCandidates]
	}
	return out
}

func parseDerivePayload(payload map[string]any, fallback []uuid.UUID) (*derivePayload, error) {
	out := &derivePayload{SourceMemoryIDs: append([]uuid.UUID{}, fallback...)}
	if payload == nil {
		return out, nil
	}
	if t, ok := payload["trigger"].(string); ok {
		out.Trigger = t
	}
	if raw, ok := payload["source_memory_ids"].([]any); ok {
		out.SourceMemoryIDs = nil
		for _, item := range raw {
			s, ok := item.(string)
			if !ok {
				continue
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return nil, fmt.Errorf("invalid source_memory_ids entry: %w", err)
			}
			out.SourceMemoryIDs = append(out.SourceMemoryIDs, id)
		}
	}
	return out, nil
}

func float32Ptr(v float32) *float32 { return &v }
func strPtr(v string) *string       { return &v }
