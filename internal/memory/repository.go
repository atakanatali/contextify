package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Store(ctx context.Context, mem *Memory) error {
	query := `
		INSERT INTO memories (id, title, content, summary, embedding, type, scope, project_id, agent_source, tags, importance, ttl_seconds, access_count, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := r.pool.Exec(ctx, query,
		mem.ID, mem.Title, mem.Content, mem.Summary, mem.Embedding,
		mem.Type, mem.Scope, mem.ProjectID, mem.AgentSource,
		mem.Tags, mem.Importance, mem.TTLSeconds, mem.AccessCount, mem.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("store memory: %w", err)
	}
	return nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (*Memory, error) {
	query := `
		SELECT id, title, content, summary, embedding, type, scope, project_id, agent_source,
		       tags, importance, ttl_seconds, access_count, created_at, updated_at, expires_at
		FROM memories WHERE id = $1
	`
	mem := &Memory{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&mem.ID, &mem.Title, &mem.Content, &mem.Summary, &mem.Embedding,
		&mem.Type, &mem.Scope, &mem.ProjectID, &mem.AgentSource,
		&mem.Tags, &mem.Importance, &mem.TTLSeconds, &mem.AccessCount,
		&mem.CreatedAt, &mem.UpdatedAt, &mem.ExpiresAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}
	return mem, nil
}

func (r *Repository) Update(ctx context.Context, id uuid.UUID, req UpdateRequest, newEmbedding *pgvector.Vector) error {
	sets := []string{}
	args := []any{}
	argIdx := 1

	if req.Title != nil {
		sets = append(sets, fmt.Sprintf("title = $%d", argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Content != nil {
		sets = append(sets, fmt.Sprintf("content = $%d", argIdx))
		args = append(args, *req.Content)
		argIdx++
	}
	if req.Summary != nil {
		sets = append(sets, fmt.Sprintf("summary = $%d", argIdx))
		args = append(args, *req.Summary)
		argIdx++
	}
	if req.Type != nil {
		sets = append(sets, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *req.Type)
		argIdx++
	}
	if req.Tags != nil {
		sets = append(sets, fmt.Sprintf("tags = $%d", argIdx))
		args = append(args, req.Tags)
		argIdx++
	}
	if req.Importance != nil {
		sets = append(sets, fmt.Sprintf("importance = $%d", argIdx))
		args = append(args, *req.Importance)
		argIdx++
	}
	if newEmbedding != nil {
		sets = append(sets, fmt.Sprintf("embedding = $%d", argIdx))
		args = append(args, newEmbedding)
		argIdx++
	}

	if len(sets) == 0 {
		return nil
	}

	args = append(args, id)
	query := fmt.Sprintf("UPDATE memories SET %s WHERE id = $%d", strings.Join(sets, ", "), argIdx)

	_, err := r.pool.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM memories WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	return nil
}

// HybridSearch performs combined vector + keyword search.
func (r *Repository) HybridSearch(ctx context.Context, queryEmbedding pgvector.Vector, req SearchRequest, vectorWeight, keywordWeight float64) ([]SearchResult, error) {
	conditions := []string{}
	args := []any{queryEmbedding}
	argIdx := 2

	if req.Type != nil {
		conditions = append(conditions, fmt.Sprintf("m.type = $%d", argIdx))
		args = append(args, *req.Type)
		argIdx++
	}
	if req.Scope != nil {
		conditions = append(conditions, fmt.Sprintf("m.scope = $%d", argIdx))
		args = append(args, *req.Scope)
		argIdx++
	}
	if req.ProjectID != nil {
		conditions = append(conditions, fmt.Sprintf("(m.project_id = $%d OR m.scope = 'global')", argIdx))
		args = append(args, *req.ProjectID)
		argIdx++
	}
	if req.AgentSource != nil {
		conditions = append(conditions, fmt.Sprintf("m.agent_source = $%d", argIdx))
		args = append(args, *req.AgentSource)
		argIdx++
	}
	if len(req.Tags) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.tags @> $%d", argIdx))
		args = append(args, req.Tags)
		argIdx++
	}
	if req.MinImportance != nil {
		conditions = append(conditions, fmt.Sprintf("m.importance >= $%d", argIdx))
		args = append(args, *req.MinImportance)
		argIdx++
	}

	// Exclude expired memories
	conditions = append(conditions, "(m.expires_at IS NULL OR m.expires_at > NOW())")
	// Require embedding to exist for vector search
	conditions = append(conditions, "m.embedding IS NOT NULL")

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := req.Offset
	if offset < 0 {
		offset = 0
	}

	args = append(args, req.Query, vectorWeight, keywordWeight, limit, offset)
	queryArgIdx := argIdx
	vwArgIdx := argIdx + 1
	kwArgIdx := argIdx + 2
	limitArgIdx := argIdx + 3
	offsetArgIdx := argIdx + 4

	query := fmt.Sprintf(`
		WITH vector_scores AS (
			SELECT m.id,
			       1 - (m.embedding <=> $1) AS vector_score
			FROM memories m
			%s
		),
		keyword_scores AS (
			SELECT m.id,
			       ts_rank(to_tsvector('english', m.content), plainto_tsquery('english', $%d)) AS keyword_score
			FROM memories m
			%s
		)
		SELECT m.id, m.title, m.content, m.summary, m.type, m.scope, m.project_id,
		       m.agent_source, m.tags, m.importance, m.ttl_seconds, m.access_count,
		       m.created_at, m.updated_at, m.expires_at,
		       COALESCE(v.vector_score, 0) * $%d + COALESCE(k.keyword_score, 0) * $%d AS combined_score
		FROM memories m
		LEFT JOIN vector_scores v ON m.id = v.id
		LEFT JOIN keyword_scores k ON m.id = k.id
		%s
		ORDER BY combined_score DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, queryArgIdx, whereClause, vwArgIdx, kwArgIdx, whereClause, limitArgIdx, offsetArgIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var sr SearchResult
		err := rows.Scan(
			&sr.Memory.ID, &sr.Memory.Title, &sr.Memory.Content, &sr.Memory.Summary,
			&sr.Memory.Type, &sr.Memory.Scope, &sr.Memory.ProjectID,
			&sr.Memory.AgentSource, &sr.Memory.Tags, &sr.Memory.Importance,
			&sr.Memory.TTLSeconds, &sr.Memory.AccessCount,
			&sr.Memory.CreatedAt, &sr.Memory.UpdatedAt, &sr.Memory.ExpiresAt,
			&sr.Score,
		)
		if err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		sr.MatchType = "hybrid"
		results = append(results, sr)
	}

	return results, rows.Err()
}

