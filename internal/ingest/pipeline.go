package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opportunity-radar/internal/jobs"
	"opportunity-radar/internal/scoring"
)

type Pipeline struct {
	normalizer Normalizer
	scorer     scoring.Scorer
	jobService JobService
	companyService CompanyService
	logger     *slog.Logger
}

func NewPipeline(
	normalizer Normalizer,
	scorer scoring.Scorer,
	jobService JobService,
	logger *slog.Logger,
) *Pipeline {
	return &Pipeline{
		normalizer: normalizer,
		scorer:     scorer,
		jobService: jobService,
		logger:     logger,
	}
}

// Run executes the full pipeline for a single scraper.
func (p *Pipeline) Run(ctx context.Context, scraper Scraper) error {
	p.logger.Info("ingest pipeline starting", "source", scraper.Source())

	// 1. Scrape
	rawJobs, err := scraper.Scrape(ctx)
	if err != nil {
		return fmt.Errorf("scraping %s: %w", scraper.Source(), err)
	}

	p.logger.Info("scrape complete", "source", scraper.Source(), "jobs_found", len(rawJobs))

	var saved, skipped, failed int

	for _, rawJob := range rawJobs {
		// 2. Normalize
		normalizedJob, err := p.normalizer.Normalize(rawJob)
		if err != nil {
			p.logger.Warn("normalization failed",
				"source", rawJob.Source,
				"error", err,
			)
			failed++
			continue
		}

		// 3. Find or create company
		var companyID int64

		company, err := p.companyService.FindOrCreate(ctx, normalizedJob.Company)
		if err != nil {
			p.logger.Warn("could not resolve company, using unknown",
				"source", normalizedJob.Source,
				"url", normalizedJob.Company.Domain,
				"error", err,
			)
			// don't skip the job, just fall through to ID 0, i.e., consider a job with an unknown company as a valid job, and give it company_id = 0 (the sentinel company record)
		} else {
			companyID = company.ID
		}

		// TODO: Add application deadline
		job := &jobs.Job{
			CompanyID: companyID,
			Title:       normalizedJob.Title,
			Description: normalizedJob.Description,
			Location:    normalizedJob.Location,
			URL:         normalizedJob.URL,
			Source:      normalizedJob.Source,
			PostedAt:    normalizedJob.PostedAt,
		}

		job.CompanyID = companyID

		// 4. Score
		job.Score = p.scorer.Score(job)

		// 4. Save
		if err := p.jobService.Save(ctx, job); err != nil {
			// If there's a duplicate, just skip it
			// TODO: Check whether this error should change ErrConflict from repository_errors.go
			if errors.Is(err, jobs.ErrJobAlreadyExists) {
				skipped++
				continue
			}
			p.logger.Error("failed to save job",
				"source", rawJob.Source,
				"error", err,
			)
			failed++
			continue
		}

		saved++
	}

	p.logger.Info("ingest pipeline complete",
		"source", scraper.Source(),
		"saved", saved,
		"skipped", skipped,
		"failed", failed,
	)

	return nil
}
