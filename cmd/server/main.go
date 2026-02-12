package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/atakanatali/contextify/internal/api"
	"github.com/atakanatali/contextify/internal/config"
	"github.com/atakanatali/contextify/internal/db"
	"github.com/atakanatali/contextify/internal/embedding"
	"github.com/atakanatali/contextify/internal/mcp"
	"github.com/atakanatali/contextify/internal/memory"
	"github.com/atakanatali/contextify/internal/scheduler"
)

var version = "dev" // set via ldflags at build time

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	slog.Info("starting contextify", "version", version)

	// Load config
	cfg, err := config.Load("config.yaml")
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	pool, err := db.Connect(ctx, cfg.Database)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Run migrations
	if err := db.RunMigrations(ctx, pool); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	// Initialize embedding client
	embedClient := embedding.NewClient(cfg.Embedding)

	// Ensure embedding model is available
	if err := embedClient.EnsureModel(ctx); err != nil {
		slog.Warn("could not ensure embedding model", "error", err)
		// Don't exit - the model might be pulled later
	}

	// Initialize memory service
	repo := memory.NewRepository(pool)
	svc := memory.NewService(repo, embedClient, cfg.Memory, cfg.Search)

	// Start TTL cleanup scheduler
	cleanup := scheduler.NewCleanup(svc, cfg.Memory.CleanupInterval)
	cleanup.Start()
	defer cleanup.Stop()

	// Start dedup scanner (if consolidation enabled)
	if cfg.Memory.Consolidation.Enabled {
		dedupScanner := scheduler.NewDedupScanner(svc, cfg.Memory.Consolidation.ScanInterval)
		dedupScanner.Start()
		defer dedupScanner.Stop()
	}

	// Start project_id normalizer background job
	if cfg.Memory.NormalizeProjectID {
		normalizerJob := scheduler.NewProjectNormalizerJob(svc, 1*time.Hour)
		normalizerJob.Start()
		defer normalizerJob.Stop()
	}

	// Create MCP server
	mcpServer := mcp.NewServer(svc)

	// Create REST API router
	apiRouter := api.NewRouter(svc)

	// Combined HTTP server
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpServer.Handler())
	mux.Handle("/", apiRouter)

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		server.Shutdown(shutdownCtx)
	}()

	slog.Info("contextify server listening",
		"addr", addr,
		"mcp", "/mcp",
		"rest", "/api/v1/",
		"health", "/health",
	)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}
