package steward

import (
	"context"
	"math"
)

type PolicyTuneExecutor struct {
	m *Manager
}

func NewPolicyTuneExecutor(m *Manager) *PolicyTuneExecutor {
	return &PolicyTuneExecutor{m: m}
}

type policyProposal struct {
	Key        string
	Prior      float64
	Next       float64
	Reason     string
	SampleSize int
	Evidence   map[string]any
}

type tunerConfig struct {
	AutoMergeThreshold      float64
	DerivationMinConfidence float64
	DerivationMinNovelty    float64
	MinSampleSize           int
}

func (e *PolicyTuneExecutor) Execute(ctx context.Context, job Job) (*ExecutionResult, error) {
	evidence, err := e.m.repo.GetPolicyTuningEvidence(ctx)
	if err != nil {
		return nil, err
	}

	e.m.mu.Lock()
	cfg := tunerConfig{
		AutoMergeThreshold:      e.m.cfg.AutoMergeThreshold,
		DerivationMinConfidence: e.m.cfg.Derivation.MinConfidence,
		DerivationMinNovelty:    e.m.cfg.Derivation.MinNovelty,
		MinSampleSize:           e.m.cfg.SelfLearn.MinSampleSize,
	}
	e.m.mu.Unlock()

	proposal := choosePolicyProposal(cfg, evidence)
	if proposal == nil {
		return &ExecutionResult{
			Status:   JobSucceeded,
			Decision: "no_policy_change",
			Output: map[string]any{
				"sample_size":      evidence.SampleSize,
				"success_rate_24h": evidence.SuccessRate24h,
				"suggestions_24h":  evidence.AcceptedSuggest24h + evidence.DismissedSuggest24h,
				"derivations_24h":  evidence.AcceptedDerive24h + evidence.SkippedDerive24h,
			},
		}, nil
	}

	prior := proposal.Prior
	next := proposal.Next
	change, err := e.m.repo.InsertPolicyChange(ctx, PolicyChange{
		PolicyKey:  proposal.Key,
		PriorValue: &prior,
		NewValue:   &next,
		Reason:     &proposal.Reason,
		SampleSize: &proposal.SampleSize,
		Evidence:   proposal.Evidence,
		ChangedBy:  "steward:self_learn",
	})
	if err != nil {
		return nil, err
	}

	e.applyProposal(proposal)

	return &ExecutionResult{
		Status:   JobSucceeded,
		Decision: "policy_updated",
		Output: map[string]any{
			"history_id":  change.ID,
			"policy_key":  proposal.Key,
			"prior_value": proposal.Prior,
			"new_value":   proposal.Next,
			"reason":      proposal.Reason,
			"sample_size": proposal.SampleSize,
			"evidence":    proposal.Evidence,
			"job_id":      job.ID,
		},
		SideEffects: []map[string]any{{
			"type":       "policy_update",
			"policy_key": proposal.Key,
			"prior":      proposal.Prior,
			"next":       proposal.Next,
		}},
	}, nil
}

func (e *PolicyTuneExecutor) applyProposal(p *policyProposal) {
	if p == nil {
		return
	}
	e.m.mu.Lock()
	defer e.m.mu.Unlock()
	switch p.Key {
	case "auto_merge_threshold":
		e.m.cfg.AutoMergeThreshold = p.Next
	case "derivation.min_confidence":
		e.m.cfg.Derivation.MinConfidence = p.Next
	case "derivation.min_novelty":
		e.m.cfg.Derivation.MinNovelty = p.Next
	}
}

func choosePolicyProposal(cfg tunerConfig, evidence *PolicyTuningEvidence) *policyProposal {
	if evidence == nil || int(evidence.SampleSize) < cfg.MinSampleSize {
		return nil
	}
	suggestTotal := evidence.AcceptedSuggest24h + evidence.DismissedSuggest24h
	deriveTotal := evidence.AcceptedDerive24h + evidence.SkippedDerive24h
	suggestAcceptance := ratio(evidence.AcceptedSuggest24h, suggestTotal)
	deriveAcceptance := ratio(evidence.AcceptedDerive24h, deriveTotal)

	// One change per cycle, conservative bounded steps.
	if suggestTotal >= int64(max(10, cfg.MinSampleSize/2)) && suggestAcceptance < 0.35 {
		next := clamp(cfg.AutoMergeThreshold+0.01, 0.90, 0.97)
		if different(next, cfg.AutoMergeThreshold) {
			return &policyProposal{
				Key:        "auto_merge_threshold",
				Prior:      cfg.AutoMergeThreshold,
				Next:       next,
				Reason:     "low suggestion acceptance; increase merge threshold conservatively",
				SampleSize: int(evidence.SampleSize),
				Evidence:   map[string]any{"suggest_acceptance_rate": suggestAcceptance, "suggest_total": suggestTotal},
			}
		}
	}
	if suggestTotal >= int64(max(10, cfg.MinSampleSize/2)) && suggestAcceptance > 0.80 && evidence.SuccessRate24h > 0.90 {
		next := clamp(cfg.AutoMergeThreshold-0.005, 0.90, 0.97)
		if different(next, cfg.AutoMergeThreshold) {
			return &policyProposal{
				Key:        "auto_merge_threshold",
				Prior:      cfg.AutoMergeThreshold,
				Next:       next,
				Reason:     "high suggestion acceptance and healthy runs; cautiously lower merge threshold",
				SampleSize: int(evidence.SampleSize),
				Evidence:   map[string]any{"suggest_acceptance_rate": suggestAcceptance, "success_rate_24h": evidence.SuccessRate24h},
			}
		}
	}
	if deriveTotal >= int64(max(10, cfg.MinSampleSize/2)) && deriveAcceptance < 0.50 {
		next := clamp(cfg.DerivationMinConfidence+0.02, 0.70, 0.90)
		if different(next, cfg.DerivationMinConfidence) {
			return &policyProposal{
				Key:        "derivation.min_confidence",
				Prior:      cfg.DerivationMinConfidence,
				Next:       next,
				Reason:     "low derivation acceptance; tighten min confidence",
				SampleSize: int(evidence.SampleSize),
				Evidence:   map[string]any{"derivation_acceptance_rate": deriveAcceptance, "derivation_total": deriveTotal},
			}
		}
	}
	if deriveTotal >= int64(max(10, cfg.MinSampleSize/2)) && deriveAcceptance > 0.75 && evidence.SuccessRate24h > 0.90 {
		next := clamp(cfg.DerivationMinNovelty-0.01, 0.15, 0.40)
		if different(next, cfg.DerivationMinNovelty) {
			return &policyProposal{
				Key:        "derivation.min_novelty",
				Prior:      cfg.DerivationMinNovelty,
				Next:       next,
				Reason:     "healthy derivation acceptance; relax min novelty slightly",
				SampleSize: int(evidence.SampleSize),
				Evidence:   map[string]any{"derivation_acceptance_rate": deriveAcceptance, "success_rate_24h": evidence.SuccessRate24h},
			}
		}
	}
	return nil
}

func ratio(a, b int64) float64 {
	if b <= 0 {
		return 0
	}
	return float64(a) / float64(b)
}

func clamp(v, minV, maxV float64) float64 {
	return math.Max(minV, math.Min(maxV, v))
}

func different(a, b float64) bool {
	return math.Abs(a-b) > 1e-9
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
