package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/atakanatali/contextify/internal/memory"
)

type Cleanup struct {
	svc      *memory.Service
	interval time.Duration
	stop     chan struct{}
}

func NewCleanup(svc *memory.Service, interval time.Duration) *Cleanup {
	return &Cleanup{
		svc:      svc,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (c *Cleanup) Start() {
	slog.Info("starting TTL cleanup scheduler", "interval", c.interval)
	go c.run()
}

func (c *Cleanup) Stop() {
	close(c.stop)
}

func (c *Cleanup) run() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			count, err := c.svc.CleanupExpired(ctx)
			cancel()
			if err != nil {
				slog.Error("cleanup failed", "error", err)
			} else if count > 0 {
				slog.Info("cleaned up expired memories", "count", count)
			}
		case <-c.stop:
			slog.Info("cleanup scheduler stopped")
			return
		}
	}
}
