package digest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/jobs"
)

func TestSendDailySendsTopJobsAndRecordsDelivery(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{getErr: ErrNotFound}
	jobLister := &stubJobLister{
		jobs: []jobs.Job{
			{ID: 1, CompanyID: 42, Title: "Backend Engineer", URL: "https://example.com/1", Source: "remotive", Score: 40, CreatedAt: time.Now()},
		},
	}
	companies := &stubCompanyGetter{
		companies: map[int64]*companies.Company{
			42: {ID: 42, Name: "acme"},
		},
	}
	sender := &stubSender{}
	service := NewService(repo, jobLister, companies, sender, Config{
		Enabled:   true,
		Recipient: "me@example.com",
		TopN:      10,
		Lookback:  24 * time.Hour,
	}, testLogger())

	now := time.Date(2026, 4, 3, 10, 0, 0, 0, time.UTC)
	if err := service.SendDaily(context.Background(), now); err != nil {
		t.Fatalf("SendDaily() error = %v", err)
	}

	if !sender.sent {
		t.Fatal("expected sender to be called")
	}
	if repo.created == nil {
		t.Fatal("expected delivery to be recorded")
	}
	if repo.created.JobCount != 1 {
		t.Fatalf("expected job count 1, got %d", repo.created.JobCount)
	}
	if sender.message.To != "me@example.com" {
		t.Fatalf("expected recipient to be preserved, got %q", sender.message.To)
	}
	if jobLister.lastFilter.SortBy != jobs.JobSortScoreDesc {
		t.Fatalf("expected score-desc sort, got %q", jobLister.lastFilter.SortBy)
	}
	if jobLister.lastFilter.CreatedAfter == nil {
		t.Fatal("expected CreatedAfter to be set")
	}
}

func TestSendDailySkipsWhenAlreadySent(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{
		existing: &Delivery{ID: 1},
	}
	sender := &stubSender{}
	service := NewService(repo, &stubJobLister{}, nil, sender, Config{
		Enabled:   true,
		Recipient: "me@example.com",
	}, testLogger())

	if err := service.SendDaily(context.Background(), time.Now()); err != nil {
		t.Fatalf("SendDaily() error = %v", err)
	}
	if sender.sent {
		t.Fatal("did not expect sender to be called")
	}
	if repo.created != nil {
		t.Fatal("did not expect delivery to be created")
	}
}

func TestSendDailySendsStatusUpdateWhenNoJobsFound(t *testing.T) {
	t.Parallel()

	repo := &stubRepository{getErr: ErrNotFound}
	sender := &stubSender{}
	service := NewService(repo, &stubJobLister{}, nil, sender, Config{
		Enabled:   true,
		Recipient: "me@example.com",
	}, testLogger())

	if err := service.SendDaily(context.Background(), time.Now()); err != nil {
		t.Fatalf("SendDaily() error = %v", err)
	}
	if !sender.sent {
		t.Fatal("expected sender to be called")
	}
	if repo.created == nil {
		t.Fatal("expected delivery to be recorded")
	}
	if repo.created.JobCount != 0 {
		t.Fatalf("expected job count 0, got %d", repo.created.JobCount)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubRepository struct {
	existing *Delivery
	getErr   error
	created  *Delivery
}

func (r *stubRepository) Create(_ context.Context, delivery *Delivery) error {
	r.created = delivery
	return nil
}

func (r *stubRepository) GetByRecipientAndDate(_ context.Context, _ string, _ string) (*Delivery, error) {
	if r.getErr != nil {
		return nil, r.getErr
	}
	if r.existing == nil {
		return nil, ErrNotFound
	}
	return r.existing, nil
}

type stubJobLister struct {
	jobs       []jobs.Job
	lastFilter jobs.JobListFilter
}

func (l *stubJobLister) List(_ context.Context, filter jobs.JobListFilter) ([]jobs.Job, error) {
	l.lastFilter = filter
	return l.jobs, nil
}

type stubCompanyGetter struct {
	companies map[int64]*companies.Company
}

func (g *stubCompanyGetter) GetByID(_ context.Context, id int64) (*companies.Company, error) {
	company, ok := g.companies[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return company, nil
}

type stubSender struct {
	sent    bool
	message Message
}

func (s *stubSender) Send(_ context.Context, message Message) error {
	s.sent = true
	s.message = message
	return nil
}
