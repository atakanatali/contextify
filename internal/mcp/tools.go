package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/atakanatali/contextify/internal/memory"
)

// Tool input structs with jsonschema tags

type StoreMemoryInput struct {
	Title       string   `json:"title" jsonschema:"Title of the memory,required"`
	Content     string   `json:"content" jsonschema:"Detailed content of the memory,required"`
	Summary     *string  `json:"summary,omitempty" jsonschema:"Brief summary"`
	Type        string   `json:"type" jsonschema:"Type: solution|problem|code_pattern|fix|error|workflow|decision|general|task|technology|command|file_context|conversation|project. Unknown types default to general."`
	Scope       string   `json:"scope" jsonschema:"Scope: global|project"`
	ProjectID   *string  `json:"project_id,omitempty" jsonschema:"Project identifier"`
	AgentSource *string  `json:"agent_source,omitempty" jsonschema:"Source agent: claude-code|cursor|gemini|antigravity"`
	Tags        []string `json:"tags,omitempty" jsonschema:"Tags for categorization"`
	Importance  float32  `json:"importance" jsonschema:"Importance score 0.0-1.0"`
	TTLSeconds  *int     `json:"ttl_seconds,omitempty" jsonschema:"Time-to-live in seconds. Null for permanent."`
}

type RecallInput struct {
	Query         string   `json:"query" jsonschema:"Natural language query,required"`
	ProjectID     *string  `json:"project_id,omitempty" jsonschema:"Filter by project"`
	Tags          []string `json:"tags,omitempty" jsonschema:"Filter by tags"`
	Type          *string  `json:"type,omitempty" jsonschema:"Filter by memory type"`
	MinImportance *float32 `json:"min_importance,omitempty" jsonschema:"Minimum importance threshold"`
	Limit         int      `json:"limit,omitempty" jsonschema:"Max results (default 20)"`
}

type SearchInput struct {
	Query         string   `json:"query,omitempty" jsonschema:"Search query"`
	Tags          []string `json:"tags,omitempty" jsonschema:"Filter by tags"`
	Type          *string  `json:"type,omitempty" jsonschema:"Filter by type"`
	Scope         *string  `json:"scope,omitempty" jsonschema:"Filter by scope"`
	ProjectID     *string  `json:"project_id,omitempty" jsonschema:"Filter by project"`
	AgentSource   *string  `json:"agent_source,omitempty" jsonschema:"Filter by agent source"`
	MinImportance *float32 `json:"min_importance,omitempty" jsonschema:"Minimum importance"`
	Limit         int      `json:"limit,omitempty" jsonschema:"Max results"`
	Offset        int      `json:"offset,omitempty" jsonschema:"Pagination offset"`
}

type GetMemoryInput struct {
	MemoryID string `json:"memory_id" jsonschema:"Memory UUID,required"`
}

type UpdateMemoryInput struct {
	MemoryID   string   `json:"memory_id" jsonschema:"Memory UUID,required"`
	Title      *string  `json:"title,omitempty" jsonschema:"New title"`
	Content    *string  `json:"content,omitempty" jsonschema:"New content"`
	Summary    *string  `json:"summary,omitempty" jsonschema:"New summary"`
	Type       *string  `json:"type,omitempty" jsonschema:"New type"`
	Tags       []string `json:"tags,omitempty" jsonschema:"New tags"`
	Importance *float32 `json:"importance,omitempty" jsonschema:"New importance"`
}

type DeleteMemoryInput struct {
	MemoryID string `json:"memory_id" jsonschema:"Memory UUID,required"`
}

type CreateRelationshipInput struct {
	FromMemoryID string  `json:"from_memory_id" jsonschema:"Source memory UUID,required"`
	ToMemoryID   string  `json:"to_memory_id" jsonschema:"Target memory UUID,required"`
	Relationship string  `json:"relationship" jsonschema:"Type: SOLVES|CAUSES|RELATED_TO|REQUIRES|ADDRESSES|SUPERSEDES,required"`
	Strength     float32 `json:"strength,omitempty" jsonschema:"Strength 0.0-1.0"`
	Context      *string `json:"context,omitempty" jsonschema:"Description of the relationship"`
}

