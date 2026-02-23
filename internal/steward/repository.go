package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) TryAcquireLeaderLock(ctx context.Context, conn *pgxpool.Conn, key int64) (bool, error) {
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&ok); err != nil {
		return false, fmt.Errorf("acquire advisory lock: %w", err)
	}
	return ok, nil
}

func (r *Repository) ReleaseLeaderLock(ctx context.Context, conn *pgxpool.Conn, key int64) error {
	var ok bool
	if err := conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", key).Scan(&ok); err != nil {
		return fmt.Errorf("release advisory lock: %w", err)
	}
	return nil
}

func (r *Repository) RecoverStaleRunningJobs(ctx context.Context, staleBefore time.Time) (int64, error) {
	query := `
		UPDATE steward_jobs
		SET status = CASE WHEN attempt_count + 1 >= max_attempts THEN 'dead_letter' ELSE 'queued' END,
		    run_after = CASE WHEN attempt_count + 1 >= max_attempts THEN run_after ELSE NOW() END,
		    last_error = COALESCE(last_error, 'recovered stale running job'),
		    locked_by = NULL,
		    locked_at = NULL,
		    lease_expires_at = NULL,
		    updated_at = NOW()
		WHERE status = 'running'
		  AND lease_expires_at IS NOT NULL
		  AND lease_expires_at < $1
	`
	res, err := r.pool.Exec(ctx, query, staleBefore)
	if err != nil {
		return 0, fmt.Errorf("recover stale jobs: %w", err)
	}
	return res.RowsAffected(), nil
}

func (r *Repository) ClaimJobs(ctx context.Context, workerID string, batchSize int, leaseDuration time.Duration) ([]Job, error) {
	if batchSize <= 0 {
		return nil, nil
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin claim tx: %w", err)
	}
	defer tx.Rollback(ctx)

	query := `
		WITH candidates AS (
			SELECT id
			FROM steward_jobs
			WHERE status = 'queued'
			  AND run_after <= NOW()
			ORDER BY priority DESC, created_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE steward_jobs j
		SET status = 'running',
		    locked_by = $2,
		    locked_at = NOW(),
		    lease_expires_at = NOW() + $3::interval,
		    updated_at = NOW()
		FROM candidates c
		WHERE j.id = c.id
		RETURNING j.id, j.job_type, j.project_id, j.source_memory_ids, j.trigger_reason, j.payload,
		          j.status, j.priority, j.attempt_count, j.max_attempts, j.run_after, j.locked_by,
		          j.locked_at, j.lease_expires_at, j.last_error, j.idempotency_key, j.cancelled_at,
		          j.created_at, j.updated_at
	`
	rows, err := tx.Query(ctx, query, batchSize, workerID, fmt.Sprintf("%d seconds", int(leaseDuration.Seconds())))
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate claimed jobs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit claim tx: %w", err)
	}
	return jobs, nil
}

func (r *Repository) EnqueueAutoMergeSuggestionJobs(ctx context.Context, threshold float64, maxAttempts, limit int) (int64, error) {
	return r.enqueueAutoMergeSuggestionJobsSlow(ctx, threshold, maxAttempts, limit)
}

