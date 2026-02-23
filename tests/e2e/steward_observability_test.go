//go:build e2e
// +build e2e

package e2e

import (
	"net/http"
	"testing"
)

func TestStewardObservabilityEndpoints(t *testing.T) {
	if _, err := http.Get("http://localhost:8420/health"); err != nil {
		t.Skip("server not running for e2e")
	}
	statusCode, statusBody := doRequest(t, "GET", "/steward/status", nil)
	if statusCode == 404 {
		t.Skip("steward not configured in e2e server")
	}
	if statusCode != 200 {
		t.Fatalf("steward status failed: status=%d body=%v", statusCode, statusBody)
	}
	if _, ok := statusBody["enabled"]; !ok {
		t.Fatalf("expected steward status payload to include enabled")
	}

	runsCode, runsBody := doRequest(t, "GET", "/steward/runs?limit=5&offset=0", nil)
	if runsCode != 200 {
		t.Fatalf("steward runs failed: status=%d body=%v", runsCode, runsBody)
	}
	if _, ok := runsBody["runs"]; !ok {
		t.Fatalf("expected steward runs payload to include runs")
	}

	metricsCode, metricsBody := doRequest(t, "GET", "/steward/metrics", nil)
	if metricsCode != 200 {
		t.Fatalf("steward metrics failed: status=%d body=%v", metricsCode, metricsBody)
	}
	if _, ok := metricsBody["runs_last_hour"]; !ok {
		t.Fatalf("expected steward metrics payload to include runs_last_hour")
	}
}
