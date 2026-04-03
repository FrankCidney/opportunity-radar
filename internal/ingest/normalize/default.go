package normalize

import (
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode"

	"opportunity-radar/internal/companies"
)

// RawJob is unvalidated, unnormalized, and unprocessed data scraped from a source.
type RawJob struct {
	Source      string
	Title       string
	Company     string
	CompanyURL  string
	ExternalID  string
	Location    string
	Description string
	URL         string
	PostedAt    string
	RawData     map[string]interface{}
}

// NormalizedJob is the canonical job shape after normalization.
type NormalizedJob struct {
	Source      string
	Title       string
	Company     *companies.Company
	ExternalID  string
	Location    string
	Description string
	URL         string
	PostedAt    time.Time
}

func Normalize(raw RawJob) (*NormalizedJob, error) {
	job, err := applyDefaultNormalization(raw)
	if err != nil {
		return nil, err
	}

	job = applySourceOverrides(raw, job)
	return &job, nil
}

func applyDefaultNormalization(raw RawJob) (NormalizedJob, error) {
	postedAt, err := parseDate(raw.PostedAt)
	if err != nil {
		return NormalizedJob{}, fmt.Errorf("parsing posted_at %q: %w", raw.PostedAt, err)
	}

	jobURL := strings.TrimSpace(raw.URL)

	company := &companies.Company{
		Name:       normalizeCompanyName(raw.Company),
		Source:     strings.TrimSpace(raw.Source),
		ExternalID: strings.TrimSpace(raw.ExternalID),
		Domain:     extractDomain(jobURL),
	}

	return NormalizedJob{
		Source:      strings.TrimSpace(raw.Source),
		Title:       strings.TrimSpace(raw.Title),
		Company:     company,
		ExternalID:  strings.TrimSpace(raw.ExternalID),
		Description: strings.TrimSpace(raw.Description),
		Location:    strings.TrimSpace(raw.Location),
		URL:         jobURL,
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

	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	host := u.Hostname()
	host = strings.ToLower(host)
	host = strings.TrimPrefix(host, "www.")
	return host
}

func normalizeCompanyName(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}

	raw = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsLetter(r), unicode.IsNumber(r), unicode.IsSpace(r):
			return r
		default:
			return ' '
		}
	}, raw)

	tokens := strings.Fields(raw)
	if len(tokens) == 0 {
		return ""
	}

	suffixes := map[string]struct{}{
		"co":           {},
		"company":      {},
		"corp":         {},
		"corporation":  {},
		"inc":          {},
		"incorporated": {},
		"limited":      {},
		"llc":          {},
		"ltd":          {},
	}

	for len(tokens) > 0 {
		if _, ok := suffixes[tokens[len(tokens)-1]]; !ok {
			break
		}
		tokens = tokens[:len(tokens)-1]
	}

	return strings.Join(tokens, " ")
}
