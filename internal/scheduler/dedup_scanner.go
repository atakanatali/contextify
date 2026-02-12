package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/atakanatali/contextify/internal/memory"
)

type DedupScanner struct {
	svc      *memory.Service
	interval time.Duration
	stop     chan struct{}
}

func NewDedupScanner(svc *memory.Service, interval time.Duration) *DedupScanner {
	return &DedupScanner{
		svc:      svc,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (d *DedupScanner) Start() {
	slog.Info("starting dedup scanner", "interval", d.interval)
	go d.run()
}

func (d *DedupScanner) Stop() {
	close(d.stop)
}

func (d *DedupScanner) run() {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)

			// Scan for duplicates
			count, err := d.svc.ScanForDuplicates(ctx)
			if err != nil {
				slog.Error("dedup scan failed", "error", err)
			} else if count > 0 {
				slog.Info("dedup scanner found suggestions", "count", count)
			}

			// Also cleanup old replaced memories
			cleaned, err := d.svc.CleanupReplaced(ctx)
			if err != nil {
				slog.Error("replaced cleanup failed", "error", err)
			} else if cleaned > 0 {
				slog.Info("cleaned up replaced memories", "count", cleaned)
			}

			cancel()
		case <-d.stop:
			slog.Info("dedup scanner stopped")
			return
		}
	}
}
