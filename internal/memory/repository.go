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
		       tags, importance, ttl_seconds, access_count, created_at, updated_at, expires_at,
		       version, merged_from, replaced_by
		FROM memories WHERE id = $1
	`
	mem := &Memory{}
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&mem.ID, &mem.Title, &mem.Content, &mem.Summary, &mem.Embedding,
		&mem.Type, &mem.Scope, &mem.ProjectID, &mem.AgentSource,
		&mem.Tags, &mem.Importance, &mem.TTLSeconds, &mem.AccessCount,
		&mem.CreatedAt, &mem.UpdatedAt, &mem.ExpiresAt,
		&mem.Version, &mem.MergedFrom, &mem.ReplacedBy,
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

// HybridSearch performs combined vector + keyword search, excluding replaced memories.
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

	// Exclude expired and replaced memories
	conditions = append(conditions, "(m.expires_at IS NULL OR m.expires_at > NOW())")
	conditions = append(conditions, "m.replaced_by IS NULL")
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

// GetStats returns memory statistics, excluding replaced memories from active counts.
func (r *Repository) GetStats(ctx context.Context) (*Stats, error) {
	stats := &Stats{
		ByType:  make(map[string]int),
		ByScope: make(map[string]int),
		ByAgent: make(map[string]int),
	}

	// Total active count (exclude replaced)
	err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE replaced_by IS NULL").Scan(&stats.TotalMemories)
	if err != nil {
		return nil, fmt.Errorf("count memories: %w", err)
	}

	// By type
	rows, err := r.pool.Query(ctx, "SELECT type, COUNT(*) FROM memories WHERE replaced_by IS NULL GROUP BY type")
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
	rows, err = r.pool.Query(ctx, "SELECT scope, COUNT(*) FROM memories WHERE replaced_by IS NULL GROUP BY scope")
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
	rows, err = r.pool.Query(ctx, "SELECT COALESCE(agent_source, 'unknown'), COUNT(*) FROM memories WHERE replaced_by IS NULL GROUP BY agent_source")
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
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE ttl_seconds IS NULL AND replaced_by IS NULL").Scan(&stats.LongTermCount)
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE ttl_seconds IS NOT NULL AND replaced_by IS NULL").Scan(&stats.ShortTermCount)

	// Expiring soon (next hour)
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM memories WHERE expires_at IS NOT NULL AND expires_at < NOW() + INTERVAL '1 hour' AND replaced_by IS NULL").Scan(&stats.ExpiringCount)

	// Pending consolidation suggestions
	r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM consolidation_suggestions WHERE status = 'pending'").Scan(&stats.PendingSuggestions)

	return stats, nil
}

// ListByProject returns all important memories for a given project, excluding replaced.
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
		  AND replaced_by IS NULL
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

// --- Consolidation repository methods ---

// FindSimilar finds memories with embedding similarity above a threshold.
func (r *Repository) FindSimilar(ctx context.Context, embedding pgvector.Vector, projectID *string, threshold float64, limit int) ([]SimilarMemory, error) {
	conditions := []string{
		"m.replaced_by IS NULL",
		"m.embedding IS NOT NULL",
		"(m.expires_at IS NULL OR m.expires_at > NOW())",
		fmt.Sprintf("1 - (m.embedding <=> $1) >= $%d", 2),
	}
	args := []any{embedding, threshold}
	argIdx := 3

	if projectID != nil {
		conditions = append(conditions, fmt.Sprintf("(m.project_id = $%d OR m.scope = 'global')", argIdx))
		args = append(args, *projectID)
		argIdx++
	}

	if limit <= 0 {
		limit = 5
	}
	args = append(args, limit)

	query := fmt.Sprintf(`
		SELECT m.id, m.title, m.content, m.summary, m.type, m.scope, m.project_id,
		       m.agent_source, m.tags, m.importance, m.ttl_seconds, m.access_count,
		       m.created_at, m.updated_at, m.expires_at,
		       1 - (m.embedding <=> $1) AS similarity
		FROM memories m
		WHERE %s
		ORDER BY similarity DESC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), argIdx)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("find similar: %w", err)
	}
	defer rows.Close()

	var results []SimilarMemory
	for rows.Next() {
		var sm SimilarMemory
		err := rows.Scan(
			&sm.Memory.ID, &sm.Memory.Title, &sm.Memory.Content, &sm.Memory.Summary,
			&sm.Memory.Type, &sm.Memory.Scope, &sm.Memory.ProjectID,
			&sm.Memory.AgentSource, &sm.Memory.Tags, &sm.Memory.Importance,
			&sm.Memory.TTLSeconds, &sm.Memory.AccessCount,
			&sm.Memory.CreatedAt, &sm.Memory.UpdatedAt, &sm.Memory.ExpiresAt,
			&sm.Similarity,
		)
		if err != nil {
			return nil, fmt.Errorf("scan similar memory: %w", err)
		}
		results = append(results, sm)
	}

	return results, rows.Err()
}

