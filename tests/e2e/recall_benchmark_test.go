//go:build e2e
// +build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
	"time"
)

type funnelSnapshot struct {
	RecallAttempts     int64 `json:"recall_attempts"`
	RecallHits         int64 `json:"recall_hits"`
	StoreOpportunities int64 `json:"store_opportunities"`
	StoreActions       int64 `json:"store_actions"`
}

type benchmarkReport struct {
	GeneratedAt  string         `json:"generated_at"`
	ProjectID    string         `json:"project_id"`
	Calls        int            `json:"calls"`
	Hits         int            `json:"hits"`
	HitRate      float64        `json:"hit_rate"`
	P50Ms        int            `json:"p50_ms"`
	P95Ms        int            `json:"p95_ms"`
	LatenciesMs  []int          `json:"latencies_ms"`
	MaxP95Ms     int            `json:"max_p95_ms"`
	MinHitRate   float64        `json:"min_hit_rate"`
	FunnelBefore funnelSnapshot `json:"funnel_before"`
	FunnelAfter  funnelSnapshot `json:"funnel_after"`
	FunnelDelta  funnelSnapshot `json:"funnel_delta"`
}

func TestRecallBenchmark(t *testing.T) {
	project := uniqueProject()
	var ids []string

	funnelBefore := getFunnelSnapshot(t, project)

	for i := 0; i < 40; i++ {
		title := fmt.Sprintf("Benchmark memory %02d", i)
		content := fmt.Sprintf("This is benchmark memory %02d about service timeout tuning and retry policy.", i)
		result := storeMemory(t, title, content, project, 0.65)
		mem := result["memory"].(map[string]any)
		ids = append(ids, mem["id"].(string))
	}
	defer func() {
		for _, id := range ids {
			deleteMemory(t, id)
		}
	}()

	queries := []string{
		"service timeout tuning",
		"retry policy",
		"benchmark memory 01",
		"benchmark memory 09",
		"benchmark memory 17",
		"benchmark memory 25",
		"benchmark memory 33",
		"how to tune timeout for service",
		"retry configuration for transient failure",
		"memory about timeout and retry",
	}

	var latencies []int
	hitCount := 0
	totalCalls := 0

	for _, q := range queries {
		for i := 0; i < 3; i++ {
			start := time.Now()
			status, results := doRequestArray(t, "POST", "/memories/recall", map[string]any{
				"query":      q,
				"project_id": project,
				"limit":      10,
			})
			elapsedMs := int(time.Since(start).Milliseconds())
			latencies = append(latencies, elapsedMs)
			totalCalls++

			if status != 200 {
				t.Fatalf("recall failed: status=%d query=%q", status, q)
			}
			if len(results) > 0 {
				hitCount++
			}
		}
	}

	sort.Ints(latencies)
	p50 := percentile(latencies, 50)
	p95 := percentile(latencies, 95)
	hitRate := float64(hitCount) / float64(totalCalls)

	maxP95 := envInt("RECALL_BENCH_MAX_P95_MS", 2500)
	minHitRate := envFloat("RECALL_BENCH_MIN_HIT_RATE", 0.80)
	funnelAfter := getFunnelSnapshot(t, project)
	funnelDelta := funnelSnapshot{
		RecallAttempts:     funnelAfter.RecallAttempts - funnelBefore.RecallAttempts,
		RecallHits:         funnelAfter.RecallHits - funnelBefore.RecallHits,
		StoreOpportunities: funnelAfter.StoreOpportunities - funnelBefore.StoreOpportunities,
		StoreActions:       funnelAfter.StoreActions - funnelBefore.StoreActions,
	}

	t.Logf("Recall benchmark summary: calls=%d hits=%d hit_rate=%.3f p50_ms=%d p95_ms=%d", totalCalls, hitCount, hitRate, p50, p95)
	t.Logf("Thresholds: max_p95_ms=%d min_hit_rate=%.3f", maxP95, minHitRate)
	t.Logf("Funnel delta: recall_attempts=%d recall_hits=%d store_opportunities=%d store_actions=%d",
		funnelDelta.RecallAttempts, funnelDelta.RecallHits, funnelDelta.StoreOpportunities, funnelDelta.StoreActions)

	if reportPath := os.Getenv("RECALL_BENCH_REPORT_PATH"); reportPath != "" {
		report := benchmarkReport{
			GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
			ProjectID:    project,
			Calls:        totalCalls,
			Hits:         hitCount,
			HitRate:      hitRate,
			P50Ms:        p50,
			P95Ms:        p95,
			LatenciesMs:  latencies,
			MaxP95Ms:     maxP95,
			MinHitRate:   minHitRate,
			FunnelBefore: funnelBefore,
			FunnelAfter:  funnelAfter,
			FunnelDelta:  funnelDelta,
		}
		writeBenchmarkReport(t, reportPath, report)
		t.Logf("Benchmark report written: %s", reportPath)
	}

	if p95 > maxP95 {
		t.Fatalf("p95 latency too high: p95=%dms max=%dms", p95, maxP95)
	}
	if hitRate < minHitRate {
		t.Fatalf("hit rate too low: got=%.3f min=%.3f", hitRate, minHitRate)
	}
}

func percentile(values []int, p int) int {
	if len(values) == 0 {
		return 0
	}
	if p <= 0 {
		return values[0]
	}
	if p >= 100 {
		return values[len(values)-1]
	}
	idx := (len(values) - 1) * p / 100
	return values[idx]
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return n
}

func getFunnelSnapshot(t *testing.T, project string) funnelSnapshot {
	t.Helper()
	path := "/analytics/funnel?days=1&project_id=" + url.QueryEscape(project)
	status, data := doRequest(t, "GET", path, nil)
	if status != 200 {
		t.Fatalf("funnel analytics failed: status=%d body=%v", status, data)
	}
	return funnelSnapshot{
		RecallAttempts:     toInt64(data["recall_attempts"]),
		RecallHits:         toInt64(data["recall_hits"]),
		StoreOpportunities: toInt64(data["store_opportunities"]),
		StoreActions:       toInt64(data["store_actions"]),
	}
}

func toInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float32:
		return int64(n)
	case float64:
		return int64(n)
	default:
		return 0
	}
}

func writeBenchmarkReport(t *testing.T, path string, report benchmarkReport) {
	t.Helper()

	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("create report directory: %v", err)
		}
	}

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatalf("marshal benchmark report: %v", err)
	}
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write benchmark report: %v", err)
	}
}
