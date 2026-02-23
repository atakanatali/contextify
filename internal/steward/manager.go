package steward

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atakanatali/contextify/internal/config"
	"github.com/atakanatali/contextify/internal/memory"
	stewardllm "github.com/atakanatali/contextify/internal/steward/llm"
)

const leaderLockKey int64 = 84201001

type Manager struct {
	pool      *pgxpool.Pool
	svc       *memory.Service
	cfg       config.StewardConfig
	ollamaURL string

	repo     *Repository
	registry *Registry

	cancel context.CancelFunc
	wg     sync.WaitGroup

	mu       sync.Mutex
	lockConn *pgxpool.Conn
	leader   bool
	workerID string
	paused   bool
}

func NewManager(pool *pgxpool.Pool, svc *memory.Service, cfg config.StewardConfig, embeddingOllamaURL string) *Manager {
	ollamaURL := cfg.OllamaURL
	if ollamaURL == "" {
		ollamaURL = embeddingOllamaURL
	}
	return &Manager{
		pool:      pool,
		svc:       svc,
		cfg:       cfg,
		ollamaURL: ollamaURL,
		repo:      NewRepository(pool),
		registry:  NewRegistry(),
		workerID:  fmt.Sprintf("steward-%d", time.Now().UnixNano()),
	}
}

func (m *Manager) Start() {
	if !m.cfg.Enabled {
		slog.Info("steward disabled")
		return
	}
	var llmClient *stewardllm.Client
	if m.cfg.LLMConflictGuardEnabled {
		llmClient = stewardllm.NewClient(m.ollamaURL, m.cfg.Model)
	}
	m.registry.Register("auto_merge_from_suggestion", NewAutoMergeSuggestionExecutor(m.repo, m.svc, m.cfg.DryRun, llmClient))
	m.registry.Register("derive_memories", NewDerivationExecutor(m.repo, m.svc, m.cfg.Derivation))
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.loop(ctx)
	}()
	slog.Info("steward manager started", "dry_run", m.cfg.DryRun, "worker_id", m.workerID, "tick_interval", m.cfg.TickInterval)
}

func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	m.releaseLeaderLock(context.Background())
}

func (m *Manager) loop(ctx context.Context) {
	leaseDuration := maxDuration(m.cfg.RequestTimeout*2, 30*time.Second)
	ticker := time.NewTicker(m.cfg.TickInterval)
	defer ticker.Stop()

	_ = m.tryBecomeLeader(ctx)
	if m.isLeader() {
		if n, err := m.repo.RecoverStaleRunningJobs(ctx, time.Now().UTC()); err == nil && n > 0 {
			slog.Info("steward recovered stale jobs", "count", n)
		}
	}

	for {
		if err := m.tick(ctx, leaseDuration); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("steward tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (m *Manager) tick(ctx context.Context, leaseDuration time.Duration) error {
	m.mu.Lock()
	paused := m.paused
	m.mu.Unlock()
	if paused {
		return nil
	}
	if !m.isLeader() {
		if err := m.tryBecomeLeader(ctx); err != nil {
			return err
		}
		if !m.isLeader() {
			return nil
		}
	}

	if m.cfg.AutoMergeFromSuggestions {
		if n, err := m.repo.EnqueueAutoMergeSuggestionJobs(ctx, m.cfg.AutoMergeThreshold, m.cfg.MaxAttempts, m.cfg.ClaimBatchSize*4); err != nil {
			slog.Warn("failed to enqueue auto-merge suggestion jobs", "error", err)
		} else if n > 0 {
			slog.Debug("enqueued auto-merge suggestion jobs", "count", n)
		}
	}

	jobs, err := m.repo.ClaimJobs(ctx, m.workerID, m.cfg.ClaimBatchSize, leaseDuration)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if err := m.executeJob(ctx, job); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("steward job execution failed", "job_id", job.ID, "job_type", job.JobType, "error", err)
		}
	}
	return nil
}