func (r *Repository) enqueueAutoMergeSuggestionJobsSlow(ctx context.Context, threshold float64, maxAttempts, limit int) (int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, memory_a_id, memory_b_id, similarity, project_id
		FROM consolidation_suggestions
		WHERE status = 'pending' AND similarity >= $1
		ORDER BY similarity DESC, created_at ASC
		LIMIT $2
	`, threshold, limit)
	if err != nil {
		return 0, fmt.Errorf("query auto-merge suggestions: %w", err)
	}
	defer rows.Close()

	var inserted int64
	for rows.Next() {
		var sid, aID, bID uuid.UUID
		var similarity float64
		var projectID *string
		if err := rows.Scan(&sid, &aID, &bID, &similarity, &projectID); err != nil {
			return inserted, fmt.Errorf("scan auto-merge suggestion: %w", err)
		}
		_, err := r.pool.Exec(ctx, `
			INSERT INTO steward_jobs (
				id, job_type, project_id, source_memory_ids, trigger_reason, payload, status, priority,
				attempt_count, max_attempts, run_after, idempotency_key
			)
			VALUES (
				uuid_generate_v4(), 'auto_merge_from_suggestion', $1, ARRAY[$2,$3]::uuid[], 'pending_suggestion_high_similarity',
				jsonb_build_object('suggestion_id',$4,'similarity',$5,'memory_a_id',$2,'memory_b_id',$3,'merge_strategy','smart_merge'),
				'queued', 100, 0, $6, NOW(), $7
			)
			ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		`, projectID, aID, bID, sid, similarity, maxAttempts, "steward:auto_merge_suggestion:"+sid.String())
		if err != nil {
			return inserted, fmt.Errorf("insert auto-merge steward job: %w", err)
		}
		inserted++
	}
	return inserted, rows.Err()
}

type SuggestionSnapshot struct {
	SuggestionID uuid.UUID
	MemoryAID    uuid.UUID
	MemoryBID    uuid.UUID
	Similarity   float64
	Status       string
	ProjectID    *string
	AMemoryProj  *string
	BMemoryProj  *string
	AReplacedBy  *uuid.UUID
	BReplacedBy  *uuid.UUID
}

func (r *Repository) GetSuggestionSnapshot(ctx context.Context, suggestionID uuid.UUID) (*SuggestionSnapshot, error) {
	var s SuggestionSnapshot
	err := r.pool.QueryRow(ctx, `
		SELECT s.id, s.memory_a_id, s.memory_b_id, s.similarity, s.status, s.project_id,
		       a.project_id, b.project_id, a.replaced_by, b.replaced_by
		FROM consolidation_suggestions s
		JOIN memories a ON a.id = s.memory_a_id
		JOIN memories b ON b.id = s.memory_b_id
		WHERE s.id = $1
	`, suggestionID).Scan(
		&s.SuggestionID, &s.MemoryAID, &s.MemoryBID, &s.Similarity, &s.Status, &s.ProjectID,
		&s.AMemoryProj, &s.BMemoryProj, &s.AReplacedBy, &s.BReplacedBy,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get suggestion snapshot: %w", err)
	}
	return &s, nil
}

func (r *Repository) EnqueueDeriveJob(ctx context.Context, projectID *string, sourceIDs []uuid.UUID, payload map[string]any, maxAttempts int, idempotencyKey string) error {
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal derive job payload: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO steward_jobs (
			id, job_type, project_id, source_memory_ids, trigger_reason, payload, status, priority,
			attempt_count, max_attempts, run_after, idempotency_key
		)
		VALUES (
			uuid_generate_v4(), 'derive_memories', $1, $2, 'post_merge_derivation', $3::jsonb, 'queued',
			50, 0, $4, NOW(), $5
		)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
	`, projectID, sourceIDs, string(b), maxAttempts, idempotencyKey)
	if err != nil {
		return fmt.Errorf("enqueue derive job: %w", err)
	}
	return nil
}

func (r *Repository) StoreDerivationRecord(ctx context.Context, d Derivation) error {
	payload := d.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal derivation payload: %w", err)
	}
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO memory_derivations (
			id, source_memory_ids, derived_memory_id, derivation_type, confidence, novelty, status, model, payload, created_at, updated_at
		)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb, COALESCE($10, NOW()), COALESCE($11, NOW()))
	`, d.ID, d.SourceMemoryIDs, d.DerivedMemoryID, d.DerivationType, d.Confidence, d.Novelty, d.Status, d.Model, string(b), d.CreatedAt, d.UpdatedAt)
	if err != nil {
		return fmt.Errorf("store derivation record: %w", err)
	}
	return nil
}

func scanJob(rows pgx.Row) (*Job, error) {
	var job Job
	var payloadBytes []byte
	if err := rows.Scan(
		&job.ID, &job.JobType, &job.ProjectID, &job.SourceMemoryIDs, &job.TriggerReason, &payloadBytes,
		&job.Status, &job.Priority, &job.AttemptCount, &job.MaxAttempts, &job.RunAfter, &job.LockedBy,
		&job.LockedAt, &job.LeaseExpiresAt, &job.LastError, &job.IdempotencyKey, &job.CancelledAt,
		&job.CreatedAt, &job.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan steward job: %w", err)
	}
	if len(payloadBytes) > 0 {
		_ = json.Unmarshal(payloadBytes, &job.Payload)
	}
	if job.Payload == nil {
		job.Payload = map[string]any{}
	}
	return &job, nil
}

func (r *Repository) CreateRun(ctx context.Context, job Job, provider, model string) (*Run, error) {
	run := &Run{
		ID:    uuid.New(),
		JobID: &job.ID,
		InputSnapshot: map[string]any{
			"schema_version":    1,
			"job_type":          job.JobType,
			"project_id":        job.ProjectID,
			"payload":           job.Payload,
			"source_memory_ids": job.SourceMemoryIDs,
		},
		Status:    string(JobRunning),
		CreatedAt: time.Now().UTC(),
	}
	if provider != "" {
		run.Provider = &provider
	}
	if model != "" {
		run.Model = &model
	}
	inputJSON, _ := json.Marshal(run.InputSnapshot)
	_, err := r.pool.Exec(ctx, `
		INSERT INTO steward_runs (id, job_id, provider, model, input_snapshot, status, created_at)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7)
	`, run.ID, run.JobID, run.Provider, run.Model, string(inputJSON), run.Status, run.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create steward run: %w", err)
	}
	return run, nil
}

func (r *Repository) AppendEvent(ctx context.Context, jobID uuid.UUID, runID *uuid.UUID, eventType string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal steward event data: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO steward_events (id, job_id, run_id, event_type, data)
		VALUES ($1, $2, $3, $4, $5::jsonb)
	`, uuid.New(), jobID, runID, eventType, string(b))
	if err != nil {
		return fmt.Errorf("append steward event: %w", err)
	}
	return nil
}

