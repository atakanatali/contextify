package steward

import (
	"time"

	"github.com/google/uuid"
)

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobRunning    JobStatus = "running"
	JobSucceeded  JobStatus = "succeeded"
	JobFailed     JobStatus = "failed"
	JobDeadLetter JobStatus = "dead_letter"
	JobCancelled  JobStatus = "cancelled"
)

type Job struct {
	ID              uuid.UUID
	JobType         string
	ProjectID       *string
	SourceMemoryIDs []uuid.UUID
	TriggerReason   *string
	Payload         map[string]any
	Status          JobStatus
	Priority        int
	AttemptCount    int
	MaxAttempts     int
	RunAfter        time.Time
	LockedBy        *string
	LockedAt        *time.Time
	LeaseExpiresAt  *time.Time
	LastError       *string
	IdempotencyKey  *string
	CancelledAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Run struct {
	ID               uuid.UUID
	JobID            *uuid.UUID
	JobType          *string
	ProjectID        *string
	JobStatus        *string
	Provider         *string
	Model            *string
	InputSnapshot    map[string]any
	OutputSnapshot   map[string]any
	InputHash        *string
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
	LatencyMs        *int
	Status           string
	ErrorClass       *string
	ErrorMessage     *string
	CreatedAt        time.Time
	CompletedAt      *time.Time
}

type Event struct {
	ID            uuid.UUID
	JobID         *uuid.UUID
	RunID         *uuid.UUID
	EventType     string
	Data          map[string]any
	SchemaVersion int
	CreatedAt     time.Time
}

type RunFilters struct {
	Status    *string
	JobType   *string
	ProjectID *string
	Model     *string
	Limit     int
	Offset    int
}

type MetricsSummary struct {
	RunsLastHour        int64              `json:"runs_last_hour"`
	SuccessRate         float64            `json:"success_rate"`
	AverageTokensPerRun float64            `json:"average_tokens_per_run"`
	P95LatencyMs        int64              `json:"p95_latency_ms"`
	TopFailureReasons   []FailureBreakdown `json:"top_failure_reasons"`
}

type FailureBreakdown struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

type QueueHealthSummary struct {
	QueuedTotal                int64               `json:"queued_total"`
	QueuedByProjectTop         []ProjectQueueDepth `json:"queued_by_project_top"`
	DeadLetterTotal            int64               `json:"dead_letter_total"`
	AverageProcessingLatencyMs int64               `json:"average_processing_latency_ms"`
}

type ProjectQueueDepth struct {
	ProjectID string `json:"project_id"`
	Count     int64  `json:"count"`
}

type Derivation struct {
	ID              uuid.UUID
	SourceMemoryIDs []uuid.UUID
	DerivedMemoryID *uuid.UUID
	DerivationType  string
	Confidence      *float32
	Novelty         *float32
	Status          string
	Model           *string
	Payload         map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type PolicyChange struct {
	ID           uuid.UUID
	PolicyKey    string
	PriorValue   *float64
	NewValue     *float64
	Reason       *string
	SampleSize   *int
	Evidence     map[string]any
	ChangedBy    string
	RollbackOfID *uuid.UUID
	CreatedAt    time.Time
}
