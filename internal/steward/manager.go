package steward

import (
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atakanatali/contextify/internal/config"
	"github.com/atakanatali/contextify/internal/memory"
)

// Manager is a bootstrap placeholder for STW04 runtime implementation.
type Manager struct {
	pool      *pgxpool.Pool
	svc       *memory.Service
	cfg       config.StewardConfig
	ollamaURL string
}

func NewManager(pool *pgxpool.Pool, svc *memory.Service, cfg config.StewardConfig, embeddingOllamaURL string) *Manager {
	ollamaURL := cfg.OllamaURL
	if ollamaURL == "" {
		ollamaURL = embeddingOllamaURL
	}
	return &Manager{
		pool:      pool,
		svc:       svc,
		cfg:       cfg,
		ollamaURL: ollamaURL,
	}
}

func (m *Manager) Start() {
	if !m.cfg.Enabled {
		slog.Info("steward disabled")
		return
	}
	slog.Info("steward bootstrap initialized", "dry_run", m.cfg.DryRun, "model", m.cfg.Model, "ollama_url", m.ollamaURL)
}

func (m *Manager) Stop() {}
