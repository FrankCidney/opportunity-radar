package ingest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/ingest/normalize"
	"opportunity-radar/internal/jobs"
)

func TestPipelineNormalizesScoresAndSavesJob(t *testing.T) {
	t.Parallel()

	jobService := &stubJobService{}
	companyService := &stubCompanyService{
		company: &companies.Company{ID: 42},
	}
	scorer := &stubScorer{score: 73}
	pipeline := NewPipeline(scorer, jobService, companyService, ingestTestLogger())
	scraper := &stubScraper{
		source: "example",
		rawJobs: []normalize.RawJob{{
			Source:      "example",
			Title:       "  Backend Engineer  ",
			Company:     "Acme Ltd.",
			CompanyURL:  "https://www.acme.example/about",
			Location:    "  Nairobi  ",
			Description: "  Build APIs  ",
			URL:         " https://jobs.example/42 ",
			PostedAt:    "2026-04-01",
		}},
	}

	if err := pipeline.Run(context.Background(), scraper); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if companyService.input == nil {
		t.Fatal("company service was not called")
	}
	if companyService.input.Name != "acme" {
		t.Fatalf("normalized company name = %q, want %q", companyService.input.Name, "acme")
	}
	if len(jobService.saved) != 1 {
		t.Fatalf("saved jobs = %d, want 1", len(jobService.saved))
	}

	saved := jobService.saved[0]
	if saved.CompanyID != 42 {
		t.Fatalf("company ID = %d, want 42", saved.CompanyID)
	}
	if saved.Title != "Backend Engineer" {
		t.Fatalf("title = %q, want normalized title", saved.Title)
	}
	if saved.Location != "Nairobi" {
		t.Fatalf("location = %q, want normalized location", saved.Location)
	}
	if saved.Score != 73 {
		t.Fatalf("score = %v, want 73", saved.Score)
	}
	if scorer.job != saved {
		t.Fatal("scorer and job service did not receive the same job")
	}
}

func TestPipelineReturnsScraperFailureWithoutSaving(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("source unavailable")
	jobService := &stubJobService{}
	pipeline := NewPipeline(
		&stubScorer{},
		jobService,
		&stubCompanyService{},
		ingestTestLogger(),
	)

	err := pipeline.Run(context.Background(), &stubScraper{
		source: "broken",
		err:    wantErr,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, wantErr)
	}
	if len(jobService.saved) != 0 {
		t.Fatalf("saved jobs = %d, want 0", len(jobService.saved))
	}
}

func TestPipelineSkipsDuplicateAndContinuesWithRemainingJobs(t *testing.T) {
	t.Parallel()

	jobService := &stubJobService{
		saveErrors: []error{jobs.ErrJobAlreadyExists, nil},
	}
	pipeline := NewPipeline(
		&stubScorer{score: 10},
		jobService,
		&stubCompanyService{company: &companies.Company{ID: 1}},
		ingestTestLogger(),
	)
	scraper := &stubScraper{
		source: "example",
		rawJobs: []normalize.RawJob{
			validRawJob("https://jobs.example/duplicate"),
			validRawJob("https://jobs.example/new"),
		},
	}

	if err := pipeline.Run(context.Background(), scraper); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if jobService.saveCalls != 2 {
		t.Fatalf("Save() calls = %d, want 2", jobService.saveCalls)
	}
	if len(jobService.saved) != 1 {
		t.Fatalf("successfully saved jobs = %d, want 1", len(jobService.saved))
	}
	if jobService.saved[0].URL != "https://jobs.example/new" {
		t.Fatalf("saved URL = %q, want second job", jobService.saved[0].URL)
	}
}

func TestServiceContinuesAfterOneScraperFails(t *testing.T) {
	t.Parallel()

	jobService := &stubJobService{}
	pipeline := NewPipeline(
		&stubScorer{},
		jobService,
		&stubCompanyService{company: &companies.Company{ID: 1}},
		ingestTestLogger(),
	)
	firstErr := errors.New("first source failed")
	service := NewService(pipeline, []Scraper{
		&stubScraper{source: "first", err: firstErr},
		&stubScraper{source: "second", rawJobs: []normalize.RawJob{
			validRawJob("https://jobs.example/success"),
		}},
	}, ingestTestLogger())

	if err := service.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll() error = %v, want nil because source failures are isolated", err)
	}
	if len(jobService.saved) != 1 {
		t.Fatalf("saved jobs = %d, want successful source job", len(jobService.saved))
	}
}

func validRawJob(url string) normalize.RawJob {
	return normalize.RawJob{
		Source:   "example",
		Title:    "Backend Engineer",
		Company:  "Acme",
		URL:      url,
		PostedAt: "2026-04-01",
	}
}

func ingestTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubScraper struct {
	source  string
	rawJobs []normalize.RawJob
	err     error
}

func (s *stubScraper) Source() string {
	return s.source
}

func (s *stubScraper) Scrape(context.Context) ([]normalize.RawJob, error) {
	return s.rawJobs, s.err
}

type stubScorer struct {
	score float64
	job   *jobs.Job
}

func (s *stubScorer) Score(job *jobs.Job) float64 {
	s.job = job
	return s.score
}

type stubJobService struct {
	saveErrors []error
	saveCalls  int
	saved      []*jobs.Job
}

func (s *stubJobService) Save(_ context.Context, job *jobs.Job) error {
	call := s.saveCalls
	s.saveCalls++
	if call < len(s.saveErrors) && s.saveErrors[call] != nil {
		return s.saveErrors[call]
	}
	s.saved = append(s.saved, job)
	return nil
}

type stubCompanyService struct {
	company *companies.Company
	err     error
	input   *companies.Company
}

func (s *stubCompanyService) FindOrCreate(_ context.Context, company *companies.Company) (*companies.Company, error) {
	s.input = company
	if s.err != nil {
		return nil, s.err
	}
	return s.company, nil
}
