package steward

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/atakanatali/contextify/internal/memory"
	"github.com/atakanatali/contextify/internal/steward/llm"
)

type AutoMergeSuggestionExecutor struct {
	repo       *Repository
	svc        *memory.Service
	dryRun     bool
	llm        *llm.Client
	llmAllowed func() bool
}

func NewAutoMergeSuggestionExecutor(repo *Repository, svc *memory.Service, dryRun bool, llmClient *llm.Client) *AutoMergeSuggestionExecutor {
	return &AutoMergeSuggestionExecutor{repo: repo, svc: svc, dryRun: dryRun, llm: llmClient}
}

func NewAutoMergeSuggestionExecutorWithGuard(repo *Repository, svc *memory.Service, dryRun bool, llmClient *llm.Client, llmAllowed func() bool) *AutoMergeSuggestionExecutor {
	return &AutoMergeSuggestionExecutor{repo: repo, svc: svc, dryRun: dryRun, llm: llmClient, llmAllowed: llmAllowed}
}

func (e *AutoMergeSuggestionExecutor) Execute(ctx context.Context, job Job) (*ExecutionResult, error) {
	req, err := parseAutoMergeSuggestionPayload(job.Payload)
	if err != nil {
		return nil, err
	}

	snap, err := e.repo.GetSuggestionSnapshot(ctx, req.SuggestionID)
	if err != nil {
		return nil, err
	}
	if snap == nil {
		return &ExecutionResult{
			Status:      JobSucceeded,
			Decision:    "skip_missing_suggestion",
			Retryable:   false,
			Output:      map[string]any{"suggestion_id": req.SuggestionID, "skip_reason": "missing_suggestion"},
			SideEffects: []map[string]any{{"type": "suggestion_skip", "suggestion_id": req.SuggestionID, "reason": "missing_suggestion"}},
		}, nil
	}
	if snap.Status != "pending" {
		return &ExecutionResult{
			Status:      JobSucceeded,
			Decision:    "skip_non_pending",
			Retryable:   false,
			Output:      map[string]any{"suggestion_id": req.SuggestionID, "status": snap.Status},
			SideEffects: []map[string]any{{"type": "suggestion_skip", "suggestion_id": req.SuggestionID, "reason": "non_pending"}},
		}, nil
	}
	if snap.AReplacedBy != nil || snap.BReplacedBy != nil {
		if !e.dryRun {
			_ = e.svc.UpdateSuggestionStatus(ctx, req.SuggestionID, "dismissed")
		}
		return &ExecutionResult{
			Status:      JobSucceeded,
			Decision:    "skip_stale_pair",
			Retryable:   false,
			Output:      map[string]any{"suggestion_id": req.SuggestionID, "skip_reason": "stale_pair"},
			SideEffects: []map[string]any{{"type": "suggestion_dismissed", "suggestion_id": req.SuggestionID, "reason": "stale_pair"}},
		}, nil
	}
	if !sameProjectOrGlobal(snap.AMemoryProj, snap.BMemoryProj) {
		if !e.dryRun {
			_ = e.svc.UpdateSuggestionStatus(ctx, req.SuggestionID, "dismissed")
		}
		return &ExecutionResult{
			Status:      JobSucceeded,
			Decision:    "skip_project_mismatch",
			Retryable:   false,
			Output:      map[string]any{"suggestion_id": req.SuggestionID, "skip_reason": "project_mismatch"},
			SideEffects: []map[string]any{{"type": "suggestion_dismissed", "suggestion_id": req.SuggestionID, "reason": "project_mismatch"}},
		}, nil
	}

	if e.llm != nil && (e.llmAllowed == nil || e.llmAllowed()) {
		aMem, err := e.svc.Get(ctx, snap.MemoryAID)
		if err == nil && aMem != nil {
			bMem, err2 := e.svc.Get(ctx, snap.MemoryBID)
			if err2 == nil && bMem != nil {
				decision, metrics, derr := e.llm.DecideMerge(ctx, llm.MergeDecisionInput{
					MemoryATitle:   aMem.Title,
					MemoryAContent: aMem.Content,
					MemoryATags:    aMem.Tags,
					MemoryBTitle:   bMem.Title,
					MemoryBContent: bMem.Content,
					MemoryBTags:    bMem.Tags,
					Similarity:     snap.Similarity,
					StrategyHints:  map[string]string{"default": req.MergeStrategy},
				})
				if derr == nil && decision != nil {
					if decision.HasConflict || decision.Confidence < 0.85 || decision.Decision != "merge" {
						if !e.dryRun {
							_ = e.svc.UpdateSuggestionStatus(ctx, req.SuggestionID, "dismissed")
						}
						out := map[string]any{
							"suggestion_id": req.SuggestionID,
							"skip_reason":   "llm_conflict_or_low_confidence",
							"llm_decision":  decision,
						}
						res := &ExecutionResult{
							Status:      JobSucceeded,
							Decision:    "llm_skip",
							Retryable:   false,
							Output:      out,
							SideEffects: []map[string]any{{"type": "suggestion_dismissed", "suggestion_id": req.SuggestionID, "reason": "llm_guard"}},
						}
						if metrics != nil {
							res.Provider = metrics.Provider
							res.Model = metrics.Model
							res.PromptTokens = metrics.PromptTokens
							res.CompletionTokens = metrics.CompletionTokens
							res.TotalTokens = metrics.TotalTokens
							res.LatencyMs = metrics.LatencyMs
						}
						return res, nil
					}
				}
			}
		}
	}

	if e.dryRun {
		return &ExecutionResult{
			Status:    JobSucceeded,
			Decision:  "dry_run_merge",
			Retryable: false,
			Output: map[string]any{
				"suggestion_id": req.SuggestionID,
				"target_id":     snap.MemoryAID,
				"source_ids":    []uuid.UUID{snap.MemoryBID},
				"strategy":      req.MergeStrategy,
				"similarity":    snap.Similarity,
			},
			SideEffects: []map[string]any{{"type": "merge_skipped", "reason": "dry_run", "suggestion_id": req.SuggestionID}},
		}, nil
	}

	strategy := memory.MergeStrategy(req.MergeStrategy)
	if strategy == "" {
		strategy = memory.MergeStrategy("smart_merge")
	}
	merged, err := e.svc.ConsolidateMemories(ctx, snap.MemoryAID, []uuid.UUID{snap.MemoryBID}, strategy, "steward:auto_merge")
	if err != nil {
		return nil, fmt.Errorf("consolidate from suggestion %s: %w", req.SuggestionID, err)
	}
	if err := e.svc.UpdateSuggestionStatus(ctx, req.SuggestionID, "accepted"); err != nil {
		return nil, fmt.Errorf("mark suggestion accepted %s: %w", req.SuggestionID, err)
	}
	return &ExecutionResult{
		Status:    JobSucceeded,
		Decision:  "merged",
		Retryable: false,
		Output: map[string]any{
			"suggestion_id":  req.SuggestionID,
			"merged_target":  snap.MemoryAID,
			"merged_source":  snap.MemoryBID,
			"result_memory":  merged,
			"similarity":     snap.Similarity,
			"merge_strategy": strategy,
		},
		SideEffects: []map[string]any{
			{"type": "merge_applied", "target_id": snap.MemoryAID, "source_id": snap.MemoryBID},
			{"type": "suggestion_accepted", "suggestion_id": req.SuggestionID},
		},
	}, e.enqueuePostMergeDerivation(ctx, snap)
}