func (r *Repository) MarkSucceeded(ctx context.Context, job Job, run *Run, result *ExecutionResult) error {
	output := result.Output
	if output == nil {
		output = map[string]any{}
	}
	if result.SideEffects != nil {
		output["side_effects"] = result.SideEffects
	}
	if result.Decision != "" {
		output["decision"] = result.Decision
	}
	outJSON, _ := json.Marshal(output)
	_, err := r.pool.Exec(ctx, `
		UPDATE steward_runs
		SET output_snapshot = $2::jsonb,
		    provider = COALESCE($3, provider),
		    model = COALESCE($4, model),
		    prompt_tokens = COALESCE($5, prompt_tokens),
		    completion_tokens = COALESCE($6, completion_tokens),
		    total_tokens = COALESCE($7, total_tokens),
		    latency_ms = COALESCE($8, latency_ms),
		    status = 'succeeded',
		    completed_at = NOW()
		WHERE id = $1
	`, run.ID, string(outJSON),
		nullableString(result.Provider), nullableString(result.Model),
		result.PromptTokens, result.CompletionTokens, result.TotalTokens, result.LatencyMs,
	)
	if err != nil {
		return fmt.Errorf("mark run succeeded: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE steward_jobs
		SET status = 'succeeded', locked_by = NULL, locked_at = NULL, lease_expires_at = NULL, updated_at = NOW()
		WHERE id = $1
	`, job.ID)
	if err != nil {
		return fmt.Errorf("mark job succeeded: %w", err)
	}
	return nil
}

func (r *Repository) MarkFailure(ctx context.Context, job Job, run *Run, err error, retryable bool) (JobStatus, time.Time, error) {
	now := time.Now().UTC()
	errorClass := "executor_error"
	errorMessage := err.Error()
	_, runErr := r.pool.Exec(ctx, `
		UPDATE steward_runs
		SET status = 'failed', error_class = $2, error_message = $3, completed_at = NOW()
		WHERE id = $1
	`, run.ID, errorClass, errorMessage)
	if runErr != nil {
		return "", time.Time{}, fmt.Errorf("mark run failed: %w", runErr)
	}

	nextAttempt := job.AttemptCount + 1
	if !retryable || nextAttempt >= job.MaxAttempts {
		_, qErr := r.pool.Exec(ctx, `
			UPDATE steward_jobs
			SET status = 'dead_letter', attempt_count = $2, last_error = $3, locked_by = NULL, locked_at = NULL, lease_expires_at = NULL, updated_at = NOW()
			WHERE id = $1
		`, job.ID, nextAttempt, errorMessage)
		if qErr != nil {
			return "", time.Time{}, fmt.Errorf("mark job dead_letter: %w", qErr)
		}
		return JobDeadLetter, now, nil
	}

	backoff := backoffWithJitter(nextAttempt)
	runAfter := now.Add(backoff)
	_, qErr := r.pool.Exec(ctx, `
		UPDATE steward_jobs
		SET status = 'queued', attempt_count = $2, run_after = $3, last_error = $4, locked_by = NULL, locked_at = NULL, lease_expires_at = NULL, updated_at = NOW()
		WHERE id = $1
	`, job.ID, nextAttempt, runAfter, errorMessage)
	if qErr != nil {
		return "", time.Time{}, fmt.Errorf("requeue failed job: %w", qErr)
	}
	return JobQueued, runAfter, nil
}

func backoffWithJitter(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	base := time.Second * time.Duration(1<<min(attempt-1, 6))
	jitter := time.Duration(rand.Int63n(int64(base / 4)))
	return base + jitter
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func nullableString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

func (r *Repository) ListRuns(ctx context.Context, f RunFilters) ([]Run, error) {
	conditions := []string{}
	args := []any{}
	arg := 1
	if f.Status != nil {
		conditions = append(conditions, fmt.Sprintf("r.status = $%d", arg))
		args = append(args, *f.Status)
		arg++
	}
	if f.JobType != nil {
		conditions = append(conditions, fmt.Sprintf("j.job_type = $%d", arg))
		args = append(args, *f.JobType)
		arg++
	}
	if f.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("j.project_id = $%d", arg))
		args = append(args, *f.ProjectID)
		arg++
	}
	if f.Model != nil {
		conditions = append(conditions, fmt.Sprintf("r.model = $%d", arg))
		args = append(args, *f.Model)
		arg++
	}
	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	args = append(args, f.Limit, f.Offset)
	query := fmt.Sprintf(`
		SELECT r.id, r.job_id, r.provider, r.model, r.input_snapshot, r.output_snapshot, r.input_hash,
		       r.prompt_tokens, r.completion_tokens, r.total_tokens, r.latency_ms, r.status,
		       r.error_class, r.error_message, r.created_at, r.completed_at
		FROM steward_runs r
		LEFT JOIN steward_jobs j ON j.id = r.job_id
		%s
		ORDER BY r.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, arg, arg+1)
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list steward runs: %w", err)
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var run Run
		var inJSON, outJSON []byte
		if err := rows.Scan(&run.ID, &run.JobID, &run.Provider, &run.Model, &inJSON, &outJSON, &run.InputHash,
			&run.PromptTokens, &run.CompletionTokens, &run.TotalTokens, &run.LatencyMs, &run.Status,
			&run.ErrorClass, &run.ErrorMessage, &run.CreatedAt, &run.CompletedAt); err != nil {
			return nil, fmt.Errorf("scan steward run: %w", err)
		}
		_ = json.Unmarshal(inJSON, &run.InputSnapshot)
		_ = json.Unmarshal(outJSON, &run.OutputSnapshot)
		out = append(out, run)
	}
	return out, rows.Err()
}

