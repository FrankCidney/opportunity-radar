package ingest

import (
	"fmt"
	"strings"

	"opportunity-radar/internal/jobs"
)

type Normalizer interface {
	Normalize(raw RawJob) (*jobs.Job, error)
}

type DefaultNormalizer struct{}

func (n *DefaultNormalizer) Normalize(raw RawJob) (*jobs.Job, error) {
	postedAt, err := parseDate(raw.PostedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing posted_at %q: %w", raw.PostedAt, err)
	}

	return &jobs.Job{
		Source:      raw.Source,
		Title:       strings.TrimSpace(raw.Title),
		Description: raw.Description,
		Location:    raw.Location,
		URL:         raw.URL,
		PostedAt:    postedAt,
	}, nil
}