func (m *Manager) executeJob(parent context.Context, job Job) error {
	jobCtx, cancel := context.WithTimeout(parent, m.cfg.RequestTimeout)
	defer cancel()

	run, err := m.repo.CreateRun(jobCtx, job, "steward", m.cfg.Model)
	if err != nil {
		return err
	}
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "job_claimed", map[string]any{"worker_id": m.workerID, "attempt_count": job.AttemptCount})
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "run_started", map[string]any{"schema_version": 1, "dry_run": m.cfg.DryRun})
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "input_prepared", map[string]any{
		"job_type":          job.JobType,
		"project_id":        job.ProjectID,
		"source_memory_ids": job.SourceMemoryIDs,
	})

	executor, err := m.registry.ExecutorFor(job.JobType)
	if err != nil {
		return err
	}
	start := time.Now()
	result, execErr := executor.Execute(jobCtx, job)
	latencyMs := int(time.Since(start).Milliseconds())

	if execErr != nil {
		status, runAfter, markErr := m.repo.MarkFailure(jobCtx, job, run, execErr, true)
		if markErr != nil {
			return markErr
		}
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "job_failed", map[string]any{"status": status, "requeued_for": runAfter})
		return execErr
	}
	if result == nil {
		result = &ExecutionResult{Status: JobSucceeded, Decision: "empty_result", Output: map[string]any{}}
	}
	if result.LatencyMs == nil {
		result.LatencyMs = &latencyMs
	}
	if result.Model != "" || result.Provider != "" || result.TotalTokens != nil {
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "model_called", map[string]any{
			"provider":          result.Provider,
			"model":             result.Model,
			"prompt_tokens":     result.PromptTokens,
			"completion_tokens": result.CompletionTokens,
			"total_tokens":      result.TotalTokens,
			"latency_ms":        result.LatencyMs,
		})
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "model_returned", map[string]any{"decision": result.Decision})
	}
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "decision_emitted", map[string]any{
		"decision":     result.Decision,
		"side_effects": result.SideEffects,
	})

	if m.cfg.DryRun {
		if result.Output == nil {
			result.Output = map[string]any{}
		}
		result.Output["dry_run"] = true
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "write_attempted", map[string]any{"dry_run": true})
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "write_skipped", map[string]any{"reason": "dry_run", "side_effects": result.SideEffects})
	} else {
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "write_attempted", map[string]any{"dry_run": false})
		_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "write_applied", map[string]any{"decision": result.Decision, "side_effects": result.SideEffects})
	}

	if err := m.repo.MarkSucceeded(jobCtx, job, run, result); err != nil {
		return err
	}
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "run_succeeded", map[string]any{"decision": result.Decision, "latency_ms": latencyMs})
	_ = m.repo.AppendEvent(jobCtx, job.ID, &run.ID, "job_completed", map[string]any{"decision": result.Decision, "latency_ms": latencyMs})
	return nil
}

func (m *Manager) tryBecomeLeader(ctx context.Context) error {
	m.mu.Lock()
	if m.leader {
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	conn, err := m.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire leader conn: %w", err)
	}
	ok, err := m.repo.TryAcquireLeaderLock(ctx, conn, leaderLockKey)
	if err != nil {
		conn.Release()
		return err
	}
	if !ok {
		conn.Release()
		return nil
	}

	m.mu.Lock()
	m.lockConn = conn
	m.leader = true
	m.mu.Unlock()
	slog.Info("steward leader lock acquired", "worker_id", m.workerID)
	return nil
}

func (m *Manager) releaseLeaderLock(ctx context.Context) {
	m.mu.Lock()
	conn := m.lockConn
	leader := m.leader
	m.lockConn = nil
	m.leader = false
	m.mu.Unlock()
	if !leader || conn == nil {
		return
	}
	if err := m.repo.ReleaseLeaderLock(ctx, conn, leaderLockKey); err != nil {
		slog.Warn("failed to release steward leader lock", "error", err)
	}
	conn.Release()
}

func (m *Manager) isLeader() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.leader
}

type Status struct {
	Enabled      bool          `json:"enabled"`
	DryRun       bool          `json:"dry_run"`
	Paused       bool          `json:"paused"`
	IsLeader     bool          `json:"is_leader"`
	WorkerID     string        `json:"worker_id"`
	TickInterval time.Duration `json:"tick_interval"`
	Model        string        `json:"model"`
}

func (m *Manager) GetStatus() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{
		Enabled:      m.cfg.Enabled,
		DryRun:       m.cfg.DryRun,
		Paused:       m.paused,
		IsLeader:     m.leader,
		WorkerID:     m.workerID,
		TickInterval: m.cfg.TickInterval,
		Model:        m.cfg.Model,
	}
}

func (m *Manager) SetMode(paused, dryRun bool) Status {
	m.mu.Lock()
	m.paused = paused
	m.cfg.DryRun = dryRun
	m.mu.Unlock()
	return m.GetStatus()
}

func (m *Manager) RunOnce(ctx context.Context) error {
	return m.tick(ctx, maxDuration(m.cfg.RequestTimeout*2, 30*time.Second))
}

func (m *Manager) ListRuns(ctx context.Context, f RunFilters) ([]Run, error) {
	return m.repo.ListRuns(ctx, f)
}

func (m *Manager) ListEventsByJob(ctx context.Context, jobID uuid.UUID, limit, offset int) ([]Event, error) {
	return m.repo.ListEventsByJob(ctx, jobID, limit, offset)
}

func (m *Manager) Metrics(ctx context.Context) (*MetricsSummary, error) {
	return m.repo.GetMetricsSummary(ctx)
}

func (m *Manager) RetryJob(ctx context.Context, id uuid.UUID) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE steward_jobs
		SET status = 'queued', run_after = NOW(), locked_by = NULL, locked_at = NULL, lease_expires_at = NULL, updated_at = NOW()
		WHERE id = $1 AND status IN ('failed','dead_letter','cancelled')
	`, id)
	return err
}

func (m *Manager) CancelJob(ctx context.Context, id uuid.UUID) error {
	_, err := m.pool.Exec(ctx, `
		UPDATE steward_jobs
		SET status = 'cancelled', cancelled_at = NOW(), locked_by = NULL, locked_at = NULL, lease_expires_at = NULL, updated_at = NOW()
		WHERE id = $1 AND status IN ('queued','running')
	`, id)
	return err
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}
