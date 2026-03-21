package ingest

import (
	"context"
	"log/slog"

	"opportunity-radar/internal/jobs"
)

// JobService is the ingest package's view of what it needs from jobs.
// The real *jobs.Service satisfies this.
type JobService interface {
	Save(ctx context.Context, job *jobs.Job) error
}

type Service struct {
	pipeline *Pipeline
	scrapers []Scraper
	logger *slog.Logger
}

func NewService(pipeline *Pipeline, scrapers []Scraper, logger *slog.Logger) *Service {
	return &Service{
		pipeline: pipeline,
		scrapers: scrapers,
		logger: logger,
	}
}

// RunAll runs the pipeline for every registered scraper.
func (s *Service) RunAll(ctx context.Context) error {
	for _, scraper := range s.scrapers {
		if err := s.pipeline.Run(ctx, scraper); err != nil {
			// log and continue - don't let one bad scraper kill the run
			s.logger.Error("pipeline failed",
				"source", scraper.Source(),
				"error", err,
			)
		}
	}
	return nil
}
