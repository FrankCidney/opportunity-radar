package remotive

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestScrapeUsesBroadFeedWithoutCategoryFilter(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/remote-jobs" {
			t.Fatalf("unexpected path: got %q want %q", r.URL.Path, "/api/remote-jobs")
		}

		if got := r.URL.Query().Get("category"); got != "" {
			t.Fatalf("expected no category query, got %q", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":[]}`))
	}))
	defer server.Close()

	scraper := newScraper(server.URL+"/api/remote-jobs", server.Client(), testLogger())

	raws, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("expected scrape to succeed: %v", err)
	}

	if len(raws) != 0 {
		t.Fatalf("expected no jobs, got %d", len(raws))
	}
}

func TestScrapeMapsResponseToRawJobs(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"jobs": [
				{
					"id": 123,
					"title": "Backend Engineer",
					"company_name": "Acme",
					"company_logo": "https://example.com/logo.png",
					"url": "https://example.com/jobs/123",
					"job_type": "full_time",
					"salary": "$50k-$70k",
					"description": "<p>Hello</p>",
					"publication_date": "2026-04-01T12:00:00",
					"candidate_required_location": "Kenya"
				}
			]
		}`))
	}))
	defer server.Close()

	scraper := newScraper(server.URL, server.Client(), testLogger())

	raws, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("expected scrape to succeed: %v", err)
	}

	if len(raws) != 1 {
		t.Fatalf("expected 1 job, got %d", len(raws))
	}

	got := raws[0]
	if got.ExternalID != "123" {
		t.Fatalf("unexpected external id: got %q want %q", got.ExternalID, "123")
	}
	if got.Source != "remotive" {
		t.Fatalf("unexpected source: got %q want %q", got.Source, "remotive")
	}
	if got.Title != "Backend Engineer" {
		t.Fatalf("unexpected title: got %q want %q", got.Title, "Backend Engineer")
	}
	if got.Company != "Acme" {
		t.Fatalf("unexpected company: got %q want %q", got.Company, "Acme")
	}
	if got.CompanyLogo != "https://example.com/logo.png" {
		t.Fatalf("unexpected company logo: got %q want %q", got.CompanyLogo, "https://example.com/logo.png")
	}
	if got.URL != "https://example.com/jobs/123" {
		t.Fatalf("unexpected url: got %q want %q", got.URL, "https://example.com/jobs/123")
	}
	if got.JobType != "full_time" {
		t.Fatalf("unexpected job type: got %q want %q", got.JobType, "full_time")
	}
	if got.Salary != "$50k-$70k" {
		t.Fatalf("unexpected salary: got %q want %q", got.Salary, "$50k-$70k")
	}
	if got.Description != "<p>Hello</p>" {
		t.Fatalf("unexpected description: got %q want %q", got.Description, "<p>Hello</p>")
	}
	if got.PostedAt != "2026-04-01T12:00:00" {
		t.Fatalf("unexpected posted_at: got %q want %q", got.PostedAt, "2026-04-01T12:00:00")
	}
	if got.Location != "Kenya" {
		t.Fatalf("unexpected location: got %q want %q", got.Location, "Kenya")
	}
}

func TestScrapeReturnsErrorOnBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer server.Close()

	scraper := newScraper(server.URL, server.Client(), testLogger())

	_, err := scraper.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected scrape to fail on bad status")
	}
}

func TestScrapeReturnsErrorOnInvalidJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jobs":`))
	}))
	defer server.Close()

	scraper := newScraper(server.URL, server.Client(), testLogger())

	_, err := scraper.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected scrape to fail on invalid json")
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestNewScraperDefaultsClientAndEndpoint(t *testing.T) {
	t.Parallel()

	scraper := newScraper("", nil, testLogger())

	if scraper.endpoint != jobsEndpoint {
		t.Fatalf("unexpected endpoint: got %q want %q", scraper.endpoint, jobsEndpoint)
	}
	if scraper.client == nil {
		t.Fatal("expected default client to be set")
	}
	if scraper.client.Timeout != 15*time.Second {
		t.Fatalf("unexpected client timeout: got %s want %s", scraper.client.Timeout, 15*time.Second)
	}
}