type GetRelatedInput struct {
	MemoryID          string   `json:"memory_id" jsonschema:"Memory UUID,required"`
	RelationshipTypes []string `json:"relationship_types,omitempty" jsonschema:"Filter by relationship types"`
}

type GetContextInput struct {
	ProjectID string `json:"project_id" jsonschema:"Project identifier,required"`
}

type PromoteMemoryInput struct {
	MemoryID string `json:"memory_id" jsonschema:"Memory UUID to promote,required"`
}

// --- Consolidation tool inputs ---

type ConsolidateMemoriesInput struct {
	TargetID  string   `json:"target_id" jsonschema:"Target memory UUID to merge into,required"`
	SourceIDs []string `json:"source_ids" jsonschema:"Source memory UUIDs to merge from,required"`
	Strategy  string   `json:"strategy,omitempty" jsonschema:"Merge strategy: latest_wins|append|smart_merge"`
}

type FindSimilarInput struct {
	MemoryID  string  `json:"memory_id" jsonschema:"Memory UUID to find similar memories for,required"`
	Threshold float64 `json:"threshold,omitempty" jsonschema:"Minimum similarity (0.0-1.0, default 0.75)"`
	Limit     int     `json:"limit,omitempty" jsonschema:"Max results (default 5)"`
}

type SuggestConsolidationsInput struct {
	ProjectID *string `json:"project_id,omitempty" jsonschema:"Filter by project"`
	Limit     int     `json:"limit,omitempty" jsonschema:"Max suggestions (default 10)"`
}

func (s *Server) registerTools() {
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "store_memory",
		Description: "Store a new memory with automatic embedding generation. Memories with importance >= 0.8 are automatically permanent.",
	}, s.storeMemory)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "recall_memories",
		Description: "Recall memories using natural language semantic search. Best for fuzzy/conceptual queries.",
	}, s.recallMemories)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "search_memories",
		Description: "Advanced search with filters (tags, type, scope, importance). Use for precise filtering.",
	}, s.searchMemories)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_memory",
		Description: "Retrieve a specific memory by its ID.",
	}, s.getMemory)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "update_memory",
		Description: "Update an existing memory. Re-embeds if content changes.",
	}, s.updateMemory)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "delete_memory",
		Description: "Delete a memory and all its relationships.",
	}, s.deleteMemory)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "create_relationship",
		Description: "Link two memories with a typed relationship (SOLVES, CAUSES, RELATED_TO, REQUIRES, ADDRESSES, SUPERSEDES).",
	}, s.createRelationship)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_related_memories",
		Description: "Find memories connected to a specific memory via relationships.",
	}, s.getRelatedMemories)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_context",
		Description: "Get all important memories for a project. Use at session start to load context.",
	}, s.getContext)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "promote_memory",
		Description: "Manually promote a short-term memory to permanent long-term storage.",
	}, s.promoteMemory)

	// Consolidation tools
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "consolidate_memories",
		Description: "Merge multiple memories into one target memory. Source memories are marked as superseded. Use when you find duplicate or overlapping memories.",
	}, s.consolidateMemories)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "find_similar",
		Description: "Find memories similar to a given memory ID using vector similarity. Useful for detecting duplicates.",
	}, s.findSimilar)

	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "suggest_consolidations",
		Description: "Get server-generated suggestions for memories that should be consolidated. Returns pairs of similar memories.",
	}, s.suggestConsolidations)
}