// MarkReplaced sets replaced_by on a memory, soft-deleting it.
func (r *Repository) MarkReplaced(ctx context.Context, sourceID, targetID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE memories SET replaced_by = $2 WHERE id = $1 AND replaced_by IS NULL",
		sourceID, targetID,
	)
	if err != nil {
		return fmt.Errorf("mark replaced: %w", err)
	}
	return nil
}

// UpdateMergeFields updates version, merged_from, and re-embeds a merged memory.
func (r *Repository) UpdateMergeFields(ctx context.Context, id uuid.UUID, title, content string, tags []string, importance float32, mergedFrom []uuid.UUID, newEmbedding *pgvector.Vector) error {
	query := `
		UPDATE memories
		SET title = $2, content = $3, tags = $4, importance = $5,
		    merged_from = $6, version = version + 1, embedding = $7
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, id, title, content, tags, importance, mergedFrom, newEmbedding)
	if err != nil {
		return fmt.Errorf("update merge fields: %w", err)
	}
	return nil
}

// CleanupReplaced hard-deletes memories that were replaced longer than retention ago.
func (r *Repository) CleanupReplaced(ctx context.Context, retention time.Duration) (int64, error) {
	query := `
		DELETE FROM memories
		WHERE replaced_by IS NOT NULL
		  AND updated_at < NOW() - $1::INTERVAL
	`
	result, err := r.pool.Exec(ctx, query, fmt.Sprintf("%d seconds", int(retention.Seconds())))
	if err != nil {
		return 0, fmt.Errorf("cleanup replaced: %w", err)
	}
	return result.RowsAffected(), nil
}

// StoreSuggestion inserts a consolidation suggestion, ignoring duplicates.
func (r *Repository) StoreSuggestion(ctx context.Context, memAID, memBID uuid.UUID, similarity float64, projectID *string) error {
	// Normalize order to avoid duplicates (smaller UUID first)
	a, b := memAID, memBID
	if a.String() > b.String() {
		a, b = b, a
	}
	query := `
		INSERT INTO consolidation_suggestions (memory_a_id, memory_b_id, similarity, project_id)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (memory_a_id, memory_b_id) DO UPDATE SET similarity = GREATEST(consolidation_suggestions.similarity, $3)
	`
	_, err := r.pool.Exec(ctx, query, a, b, similarity, projectID)
	if err != nil {
		return fmt.Errorf("store suggestion: %w", err)
	}
	return nil
}

// GetSuggestions returns consolidation suggestions with their associated memories.
func (r *Repository) GetSuggestions(ctx context.Context, projectID *string, status string, limit, offset int) ([]ConsolidationSuggestion, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if status == "" {
		status = "pending"
	}

	conditions := []string{"s.status = $1"}
	args := []any{status}
	argIdx := 2

	if projectID != nil {
		conditions = append(conditions, fmt.Sprintf("s.project_id = $%d", argIdx))
		args = append(args, *projectID)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count total
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM consolidation_suggestions s WHERE %s", where), countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count suggestions: %w", err)
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT s.id, s.memory_a_id, s.memory_b_id, s.similarity, s.status, s.project_id, s.created_at, s.resolved_at,
		       a.id, a.title, a.content, a.summary, a.type, a.scope, a.project_id, a.agent_source, a.tags, a.importance, a.ttl_seconds, a.access_count, a.created_at, a.updated_at, a.expires_at,
		       b.id, b.title, b.content, b.summary, b.type, b.scope, b.project_id, b.agent_source, b.tags, b.importance, b.ttl_seconds, b.access_count, b.created_at, b.updated_at, b.expires_at
		FROM consolidation_suggestions s
		JOIN memories a ON a.id = s.memory_a_id
		JOIN memories b ON b.id = s.memory_b_id
		WHERE %s
		ORDER BY s.similarity DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("get suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []ConsolidationSuggestion
	for rows.Next() {
		var s ConsolidationSuggestion
		var a, b Memory
		err := rows.Scan(
			&s.ID, &s.MemoryAID, &s.MemoryBID, &s.Similarity, &s.Status, &s.ProjectID, &s.CreatedAt, &s.ResolvedAt,
			&a.ID, &a.Title, &a.Content, &a.Summary, &a.Type, &a.Scope, &a.ProjectID, &a.AgentSource, &a.Tags, &a.Importance, &a.TTLSeconds, &a.AccessCount, &a.CreatedAt, &a.UpdatedAt, &a.ExpiresAt,
			&b.ID, &b.Title, &b.Content, &b.Summary, &b.Type, &b.Scope, &b.ProjectID, &b.AgentSource, &b.Tags, &b.Importance, &b.TTLSeconds, &b.AccessCount, &b.CreatedAt, &b.UpdatedAt, &b.ExpiresAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan suggestion: %w", err)
		}
		s.MemoryA = &a
		s.MemoryB = &b
		suggestions = append(suggestions, s)
	}

	return suggestions, total, rows.Err()
}

// UpdateSuggestionStatus updates a suggestion's status to accepted or dismissed.
func (r *Repository) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE consolidation_suggestions SET status = $2, resolved_at = NOW() WHERE id = $1",
		id, status,
	)
	if err != nil {
		return fmt.Errorf("update suggestion status: %w", err)
	}
	return nil
}

// StoreConsolidationLog records a merge operation.
func (r *Repository) StoreConsolidationLog(ctx context.Context, log *ConsolidationLog) error {
	query := `
		INSERT INTO consolidation_log (id, target_id, source_ids, merge_strategy, similarity_score, content_before, content_after, performed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		log.ID, log.TargetID, log.SourceIDs, log.MergeStrategy,
		log.SimilarityScore, log.ContentBefore, log.ContentAfter, log.PerformedBy,
	)
	if err != nil {
		return fmt.Errorf("store consolidation log: %w", err)
	}
	return nil
}

