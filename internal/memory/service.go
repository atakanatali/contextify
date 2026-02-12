package memory

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/pgvector/pgvector-go"

	"github.com/atakanatali/contextify/internal/config"
	"github.com/atakanatali/contextify/internal/embedding"
)

type Service struct {
	repo       *Repository
	embedder   *embedding.Client
	normalizer *ProjectNormalizer
	cfg        config.MemoryConfig
	searchCfg  config.SearchConfig
}

func NewService(repo *Repository, embedder *embedding.Client, cfg config.MemoryConfig, searchCfg config.SearchConfig) *Service {
	return &Service{
		repo:       repo,
		embedder:   embedder,
		normalizer: NewProjectNormalizer(),
		cfg:        cfg,
		searchCfg:  searchCfg,
	}
}

// normalizeProject resolves a raw project_id into a canonical identifier.
func (s *Service) normalizeProject(id string) string {
	if !s.cfg.NormalizeProjectID {
		return id
	}
	return s.normalizer.Normalize(id)
}

// normalizeProjectPtr normalizes a *string project_id in place.
func (s *Service) normalizeProjectPtr(id *string) {
	if id == nil || !s.cfg.NormalizeProjectID {
		return
	}
	*id = s.normalizer.Normalize(*id)
}

// Store creates a new memory with smart dedup.
// If consolidation is enabled and a highly similar memory exists (>= AutoMergeThreshold),
// the existing memory is updated instead. Moderate similarity returns suggestions.
func (s *Service) Store(ctx context.Context, req StoreRequest) (*StoreResult, error) {
	// Normalize project_id
	s.normalizeProjectPtr(req.ProjectID)

	// Generate embedding
	vec, err := s.embedder.Embed(ctx, req.Title+" "+req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	emb := pgvector.NewVector(vec)

	// Smart dedup: check for similar memories
	if s.cfg.Consolidation.Enabled {
		similar, err := s.repo.FindSimilar(ctx, emb, req.ProjectID, s.cfg.Consolidation.SuggestThreshold, 5)
		if err != nil {
			slog.Warn("similarity check failed, storing normally", "error", err)
		} else if len(similar) > 0 && similar[0].Similarity >= s.cfg.Consolidation.AutoMergeThreshold {
			// Auto-merge into existing memory
			existing := similar[0].Memory
			strategy := MergeStrategy(s.cfg.Consolidation.MergeStrategy)
			title, content, tags := mergeContent(&existing, req, strategy)

			// Take highest importance
			importance := existing.Importance
			if req.Importance > importance {
				importance = req.Importance
			}

			// Re-embed merged content
			mergedVec, err := s.embedder.Embed(ctx, title+" "+content)
			if err != nil {
				return nil, fmt.Errorf("re-embed merged content: %w", err)
			}
			mergedEmb := pgvector.NewVector(mergedVec)

			// Build merged_from list
			mergedFrom := append(existing.MergedFrom, uuid.Nil) // placeholder
			copy(mergedFrom[1:], existing.MergedFrom)
			mergedFrom[0] = existing.ID // track the original state

			if err := s.repo.UpdateMergeFields(ctx, existing.ID, title, content, tags, importance, existing.MergedFrom, &mergedEmb); err != nil {
				return nil, fmt.Errorf("auto-merge update: %w", err)
			}

			// Log the auto-merge
			logEntry := &ConsolidationLog{
				ID:              uuid.New(),
				TargetID:        existing.ID,
				SourceIDs:       []uuid.UUID{},
				MergeStrategy:   string(strategy),
				SimilarityScore: &similar[0].Similarity,
				ContentBefore:   existing.Content,
				ContentAfter:    content,
				PerformedBy:     "system",
				CreatedAt:       time.Now(),
			}
			if err := s.repo.StoreConsolidationLog(ctx, logEntry); err != nil {
				slog.Warn("failed to log auto-merge", "error", err)
			}

			// Promote to long-term if importance is high
			if importance >= float32(s.cfg.PromoteImportance) {
				s.repo.PromoteToLongTerm(ctx, existing.ID)
			}

			updated, _ := s.repo.Get(ctx, existing.ID)

			slog.Info("auto-merged into existing memory",
				"existing_id", existing.ID,
				"similarity", similar[0].Similarity,
				"strategy", strategy,
			)

			return &StoreResult{
				Memory:          updated,
				Action:          "updated",
				UpdatedExisting: updated,
			}, nil
		} else if len(similar) > 0 {
			// Store normally but attach suggestions
			mem, err := s.storeNew(ctx, req, &emb)
			if err != nil {
				return nil, err
			}

			// Filter to only moderate-similarity suggestions
			var suggestions []SimilarMemory
			for _, sim := range similar {
				if sim.Similarity >= s.cfg.Consolidation.SuggestThreshold {
					suggestions = append(suggestions, sim)
				}
			}

			return &StoreResult{
				Memory:      mem,
				Action:      "created_with_suggestions",
				Suggestions: suggestions,
			}, nil
		}
	}

	// Normal store (no dedup or consolidation disabled)
	mem, err := s.storeNew(ctx, req, &emb)
	if err != nil {
		return nil, err
	}

	return &StoreResult{
		Memory: mem,
		Action: "created",
	}, nil
}

// storeNew creates a new memory (the original Store logic).
func (s *Service) storeNew(ctx context.Context, req StoreRequest, emb *pgvector.Vector) (*Memory, error) {
	now := time.Now()

	mem := &Memory{
		ID:          uuid.New(),
		Title:       req.Title,
		Content:     req.Content,
		Summary:     req.Summary,
		Embedding:   emb,
		Type:        req.Type,
		Scope:       req.Scope,
		ProjectID:   req.ProjectID,
		AgentSource: req.AgentSource,
		Tags:        req.Tags,
		Importance:  req.Importance,
		TTLSeconds:  req.TTLSeconds,
		AccessCount: 0,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}

	if mem.Tags == nil {
		mem.Tags = []string{}
	}
	mem.Type = NormalizeType(mem.Type)
	if mem.Scope == "" {
		mem.Scope = ScopeProject
	}

	// Auto long-term if importance is high enough
	if mem.Importance >= float32(s.cfg.PromoteImportance) {
		mem.TTLSeconds = nil
		mem.ExpiresAt = nil
	} else {
		if mem.TTLSeconds == nil {
			ttl := s.cfg.DefaultTTL
			mem.TTLSeconds = &ttl
		}
		expiresAt := now.Add(time.Duration(*mem.TTLSeconds) * time.Second)
		mem.ExpiresAt = &expiresAt
	}

	if err := s.repo.Store(ctx, mem); err != nil {
		return nil, err
	}

	slog.Info("stored memory",
		"id", mem.ID,
		"title", mem.Title,
		"type", mem.Type,
		"scope", mem.Scope,
		"importance", mem.Importance,
		"long_term", mem.TTLSeconds == nil,
	)

	return mem, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Memory, error) {
	mem, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if mem == nil {
		return nil, nil
	}

	// Increment access and check for auto-promotion
	if err := s.repo.IncrementAccess(ctx, id, s.cfg.TTLExtendFactor); err != nil {
		slog.Warn("failed to increment access", "id", id, "error", err)
	}

	// Auto-promote if access count threshold reached
	if mem.TTLSeconds != nil && mem.AccessCount+1 >= s.cfg.PromoteAccessCount {
		if err := s.repo.PromoteToLongTerm(ctx, id); err != nil {
			slog.Warn("failed to auto-promote", "id", id, "error", err)
		} else {
			slog.Info("auto-promoted memory to long-term", "id", id, "access_count", mem.AccessCount+1)
			mem.TTLSeconds = nil
			mem.ExpiresAt = nil
		}
	}

	return mem, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*Memory, error) {
	var newEmbedding *pgvector.Vector

	// Re-embed if content changed
	if req.Content != nil {
		title := ""
		if req.Title != nil {
			title = *req.Title
		}
		vec, err := s.embedder.Embed(ctx, title+" "+*req.Content)
		if err != nil {
			return nil, fmt.Errorf("re-embed: %w", err)
		}
		emb := pgvector.NewVector(vec)
		newEmbedding = &emb
	}

	if err := s.repo.Update(ctx, id, req, newEmbedding); err != nil {
		return nil, err
	}

	return s.repo.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	// Normalize project_id
	s.normalizeProjectPtr(req.ProjectID)

	if req.Limit <= 0 {
		req.Limit = s.searchCfg.DefaultLimit
	}
	if req.Limit > s.searchCfg.MaxLimit {
		req.Limit = s.searchCfg.MaxLimit
	}

	vec, err := s.embedder.Embed(ctx, req.Query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	queryEmbedding := pgvector.NewVector(vec)
	results, err := s.repo.HybridSearch(ctx, queryEmbedding, req, s.searchCfg.VectorWeight, s.searchCfg.KeywordWeight)
	if err != nil {
		return nil, err
	}

	// Increment access for returned results
	for _, r := range results {
		go func(id uuid.UUID) {
			bgCtx := context.Background()
			s.repo.IncrementAccess(bgCtx, id, s.cfg.TTLExtendFactor)
		}(r.Memory.ID)
	}

	return results, nil
}

func (s *Service) Promote(ctx context.Context, id uuid.UUID) error {
	return s.repo.PromoteToLongTerm(ctx, id)
}

func (s *Service) CreateRelationship(ctx context.Context, req RelationshipRequest) (*Relationship, error) {
	rel := &Relationship{
		ID:           uuid.New(),
		FromMemoryID: req.FromMemoryID,
		ToMemoryID:   req.ToMemoryID,
		Relationship: req.Relationship,
		Strength:     req.Strength,
		Context:      req.Context,
		CreatedAt:    time.Now(),
	}

	if rel.Strength <= 0 {
		rel.Strength = 0.5
	}

	if err := s.repo.StoreRelationship(ctx, rel); err != nil {
		return nil, err
	}
	return rel, nil
}

func (s *Service) GetRelated(ctx context.Context, memoryID uuid.UUID, relationshipTypes []string) ([]Memory, []Relationship, error) {
	return s.repo.GetRelated(ctx, memoryID, relationshipTypes)
}

func (s *Service) GetContext(ctx context.Context, projectID string) ([]Memory, error) {
	projectID = s.normalizeProject(projectID)
	return s.repo.ListByProject(ctx, projectID, 50)
}

func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	return s.repo.GetStats(ctx)
}

func (s *Service) GetAnalytics(ctx context.Context) (*AnalyticsData, error) {
	return s.repo.GetAnalytics(ctx)
}

func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpired(ctx)
}