func (s *Server) storeMemory(ctx context.Context, req *mcp.CallToolRequest, input *StoreMemoryInput) (*mcp.CallToolResult, any, error) {
	memType := memory.MemoryType(input.Type)
	if memType == "" {
		memType = memory.TypeGeneral
	}
	scope := memory.MemoryScope(input.Scope)
	if scope == "" {
		scope = memory.ScopeProject
	}

	storeReq := memory.StoreRequest{
		Title:       input.Title,
		Content:     input.Content,
		Summary:     input.Summary,
		Type:        memType,
		Scope:       scope,
		ProjectID:   input.ProjectID,
		AgentSource: input.AgentSource,
		Tags:        input.Tags,
		Importance:  input.Importance,
		TTLSeconds:  input.TTLSeconds,
	}

	result, err := s.svc.Store(ctx, storeReq)
	if err != nil {
		return nil, nil, fmt.Errorf("store memory: %w", err)
	}

	// Build response message based on action
	switch result.Action {
	case "updated":
		msg := fmt.Sprintf("Auto-merged into existing memory: %s (id: %s). A highly similar memory already existed, so the content was merged.",
			result.Memory.Title, result.Memory.ID)
		return makeTextResult(msg), nil, nil
	case "created_with_suggestions":
		return makeJSONResult(result)
	default:
		return makeTextResult(fmt.Sprintf("Stored memory: %s (id: %s, type: %s, long-term: %v)",
			result.Memory.Title, result.Memory.ID, result.Memory.Type, result.Memory.TTLSeconds == nil)), nil, nil
	}
}

func (s *Server) recallMemories(ctx context.Context, req *mcp.CallToolRequest, input *RecallInput) (*mcp.CallToolResult, any, error) {
	searchReq := memory.SearchRequest{
		Query:     input.Query,
		ProjectID: input.ProjectID,
		Tags:      input.Tags,
		Limit:     input.Limit,
	}
	if input.Type != nil {
		t := memory.MemoryType(*input.Type)
		searchReq.Type = &t
	}
	searchReq.MinImportance = input.MinImportance

	results, err := s.svc.Search(ctx, searchReq)
	if err != nil {
		return nil, nil, fmt.Errorf("recall: %w", err)
	}

	return makeJSONResult(results)
}

func (s *Server) searchMemories(ctx context.Context, req *mcp.CallToolRequest, input *SearchInput) (*mcp.CallToolResult, any, error) {
	searchReq := memory.SearchRequest{
		Query:         input.Query,
		ProjectID:     input.ProjectID,
		AgentSource:   input.AgentSource,
		Tags:          input.Tags,
		MinImportance: input.MinImportance,
		Limit:         input.Limit,
		Offset:        input.Offset,
	}
	if input.Type != nil {
		t := memory.MemoryType(*input.Type)
		searchReq.Type = &t
	}
	if input.Scope != nil {
		sc := memory.MemoryScope(*input.Scope)
		searchReq.Scope = &sc
	}

	results, err := s.svc.Search(ctx, searchReq)
	if err != nil {
		return nil, nil, fmt.Errorf("search: %w", err)
	}

	return makeJSONResult(results)
}

func (s *Server) getMemory(ctx context.Context, req *mcp.CallToolRequest, input *GetMemoryInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	mem, err := s.svc.Get(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get memory: %w", err)
	}
	if mem == nil {
		return makeTextResult("Memory not found"), nil, nil
	}

	return makeJSONResult(mem)
}

func (s *Server) updateMemory(ctx context.Context, req *mcp.CallToolRequest, input *UpdateMemoryInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	updateReq := memory.UpdateRequest{
		Title:      input.Title,
		Content:    input.Content,
		Summary:    input.Summary,
		Tags:       input.Tags,
		Importance: input.Importance,
	}
	if input.Type != nil {
		t := memory.MemoryType(*input.Type)
		updateReq.Type = &t
	}

	mem, err := s.svc.Update(ctx, id, updateReq)
	if err != nil {
		return nil, nil, fmt.Errorf("update memory: %w", err)
	}

	return makeJSONResult(mem)
}

func (s *Server) deleteMemory(ctx context.Context, req *mcp.CallToolRequest, input *DeleteMemoryInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	if err := s.svc.Delete(ctx, id); err != nil {
		return nil, nil, fmt.Errorf("delete memory: %w", err)
	}

	return makeTextResult(fmt.Sprintf("Deleted memory: %s", id)), nil, nil
}

