package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/atakanatali/contextify/internal/config"
)

type Client struct {
	baseURL    string
	model      string
	dimensions int
	httpClient *http.Client
}

type embedRequest struct {
	Model string `json:"model"`
	Input any    `json:"input"` // string or []string
}

type embedResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

func NewClient(cfg config.EmbeddingConfig) *Client {
	return &Client{
		baseURL:    cfg.OllamaURL,
		model:      cfg.Model,
		dimensions: cfg.Dimensions,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Embed generates an embedding for a single text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	results, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}
	return results[0], nil
}

// EmbedBatch generates embeddings for multiple texts in a single request.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	reqBody := embedRequest{
		Model: c.model,
		Input: texts,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	return result.Embeddings, nil
}

// EnsureModel pulls the embedding model if it's not already available.
func (c *Client) EnsureModel(ctx context.Context) error {
	slog.Info("checking embedding model", "model", c.model)

	// Check if model exists
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/show", bytes.NewReader([]byte(`{"model":"`+c.model+`"}`)))
	if err != nil {
		return fmt.Errorf("create show request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("check model: %w", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		slog.Info("embedding model already available", "model", c.model)
		return nil
	}

	// Pull the model
	slog.Info("pulling embedding model", "model", c.model)
	pullReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/pull", bytes.NewReader([]byte(`{"model":"`+c.model+`","stream":false}`)))
	if err != nil {
		return fmt.Errorf("create pull request: %w", err)
	}
	pullReq.Header.Set("Content-Type", "application/json")

	// Use a longer timeout for model pulling
	pullClient := &http.Client{Timeout: 30 * time.Minute}
	pullResp, err := pullClient.Do(pullReq)
	if err != nil {
		return fmt.Errorf("pull model: %w", err)
	}
	defer pullResp.Body.Close()

	if pullResp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(pullResp.Body)
		return fmt.Errorf("pull model failed (status %d): %s", pullResp.StatusCode, string(respBody))
	}

	slog.Info("embedding model pulled successfully", "model", c.model)
	return nil
}

// Dimensions returns the expected embedding dimensions.
func (c *Client) Dimensions() int {
	return c.dimensions
}