// GetConsolidationLog returns consolidation log entries.
func (r *Repository) GetConsolidationLog(ctx context.Context, targetID *uuid.UUID, limit, offset int) ([]ConsolidationLog, error) {
	if limit <= 0 {
		limit = 20
	}

	conditions := []string{}
	args := []any{}
	argIdx := 1

	if targetID != nil {
		conditions = append(conditions, fmt.Sprintf("target_id = $%d", argIdx))
		args = append(args, *targetID)
		argIdx++
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT id, target_id, source_ids, merge_strategy, similarity_score, content_before, content_after, performed_by, created_at
		FROM consolidation_log
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get consolidation log: %w", err)
	}
	defer rows.Close()

	var logs []ConsolidationLog
	for rows.Next() {
		var l ConsolidationLog
		err := rows.Scan(
			&l.ID, &l.TargetID, &l.SourceIDs, &l.MergeStrategy,
			&l.SimilarityScore, &l.ContentBefore, &l.ContentAfter, &l.PerformedBy, &l.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan consolidation log: %w", err)
		}
		logs = append(logs, l)
	}

	return logs, rows.Err()
}

// ScanDuplicates finds memory pairs with high vector similarity for a project.
func (r *Repository) ScanDuplicates(ctx context.Context, threshold float64, batchSize int) ([]struct {
	MemAID     uuid.UUID
	MemBID     uuid.UUID
	Similarity float64
	ProjectID  *string
}, error) {
	if batchSize <= 0 {
		batchSize = 100
	}
	query := `
		SELECT m1.id, m2.id, 1 - (m1.embedding <=> m2.embedding) AS similarity, m1.project_id
		FROM memories m1
		CROSS JOIN LATERAL (
			SELECT m2.id, m2.embedding
			FROM memories m2
			WHERE m2.id > m1.id
			  AND m2.replaced_by IS NULL
			  AND m2.embedding IS NOT NULL
			  AND (m2.expires_at IS NULL OR m2.expires_at > NOW())
			  AND (m2.project_id = m1.project_id OR (m2.project_id IS NULL AND m1.project_id IS NULL))
			  AND 1 - (m2.embedding <=> m1.embedding) >= $1
			ORDER BY m2.embedding <=> m1.embedding
			LIMIT 3
		) m2
		WHERE m1.replaced_by IS NULL
		  AND m1.embedding IS NOT NULL
		  AND (m1.expires_at IS NULL OR m1.expires_at > NOW())
		LIMIT $2
	`

	rows, err := r.pool.Query(ctx, query, threshold, batchSize)
	if err != nil {
		return nil, fmt.Errorf("scan duplicates: %w", err)
	}
	defer rows.Close()

	type pair struct {
		MemAID     uuid.UUID
		MemBID     uuid.UUID
		Similarity float64
		ProjectID  *string
	}
	var pairs []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.MemAID, &p.MemBID, &p.Similarity, &p.ProjectID); err != nil {
			return nil, fmt.Errorf("scan duplicate pair: %w", err)
		}
		pairs = append(pairs, p)
	}

	// Convert to return type
	result := make([]struct {
		MemAID     uuid.UUID
		MemBID     uuid.UUID
		Similarity float64
		ProjectID  *string
	}, len(pairs))
	for i, p := range pairs {
		result[i] = struct {
			MemAID     uuid.UUID
			MemBID     uuid.UUID
			Similarity float64
			ProjectID  *string
		}(p)
	}

	return result, rows.Err()
}

// ListDistinctProjectIDs returns all unique non-null project_id values from the memories table.
func (r *Repository) ListDistinctProjectIDs(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT project_id FROM memories WHERE project_id IS NOT NULL AND project_id != '' ORDER BY project_id`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list distinct project ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan project id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// UpdateProjectID updates all memories and consolidation_suggestions with oldID to newID.
func (r *Repository) UpdateProjectID(ctx context.Context, oldID, newID string) (int, error) {
	total := 0

	// Update memories
	tag, err := r.pool.Exec(ctx, `UPDATE memories SET project_id = $1 WHERE project_id = $2`, newID, oldID)
	if err != nil {
		return 0, fmt.Errorf("update memories project_id: %w", err)
	}
	total += int(tag.RowsAffected())

	// Update consolidation_suggestions
	tag, err = r.pool.Exec(ctx, `UPDATE consolidation_suggestions SET project_id = $1 WHERE project_id = $2`, newID, oldID)
	if err != nil {
		return total, fmt.Errorf("update suggestions project_id: %w", err)
	}
	total += int(tag.RowsAffected())

	return total, nil
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
