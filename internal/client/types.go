package client

import "time"

type Memory struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Summary     *string   `json:"summary,omitempty"`
	Type        string    `json:"type"`
	Scope       string    `json:"scope"`
	ProjectID   *string   `json:"project_id,omitempty"`
	AgentSource *string   `json:"agent_source,omitempty"`
	Tags        []string  `json:"tags"`
	Importance  float32   `json:"importance"`
	TTLSeconds  *int      `json:"ttl_seconds,omitempty"`
	AccessCount int       `json:"access_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

type StoreResult struct {
	Memory *Memory `json:"memory"`
	Action string  `json:"action"`
}

type PromoteResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

type StoreRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Summary     *string  `json:"summary,omitempty"`
	Type        string   `json:"type"`
	Scope       string   `json:"scope"`
	ProjectID   *string  `json:"project_id,omitempty"`
	AgentSource *string  `json:"agent_source,omitempty"`
	Tags        []string `json:"tags"`
	Importance  float32  `json:"importance"`
	TTLSeconds  *int     `json:"ttl_seconds,omitempty"`
}

type UpdateRequest struct {
	Title      *string  `json:"title,omitempty"`
	Content    *string  `json:"content,omitempty"`
	Summary    *string  `json:"summary,omitempty"`
	Type       *string  `json:"type,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Importance *float32 `json:"importance,omitempty"`
}

type SearchRequest struct {
	Query         string   `json:"query"`
	Type          *string  `json:"type,omitempty"`
	Scope         *string  `json:"scope,omitempty"`
	ProjectID     *string  `json:"project_id,omitempty"`
	AgentSource   *string  `json:"agent_source,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	MinImportance *float32 `json:"min_importance,omitempty"`
	Limit         int      `json:"limit"`
	Offset        int      `json:"offset"`
}

type SearchResult struct {
	Memory    Memory  `json:"memory"`
	Score     float64 `json:"score"`
	MatchType string  `json:"match_type"`
}

type Stats struct {
	TotalMemories  int            `json:"total_memories"`
	ByType         map[string]int `json:"by_type"`
	ByScope        map[string]int `json:"by_scope"`
	ByAgent        map[string]int `json:"by_agent"`
	LongTermCount  int            `json:"long_term_count"`
	ShortTermCount int            `json:"short_term_count"`
	ExpiringCount  int            `json:"expiring_count"`
}
