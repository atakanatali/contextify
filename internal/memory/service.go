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
	repo     *Repository
	embedder *embedding.Client
	cfg      config.MemoryConfig
	searchCfg config.SearchConfig
}

func NewService(repo *Repository, embedder *embedding.Client, cfg config.MemoryConfig, searchCfg config.SearchConfig) *Service {
	return &Service{
		repo:      repo,
		embedder:  embedder,
		cfg:       cfg,
		searchCfg: searchCfg,
	}
}

func (s *Service) Store(ctx context.Context, req StoreRequest) (*Memory, error) {
	// Generate embedding
	vec, err := s.embedder.Embed(ctx, req.Title+" "+req.Content)
	if err != nil {
		return nil, fmt.Errorf("generate embedding: %w", err)
	}

	emb := pgvector.NewVector(vec)
	now := time.Now()

	mem := &Memory{
		ID:          uuid.New(),
		Title:       req.Title,
		Content:     req.Content,
		Summary:     req.Summary,
		Embedding:   &emb,
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
	}

	if mem.Tags == nil {
		mem.Tags = []string{}
	}
	if mem.Type == "" {
		mem.Type = TypeGeneral
	}
	if mem.Scope == "" {
		mem.Scope = ScopeProject
	}

	// Auto long-term if importance is high enough
	if mem.Importance >= float32(s.cfg.PromoteImportance) {
		mem.TTLSeconds = nil
		mem.ExpiresAt = nil
	} else {
		// Set default TTL if none provided
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
	if req.Limit <= 0 {
		req.Limit = s.searchCfg.DefaultLimit
	}
	if req.Limit > s.searchCfg.MaxLimit {
		req.Limit = s.searchCfg.MaxLimit
	}

	// Generate embedding for query
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
	return s.repo.ListByProject(ctx, projectID, 50)
}

func (s *Service) GetStats(ctx context.Context) (*Stats, error) {
	return s.repo.GetStats(ctx)
}

func (s *Service) CleanupExpired(ctx context.Context) (int64, error) {
	return s.repo.DeleteExpired(ctx)
}