func (s *Server) createRelationship(ctx context.Context, req *mcp.CallToolRequest, input *CreateRelationshipInput) (*mcp.CallToolResult, any, error) {
	fromID, err := uuid.Parse(input.FromMemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid from_memory_id: %w", err)
	}
	toID, err := uuid.Parse(input.ToMemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid to_memory_id: %w", err)
	}

	relReq := memory.RelationshipRequest{
		FromMemoryID: fromID,
		ToMemoryID:   toID,
		Relationship: input.Relationship,
		Strength:     input.Strength,
		Context:      input.Context,
	}

	rel, err := s.svc.CreateRelationship(ctx, relReq)
	if err != nil {
		return nil, nil, fmt.Errorf("create relationship: %w", err)
	}

	return makeTextResult(fmt.Sprintf("Created relationship: %s -[%s]-> %s (id: %s)",
		rel.FromMemoryID, rel.Relationship, rel.ToMemoryID, rel.ID)), nil, nil
}

func (s *Server) getRelatedMemories(ctx context.Context, req *mcp.CallToolRequest, input *GetRelatedInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	memories, relationships, err := s.svc.GetRelated(ctx, id, input.RelationshipTypes)
	if err != nil {
		return nil, nil, fmt.Errorf("get related: %w", err)
	}

	result := map[string]any{
		"memories":      memories,
		"relationships": relationships,
	}
	return makeJSONResult(result)
}

func (s *Server) getContext(ctx context.Context, req *mcp.CallToolRequest, input *GetContextInput) (*mcp.CallToolResult, any, error) {
	memories, err := s.svc.GetContext(ctx, input.ProjectID)
	if err != nil {
		return nil, nil, fmt.Errorf("get context: %w", err)
	}

	return makeJSONResult(memories)
}

func (s *Server) promoteMemory(ctx context.Context, req *mcp.CallToolRequest, input *PromoteMemoryInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	if err := s.svc.Promote(ctx, id); err != nil {
		return nil, nil, fmt.Errorf("promote: %w", err)
	}

	return makeTextResult(fmt.Sprintf("Promoted memory %s to long-term storage", id)), nil, nil
}

// --- Consolidation tool handlers ---

func (s *Server) consolidateMemories(ctx context.Context, req *mcp.CallToolRequest, input *ConsolidateMemoriesInput) (*mcp.CallToolResult, any, error) {
	targetID, err := uuid.Parse(input.TargetID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid target_id: %w", err)
	}

	var sourceIDs []uuid.UUID
	for _, sid := range input.SourceIDs {
		id, err := uuid.Parse(sid)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid source_id %s: %w", sid, err)
		}
		sourceIDs = append(sourceIDs, id)
	}

	strategy := memory.MergeStrategy(input.Strategy)
	result, err := s.svc.ConsolidateMemories(ctx, targetID, sourceIDs, strategy, "agent")
	if err != nil {
		return nil, nil, fmt.Errorf("consolidate: %w", err)
	}

	return makeJSONResult(map[string]any{
		"memory":       result,
		"merged_count": len(sourceIDs),
		"message":      fmt.Sprintf("Consolidated %d memories into %s", len(sourceIDs), targetID),
	})
}

func (s *Server) findSimilar(ctx context.Context, req *mcp.CallToolRequest, input *FindSimilarInput) (*mcp.CallToolResult, any, error) {
	id, err := uuid.Parse(input.MemoryID)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid memory_id: %w", err)
	}

	results, err := s.svc.FindSimilarTo(ctx, id, input.Threshold, input.Limit)
	if err != nil {
		return nil, nil, fmt.Errorf("find similar: %w", err)
	}

	return makeJSONResult(results)
}

func (s *Server) suggestConsolidations(ctx context.Context, req *mcp.CallToolRequest, input *SuggestConsolidationsInput) (*mcp.CallToolResult, any, error) {
	limit := input.Limit
	if limit <= 0 {
		limit = 10
	}

	suggestions, total, err := s.svc.GetSuggestions(ctx, input.ProjectID, "pending", limit, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("get suggestions: %w", err)
	}

	return makeJSONResult(map[string]any{
		"suggestions": suggestions,
		"total":       total,
	})
}

// Helper functions

func makeTextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: text},
		},
	}
}

func makeJSONResult(data any) (*mcp.CallToolResult, any, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, nil, fmt.Errorf("marshal result: %w", err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}, nil, nil
}
