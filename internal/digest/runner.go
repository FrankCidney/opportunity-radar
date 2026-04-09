package digest

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type IngestRunner interface {
	RunAll(ctx context.Context) error
}

type RunEligibilityChecker interface {
	CanRun(ctx context.Context) (bool, error)
}

type Runner struct {
	ingestRunner IngestRunner
	eligibility  RunEligibilityChecker
	digest       *Service
	logger       *slog.Logger
	now          func() time.Time
	mu           sync.RWMutex
	lastSummary  string
}

func NewRunner(ingestRunner IngestRunner, eligibility RunEligibilityChecker, digest *Service, logger *slog.Logger) *Runner {
	return &Runner{
		ingestRunner: ingestRunner,
		eligibility:  eligibility,
		digest:       digest,
		logger:       logger,
		now:          time.Now,
	}
}

func (r *Runner) RunAll(ctx context.Context) error {
	if r.eligibility != nil {
		canRun, err := r.eligibility.CanRun(ctx)
		if err != nil {
			r.setLastSummary("Run failed before ingest.")
			return err
		}
		if !canRun {
			r.logger.Info("scheduled run skipped because setup is incomplete")
			r.setLastSummary("Run skipped because setup was incomplete.")
			return nil
		}
	}

	if err := r.ingestRunner.RunAll(ctx); err != nil {
		r.setLastSummary("Run failed during ingest.")
		return err
	}

	if r.digest == nil {
		r.setLastSummary("Run completed.")
		return nil
	}

	result, err := r.digest.SendDailyResult(ctx, r.now())
	if err != nil {
		r.logger.Error("daily digest failed after ingest", "error", err)
		r.setLastSummary("Run completed, but email updates failed.")
		return nil
	}

	r.setLastSummary(result.Summary)
	return nil
}

func (r *Runner) LastSummary() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastSummary
}

func (r *Runner) setLastSummary(summary string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lastSummary = summary
}
