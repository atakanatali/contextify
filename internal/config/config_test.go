package config

import (
	"os"
	"testing"
)

func TestLoad_AppliesStewardEnvOverrides(t *testing.T) {
	t.Setenv("STEWARD_ENABLED", "1")
	t.Setenv("STEWARD_DRY_RUN", "0")
	t.Setenv("STEWARD_MODEL", "qwen2.5:1.5b")
	t.Setenv("STEWARD_TICK_INTERVAL", "45s")
	t.Setenv("STEWARD_AUTO_MERGE_THRESHOLD", "0.95")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if !cfg.Steward.Enabled {
		t.Fatalf("expected steward enabled")
	}
	if cfg.Steward.DryRun {
		t.Fatalf("expected steward dry_run false from env override")
	}
	if cfg.Steward.Model != "qwen2.5:1.5b" {
		t.Fatalf("unexpected steward model: %q", cfg.Steward.Model)
	}
	if cfg.Steward.TickInterval.String() != "45s" {
		t.Fatalf("unexpected tick interval: %v", cfg.Steward.TickInterval)
	}
	if cfg.Steward.AutoMergeThreshold != 0.95 {
		t.Fatalf("unexpected auto_merge_threshold: %v", cfg.Steward.AutoMergeThreshold)
	}
}

func TestLoad_RejectsInvalidStewardThreshold(t *testing.T) {
	t.Setenv("STEWARD_AUTO_MERGE_THRESHOLD", "1.5")

	if _, err := Load(""); err == nil {
		t.Fatalf("expected validation error for invalid threshold")
	}
}

func TestLoad_RejectsInvalidStewardDuration(t *testing.T) {
	t.Setenv("STEWARD_TICK_INTERVAL", "not-a-duration")

	if _, err := Load(""); err == nil {
		t.Fatalf("expected parse error for invalid duration")
	}
}

func TestMain(m *testing.M) {
	// Prevent ambient environment from affecting config tests unpredictably.
	os.Unsetenv("STEWARD_ENABLED")
	os.Unsetenv("STEWARD_DRY_RUN")
	os.Unsetenv("STEWARD_MODEL")
	os.Unsetenv("STEWARD_TICK_INTERVAL")
	os.Unsetenv("STEWARD_AUTO_MERGE_THRESHOLD")
	os.Unsetenv("STEWARD_DERIVATION_MIN_CONFIDENCE")
	os.Unsetenv("STEWARD_DERIVATION_MIN_NOVELTY")
	os.Exit(m.Run())
}
