package ingest

import "context"

// RawJob is unvalidated, unnormalized, and unprocessed data that is scraped from a source. It is the raw input to the ingest pipeline.
type RawJob struct {
	Source string // "linkedin", "indeed", etc.
	Title string
	Company string
	CompanyURL string
	ExternalID string // if the scraper can provide it, otherwise empty
	Location string
	Description string
	URL string
	PostedAt string // raw string, to be parsed later by the normalizer
	RawData map[string]interface{} // full source payload for debugging
}

// Scraper is an interface that defines the contract for any job scraper.
type Scraper interface {
	// SourceID returns the stable identifier for the source, e.g. "indeed", "linkedin", etc.
	Source() string

	// Scrape fetches raw job data from the source and returns a slice of RawJob.
	// It should be idempotent and stateless
	Scrape(ctx context.Context) ([]RawJob, error)
}
