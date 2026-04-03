package remotive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"log/slog"

	"opportunity-radar/internal/ingest/normalize"
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

func (s *Scraper) Source() string {
	return "remotive"
}

func (s *Scraper) Scrape(ctx context.Context) ([]normalize.RawJob, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://remotive.com/api/remote-jobs?category=software-dev", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()

	// Always check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var result struct {
		Jobs []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			CompanyName string `json:"company_name"`
			URL         string `json:"url"`
			Description string `json:"description"`
			PublishedAt string `json:"publication_date"`
			Location    string `json:"candidate_required_location"`
		} `json:"jobs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	raws := make([]normalize.RawJob, 0, len(result.Jobs))

	for _, j := range result.Jobs {
		raws = append(raws, normalize.RawJob{
			ExternalID:  strconv.Itoa(j.ID),
			Source:      s.Source(),
			Title:       j.Title,
			Company:     j.CompanyName,
			URL:         j.URL,
			Description: j.Description,
			PostedAt:    j.PublishedAt,
			Location:    j.Location,
		})
	}

	s.logger.Info("remotive scrape complete", "count", len(raws))

	return raws, nil
}
