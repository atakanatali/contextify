package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Embedding EmbeddingConfig `yaml:"embedding"`
	Memory    MemoryConfig    `yaml:"memory"`
	Search    SearchConfig    `yaml:"search"`
	Steward   StewardConfig   `yaml:"steward"`
}

type ServerConfig struct {
	Port int    `yaml:"port"`
	Host string `yaml:"host"`
}

type DatabaseConfig struct {
	URL             string        `yaml:"url"`
	MaxOpenConns    int           `yaml:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime"`
}

type EmbeddingConfig struct {
	Provider   string `yaml:"provider"`
	OllamaURL  string `yaml:"ollama_url"`
	Model      string `yaml:"model"`
	Dimensions int    `yaml:"dimensions"`
}

type MemoryConfig struct {
	DefaultTTL         int                 `yaml:"default_ttl"`
	PromoteAccessCount int                 `yaml:"promote_access_count"`
	PromoteImportance  float64             `yaml:"promote_importance"`
	TTLExtendFactor    float64             `yaml:"ttl_extend_factor"`
	CleanupInterval    time.Duration       `yaml:"cleanup_interval"`
	NormalizeProjectID bool                `yaml:"normalize_project_id"`
	Consolidation      ConsolidationConfig `yaml:"consolidation"`
}

type ConsolidationConfig struct {
	Enabled            bool          `yaml:"enabled"`
	AutoMergeThreshold float64       `yaml:"auto_merge_threshold"`
	SuggestThreshold   float64       `yaml:"suggest_threshold"`
	MergeStrategy      string        `yaml:"merge_strategy"`
	ScanInterval       time.Duration `yaml:"scan_interval"`
	ScanBatchSize      int           `yaml:"scan_batch_size"`
	ReplacedRetention  time.Duration `yaml:"replaced_retention"`
}

type SearchConfig struct {
	VectorWeight    float64       `yaml:"vector_weight"`
	KeywordWeight   float64       `yaml:"keyword_weight"`
	DefaultLimit    int           `yaml:"default_limit"`
	MaxLimit        int           `yaml:"max_limit"`
	CacheEnabled    bool          `yaml:"cache_enabled"`
	CacheTTL        time.Duration `yaml:"cache_ttl"`
	CacheMaxEntries int           `yaml:"cache_max_entries"`
}

type StewardConfig struct {
	Enabled                    bool                 `yaml:"enabled"`
	DryRun                     bool                 `yaml:"dry_run"`
	TickInterval               time.Duration        `yaml:"tick_interval"`
	ClaimBatchSize             int                  `yaml:"claim_batch_size"`
	MaxAttempts                int                  `yaml:"max_attempts"`
	RequestTimeout             time.Duration        `yaml:"request_timeout"`
	Model                      string               `yaml:"model"`
	OllamaURL                  string               `yaml:"ollama_url"`
	AutoMergeThreshold         float64              `yaml:"auto_merge_threshold"`
	AutoMergeFromSuggestions   bool                 `yaml:"auto_merge_from_suggestions"`
	LLMConflictGuardEnabled    bool                 `yaml:"llm_conflict_guard_enabled"`
	Derivation                 StewardDerivation    `yaml:"derivation"`
	SelfLearn                  StewardSelfLearn     `yaml:"self_learn"`
	Retention                  StewardRetention     `yaml:"retention"`
}

type StewardDerivation struct {
	Enabled       bool    `yaml:"enabled"`
	MaxCandidates int     `yaml:"max_candidates"`
	MinConfidence float64 `yaml:"min_confidence"`
	MinNovelty    float64 `yaml:"min_novelty"`
}

type StewardSelfLearn struct {
	Enabled       bool          `yaml:"enabled"`
	EvalInterval  time.Duration `yaml:"eval_interval"`
	MinSampleSize int           `yaml:"min_sample_size"`
}

