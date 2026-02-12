package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/atakanatali/contextify/internal/memory"
)

// ProjectNormalizerJob periodically scans all project_ids in the database
// and normalizes them using the ProjectNormalizer. This cleans up legacy
// entries that were stored before normalization was enabled.
type ProjectNormalizerJob struct {
	svc      *memory.Service
	interval time.Duration
	stop     chan struct{}
}

func NewProjectNormalizerJob(svc *memory.Service, interval time.Duration) *ProjectNormalizerJob {
	return &ProjectNormalizerJob{
		svc:      svc,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (j *ProjectNormalizerJob) Start() {
	slog.Info("starting project normalizer job", "interval", j.interval)
	go j.run()
}

func (j *ProjectNormalizerJob) Stop() {
	close(j.stop)
}

func (j *ProjectNormalizerJob) run() {
	// Run once immediately at startup
	j.normalize()

	ticker := time.NewTicker(j.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			j.normalize()
		case <-j.stop:
			slog.Info("project normalizer job stopped")
			return
		}
	}
}

func (j *ProjectNormalizerJob) normalize() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	updated, err := j.svc.NormalizeAllProjectIDs(ctx)
	if err != nil {
		slog.Error("project normalization failed", "error", err)
	} else if updated > 0 {
		slog.Info("project normalizer updated project_ids", "count", updated)
	}
}