func (e *AutoMergeSuggestionExecutor) enqueuePostMergeDerivation(ctx context.Context, snap *SuggestionSnapshot) error {
	if snap == nil {
		return nil
	}
	return e.repo.EnqueueDeriveJob(ctx, snap.ProjectID, []uuid.UUID{snap.MemoryAID, snap.MemoryBID}, map[string]any{
		"source_memory_ids": []string{snap.MemoryAID.String(), snap.MemoryBID.String()},
		"trigger":           "auto_merge_from_suggestion",
	}, 3, "steward:derive:post_merge:"+snap.SuggestionID.String())
}

type autoMergeSuggestionPayload struct {
	SuggestionID  uuid.UUID `json:"suggestion_id"`
	MergeStrategy string    `json:"merge_strategy"`
}

func parseAutoMergeSuggestionPayload(payload map[string]any) (*autoMergeSuggestionPayload, error) {
	if payload == nil {
		return nil, fmt.Errorf("missing payload")
	}
	raw, ok := payload["suggestion_id"]
	if !ok {
		return nil, fmt.Errorf("missing payload.suggestion_id")
	}
	var sid uuid.UUID
	switch v := raw.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return nil, fmt.Errorf("invalid payload.suggestion_id: %w", err)
		}
		sid = id
	default:
		return nil, fmt.Errorf("invalid payload.suggestion_id type")
	}
	var strategy string
	if s, ok := payload["merge_strategy"].(string); ok {
		strategy = s
	}
	return &autoMergeSuggestionPayload{SuggestionID: sid, MergeStrategy: strategy}, nil
}

func sameProjectOrGlobal(a, b *string) bool {
	// nil projects are treated as globally eligible for now.
	if a == nil || b == nil {
		return true
	}
	return *a == *b
}
