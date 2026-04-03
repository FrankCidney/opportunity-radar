package digest

import (
	"context"
	"log/slog"
	"time"
)

type IngestRunner interface {
	RunAll(ctx context.Context) error
}

type Runner struct {
	ingestRunner IngestRunner
	digest       *Service
	logger       *slog.Logger
	now          func() time.Time
}

func NewRunner(ingestRunner IngestRunner, digest *Service, logger *slog.Logger) *Runner {
	return &Runner{
		ingestRunner: ingestRunner,
		digest:       digest,
		logger:       logger,
		now:          time.Now,
	}
}

func (r *Runner) RunAll(ctx context.Context) error {
	if err := r.ingestRunner.RunAll(ctx); err != nil {
		return err
	}

	if r.digest == nil {
		return nil
	}

	if err := r.digest.SendDaily(ctx, r.now()); err != nil {
		r.logger.Error("daily digest failed after ingest", "error", err)
	}

	return nil
}
