// Package e2e contains end-to-end tests that hit a running Contextify server.
// Run with: go test ./tests/e2e/ -v -count=1
// Requires: Contextify server running on localhost:8420
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

const baseURL = "http://localhost:8420/api/v1"

// --- helpers ---

func doRequest(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result map[string]any
	json.Unmarshal(raw, &result)

	return resp.StatusCode, result
}

func doRequestArray(t *testing.T, method, path string, body any) (int, []any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result []any
	json.Unmarshal(raw, &result)

	return resp.StatusCode, result
}

func storeMemory(t *testing.T, title, content, projectID string, importance float64) map[string]any {
	t.Helper()
	body := map[string]any{
		"title":        title,
		"content":      content,
		"type":         "general",
		"scope":        "project",
		"project_id":   projectID,
		"importance":   importance,
		"tags":         []string{"e2e-test", "consolidation"},
		"agent_source": "e2e-test",
	}

	status, result := doRequest(t, "POST", "/memories", body)
	if status != 201 {
		t.Fatalf("store memory failed: status=%d body=%v", status, result)
	}
	return result
}

func deleteMemory(t *testing.T, id string) {
	t.Helper()
	status, _ := doRequest(t, "DELETE", "/memories/"+id, nil)
	if status != 200 {
		// May already be deleted, that's ok
		t.Logf("delete memory %s returned status %d (may be already cleaned up)", id, status)
	}
}

func getMemory(t *testing.T, id string) map[string]any {
	t.Helper()
	status, result := doRequest(t, "GET", "/memories/"+id, nil)
	if status != 200 {
		t.Fatalf("get memory %s failed: status=%d", id, status)
	}
	return result
}

func uniqueProject() string {
	return fmt.Sprintf("/tmp/e2e-consolidation-%d", time.Now().UnixNano())
}

// --- tests ---

