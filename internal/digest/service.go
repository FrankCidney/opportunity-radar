package digest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/jobs"
)

type JobLister interface {
	List(ctx context.Context, filter jobs.JobListFilter) ([]jobs.Job, error)
}

type CompanyGetter interface {
	GetByID(ctx context.Context, id int64) (*companies.Company, error)
}

type Config struct {
	Enabled   bool
	Recipient string
	TopN      int
	Lookback  time.Duration
}

type Service struct {
	repo      Repository
	jobLister JobLister
	companies CompanyGetter
	sender    Sender
	logger    *slog.Logger
	configMu  sync.RWMutex
	config    Config
}

func NewService(
	repo Repository,
	jobLister JobLister,
	companies CompanyGetter,
	sender Sender,
	config Config,
	logger *slog.Logger,
) *Service {
	return &Service{
		repo:      repo,
		jobLister: jobLister,
		companies: companies,
		sender:    sender,
		logger:    logger,
		config:    config,
	}
}

func (s *Service) SendDaily(ctx context.Context, now time.Time) error {
	config := s.CurrentConfig()

	if !config.Enabled {
		s.logger.Info("daily digest disabled; skipping")
		return nil
	}
	if s.repo == nil || s.jobLister == nil || s.sender == nil {
		return fmt.Errorf("%w: digest dependencies are not configured", ErrServiceInternal)
	}
	if strings.TrimSpace(config.Recipient) == "" {
		s.logger.Warn("daily digest enabled but recipient is empty; skipping")
		return nil
	}

	digestDate := startOfUTCDay(now)
	digestDateKey := digestDate.Format(time.DateOnly)

	if _, err := s.repo.GetByRecipientAndDate(ctx, config.Recipient, digestDateKey); err == nil {
		s.logger.Info("daily digest already sent; skipping",
			"recipient", config.Recipient,
			"digest_date", digestDateKey,
		)
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return s.translateRepositoryError("checking existing digest", err)
	}

	active := jobs.StatusActive
	lookbackStart := now.UTC().Add(-s.effectiveLookback())
	topN := s.effectiveTopN()

	jobResults, err := s.jobLister.List(ctx, jobs.JobListFilter{
		Status:       &active,
		CreatedAfter: &lookbackStart,
		Limit:        topN,
		SortBy:       jobs.JobSortScoreDesc,
	})
	if err != nil {
		return fmt.Errorf("%w: listing digest jobs", ErrServiceInternal)
	}

	if len(jobResults) == 0 {
		s.logger.Info("daily digest skipped because no recent jobs were found",
			"recipient", config.Recipient,
			"since", lookbackStart,
		)
		return nil
	}

	items := s.buildDigestItems(ctx, jobResults)
	message := RenderMessage(config.Recipient, items)

	if err := s.sender.Send(ctx, message); err != nil {
		s.logger.Error("failed to send daily digest",
			"recipient", config.Recipient,
			"error", err,
		)
		return fmt.Errorf("%w: sending digest", ErrServiceInternal)
	}

	delivery := &Delivery{
		Recipient:  config.Recipient,
		DigestDate: digestDate,
		JobCount:   len(items),
		Subject:    message.Subject,
	}

	if err := s.repo.Create(ctx, delivery); err != nil {
		if errors.Is(err, ErrConflict) {
			s.logger.Info("daily digest was recorded by another path; treating as already sent",
				"recipient", config.Recipient,
				"digest_date", digestDateKey,
			)
			return nil
		}
		return s.translateRepositoryError("recording sent digest", err)
	}

	s.logger.Info("daily digest complete",
		"recipient", config.Recipient,
		"digest_date", digestDateKey,
		"job_count", len(items),
	)

	return nil
}

func (s *Service) buildDigestItems(ctx context.Context, jobResults []jobs.Job) []JobDigestItem {
	items := make([]JobDigestItem, 0, len(jobResults))

	for _, job := range jobResults {
		companyName := "Unknown company"
		if s.companies != nil && job.CompanyID > 0 {
			company, err := s.companies.GetByID(ctx, job.CompanyID)
			if err != nil {
				s.logger.Warn("failed to resolve company for digest job",
					"job_id", job.ID,
					"company_id", job.CompanyID,
					"error", err,
				)
			} else if company != nil && company.Name != "" {
				companyName = company.Name
			}
		}

		items = append(items, JobDigestItem{
			Title:       job.Title,
			CompanyName: companyName,
			Location:    job.Location,
			URL:         job.URL,
			Source:      job.Source,
			Score:       job.Score,
			PostedAt:    job.PostedAt,
		})
	}

	return items
}

func (s *Service) effectiveTopN() int {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	if s.config.TopN <= 0 {
		return 10
	}
	if s.config.TopN > 100 {
		return 100
	}
	return s.config.TopN
}

func (s *Service) effectiveLookback() time.Duration {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	if s.config.Lookback <= 0 {
		return 24 * time.Hour
	}
	return s.config.Lookback
}

func (s *Service) UpdateConfig(config Config) {
	s.configMu.Lock()
	defer s.configMu.Unlock()

	s.config = config
}

func (s *Service) CurrentConfig() Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()

	return s.config
}

func (s *Service) translateRepositoryError(action string, err error) error {
	switch {
	case errors.Is(err, ErrTimeout):
		return fmt.Errorf("%w: timed out %s", ErrServiceInternal, action)
	default:
		s.logger.Error("digest repository operation failed",
			"action", action,
			"error", err,
		)
		return fmt.Errorf("%w: %s", ErrServiceInternal, action)
	}
}

func startOfUTCDay(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
