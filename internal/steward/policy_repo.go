package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) EnqueuePolicyTuneJob(ctx context.Context, maxAttempts int, idempotencyKey string) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if idempotencyKey == "" {
		idempotencyKey = "steward:policy_tune:" + time.Now().UTC().Format(time.RFC3339Nano)
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO steward_jobs (
			id, job_type, trigger_reason, payload, status, priority, attempt_count, max_attempts, run_after, idempotency_key
		)
		VALUES (
			uuid_generate_v4(), 'policy_tune', 'periodic_self_learn', '{}'::jsonb, 'queued', 20, 0, $1, NOW(), $2
		)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
	`, maxAttempts, idempotencyKey)
	if err != nil {
		return fmt.Errorf("enqueue policy tune job: %w", err)
	}
	return nil
}

type PolicyTuningEvidence struct {
	SampleSize          int64
	SuccessRate24h      float64
	AcceptedSuggest24h  int64
	DismissedSuggest24h int64
	AcceptedDerive24h   int64
	SkippedDerive24h    int64
}

func (r *Repository) GetPolicyTuningEvidence(ctx context.Context) (*PolicyTuningEvidence, error) {
	out := &PolicyTuningEvidence{}
	if err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*)::bigint,
			COALESCE(AVG(CASE WHEN status = 'succeeded' THEN 1.0 ELSE 0.0 END), 0)
		FROM steward_runs
		WHERE created_at >= NOW() - INTERVAL '24 hour'
	`).Scan(&out.SampleSize, &out.SuccessRate24h); err != nil {
		return nil, fmt.Errorf("policy evidence from steward_runs: %w", err)
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'accepted'),
			COUNT(*) FILTER (WHERE status = 'dismissed')
		FROM consolidation_suggestions
		WHERE created_at >= NOW() - INTERVAL '24 hour'
	`).Scan(&out.AcceptedSuggest24h, &out.DismissedSuggest24h); err != nil {
		return nil, fmt.Errorf("policy evidence from suggestions: %w", err)
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'accepted'),
			COUNT(*) FILTER (WHERE status = 'skipped')
		FROM memory_derivations
		WHERE created_at >= NOW() - INTERVAL '24 hour'
	`).Scan(&out.AcceptedDerive24h, &out.SkippedDerive24h); err != nil {
		return nil, fmt.Errorf("policy evidence from derivations: %w", err)
	}
	return out, nil
}

func (r *Repository) InsertPolicyChange(ctx context.Context, c PolicyChange) (*PolicyChange, error) {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.ChangedBy == "" {
		c.ChangedBy = "steward"
	}
	if c.Evidence == nil {
		c.Evidence = map[string]any{}
	}
	b, err := json.Marshal(c.Evidence)
	if err != nil {
		return nil, fmt.Errorf("marshal policy evidence: %w", err)
	}
	if err := r.pool.QueryRow(ctx, `
		INSERT INTO steward_policy_history (
			id, policy_key, prior_value, new_value, reason, sample_size, evidence, changed_by, rollback_of_id
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9)
		RETURNING created_at
	`, c.ID, c.PolicyKey, c.PriorValue, c.NewValue, c.Reason, c.SampleSize, string(b), c.ChangedBy, c.RollbackOfID).Scan(&c.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert policy change: %w", err)
	}
	return &c, nil
}

func (r *Repository) ListPolicyChanges(ctx context.Context, policyKey *string, limit, offset int) ([]PolicyChange, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, policy_key, prior_value, new_value, reason, sample_size, evidence, changed_by, rollback_of_id, created_at
		FROM steward_policy_history
	`
	args := []any{}
	if policyKey != nil && *policyKey != "" {
		query += " WHERE policy_key = $1"
		args = append(args, *policyKey)
		query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	} else {
		query += " ORDER BY created_at DESC LIMIT $1 OFFSET $2"
		args = append(args, limit, offset)
	}
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list policy history: %w", err)
	}
	defer rows.Close()
	var out []PolicyChange
	for rows.Next() {
		var c PolicyChange
		var raw []byte
		if err := rows.Scan(&c.ID, &c.PolicyKey, &c.PriorValue, &c.NewValue, &c.Reason, &c.SampleSize, &raw, &c.ChangedBy, &c.RollbackOfID, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan policy history: %w", err)
		}
		_ = json.Unmarshal(raw, &c.Evidence)
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) LatestPolicyChange(ctx context.Context, policyKey string) (*PolicyChange, error) {
	var c PolicyChange
	var raw []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, policy_key, prior_value, new_value, reason, sample_size, evidence, changed_by, rollback_of_id, created_at
		FROM steward_policy_history
		WHERE policy_key = $1
		ORDER BY created_at DESC
		LIMIT 1
	`, policyKey).Scan(&c.ID, &c.PolicyKey, &c.PriorValue, &c.NewValue, &c.Reason, &c.SampleSize, &raw, &c.ChangedBy, &c.RollbackOfID, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("no policy history for key %q", policyKey)
	}
	if err != nil {
		return nil, fmt.Errorf("latest policy change: %w", err)
	}
	_ = json.Unmarshal(raw, &c.Evidence)
	return &c, nil
}

func (r *Repository) RollbackLatestPolicyChange(ctx context.Context, policyKey string) (*PolicyChange, error) {
	latest, err := r.LatestPolicyChange(ctx, policyKey)
	if err != nil {
		return nil, err
	}
	if latest.PriorValue == nil || latest.NewValue == nil {
		return nil, fmt.Errorf("latest policy change has no rollback value")
	}
	reason := "rollback latest policy change"
	sampleSize := 0
	rollback := PolicyChange{
		PolicyKey:    policyKey,
		PriorValue:   latest.NewValue,
		NewValue:     latest.PriorValue,
		Reason:       &reason,
		SampleSize:   &sampleSize,
		Evidence:     map[string]any{"rollback_of_id": latest.ID},
		ChangedBy:    "steward:rollback",
		RollbackOfID: &latest.ID,
	}
	return r.InsertPolicyChange(ctx, rollback)
}