func (r *Repository) ListEventsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]Event, error) {
	if limit <= 0 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, job_id, run_id, event_type, data, schema_version, created_at
		FROM steward_events
		WHERE job_id = $1
		ORDER BY created_at ASC
		LIMIT $2 OFFSET $3
	`, jobID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list steward events: %w", err)
	}
	defer rows.Close()
	var events []Event
	for rows.Next() {
		var e Event
		var data []byte
		if err := rows.Scan(&e.ID, &e.JobID, &e.RunID, &e.EventType, &data, &e.SchemaVersion, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan steward event: %w", err)
		}
		_ = json.Unmarshal(data, &e.Data)
		events = append(events, e)
	}
	return events, rows.Err()
}

func (r *Repository) GetMetricsSummary(ctx context.Context) (*MetricsSummary, error) {
	out := &MetricsSummary{}
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM steward_runs WHERE created_at >= NOW() - INTERVAL '1 hour'
	`).Scan(&out.RunsLastHour); err != nil {
		return nil, fmt.Errorf("count steward runs last hour: %w", err)
	}
	var successCount, totalCount int64
	if err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE status = 'succeeded'),
		       COUNT(*)
		FROM steward_runs
		WHERE created_at >= NOW() - INTERVAL '24 hour'
	`).Scan(&successCount, &totalCount); err != nil {
		return nil, fmt.Errorf("compute steward success rate: %w", err)
	}
	if totalCount > 0 {
		out.SuccessRate = float64(successCount) / float64(totalCount)
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE(AVG(total_tokens), 0)
		FROM steward_runs
		WHERE total_tokens IS NOT NULL
		  AND created_at >= NOW() - INTERVAL '24 hour'
	`).Scan(&out.AverageTokensPerRun); err != nil {
		return nil, fmt.Errorf("avg steward tokens: %w", err)
	}
	if err := r.pool.QueryRow(ctx, `
		SELECT COALESCE((
			SELECT percentile_disc(0.95) WITHIN GROUP (ORDER BY latency_ms)
			FROM steward_runs
			WHERE latency_ms IS NOT NULL
			  AND created_at >= NOW() - INTERVAL '24 hour'
		), 0)
	`).Scan(&out.P95LatencyMs); err != nil {
		return nil, fmt.Errorf("p95 steward latency: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT COALESCE(error_class, 'unknown') AS reason, COUNT(*) AS c
		FROM steward_runs
		WHERE status = 'failed'
		  AND created_at >= NOW() - INTERVAL '24 hour'
		GROUP BY 1
		ORDER BY c DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, fmt.Errorf("top steward failures: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var fb FailureBreakdown
		if err := rows.Scan(&fb.Reason, &fb.Count); err != nil {
			return nil, fmt.Errorf("scan steward failure breakdown: %w", err)
		}
		out.TopFailureReasons = append(out.TopFailureReasons, fb)
	}
	return out, rows.Err()
}
