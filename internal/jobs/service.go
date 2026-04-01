package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

type Service struct {
	repo Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo: repo,
		logger: logger,
	}
}

// Save is the ingest path. Creates a new job if one with the same source + url doesn't already exist
func (s *Service) Save(ctx context.Context, input *Job) error {
	job := &Job{
		CompanyID:           input.CompanyID,
        Title:               input.Title,
        Description:         input.Description,
        Location:            input.Location,
        URL:                 input.URL,
        Source:              input.Source,
        PostedAt:            input.PostedAt,
        ApplicationDeadline: input.ApplicationDeadline,
        Score:               input.Score,
        Status:              StatusActive,
	}

	if err := s.repo.Create(ctx, job); err != nil {
		switch {
		case errors.Is(err, ErrConflict):
			return ErrJobAlreadyExists
		case errors.Is(err, ErrReferenceNotFound):
			return fmt.Errorf("%w: company does not exist", ErrJobNotFound)
		case errors.Is(err, ErrTimeout):
			return fmt.Errorf("%w: timed out saving job", ErrServiceInternal)
		default:
			s.logger.Error("failed to create job",
				"source", input.Source,
				"url", input.URL,
				"error", err,
			)
			return ErrServiceInternal
		}
	}

	return nil
} 

// GetByID is the API read path.
func (s *Service) GetByID(ctx context.Context, id int64) (*Job, error) {
    job, err := s.repo.GetByID(ctx, id)
    if err != nil {
        switch {
        case errors.Is(err, ErrNotFound):
            return nil, ErrJobNotFound
        case errors.Is(err, ErrTimeout):
            return nil, fmt.Errorf("%w: timed out fetching job", ErrServiceInternal)
        default:
            s.logger.Error("failed to get job", "job_id", id, "error", err)
            return nil, ErrServiceInternal
        }
    }
    return job, nil
}


// List is the API list/filter path.
func (s *Service) List(ctx context.Context, filter JobListFilter) ([]Job, error) {
    if filter.Limit <= 0 || filter.Limit > 100 {
        filter.Limit = 50
    }

    jobs, err := s.repo.List(ctx, filter)
    if err != nil {
        switch {
        case errors.Is(err, ErrTimeout):
            return nil, fmt.Errorf("%w: timed out listing jobs", ErrServiceInternal)
        default:
            s.logger.Error("failed to list jobs", "filter", filter, "error", err)
            return nil, ErrServiceInternal
        }
    }

    return jobs, nil
}

// Archive transitions a job to StatusArchived.
func (s *Service) Archive(ctx context.Context, id int64) error {
    job, err := s.GetByID(ctx, id)
    if err != nil {
        return err // already translated
    }

    if job.Status == StatusArchived {
        return ErrInvalidTransition
    }

    job.Status = StatusArchived

    if err := s.repo.Update(ctx, job); err != nil {
        switch {
        case errors.Is(err, ErrTimeout):
            return fmt.Errorf("%w: timed out archiving job", ErrServiceInternal)
        default:
            s.logger.Error("failed to archive job", "job_id", id, "error", err)
            return ErrServiceInternal
        }
    }

    return nil
}

// UpdateScore re-scores an existing job. Used by a future re-scoring pass.
func (s *Service) UpdateScore(ctx context.Context, id int64, score float64) error {
    job, err := s.GetByID(ctx, id)
    if err != nil {
        return err
    }

    job.Score = score

    if err := s.repo.Update(ctx, job); err != nil {
        switch {
        case errors.Is(err, ErrTimeout):
            return fmt.Errorf("%w: timed out updating score", ErrServiceInternal)
        default:
            s.logger.Error("failed to update job score", "job_id", id, "error", err)
            return ErrServiceInternal
        }
    }

    return nil
}
