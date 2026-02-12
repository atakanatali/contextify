package memory

import (
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

type MemoryType string

const (
	TypeSolution     MemoryType = "solution"
	TypeProblem      MemoryType = "problem"
	TypeCodePattern  MemoryType = "code_pattern"
	TypeFix          MemoryType = "fix"
	TypeError        MemoryType = "error"
	TypeWorkflow     MemoryType = "workflow"
	TypeDecision     MemoryType = "decision"
	TypeGeneral      MemoryType = "general"
	TypeTask         MemoryType = "task"
	TypeTechnology   MemoryType = "technology"
	TypeCommand      MemoryType = "command"
	TypeFileContext  MemoryType = "file_context"
	TypeConversation MemoryType = "conversation"
	TypeProject      MemoryType = "project"
)

// ValidTypes contains all known memory types accepted by the database.
var ValidTypes = map[MemoryType]bool{
	TypeSolution: true, TypeProblem: true, TypeCodePattern: true,
	TypeFix: true, TypeError: true, TypeWorkflow: true,
	TypeDecision: true, TypeGeneral: true, TypeTask: true,
	TypeTechnology: true, TypeCommand: true, TypeFileContext: true,
	TypeConversation: true, TypeProject: true,
}

// NormalizeType returns the type as-is if valid, or TypeGeneral for unknown types.
func NormalizeType(t MemoryType) MemoryType {
	if t == "" {
		return TypeGeneral
	}
	if ValidTypes[t] {
		return t
	}
	return TypeGeneral
}

type MemoryScope string

const (
	ScopeGlobal  MemoryScope = "global"
	ScopeProject MemoryScope = "project"
)

type Memory struct {
	ID          uuid.UUID        `json:"id"`
	Title       string           `json:"title"`
	Content     string           `json:"content"`
	Summary     *string          `json:"summary,omitempty"`
	Embedding   *pgvector.Vector `json:"-"`
	Type        MemoryType       `json:"type"`
	Scope       MemoryScope      `json:"scope"`
	ProjectID   *string          `json:"project_id,omitempty"`
	AgentSource *string          `json:"agent_source,omitempty"`
	Tags        []string         `json:"tags"`
	Importance  float32          `json:"importance"`
	TTLSeconds  *int             `json:"ttl_seconds,omitempty"`
	AccessCount int              `json:"access_count"`
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
	ExpiresAt   *time.Time       `json:"expires_at,omitempty"`
	Version     int              `json:"version"`
	MergedFrom  []uuid.UUID      `json:"merged_from,omitempty"`
	ReplacedBy  *uuid.UUID       `json:"replaced_by,omitempty"`
}

type Relationship struct {
	ID           uuid.UUID `json:"id"`
	FromMemoryID uuid.UUID `json:"from_memory_id"`
	ToMemoryID   uuid.UUID `json:"to_memory_id"`
	Relationship string    `json:"relationship"`
	Strength     float32   `json:"strength"`
	Context      *string   `json:"context,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type StoreRequest struct {
	Title       string      `json:"title"`
	Content     string      `json:"content"`
	Summary     *string     `json:"summary,omitempty"`
	Type        MemoryType  `json:"type"`
	Scope       MemoryScope `json:"scope"`
	ProjectID   *string     `json:"project_id,omitempty"`
	AgentSource *string     `json:"agent_source,omitempty"`
	Tags        []string    `json:"tags"`
	Importance  float32     `json:"importance"`
	TTLSeconds  *int        `json:"ttl_seconds,omitempty"`
}

type UpdateRequest struct {
	Title      *string     `json:"title,omitempty"`
	Content    *string     `json:"content,omitempty"`
	Summary    *string     `json:"summary,omitempty"`
	Type       *MemoryType `json:"type,omitempty"`
	Tags       []string    `json:"tags,omitempty"`
	Importance *float32    `json:"importance,omitempty"`
}

type SearchRequest struct {
	Query         string       `json:"query"`
	Type          *MemoryType  `json:"type,omitempty"`
	Scope         *MemoryScope `json:"scope,omitempty"`
	ProjectID     *string      `json:"project_id,omitempty"`
	AgentSource   *string      `json:"agent_source,omitempty"`
	Tags          []string     `json:"tags,omitempty"`
	MinImportance *float32     `json:"min_importance,omitempty"`
	Limit         int          `json:"limit"`
	Offset        int          `json:"offset"`
}

type SearchResult struct {
	Memory    Memory  `json:"memory"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"` // "semantic", "keyword", "hybrid"
}

type TelemetryEventType string

const (
	TelemetryRecallAttempt    TelemetryEventType = "recall_attempt"
	TelemetryRecallHit        TelemetryEventType = "recall_hit"
	TelemetryStoreOpportunity TelemetryEventType = "store_opportunity"
	TelemetryStoreAction      TelemetryEventType = "store_action"
)

type TelemetryEvent struct {
	ID          uuid.UUID          `json:"id"`
	EventType   TelemetryEventType `json:"event_type"`
	SessionID   *string            `json:"session_id,omitempty"`
	RequestID   *string            `json:"request_id,omitempty"`
	AgentSource *string            `json:"agent_source,omitempty"`
	ProjectID   *string            `json:"project_id,omitempty"`
	MemoryID    *uuid.UUID         `json:"memory_id,omitempty"`
	QueryText   *string            `json:"query_text,omitempty"`
	Action      *string            `json:"action,omitempty"`
	HitCount    *int               `json:"hit_count,omitempty"`
	LatencyMs   *int               `json:"latency_ms,omitempty"`
	Metadata    map[string]any     `json:"metadata,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
}

type RelationshipRequest struct {
	FromMemoryID uuid.UUID `json:"from_memory_id"`
	ToMemoryID   uuid.UUID `json:"to_memory_id"`
	Relationship string    `json:"relationship"`
	Strength     float32   `json:"strength"`
	Context      *string   `json:"context,omitempty"`
}

type Stats struct {
	TotalMemories      int            `json:"total_memories"`
	ByType             map[string]int `json:"by_type"`
	ByScope            map[string]int `json:"by_scope"`
	ByAgent            map[string]int `json:"by_agent"`
	LongTermCount      int            `json:"long_term_count"`
	ShortTermCount     int            `json:"short_term_count"`
	ExpiringCount      int            `json:"expiring_count"`
	PendingSuggestions int            `json:"pending_suggestions"`
}

type AnalyticsData struct {
	TotalTokensStored   int64            `json:"total_tokens_stored"`
	TotalTokensSaved    int64            `json:"total_tokens_saved"`
	TotalHits           int64            `json:"total_hits"`
	HitRate             float64          `json:"hit_rate"`
	TopAccessedMemories []MemorySummary  `json:"top_accessed_memories"`
	TokensByAgent       map[string]int64 `json:"tokens_by_agent"`
	Timeline            []TimelineEntry  `json:"timeline"`
}

type FunnelAnalyticsRequest struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	AgentSource *string   `json:"agent_source,omitempty"`
	ProjectID   *string   `json:"project_id,omitempty"`
}

type FunnelAnalyticsData struct {
	RecallAttempts     int64                 `json:"recall_attempts"`
	RecallHits         int64                 `json:"recall_hits"`
	StoreOpportunities int64                 `json:"store_opportunities"`
	StoreActions       int64                 `json:"store_actions"`
	RecallHitRate      float64               `json:"recall_hit_rate"`
	StoreCaptureRate   float64               `json:"store_capture_rate"`
	Timeline           []FunnelTimelineEntry `json:"timeline"`
	ByAgent            []FunnelBreakdown     `json:"by_agent"`
	ByProject          []FunnelBreakdown     `json:"by_project"`
}

type FunnelTimelineEntry struct {
	Date               string `json:"date"`
	RecallAttempts     int64  `json:"recall_attempts"`
	RecallHits         int64  `json:"recall_hits"`
	StoreOpportunities int64  `json:"store_opportunities"`
	StoreActions       int64  `json:"store_actions"`
}

type FunnelBreakdown struct {
	Key                string `json:"key"`
	RecallAttempts     int64  `json:"recall_attempts"`
	RecallHits         int64  `json:"recall_hits"`
	StoreOpportunities int64  `json:"store_opportunities"`
	StoreActions       int64  `json:"store_actions"`
}

type MemorySummary struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Type        MemoryType `json:"type"`
	AccessCount int        `json:"access_count"`
	TokenCount  int        `json:"token_count"`
	AgentSource *string    `json:"agent_source,omitempty"`
}

type TimelineEntry struct {
	Date    string `json:"date"`
	Created int    `json:"created"`
	Hits    int    `json:"hits"`
}

// StoreResult is returned by Store() with dedup information.
type StoreResult struct {
	Memory          *Memory         `json:"memory"`
	Action          string          `json:"action"` // "created", "updated", "created_with_suggestions"
	UpdatedExisting *Memory         `json:"updated_existing,omitempty"`
	Suggestions     []SimilarMemory `json:"suggestions,omitempty"`
}

// SimilarMemory pairs a memory with its similarity score.
type SimilarMemory struct {
	Memory     Memory  `json:"memory"`
	Similarity float64 `json:"similarity"`
}

// ConsolidationLog records a merge operation for audit.
type ConsolidationLog struct {
	ID              uuid.UUID   `json:"id"`
	TargetID        uuid.UUID   `json:"target_id"`
	SourceIDs       []uuid.UUID `json:"source_ids"`
	MergeStrategy   string      `json:"merge_strategy"`
	SimilarityScore *float64    `json:"similarity_score,omitempty"`
	ContentBefore   string      `json:"content_before"`
	ContentAfter    string      `json:"content_after"`
	PerformedBy     string      `json:"performed_by"`
	CreatedAt       time.Time   `json:"created_at"`
}

// ConsolidationSuggestion represents a pair of memories that may be duplicates.
type ConsolidationSuggestion struct {
	ID         uuid.UUID  `json:"id"`
	MemoryAID  uuid.UUID  `json:"memory_a_id"`
	MemoryBID  uuid.UUID  `json:"memory_b_id"`
	Similarity float64    `json:"similarity"`
	Status     string     `json:"status"` // "pending", "accepted", "dismissed"
	ProjectID  *string    `json:"project_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	// Populated when fetching with memories
	MemoryA *Memory `json:"memory_a,omitempty"`
	MemoryB *Memory `json:"memory_b,omitempty"`
}

// MergeRequest is the API request for merging memories.
type MergeRequest struct {
	SourceIDs []uuid.UUID `json:"source_ids"`
	Strategy  string      `json:"strategy,omitempty"`
}

// BatchConsolidateRequest is the API request for batch merge.
type BatchConsolidateRequest struct {
	Operations []BatchMergeOp `json:"operations"`
	Strategy   string         `json:"strategy,omitempty"`
}

// BatchMergeOp represents one merge operation in a batch.
type BatchMergeOp struct {
	TargetID  uuid.UUID   `json:"target_id"`
	SourceIDs []uuid.UUID `json:"source_ids"`
}

// SuggestionStatusUpdate is the API request for updating suggestion status.
type SuggestionStatusUpdate struct {
	Status string `json:"status"` // "accepted", "dismissed"
}
