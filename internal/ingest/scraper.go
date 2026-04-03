package ingest

import (
	"context"

	"opportunity-radar/internal/ingest/normalize"
)

// Scraper is an interface that defines the contract for any job scraper.
type Scraper interface {
	// SourceID returns the stable identifier for the source, e.g. "indeed", "linkedin", etc.
	Source() string

	// Scrape fetches raw job data from the source and returns a slice of RawJob.
	// It should be idempotent and stateless
	Scrape(ctx context.Context) ([]normalize.RawJob, error)
}
