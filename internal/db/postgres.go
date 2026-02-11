package db

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/atakanatali/contextify/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	poolCfg.MinConns = int32(cfg.MaxIdleConns)
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime

	const maxRetries = 10

	var pool *pgxpool.Pool
	for attempt := 1; attempt <= maxRetries; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
		if err != nil {
			slog.Warn("database connection failed, retrying...", "attempt", attempt, "max", maxRetries, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
			continue
		}

		if err = pool.Ping(ctx); err != nil {
			pool.Close()
			slog.Warn("database ping failed, retrying...", "attempt", attempt, "max", maxRetries, "error", err)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
			continue
		}

		slog.Info("connected to PostgreSQL", "url", maskURL(cfg.URL))
		return pool, nil
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

func RunMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Create migrations tracking table
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version := entry.Name()

		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", version, err)
		}
		if exists {
			continue
		}

		content, err := migrationsFS.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", version, err)
		}

		// Migrations with "-- no-transaction" marker run outside a transaction.
		// Required for statements like ALTER TYPE ADD VALUE which cannot run in a transaction.
		noTx := strings.HasPrefix(string(content), "-- no-transaction")

		if noTx {
			if _, err := pool.Exec(ctx, string(content)); err != nil {
				return fmt.Errorf("execute migration %s: %w", version, err)
			}
			if _, err := pool.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
				return fmt.Errorf("record migration %s: %w", version, err)
			}
		} else {
			tx, err := pool.Begin(ctx)
			if err != nil {
				return fmt.Errorf("begin transaction for %s: %w", version, err)
			}

			if _, err := tx.Exec(ctx, string(content)); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("execute migration %s: %w", version, err)
			}

			if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
				tx.Rollback(ctx)
				return fmt.Errorf("record migration %s: %w", version, err)
			}

			if err := tx.Commit(ctx); err != nil {
				return fmt.Errorf("commit migration %s: %w", version, err)
			}
		}

		slog.Info("applied migration", "version", version)
	}

	return nil
}

func maskURL(url string) string {
	// Mask password in URL for logging
	parts := strings.SplitN(url, "@", 2)
	if len(parts) != 2 {
		return url
	}
	prefix := strings.SplitN(parts[0], ":", 3)
	if len(prefix) < 3 {
		return url
	}
	return prefix[0] + ":" + prefix[1] + ":***@" + parts[1]
}
