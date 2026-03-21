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