func (r *Repository) IncrementAccess(ctx context.Context, id uuid.UUID, ttlExtendFactor float64) error {
	query := `
		UPDATE memories
		SET access_count = access_count + 1,
		    expires_at = CASE
		        WHEN ttl_seconds IS NOT NULL AND expires_at IS NOT NULL
		        THEN expires_at + (ttl_seconds * $2 || ' seconds')::INTERVAL
		        ELSE expires_at
		    END
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, ttlExtendFactor)
	if err != nil {
		return fmt.Errorf("increment access: %w", err)
	}
	return nil
}

func (r *Repository) PromoteToLongTerm(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, "UPDATE memories SET ttl_seconds = NULL, expires_at = NULL WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("promote to long-term: %w", err)
	}
	return nil
}

func (r *Repository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.pool.Exec(ctx, "DELETE FROM memories WHERE expires_at IS NOT NULL AND expires_at < NOW()")
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	return result.RowsAffected(), nil
}

// StoreRelationship creates a relationship between two memories.
func (r *Repository) StoreRelationship(ctx context.Context, rel *Relationship) error {
	query := `
		INSERT INTO memory_relationships (id, from_memory_id, to_memory_id, relationship, strength, context)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (from_memory_id, to_memory_id, relationship) DO UPDATE
		SET strength = $5, context = $6
	`
	_, err := r.pool.Exec(ctx, query, rel.ID, rel.FromMemoryID, rel.ToMemoryID, rel.Relationship, rel.Strength, rel.Context)
	if err != nil {
		return fmt.Errorf("store relationship: %w", err)
	}
	return nil
}

// GetRelated returns memories related to the given memory ID.
func (r *Repository) GetRelated(ctx context.Context, memoryID uuid.UUID, relationshipTypes []string) ([]Memory, []Relationship, error) {
	conditions := "WHERE r.from_memory_id = $1 OR r.to_memory_id = $1"
	args := []any{memoryID}

	if len(relationshipTypes) > 0 {
		conditions += " AND r.relationship = ANY($2)"
		args = append(args, relationshipTypes)
	}

	query := fmt.Sprintf(`
		SELECT m.id, m.title, m.content, m.summary, m.type, m.scope, m.project_id,
		       m.agent_source, m.tags, m.importance, m.ttl_seconds, m.access_count,
		       m.created_at, m.updated_at, m.expires_at,
		       r.id, r.from_memory_id, r.to_memory_id, r.relationship, r.strength, r.context, r.created_at
		FROM memory_relationships r
		JOIN memories m ON (m.id = r.from_memory_id OR m.id = r.to_memory_id) AND m.id != $1
		%s
		ORDER BY r.strength DESC
	`, conditions)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("get related: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	var relationships []Relationship
	for rows.Next() {
		var m Memory
		var rel Relationship
		err := rows.Scan(
			&m.ID, &m.Title, &m.Content, &m.Summary, &m.Type, &m.Scope, &m.ProjectID,
			&m.AgentSource, &m.Tags, &m.Importance, &m.TTLSeconds, &m.AccessCount,
			&m.CreatedAt, &m.UpdatedAt, &m.ExpiresAt,
			&rel.ID, &rel.FromMemoryID, &rel.ToMemoryID, &rel.Relationship, &rel.Strength, &rel.Context, &rel.CreatedAt,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("scan related: %w", err)
		}
		memories = append(memories, m)
		relationships = append(relationships, rel)
	}

	return memories, relationships, rows.Err()
}

// GetStats returns memory statistics.
func (r *Repository) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		ByType:  make(map[string]int),
		ByScope: make(map[string]int),
		ByAgent: make(map[string]int),
	}

	// Total count
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories").Scan(&stats.TotalMemories)
	if err != nil {
		return nil, fmt.Errorf("count memories: %w", err)
	}

	// By type
	rows, err := r.pool.Query(ctx, "SELECT type, COUNT(*) FROM memories GROUP BY type")
	if err != nil {
		return nil, fmt.Errorf("count by type: %w", err)
	}
	for rows.Next() {
		var t string
		var c int
		rows.Scan(&t, &c)
		stats.ByType[t] = c
	}
	rows.Close()

	// By scope
	rows, err = r.pool.Query(ctx, "SELECT scope, COUNT(*) FROM memories GROUP BY scope")
	if err != nil {
		return nil, fmt.Errorf("count by scope: %w", err)
	}
	for rows.Next() {
		var s string
		var c int
		rows.Scan(&s, &c)
		stats.ByScope[s] = c
	}
	rows.Close()

	// By agent
	rows, err = r.pool.Query(ctx, "SELECT COALESCE(agent_source, 'unknown'), COUNT(*) FROM memories GROUP BY agent_source")
	if err != nil {
		return nil, fmt.Errorf("count by agent: %w", err)
	}
	for rows.Next() {
		var a string
		var c int
		rows.Scan(&a, &c)
		stats.ByAgent[a] = c
	}
	rows.Close()

	// Long-term vs short-term
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE ttl_seconds IS NULL").Scan(&stats.LongTermCount)
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE ttl_seconds IS NOT NULL").Scan(&stats.ShortTermCount)

	// Expiring soon (next hour)
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE expires_at IS NOT NULL AND expires_at < NOW() + INTERVAL '1 hour'").Scan(&stats.ExpiringCount)

	return stats, nil
}

// ListByProject returns all important memories for a given project.
func (r *Repository) ListByProject(ctx context.Context, projectID string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `
		SELECT id, title, content, summary, type, scope, project_id, agent_source,
		       tags, importance, ttl_seconds, access_count, created_at, updated_at, expires_at
		FROM memories
		WHERE (project_id = $1 OR scope = 'global')
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY importance DESC, updated_at DESC
		LIMIT $2
	`
	rows, err := r.pool.Query(ctx, query, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list by project: %w", err)
	}
	defer rows.Close()

	var memories []Memory
	for rows.Next() {
		var m Memory
		err := rows.Scan(
			&m.ID, &m.Title, &m.Content, &m.Summary, &m.Type, &m.Scope, &m.ProjectID,
			&m.AgentSource, &m.Tags, &m.Importance, &m.TTLSeconds, &m.AccessCount,
			&m.CreatedAt, &m.UpdatedAt, &m.ExpiresAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		memories = append(memories, m)
	}

	return memories, rows.Err()
}

// GetAnalytics returns analytics data for the dashboard.
func (r *Repository) GetAnalytics(ctx context.Context) (*AnalyticsData, error) {
	data := &AnalyticsData{
		TokensByAgent: make(map[string]int64),
	}

	// Token metrics + hit rate
	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(LENGTH(content) / 4), 0),
			COALESCE(SUM(LENGTH(content) * access_count / 4), 0),
			COALESCE(SUM(access_count), 0),
			CASE WHEN COUNT(*) > 0
				THEN COUNT(*) FILTER (WHERE access_count > 0)::float / COUNT(*)
				ELSE 0
			END
		FROM memories
	`).Scan(&data.TotalTokensStored, &data.TotalTokensSaved, &data.TotalHits, &data.HitRate)
	if err != nil {
		return nil, fmt.Errorf("analytics token metrics: %w", err)
	}

	// Top accessed memories
	rows, err := r.pool.Query(ctx, `
		SELECT id, title, type, access_count, LENGTH(content)/4, agent_source
		FROM memories
		WHERE access_count > 0
		ORDER BY access_count DESC
		LIMIT 10
	`)
	if err != nil {
		return nil, fmt.Errorf("analytics top accessed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var m MemorySummary
		if err := rows.Scan(&m.ID, &m.Title, &m.Type, &m.AccessCount, &m.TokenCount, &m.AgentSource); err != nil {
			return nil, fmt.Errorf("scan top accessed: %w", err)
		}
		data.TopAccessedMemories = append(data.TopAccessedMemories, m)
	}
	rows.Close()

	if data.TopAccessedMemories == nil {
		data.TopAccessedMemories = []MemorySummary{}
	}

	// Tokens saved by agent
	rows, err = r.pool.Query(ctx, `
		SELECT COALESCE(agent_source, 'unknown'), COALESCE(SUM(LENGTH(content) * access_count / 4), 0)
		FROM memories
		GROUP BY agent_source
		HAVING SUM(LENGTH(content) * access_count / 4) > 0
	`)
	if err != nil {
		return nil, fmt.Errorf("analytics tokens by agent: %w", err)
	}
	for rows.Next() {
		var agent string
		var tokens int64
		rows.Scan(&agent, &tokens)
		data.TokensByAgent[agent] = tokens
	}
	rows.Close()

	// Timeline (last 30 days)
	rows, err = r.pool.Query(ctx, `
		SELECT d::date,
			COALESCE(COUNT(m.id), 0),
			COALESCE(SUM(m.access_count), 0)
		FROM generate_series(NOW() - INTERVAL '29 days', NOW(), '1 day') d
		LEFT JOIN memories m ON m.created_at::date = d::date
		GROUP BY d::date
		ORDER BY d::date
	`)
	if err != nil {
		return nil, fmt.Errorf("analytics timeline: %w", err)
	}
	for rows.Next() {
		var e TimelineEntry
		var d time.Time
		rows.Scan(&d, &e.Created, &e.Hits)
		e.Date = d.Format("2006-01-02")
		data.Timeline = append(data.Timeline, e)
	}
	rows.Close()

	if data.Timeline == nil {
		data.Timeline = []TimelineEntry{}
	}

	return data, nil
}

// Ensure unused import doesn't cause error
var _ = time.Now