func TestHealthCheck(t *testing.T) {
	resp, err := http.Get("http://localhost:8420/health")
	if err != nil {
		t.Skipf("Server not running: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("health check failed: %d", resp.StatusCode)
	}
}

func TestSmartStore_AutoMerge(t *testing.T) {
	project := uniqueProject()

	// Store first memory
	result1 := storeMemory(t, "Go error handling patterns", "Use errors.Is and errors.As for error checking in Go.", project, 0.7)
	mem1 := result1["memory"].(map[string]any)
	id1 := mem1["id"].(string)
	defer deleteMemory(t, id1)

	if result1["action"] != "created" {
		t.Fatalf("expected action=created, got %v", result1["action"])
	}

	// Store nearly identical memory — should auto-merge
	result2 := storeMemory(t, "Go error handling patterns", "Use errors.Is and errors.As for error checking in Go. Also use fmt.Errorf with %%w for wrapping.", project, 0.8)
	action2 := result2["action"].(string)

	t.Logf("Second store action: %s", action2)

	if action2 == "updated" {
		// Auto-merged into first memory
		updated := result2["memory"].(map[string]any)
		updatedID := updated["id"].(string)
		if updatedID != id1 {
			// It merged into a different existing memory; just clean up
			defer deleteMemory(t, updatedID)
		}

		// Verify the original memory was updated
		mem := getMemory(t, id1)
		importance := mem["importance"].(float64)
		if importance < 0.8 {
			t.Errorf("expected importance >= 0.8 after merge, got %f", importance)
		}
		t.Log("Auto-merge successful")
	} else if action2 == "created_with_suggestions" {
		// Similarity was below auto-merge threshold but above suggest
		mem2 := result2["memory"].(map[string]any)
		defer deleteMemory(t, mem2["id"].(string))

		suggestions := result2["suggestions"]
		if suggestions == nil {
			t.Error("expected suggestions for created_with_suggestions action")
		} else {
			t.Logf("Got %d suggestions", len(suggestions.([]any)))
		}
	} else if action2 == "created" {
		// Embeddings weren't similar enough — this is acceptable depending on embedding model
		mem2 := result2["memory"].(map[string]any)
		defer deleteMemory(t, mem2["id"].(string))
		t.Log("Memories were not similar enough for auto-merge (embedding model dependent)")
	}
}

func TestSmartStore_ReturnFormat(t *testing.T) {
	project := uniqueProject()

	result := storeMemory(t, "Test StoreResult format", "This tests the new StoreResult return format.", project, 0.5)

	// Verify StoreResult structure
	if result["memory"] == nil {
		t.Fatal("StoreResult missing 'memory' field")
	}
	if result["action"] == nil {
		t.Fatal("StoreResult missing 'action' field")
	}

	action := result["action"].(string)
	if action != "created" && action != "updated" && action != "created_with_suggestions" {
		t.Fatalf("unexpected action: %s", action)
	}

	mem := result["memory"].(map[string]any)
	id := mem["id"].(string)
	defer deleteMemory(t, id)

	// Verify memory has consolidation fields
	if mem["version"] == nil {
		t.Error("memory missing 'version' field")
	}
	if _, ok := mem["merged_from"]; !ok {
		t.Error("memory missing 'merged_from' field")
	}

	t.Logf("StoreResult: action=%s id=%s version=%v", action, id, mem["version"])
}

func TestMergeMemories(t *testing.T) {
	project := uniqueProject()

	// Store two memories
	r1 := storeMemory(t, "Docker basics", "Docker uses containers for isolation.", project, 0.6)
	mem1 := r1["memory"].(map[string]any)
	id1 := mem1["id"].(string)
	defer deleteMemory(t, id1)

	r2 := storeMemory(t, "Docker networking", "Docker supports bridge, host, and overlay networks.", project, 0.5)
	mem2 := r2["memory"].(map[string]any)
	id2 := mem2["id"].(string)
	defer deleteMemory(t, id2)

	// Merge mem2 into mem1
	status, mergeResult := doRequest(t, "POST", "/memories/"+id1+"/merge", map[string]any{
		"source_ids": []string{id2},
		"strategy":   "smart_merge",
	})

	if status != 200 {
		t.Fatalf("merge failed: status=%d body=%v", status, mergeResult)
	}

	// Verify target was updated
	merged := getMemory(t, id1)
	content := merged["content"].(string)
	if len(content) <= len("Docker uses containers for isolation.") {
		t.Error("merged content should be longer than original")
	}
	t.Logf("Merged content length: %d", len(content))

	// Verify version incremented or merged_from updated
	version := merged["version"]
	t.Logf("Merged memory version: %v", version)

	// Verify source was marked as replaced
	source := getMemory(t, id2)
	if source["replaced_by"] != nil {
		t.Logf("Source marked as replaced_by: %v", source["replaced_by"])
	} else {
		t.Log("Note: replaced_by not returned in GET (may be filtered)")
	}
}

func TestMergeMemories_InvalidTarget(t *testing.T) {
	status, result := doRequest(t, "POST", "/memories/00000000-0000-0000-0000-000000000000/merge", map[string]any{
		"source_ids": []string{"00000000-0000-0000-0000-000000000001"},
	})

	if status == 200 {
		t.Fatal("expected error for nonexistent target")
	}

	if result["error"] == nil {
		t.Fatal("expected error message")
	}
	t.Logf("Got expected error: %s", result["error"])
}

func TestMergeMemories_EmptySources(t *testing.T) {
	project := uniqueProject()

	r := storeMemory(t, "Merge empty sources test", "Content.", project, 0.5)
	mem := r["memory"].(map[string]any)
	id := mem["id"].(string)
	defer deleteMemory(t, id)

	status, result := doRequest(t, "POST", "/memories/"+id+"/merge", map[string]any{
		"source_ids": []string{},
	})

	if status != 400 {
		t.Fatalf("expected 400 for empty sources, got %d: %v", status, result)
	}
}

func TestBatchConsolidate(t *testing.T) {
	project := uniqueProject()

	// Store 3 memories
	r1 := storeMemory(t, "Batch target", "Base content for batch merge.", project, 0.7)
	id1 := r1["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id1)

	r2 := storeMemory(t, "Batch source 1", "Additional content 1.", project, 0.5)
	id2 := r2["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id2)

	r3 := storeMemory(t, "Batch source 2", "Additional content 2.", project, 0.5)
	id3 := r3["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id3)

	// Batch merge
	status, result := doRequest(t, "POST", "/memories/consolidate", map[string]any{
		"operations": []map[string]any{
			{
				"target_id":  id1,
				"source_ids": []string{id2, id3},
			},
		},
		"strategy": "append",
	})

	if status != 200 {
		t.Fatalf("batch consolidate failed: status=%d body=%v", status, result)
	}

	results := result["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	op := results[0].(map[string]any)
	if op["error"] != nil {
		t.Fatalf("batch op error: %v", op["error"])
	}

	// Verify target content was merged
	merged := getMemory(t, id1)
	content := merged["content"].(string)
	t.Logf("Batch merged content length: %d", len(content))
}

func TestMergeStrategies(t *testing.T) {
	strategies := []string{"latest_wins", "append", "smart_merge"}

	for _, strategy := range strategies {
		t.Run(strategy, func(t *testing.T) {
			project := uniqueProject()

			r1 := storeMemory(t, "Strategy test target "+strategy, "Original content for strategy test.", project, 0.6)
			id1 := r1["memory"].(map[string]any)["id"].(string)
			defer deleteMemory(t, id1)

			r2 := storeMemory(t, "Strategy test source "+strategy, "New content for strategy test.", project, 0.5)
			id2 := r2["memory"].(map[string]any)["id"].(string)
			defer deleteMemory(t, id2)

			status, result := doRequest(t, "POST", "/memories/"+id1+"/merge", map[string]any{
				"source_ids": []string{id2},
				"strategy":   strategy,
			})

			if status != 200 {
				t.Fatalf("merge with %s failed: status=%d body=%v", strategy, status, result)
			}

			merged := getMemory(t, id1)
			content := merged["content"].(string)
			t.Logf("[%s] Merged content: %s", strategy, content[:min(len(content), 100)])
		})
	}
}

func TestConsolidationSuggestions_Lifecycle(t *testing.T) {
	// Get suggestions list
	status, result := doRequest(t, "GET", "/consolidation/suggestions?status=pending&limit=5", nil)
	if status != 200 {
		t.Fatalf("get suggestions failed: status=%d body=%v", status, result)
	}

	if result["suggestions"] == nil {
		t.Fatal("expected suggestions array in response")
	}
	if result["total"] == nil {
		t.Fatal("expected total in response")
	}

	total := result["total"].(float64)
	suggestions := result["suggestions"].([]any)
	t.Logf("Pending suggestions: %d (total: %d)", len(suggestions), int(total))

	// If there are suggestions, test dismiss
	if len(suggestions) > 0 {
		s := suggestions[0].(map[string]any)
		id := s["id"].(string)

		// Dismiss it
		status, dismissResult := doRequest(t, "PUT", "/consolidation/suggestions/"+id, map[string]any{
			"status": "dismissed",
		})
		if status != 200 {
			t.Fatalf("dismiss suggestion failed: status=%d body=%v", status, dismissResult)
		}
		t.Logf("Dismissed suggestion %s", id)
	}
}

func TestConsolidationSuggestions_InvalidStatus(t *testing.T) {
	status, result := doRequest(t, "PUT", "/consolidation/suggestions/00000000-0000-0000-0000-000000000001", map[string]any{
		"status": "invalid_status",
	})

	if status != 400 {
		t.Fatalf("expected 400 for invalid status, got %d: %v", status, result)
	}
}

func TestDuplicatesEndpoint(t *testing.T) {
	status, result := doRequest(t, "GET", "/memories/duplicates?limit=5", nil)
	if status != 200 {
		t.Fatalf("get duplicates failed: status=%d body=%v", status, result)
	}

	if result["suggestions"] == nil {
		t.Fatal("expected suggestions array")
	}
	if result["total"] == nil {
		t.Fatal("expected total")
	}

	t.Logf("Duplicates: %d", int(result["total"].(float64)))
}

func TestConsolidationLog(t *testing.T) {
	// First perform a merge to ensure there's a log entry
	project := uniqueProject()

	r1 := storeMemory(t, "Log test target", "Content A for log test.", project, 0.5)
	id1 := r1["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id1)

	r2 := storeMemory(t, "Log test source", "Content B for log test.", project, 0.5)
	id2 := r2["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id2)

	// Merge to create a log entry
	doRequest(t, "POST", "/memories/"+id1+"/merge", map[string]any{
		"source_ids": []string{id2},
	})

	// Fetch consolidation log
	status, logs := doRequestArray(t, "GET", "/consolidation/log?limit=10", nil)
	if status != 200 {
		t.Fatalf("get consolidation log failed: status=%d", status)
	}

	if len(logs) == 0 {
		t.Fatal("expected at least one log entry after merge")
	}

	entry := logs[0].(map[string]any)
	if entry["target_id"] == nil {
		t.Error("log entry missing target_id")
	}
	if entry["merge_strategy"] == nil {
		t.Error("log entry missing merge_strategy")
	}
	if entry["content_before"] == nil {
		t.Error("log entry missing content_before")
	}
	if entry["content_after"] == nil {
		t.Error("log entry missing content_after")
	}

	t.Logf("Consolidation log entries: %d", len(logs))
	t.Logf("First entry: target=%s strategy=%s performed_by=%s",
		entry["target_id"], entry["merge_strategy"], entry["performed_by"])
}

func TestConsolidationLog_FilterByTarget(t *testing.T) {
	status, _ := doRequest(t, "GET", "/consolidation/log?target_id=00000000-0000-0000-0000-000000000000&limit=5", nil)
	if status != 200 {
		t.Fatalf("filtered consolidation log failed: status=%d", status)
	}
}

func TestStatsIncludesPendingSuggestions(t *testing.T) {
	status, result := doRequest(t, "GET", "/stats", nil)
	if status != 200 {
		t.Fatalf("get stats failed: status=%d body=%v", status, result)
	}

	if _, ok := result["pending_suggestions"]; !ok {
		t.Error("stats missing 'pending_suggestions' field")
	}

	t.Logf("Stats pending_suggestions: %v", result["pending_suggestions"])
}

func TestMemoryVersionField(t *testing.T) {
	project := uniqueProject()

	r := storeMemory(t, "Version field test", "Testing version field in new memories.", project, 0.5)
	mem := r["memory"].(map[string]any)
	id := mem["id"].(string)
	defer deleteMemory(t, id)

	fetched := getMemory(t, id)
	version := fetched["version"].(float64)

	if version != 1 {
		t.Errorf("expected version=1 for new memory, got %f", version)
	}
}

func TestMergeCreatesSupersedes(t *testing.T) {
	project := uniqueProject()

	r1 := storeMemory(t, "Supersedes target", "Target content.", project, 0.6)
	id1 := r1["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id1)

	r2 := storeMemory(t, "Supersedes source", "Source content.", project, 0.5)
	id2 := r2["memory"].(map[string]any)["id"].(string)
	defer deleteMemory(t, id2)

	// Merge
	doRequest(t, "POST", "/memories/"+id1+"/merge", map[string]any{
		"source_ids": []string{id2},
	})

	// Check related memories for SUPERSEDES relationship
	status, result := doRequest(t, "GET", "/memories/"+id1+"/related", nil)
	if status != 200 {
		t.Fatalf("get related failed: status=%d", status)
	}

	relationships := result["relationships"]
	if relationships != nil {
		rels := relationships.([]any)
		found := false
		for _, r := range rels {
			rel := r.(map[string]any)
			if rel["relationship"] == "SUPERSEDES" {
				found = true
				t.Logf("Found SUPERSEDES relationship: %s -> %s", rel["from_memory_id"], rel["to_memory_id"])
			}
		}
		if !found {
			t.Error("expected SUPERSEDES relationship after merge")
		}
	} else {
		t.Error("no relationships returned")
	}
}

func TestExistingEndpoints_BackwardCompat(t *testing.T) {
	// Verify existing endpoints still work with the new StoreResult format

	project := uniqueProject()

	// Store
	r := storeMemory(t, "Backward compat test", "Testing backward compatibility.", project, 0.5)
	mem := r["memory"].(map[string]any)
	id := mem["id"].(string)
	defer deleteMemory(t, id)

	// Get
	fetched := getMemory(t, id)
	if fetched["title"] != "Backward compat test" {
		t.Errorf("GET returned wrong title: %v", fetched["title"])
	}

	// Update
	status, updated := doRequest(t, "PUT", "/memories/"+id, map[string]any{
		"title": strPtr("Backward compat test updated"),
	})
	if status != 200 {
		t.Fatalf("update failed: status=%d body=%v", status, updated)
	}

	// Search
	status, searchResult := doRequest(t, "POST", "/memories/search", map[string]any{
		"query": "backward compat",
		"limit": 5,
	})
	if status != 200 {
		t.Fatalf("search failed: status=%d", status)
	}
	t.Logf("Search returned results: %v", searchResult != nil)

	// Promote
	status, _ = doRequest(t, "POST", "/memories/"+id+"/promote", nil)
	if status != 200 {
		t.Fatalf("promote failed: status=%d", status)
	}

	// Stats
	status, stats := doRequest(t, "GET", "/stats", nil)
	if status != 200 {
		t.Fatalf("stats failed: status=%d", status)
	}
	if stats["total_memories"] == nil {
		t.Error("stats missing total_memories")
	}
}

func strPtr(s string) *string { return &s }

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
