package fuzu

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"
)

func TestExtractJSONLDItemListExtractsJobs(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"ItemList","itemListElement":[{"@type":"ListItem","position":1,"name":"Backend Developer","url":"https://www.fuzu.com/kenya/jobs/backend-developer-abc123"},{"@type":"ListItem","position":2,"name":"Data Engineer","url":"https://www.fuzu.com/kenya/jobs/data-engineer-def456"}]}</script></head><body></body></html>`)

	items := extractJSONLDItemList(doc)

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].Name != "Backend Developer" {
		t.Fatalf("unexpected first item name: %q", items[0].Name)
	}
	if items[0].URL != "https://www.fuzu.com/kenya/jobs/backend-developer-abc123" {
		t.Fatalf("unexpected first item URL: %q", items[0].URL)
	}
	if items[1].Name != "Data Engineer" {
		t.Fatalf("unexpected second item name: %q", items[1].Name)
	}
}

func TestExtractJSONLDItemListFallsBackToHTML(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `<html><head></head><body><article><a href="/jobs/backend-developer-abc123">Backend Developer</a></article></body></html>`)

	base := mustParseURL(t, "https://www.fuzu.com")
	candidates, _ := parseListingPage(doc, base, "https://www.fuzu.com/kenya/job/computers-software-development")

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate from HTML fallback, got %d", len(candidates))
	}

	if candidates[0].Title != "Backend Developer" {
		t.Fatalf("unexpected title: %q", candidates[0].Title)
	}
	if candidates[0].URL != "https://www.fuzu.com/jobs/backend-developer-abc123" {
		t.Fatalf("unexpected URL: %q", candidates[0].URL)
	}
}

func TestParseJobPostingExtractsStructuredFields(t *testing.T) {
	t.Parallel()

	jsonLD := `{"@context":"http://schema.org","@type":"JobPosting","identifier":{"@type":"PropertyValue","name":"Fuzu","value":"902778"},"datePosted":"2026-04-01","description":"<p>We are hiring a developer.</p><p>RESPONSIBILITIES</p><p>Build software.</p>","employmentType":"FULL_TIME","industry":"Technology","skills":"Go,PostgreSQL","hiringOrganization":{"@type":"Organization","name":"Acme Labs"},"jobLocation":{"@type":"Place","address":{"@type":"PostalAddress","addressLocality":"Nairobi"}},"title":"Backend Engineer","url":"https://www.fuzu.com/kenya/jobs/backend-engineer-acme","validThrough":"2026-06-13T00:00:00+00:00"}`

	job, err := parseJobPosting(jsonLD)
	if err != nil {
		t.Fatalf("expected parse to succeed: %v", err)
	}

	if job.ExternalID != "902778" {
		t.Fatalf("unexpected external ID: %q", job.ExternalID)
	}
	if job.DatePosted != "2026-04-01" {
		t.Fatalf("unexpected date posted: %q", job.DatePosted)
	}
	if job.Title != "Backend Engineer" {
		t.Fatalf("unexpected title: %q", job.Title)
	}
	if job.Company != "Acme Labs" {
		t.Fatalf("unexpected company: %q", job.Company)
	}
	if job.Location != "Nairobi" {
		t.Fatalf("unexpected location: %q", job.Location)
	}
	if job.EmploymentType != "FULL_TIME" {
		t.Fatalf("unexpected employment type: %q", job.EmploymentType)
	}
	if job.Industry != "Technology" {
		t.Fatalf("unexpected industry: %q", job.Industry)
	}
	if job.Skills != "Go,PostgreSQL" {
		t.Fatalf("unexpected skills: %q", job.Skills)
	}
	if job.ValidThrough != "2026-06-13T00:00:00+00:00" {
		t.Fatalf("unexpected valid through: %q", job.ValidThrough)
	}
	if !strings.Contains(job.Description, "We are hiring a developer.") {
		t.Fatalf("unexpected description: %q", job.Description)
	}
}

func TestStripHTMLEntities(t *testing.T) {
	t.Parallel()

	input := `<p>Hello <b>world</b>!</p><p>Line 2</p>`
	result := stripHTML(input)

	if !strings.Contains(result, "Hello") {
		t.Fatalf("expected 'Hello' in result, got %q", result)
	}
	if !strings.Contains(result, "world") {
		t.Fatalf("expected 'world' in result, got %q", result)
	}
	if !strings.Contains(result, "Line 2") {
		t.Fatalf("expected 'Line 2' in result, got %q", result)
	}
}

func TestCleanDescriptionSections(t *testing.T) {
	t.Parallel()

	input := "We are hiring a developer.\nRESPONSIBILITIES\nBuild software.\n\nAbout the company\nWe are great.\nrequirements\nMust know Go."
	result := cleanDescriptionSections(input)

	if !strings.Contains(result, "We are hiring a developer.") {
		t.Fatalf("expected first paragraph in result, got %q", result)
	}
	if !strings.Contains(result, "Build software.") {
		t.Fatalf("expected responsibilities content in result, got %q", result)
	}
	if !strings.Contains(result, "We are great.") {
		t.Fatalf("expected about the company in result, got %q", result)
	}
	if !strings.Contains(result, "Must know Go.") {
		t.Fatalf("expected content after requirements marker in result, got %q", result)
	}
}

func TestScrapeFollowsPaginationAndFetchesDetailPages(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/kenya/job/computers-software-development", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "":
			_, _ = io.WriteString(w, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"ItemList","itemListElement":[{"@type":"ListItem","name":"Backend Developer","url":"/jobs/backend-developer-abc123"}]}</script></head><body><nav><a href="?page=2" rel="next">Next</a></nav></body></html>`)
		case "2":
			_, _ = io.WriteString(w, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"ItemList","itemListElement":[{"@type":"ListItem","name":"Data Engineer","url":"/jobs/data-engineer-def456"}]}</script></head><body></body></html>`)
		default:
			_, _ = io.WriteString(w, `<html><body></body></html>`)
		}
	})
	mux.HandleFunc("/jobs/backend-developer-abc123", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"JobPosting","identifier":{"value":"12345"},"datePosted":"2026-04-01","title":"Backend Developer","hiringOrganization":{"name":"Acme Labs"},"jobLocation":{"address":{"addressLocality":"Remote"}},"description":"<p>Build backend services.</p>"}</script></head><body><h1>Backend Developer</h1></body></html>`)
	})
	mux.HandleFunc("/jobs/data-engineer-def456", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"JobPosting","identifier":{"value":"12346"},"datePosted":"2026-04-02","title":"Data Engineer","hiringOrganization":{"name":"Data Corp"},"jobLocation":{"address":{"addressLocality":"Nairobi"}},"description":"<p>Build data pipelines.</p>"}</script></head><body><h1>Data Engineer</h1></body></html>`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	scraper := newScraper(Config{
		BaseURL:         server.URL,
		ListingPaths:    []string{"/kenya/job/computers-software-development"},
		MaxPagesPerPath: 2,
		RequestDelay:    0,
	}, server.Client(), testLogger())

	raws, err := scraper.Scrape(context.Background())
	if err != nil {
		t.Fatalf("expected scrape to succeed: %v", err)
	}

	if len(raws) != 2 {
		t.Fatalf("expected 2 raw jobs, got %d", len(raws))
	}

	if raws[0].Source != "fuzu" {
		t.Fatalf("unexpected source: %q", raws[0].Source)
	}
	if raws[0].ExternalID == "" {
		t.Fatalf("expected external ID, got empty")
	}
	if raws[0].Title != "Backend Developer" {
		t.Fatalf("unexpected title: %q", raws[0].Title)
	}
}

func TestScrapeReturnsErrorOnBadStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	scraper := newScraper(Config{
		BaseURL: server.URL,
		RequestDelay: 0,
	}, server.Client(), testLogger())

	_, err := scraper.Scrape(context.Background())
	if err == nil {
		t.Fatal("expected scrape to fail on bad status")
	}
}

func TestNewScraperDefaults(t *testing.T) {
	t.Parallel()

	scraper := newScraper(Config{}, nil, testLogger())

	if scraper.maxPages != defaultMaxPages {
		t.Fatalf("unexpected max pages: got %d want %d", scraper.maxPages, defaultMaxPages)
	}
	if len(scraper.listingPaths) != 1 || scraper.listingPaths[0] != defaultListingPath {
		t.Fatalf("unexpected listing paths: %v", scraper.listingPaths)
	}
	if scraper.client == nil {
		t.Fatal("expected default client")
	}
}

func TestExternalIDFromURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		input string
		want  string
	}{
		{input: "https://www.fuzu.com/kenya/jobs/backend-developer-acme", want: "backend-developer-acme"},
		{input: "https://www.fuzu.com/kenya/jobs/data-engineer-123", want: "data-engineer-123"},
		{input: "https://www.fuzu.com/jobs/test", want: "test"},
		{input: "https://www.fuzu.com/", want: ""},
		{input: "https://www.fuzu.com", want: ""},
	}

	for _, tc := range cases {
		got := externalIDFromURL(tc.input)
		if got != tc.want {
			t.Errorf("externalIDFromURL(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestParseRelativePostedAt(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		input string
		want  time.Time
	}{
		{input: "New", want: now},
		{input: "1 week ago", want: now.AddDate(0, 0, -7)},
		{input: "2 weeks ago", want: now.AddDate(0, 0, -14)},
		{input: "1 month ago", want: now.AddDate(0, -1, 0)},
	}

	for _, tc := range cases {
		got, ok := parseRelativePostedAt(tc.input, now)
		if !ok {
			t.Fatalf("expected %q to parse", tc.input)
		}
		if !got.Equal(tc.want) {
			t.Fatalf("unexpected parsed time for %q: got %s want %s", tc.input, got, tc.want)
		}
	}
}

func TestParseDetailPageFallsBackToHTML(t *testing.T) {
	t.Parallel()

	doc := mustParseHTML(t, `<html><head><script type="application/ld+json">{"@context":"http://schema.org","@type":"WrongType"}</script></head><body><h1>Backend Developer</h1><h2>Acme Labs</h2><main><p>We are hiring.</p><p>Work remotely.</p></main></body></html>`)

	base := mustParseURL(t, "https://www.fuzu.com")
	detail, err := parseDetailPage(doc, base, "https://www.fuzu.com/kenya/jobs/backend-developer-acme")
	if err != nil {
		t.Fatalf("expected parse to succeed: %v", err)
	}

	if detail.Title != "Backend Developer" {
		t.Fatalf("unexpected title: %q", detail.Title)
	}
	if detail.Company != "Acme Labs" {
		t.Fatalf("unexpected company: %q", detail.Company)
	}
	if !strings.Contains(detail.Description, "We are hiring.") {
		t.Fatalf("unexpected description: %q", detail.Description)
	}
}

func mustParseHTML(t *testing.T, raw string) *html.Node {
	t.Helper()

	doc, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("failed to parse HTML: %v", err)
	}
	return doc
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}
	return parsed
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}