type StewardRetention struct {
	RunLogDays   int `yaml:"run_log_days"`
	EventLogDays int `yaml:"event_log_days"`
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		Server:    ServerConfig{Port: 8420, Host: "0.0.0.0"},
		Database:  DatabaseConfig{URL: "postgres://contextify:contextify_local@localhost:5432/contextify?sslmode=disable", MaxOpenConns: 25, MaxIdleConns: 5, ConnMaxLifetime: 5 * time.Minute},
		Embedding: EmbeddingConfig{Provider: "ollama", OllamaURL: "http://localhost:11434", Model: "nomic-embed-text", Dimensions: 768},
		Memory: MemoryConfig{
			DefaultTTL: 86400, PromoteAccessCount: 5, PromoteImportance: 0.8, TTLExtendFactor: 0.5, CleanupInterval: 5 * time.Minute, NormalizeProjectID: true,
			Consolidation: ConsolidationConfig{
				Enabled:            true,
				AutoMergeThreshold: 0.92,
				SuggestThreshold:   0.75,
				MergeStrategy:      "smart_merge",
				ScanInterval:       1 * time.Hour,
				ScanBatchSize:      100,
				ReplacedRetention:  7 * 24 * time.Hour,
			},
		},
		Search: SearchConfig{
			VectorWeight:    0.7,
			KeywordWeight:   0.3,
			DefaultLimit:    20,
			MaxLimit:        100,
			CacheEnabled:    true,
			CacheTTL:        30 * time.Second,
			CacheMaxEntries: 500,
		},
		Steward: StewardConfig{
			Enabled:                  false,
			DryRun:                   true,
			TickInterval:             30 * time.Second,
			ClaimBatchSize:           10,
			MaxAttempts:              3,
			RequestTimeout:           30 * time.Second,
			Model:                    "qwen2.5:3b",
			OllamaURL:                "",
			AutoMergeThreshold:       0.92,
			AutoMergeFromSuggestions: true,
			LLMConflictGuardEnabled:  false,
			Derivation: StewardDerivation{
				Enabled:       false,
				MaxCandidates: 3,
				MinConfidence: 0.8,
				MinNovelty:    0.2,
			},
			SelfLearn: StewardSelfLearn{
				Enabled:       false,
				EvalInterval:  24 * time.Hour,
				MinSampleSize: 100,
			},
			Retention: StewardRetention{
				RunLogDays:   14,
				EventLogDays: 14,
			},
		},
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}
	if err := validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) error {
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}
	if v := os.Getenv("OLLAMA_URL"); v != "" {
		cfg.Embedding.OllamaURL = v
	}
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		cfg.Embedding.Model = v
	}
	if v := os.Getenv("SERVER_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}
	if v := os.Getenv("CONSOLIDATION_ENABLED"); v != "" {
		cfg.Memory.Consolidation.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("CONSOLIDATION_AUTO_MERGE_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Memory.Consolidation.AutoMergeThreshold = f
		}
	}
	if v := os.Getenv("CONSOLIDATION_SUGGEST_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Memory.Consolidation.SuggestThreshold = f
		}
	}
	if v := os.Getenv("CONSOLIDATION_MERGE_STRATEGY"); v != "" {
		cfg.Memory.Consolidation.MergeStrategy = v
	}
	if v := os.Getenv("NORMALIZE_PROJECT_ID"); v != "" {
		cfg.Memory.NormalizeProjectID = v == "true" || v == "1"
	}
	if v := os.Getenv("SEARCH_CACHE_ENABLED"); v != "" {
		cfg.Search.CacheEnabled = v == "true" || v == "1"
	}
	if v := os.Getenv("SEARCH_CACHE_TTL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid SEARCH_CACHE_TTL: %w", err)
		}
		cfg.Search.CacheTTL = d
	}
	if v := os.Getenv("SEARCH_CACHE_MAX_ENTRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Search.CacheMaxEntries = n
		}
	}
	if v := os.Getenv("STEWARD_ENABLED"); v != "" {
		cfg.Steward.Enabled = parseBool(v)
	}
	if v := os.Getenv("STEWARD_DRY_RUN"); v != "" {
		cfg.Steward.DryRun = parseBool(v)
	}
	if v := os.Getenv("STEWARD_TICK_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_TICK_INTERVAL: %w", err)
		}
		cfg.Steward.TickInterval = d
	}
	if v := os.Getenv("STEWARD_CLAIM_BATCH_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_CLAIM_BATCH_SIZE: %w", err)
		}
		cfg.Steward.ClaimBatchSize = n
	}
	if v := os.Getenv("STEWARD_MAX_ATTEMPTS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_MAX_ATTEMPTS: %w", err)
		}
		cfg.Steward.MaxAttempts = n
	}
	if v := os.Getenv("STEWARD_REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_REQUEST_TIMEOUT: %w", err)
		}
		cfg.Steward.RequestTimeout = d
	}
	if v := os.Getenv("STEWARD_MODEL"); v != "" {
		cfg.Steward.Model = v
	}
	if v := os.Getenv("STEWARD_OLLAMA_URL"); v != "" {
		cfg.Steward.OllamaURL = v
	}
	if v := os.Getenv("STEWARD_AUTO_MERGE_THRESHOLD"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_AUTO_MERGE_THRESHOLD: %w", err)
		}
		cfg.Steward.AutoMergeThreshold = f
	}
	if v := os.Getenv("STEWARD_AUTO_MERGE_FROM_SUGGESTIONS"); v != "" {
		cfg.Steward.AutoMergeFromSuggestions = parseBool(v)
	}
	if v := os.Getenv("STEWARD_LLM_CONFLICT_GUARD_ENABLED"); v != "" {
		cfg.Steward.LLMConflictGuardEnabled = parseBool(v)
	}
	if v := os.Getenv("STEWARD_DERIVATION_ENABLED"); v != "" {
		cfg.Steward.Derivation.Enabled = parseBool(v)
	}
	if v := os.Getenv("STEWARD_DERIVATION_MAX_CANDIDATES"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_DERIVATION_MAX_CANDIDATES: %w", err)
		}
		cfg.Steward.Derivation.MaxCandidates = n
	}
	if v := os.Getenv("STEWARD_DERIVATION_MIN_CONFIDENCE"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_DERIVATION_MIN_CONFIDENCE: %w", err)
		}
		cfg.Steward.Derivation.MinConfidence = f
	}
	if v := os.Getenv("STEWARD_DERIVATION_MIN_NOVELTY"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_DERIVATION_MIN_NOVELTY: %w", err)
		}
		cfg.Steward.Derivation.MinNovelty = f
	}
	if v := os.Getenv("STEWARD_SELF_LEARN_ENABLED"); v != "" {
		cfg.Steward.SelfLearn.Enabled = parseBool(v)
	}
	if v := os.Getenv("STEWARD_SELF_LEARN_EVAL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_SELF_LEARN_EVAL_INTERVAL: %w", err)
		}
		cfg.Steward.SelfLearn.EvalInterval = d
	}
	if v := os.Getenv("STEWARD_SELF_LEARN_MIN_SAMPLE_SIZE"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_SELF_LEARN_MIN_SAMPLE_SIZE: %w", err)
		}
		cfg.Steward.SelfLearn.MinSampleSize = n
	}
	if v := os.Getenv("STEWARD_RETENTION_RUN_LOG_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_RETENTION_RUN_LOG_DAYS: %w", err)
		}
		cfg.Steward.Retention.RunLogDays = n
	}
	if v := os.Getenv("STEWARD_RETENTION_EVENT_LOG_DAYS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return fmt.Errorf("invalid STEWARD_RETENTION_EVENT_LOG_DAYS: %w", err)
		}
		cfg.Steward.Retention.EventLogDays = n
	}
	return nil
}

