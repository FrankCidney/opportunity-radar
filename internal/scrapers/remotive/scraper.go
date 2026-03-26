package remotive

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"opportunity-radar/internal/ingest"
	"time"
)

type Scraper struct {
	client *http.Client
	logger *slog.Logger
}

func NewScraper(logger *slog.Logger) *Scraper {
	return &Scraper{
		client: &http.Client{Timeout: 15 * time.Second},
		logger: logger,
	}
}

func (s *Scraper) Source() string { return "remotive"}

func (s *Scraper) Scrape(ctx context.Context) ([]ingest.RawJob, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://example.com", // TODO: Change this to a real URL
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching remotive: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Jobs []struct {
			Title string `json:"title"`
			CompanyName string `jsong:"company_name"`
			URL string `json:"url"`
			Description string `json:"description"`
			PublishedAt string `json:"publication_date"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding remotive response: %w", err)
	}

	rawJobs := make([]ingest.RawJob, 0, len(result.Jobs))
	for _, j := range result.Jobs {
		rawJobs = append(rawJobs, ingest.RawJob{
            Source:      s.Source(),
            Title:       j.Title,
            Company:     j.CompanyName,
            URL:         j.URL,
            Description: j.Description,
            PostedAt:    j.PublishedAt,
		})
	}
	return rawJobs, nil
}