// --- Consolidation service methods ---

// ConsolidateMemories merges source memories into a target memory.
func (s *Service) ConsolidateMemories(ctx context.Context, targetID uuid.UUID, sourceIDs []uuid.UUID, strategy MergeStrategy, performedBy string) (*Memory, error) {
	if strategy == "" {
		strategy = MergeStrategy(s.cfg.Consolidation.MergeStrategy)
	}
	if performedBy == "" {
		performedBy = "system"
	}

	// Get target memory
	target, err := s.repo.Get(ctx, targetID)
	if err != nil {
		return nil, fmt.Errorf("get target: %w", err)
	}
	if target == nil {
		return nil, fmt.Errorf("target memory not found: %s", targetID)
	}
	if target.ReplacedBy != nil {
		return nil, fmt.Errorf("target memory is already replaced: %s", targetID)
	}

	// Get source memories
	var sources []*Memory
	for _, sid := range sourceIDs {
		if sid == targetID {
			continue // skip self
		}
		src, err := s.repo.Get(ctx, sid)
		if err != nil {
			return nil, fmt.Errorf("get source %s: %w", sid, err)
		}
		if src == nil {
			slog.Warn("source memory not found, skipping", "id", sid)
			continue
		}
		if src.ReplacedBy != nil {
			slog.Warn("source already replaced, skipping", "id", sid)
			continue
		}
		sources = append(sources, src)
	}

	if len(sources) == 0 {
		return target, nil
	}

	contentBefore := target.Content

	// Merge content
	title, content, tags := mergeMultipleContents(target, sources, strategy)
	importance := maxImportance(target.Importance, sources)

	// Re-embed merged content
	vec, err := s.embedder.Embed(ctx, title+" "+content)
	if err != nil {
		return nil, fmt.Errorf("re-embed merged: %w", err)
	}
	newEmb := pgvector.NewVector(vec)

	// Build merged_from list
	mergedFrom := append([]uuid.UUID{}, target.MergedFrom...)
	for _, src := range sources {
		mergedFrom = append(mergedFrom, src.ID)
	}

	// Update target with merged content
	if err := s.repo.UpdateMergeFields(ctx, targetID, title, content, tags, importance, mergedFrom, &newEmb); err != nil {
		return nil, fmt.Errorf("update target: %w", err)
	}

	// Mark sources as replaced
	for _, src := range sources {
		if err := s.repo.MarkReplaced(ctx, src.ID, targetID); err != nil {
			slog.Warn("failed to mark replaced", "source", src.ID, "error", err)
		}

		// Create SUPERSEDES relationship
		s.repo.StoreRelationship(ctx, &Relationship{
			ID:           uuid.New(),
			FromMemoryID: targetID,
			ToMemoryID:   src.ID,
			Relationship: "SUPERSEDES",
			Strength:     1.0,
			CreatedAt:    time.Now(),
		})
	}

	// Promote to long-term if high importance
	if importance >= float32(s.cfg.PromoteImportance) {
		s.repo.PromoteToLongTerm(ctx, targetID)
	}

	// Log the consolidation
	actualSourceIDs := make([]uuid.UUID, len(sources))
	for i, src := range sources {
		actualSourceIDs[i] = src.ID
	}
	logEntry := &ConsolidationLog{
		ID:            uuid.New(),
		TargetID:      targetID,
		SourceIDs:     actualSourceIDs,
		MergeStrategy: string(strategy),
		ContentBefore: contentBefore,
		ContentAfter:  content,
		PerformedBy:   performedBy,
		CreatedAt:     time.Now(),
	}
	if err := s.repo.StoreConsolidationLog(ctx, logEntry); err != nil {
		slog.Warn("failed to log consolidation", "error", err)
	}

	slog.Info("consolidated memories",
		"target", targetID,
		"sources", len(sources),
		"strategy", strategy,
	)

	return s.repo.Get(ctx, targetID)
}

