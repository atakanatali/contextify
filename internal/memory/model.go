package memory

import (
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"
)

type MemoryType string

const (
	TypeSolution    MemoryType = "solution"
	TypeProblem     MemoryType = "problem"
	TypeCodePattern MemoryType = "code_pattern"
	TypeFix         MemoryType = "fix"
	TypeError       MemoryType = "error"
	TypeWorkflow    MemoryType = "workflow"
	TypeDecision    MemoryType = "decision"
	TypeGeneral     MemoryType = "general"
)

type MemoryScope string

const (
	ScopeGlobal  MemoryScope = "global"
	ScopeProject MemoryScope = "project"
)

type Memory struct {
	ID          uuid.UUID       `json:"id"`
	Title       string          `json:"title"`
	Content     string          `json:"content"`
	Summary     *string         `json:"summary,omitempty"`
	Embedding   *pgvector.Vector `json:"-"`
	Type        MemoryType      `json:"type"`
	Scope       MemoryScope     `json:"scope"`
	ProjectID   *string         `json:"project_id,omitempty"`
	AgentSource *string         `json:"agent_source,omitempty"`
	Tags        []string        `json:"tags"`
	Importance  float32         `json:"importance"`
	TTLSeconds  *int            `json:"ttl_seconds,omitempty"`
	AccessCount int             `json:"access_count"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
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
	Query         string      `json:"query"`
	Type          *MemoryType `json:"type,omitempty"`
	Scope         *MemoryScope `json:"scope,omitempty"`
	ProjectID     *string     `json:"project_id,omitempty"`
	AgentSource   *string     `json:"agent_source,omitempty"`
	Tags          []string    `json:"tags,omitempty"`
	MinImportance *float32    `json:"min_importance,omitempty"`
	Limit         int         `json:"limit"`
	Offset        int         `json:"offset"`
}

type SearchResult struct {
	Memory     Memory  `json:"memory"`
	Score      float64 `json:"score"`
	MatchType  string  `json:"match_type"` // "semantic", "keyword", "hybrid"
}

type RelationshipRequest struct {
	FromMemoryID uuid.UUID `json:"from_memory_id"`
	ToMemoryID   uuid.UUID `json:"to_memory_id"`
	Relationship string    `json:"relationship"`
	Strength     float32   `json:"strength"`
	Context      *string   `json:"context,omitempty"`
}

type Stats struct {
	TotalMemories   int            `json:"total_memories"`
	ByType          map[string]int `json:"by_type"`
	ByScope         map[string]int `json:"by_scope"`
	ByAgent         map[string]int `json:"by_agent"`
	LongTermCount   int            `json:"long_term_count"`
	ShortTermCount  int            `json:"short_term_count"`
	ExpiringCount   int            `json:"expiring_count"`
}
