package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL:    baseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check failed: %s", resp.Status)
	}
	return nil
}

func (c *Client) StoreMemory(ctx context.Context, req StoreRequest) (*Memory, error) {
	var result StoreResult
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/memories", req, &result); err != nil {
		return nil, err
	}
	if result.Memory == nil {
		return nil, fmt.Errorf("store memory: empty memory in response")
	}
	return result.Memory, nil
}

func (c *Client) GetMemory(ctx context.Context, id string) (*Memory, error) {
	var mem Memory
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/memories/"+id, nil, &mem); err != nil {
		return nil, err
	}
	return &mem, nil
}

func (c *Client) UpdateMemory(ctx context.Context, id string, req UpdateRequest) (*Memory, error) {
	var mem Memory
	if err := c.doJSON(ctx, http.MethodPut, "/api/v1/memories/"+id, req, &mem); err != nil {
		return nil, err
	}
	return &mem, nil
}

func (c *Client) DeleteMemory(ctx context.Context, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/api/v1/memories/"+id, nil, nil)
}

func (c *Client) PromoteMemory(ctx context.Context, id string) (*PromoteResponse, error) {
	var resp PromoteResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/memories/"+id+"/promote", nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Recall(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	var results []SearchResult
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/memories/recall", req, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) Search(ctx context.Context, req SearchRequest) ([]SearchResult, error) {
	var results []SearchResult
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/memories/search", req, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Client) GetContext(ctx context.Context, projectID string) ([]Memory, error) {
	var memories []Memory
	encoded := url.PathEscape(projectID)
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/context/"+encoded, nil, &memories); err != nil {
		return nil, err
	}
	return memories, nil
}

func (c *Client) GetStats(ctx context.Context) (*Stats, error) {
	var stats Stats
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/stats", nil, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bodyReader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error %s: %s", resp.Status, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}