// FindSimilarTo finds memories similar to a given memory ID.
func (s *Service) FindSimilarTo(ctx context.Context, memoryID uuid.UUID, threshold float64, limit int) ([]SimilarMemory, error) {
	mem, err := s.repo.Get(ctx, memoryID)
	if err != nil {
		return nil, fmt.Errorf("get memory: %w", err)
	}
	if mem == nil {
		return nil, fmt.Errorf("memory not found: %s", memoryID)
	}
	if mem.Embedding == nil {
		return nil, fmt.Errorf("memory has no embedding: %s", memoryID)
	}

	if threshold <= 0 {
		threshold = s.cfg.Consolidation.SuggestThreshold
	}

	results, err := s.repo.FindSimilar(ctx, *mem.Embedding, mem.ProjectID, threshold, limit)
	if err != nil {
		return nil, err
	}

	// Filter out the source memory itself
	var filtered []SimilarMemory
	for _, r := range results {
		if r.Memory.ID != memoryID {
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

// ScanForDuplicates runs the background dedup scanner.
func (s *Service) ScanForDuplicates(ctx context.Context) (int, error) {
	pairs, err := s.repo.ScanDuplicates(ctx, s.cfg.Consolidation.SuggestThreshold, s.cfg.Consolidation.ScanBatchSize)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, p := range pairs {
		if err := s.repo.StoreSuggestion(ctx, p.MemAID, p.MemBID, p.Similarity, p.ProjectID); err != nil {
			slog.Warn("failed to store suggestion", "a", p.MemAID, "b", p.MemBID, "error", err)
			continue
		}
		count++
	}

	return count, nil
}

// GetSuggestions returns pending consolidation suggestions.
func (s *Service) GetSuggestions(ctx context.Context, projectID *string, status string, limit, offset int) ([]ConsolidationSuggestion, int, error) {
	if projectID != nil {
		normalized := s.normalizeProject(*projectID)
		projectID = &normalized
	}
	return s.repo.GetSuggestions(ctx, projectID, status, limit, offset)
}

// UpdateSuggestionStatus updates a suggestion's status.
func (s *Service) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string) error {
	return s.repo.UpdateSuggestionStatus(ctx, id, status)
}

// GetConsolidationLog returns consolidation log entries.
func (s *Service) GetConsolidationLog(ctx context.Context, targetID *uuid.UUID, limit, offset int) ([]ConsolidationLog, error) {
	return s.repo.GetConsolidationLog(ctx, targetID, limit, offset)
}

// CleanupReplaced hard-deletes old replaced memories.
func (s *Service) CleanupReplaced(ctx context.Context) (int64, error) {
	return s.repo.CleanupReplaced(ctx, s.cfg.Consolidation.ReplacedRetention)
}

// NormalizeAllProjectIDs scans all unique project_ids in the database and
// normalizes them using the ProjectNormalizer. Returns the number of updated rows.
func (s *Service) NormalizeAllProjectIDs(ctx context.Context) (int, error) {
	projects, err := s.repo.ListDistinctProjectIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("list project ids: %w", err)
	}

	updated := 0
	for _, oldID := range projects {
		if oldID == "" {
			continue
		}

		newID := s.normalizer.Normalize(oldID)
		if newID == oldID {
			continue
		}

		n, err := s.repo.UpdateProjectID(ctx, oldID, newID)
		if err != nil {
			slog.Warn("failed to normalize project_id", "old", oldID, "new", newID, "error", err)
			continue
		}
		if n > 0 {
			slog.Info("normalized project_id", "old", oldID, "new", newID, "rows", n)
			updated += n
		}
	}

	return updated, nil
}
