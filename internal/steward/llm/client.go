package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL      string
	model        string
	fallback     string
	httpClient   *http.Client
	maxRespChars int
}

func NewClient(baseURL, model string) *Client {
	if model == "" {
		model = "qwen2.5:3b"
	}
	return &Client{
		baseURL:      baseURL,
		model:        model,
		fallback:     "qwen2.5:1.5b",
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		maxRespChars: 16000,
	}
}

type MergeDecisionInput struct {
	MemoryATitle   string            `json:"memory_a_title"`
	MemoryAContent string            `json:"memory_a_content"`
	MemoryATags    []string          `json:"memory_a_tags"`
	MemoryBTitle   string            `json:"memory_b_title"`
	MemoryBContent string            `json:"memory_b_content"`
	MemoryBTags    []string          `json:"memory_b_tags"`
	Similarity     float64           `json:"similarity"`
	StrategyHints  map[string]string `json:"strategy_hints,omitempty"`
}

type MergeDecision struct {
	IsDuplicate         bool     `json:"is_duplicate"`
	HasConflict         bool     `json:"has_conflict"`
	Decision            string   `json:"decision"`
	Confidence          float64  `json:"confidence"`
	RecommendedStrategy string   `json:"recommended_strategy"`
	MergedTitle         string   `json:"merged_title"`
	MergedContent       string   `json:"merged_content"`
	ReasonCodes         []string `json:"reason_codes"`
}

type DecisionMetrics struct {
	Provider         string
	Model            string
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
	LatencyMs        *int
}

type ollamaChatRequest struct {
	Model    string         `json:"model"`
	Stream   bool           `json:"stream"`
	Format   string         `json:"format,omitempty"`
	Messages []ollamaMsg    `json:"messages"`
	Options  map[string]any `json:"options,omitempty"`
}

type ollamaMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResponse struct {
	Model   string `json:"model"`
	Message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	PromptEvalCount *int `json:"prompt_eval_count"`
	EvalCount       *int `json:"eval_count"`
}

func (c *Client) DecideMerge(ctx context.Context, in MergeDecisionInput) (*MergeDecision, *DecisionMetrics, error) {
	decision, metrics, err := c.decideMergeOnce(ctx, c.model, in)
	if err == nil {
		return decision, metrics, nil
	}
	// Retry once on parse/validation failure or transient inference failure using fallback model.
	return c.decideMergeOnce(ctx, c.fallback, in)
}

func (c *Client) decideMergeOnce(ctx context.Context, model string, in MergeDecisionInput) (*MergeDecision, *DecisionMetrics, error) {
	start := time.Now()
	prompt := buildPrompt(in)
	reqBody := ollamaChatRequest{
		Model:  model,
		Stream: false,
		Format: "json",
		Messages: []ollamaMsg{
			{Role: "system", Content: "Return strict JSON only matching the requested schema. No markdown."},
			{Role: "user", Content: prompt},
		},
		Options: map[string]any{"temperature": 0.1},
	}
	b, err := json.Marshal(reqBody)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal llm request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/chat", bytes.NewReader(b))
	if err != nil {
		return nil, nil, fmt.Errorf("create llm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("send llm request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(c.maxRespChars)))
	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("ollama chat failed (%d): %s", resp.StatusCode, string(body))
	}
	var out ollamaChatResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, nil, fmt.Errorf("decode llm response: %w", err)
	}
	decision, err := ParseAndValidateDecision([]byte(out.Message.Content))
	if err != nil {
		return nil, nil, err
	}
	lat := int(time.Since(start).Milliseconds())
	metrics := &DecisionMetrics{
		Provider:         "ollama",
		Model:            out.Model,
		PromptTokens:     out.PromptEvalCount,
		CompletionTokens: out.EvalCount,
		TotalTokens:      sumPtrs(out.PromptEvalCount, out.EvalCount),
		LatencyMs:        &lat,
	}
	return decision, metrics, nil
}

func ParseAndValidateDecision(raw []byte) (*MergeDecision, error) {
	var d MergeDecision
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("parse merge decision json: %w", err)
	}
	if d.Decision != "merge" && d.Decision != "skip" && d.Decision != "needs_review" {
		return nil, fmt.Errorf("invalid decision")
	}
	if d.Confidence < 0 || d.Confidence > 1 {
		return nil, fmt.Errorf("invalid confidence")
	}
	if d.ReasonCodes == nil {
		d.ReasonCodes = []string{}
	}
	if d.RecommendedStrategy == "" {
		d.RecommendedStrategy = "smart_merge"
	}
	return &d, nil
}

func buildPrompt(in MergeDecisionInput) string {
	b, _ := json.Marshal(in)
	return "Analyze duplicate merge risk and return JSON with keys: is_duplicate, has_conflict, decision, confidence, recommended_strategy, merged_title, merged_content, reason_codes. Input: " + string(b)
}

func sumPtrs(a, b *int) *int {
	if a == nil && b == nil {
		return nil
	}
	total := 0
	if a != nil {
		total += *a
	}
	if b != nil {
		total += *b
	}
	return &total
}
