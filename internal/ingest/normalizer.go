package ingest

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"opportunity-radar/internal/companies"
)

// Normalized job structure after normalization. This is the input to the company, step of the pipeline.
type NormalizedJob struct {
	Source      string // "linkedin", "indeed", etc.
	Title       string
	Company     *companies.Company
	ExternalID  string
	Location    string
	Description string
	URL         string
	PostedAt    time.Time
}

type Normalizer interface {
	Normalize(raw RawJob) (*NormalizedJob, error)
}

type DefaultNormalizer struct{}

func (n *DefaultNormalizer) Normalize(raw RawJob) (*NormalizedJob, error) {
	postedAt, err := parseDate(raw.PostedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing posted_at %q: %w", raw.PostedAt, err)
	}

	url := strings.TrimSpace(raw.URL)

	company := &companies.Company{
		Name:       raw.Company,
		Source:     raw.Source,
		ExternalID: raw.ExternalID,
		Domain:     extractDomain(url),
	}

	return &NormalizedJob{
		Source:      raw.Source,
		Title:       strings.TrimSpace(raw.Title),
		Company: company,
		Description: raw.Description,
		Location:    raw.Location,
		URL:         strings.TrimSpace(raw.URL),
		PostedAt:    postedAt,
	}, nil
}

var parseDateLayouts = [...]string{
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	for _, layout := range parseDateLayouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported date format")
}

func extractDomain(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	// Ensure a scheme is present so url.Parse can extract the host reliably.
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	host = strings.ToLower(host)
	if strings.HasPrefix(host, "www.") {
		host = strings.TrimPrefix(host, "www.")
	}
	return host
}

// TODO: Add better normalizers, for the different scrapers
