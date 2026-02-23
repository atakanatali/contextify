package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
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
		ID:            uuid.New(),
		JobID:         &job.ID,
		InputSnapshot: map[string]any{"job_type": job.JobType, "project_id": job.ProjectID},
		Status:        string(JobRunning),
		CreatedAt:     time.Now().UTC(),
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
	outJSON, _ := json.Marshal(output)
	_, err := r.pool.Exec(ctx, `
		UPDATE steward_runs
		SET output_snapshot = $2::jsonb, status = 'succeeded', completed_at = NOW()
		WHERE id = $1
	`, run.ID, string(outJSON))
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
