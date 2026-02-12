package config

import (
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
	VectorWeight  float64 `yaml:"vector_weight"`
	KeywordWeight float64 `yaml:"keyword_weight"`
	DefaultLimit  int     `yaml:"default_limit"`
	MaxLimit      int     `yaml:"max_limit"`
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
		Search:    SearchConfig{VectorWeight: 0.7, KeywordWeight: 0.3, DefaultLimit: 20, MaxLimit: 100},
	}

	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, err
			}
		}
	}

	applyEnvOverrides(cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
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
}