func parseBool(v string) bool {
	return v == "true" || v == "1" || v == "yes" || v == "on"
}

func validate(cfg *Config) error {
	if err := validateUnit("steward.auto_merge_threshold", cfg.Steward.AutoMergeThreshold); err != nil {
		return err
	}
	if err := validateUnit("steward.derivation.min_confidence", cfg.Steward.Derivation.MinConfidence); err != nil {
		return err
	}
	if err := validateUnit("steward.derivation.min_novelty", cfg.Steward.Derivation.MinNovelty); err != nil {
		return err
	}
	if cfg.Steward.Derivation.MinConfidence < cfg.Steward.Derivation.MinNovelty {
		return fmt.Errorf("invalid steward thresholds: derivation.min_confidence must be >= derivation.min_novelty")
	}
	if cfg.Steward.ClaimBatchSize <= 0 {
		return fmt.Errorf("invalid steward.claim_batch_size: must be > 0")
	}
	if cfg.Steward.MaxAttempts < 1 {
		return fmt.Errorf("invalid steward.max_attempts: must be >= 1")
	}
	if cfg.Steward.TickInterval <= 0 {
		return fmt.Errorf("invalid steward.tick_interval: must be > 0")
	}
	if cfg.Steward.RequestTimeout <= 0 {
		return fmt.Errorf("invalid steward.request_timeout: must be > 0")
	}
	if cfg.Steward.Derivation.MaxCandidates < 0 {
		return fmt.Errorf("invalid steward.derivation.max_candidates: must be >= 0")
	}
	if cfg.Steward.SelfLearn.MinSampleSize < 0 {
		return fmt.Errorf("invalid steward.self_learn.min_sample_size: must be >= 0")
	}
	return nil
}

func validateUnit(name string, v float64) error {
	if v < 0 || v > 1 {
		return fmt.Errorf("invalid %s: must be within [0,1]", name)
	}
	return nil
}
