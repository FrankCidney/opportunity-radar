package companies

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// Save is the create path. It creates a new company if one with the same unique
// identity doesn't already exist.
func (s *Service) Save(ctx context.Context, input *Company) error {
	company := &Company{
		Name:       input.Name,
		LogoURL:    input.LogoURL,
		Source:     input.Source,
		ExternalID: input.ExternalID,
		Domain:     input.Domain,
	}

	if err := s.repo.Create(ctx, company); err != nil {
		switch {
		case errors.Is(err, ErrConflict):
			return ErrCompanyAlreadyExists
		case errors.Is(err, ErrTimeout):
			return fmt.Errorf("%w: timed out saving company", ErrServiceInternal)
		default:
			s.logger.Error("failed to create company",
				"name", input.Name,
				"source", input.Source,
				"external_id", input.ExternalID,
				"domain", input.Domain,
				"error", err,
			)
			return ErrServiceInternal
		}
	}

	return nil
}

// GetByID is the API read path.
func (s *Service) GetByID(ctx context.Context, id int64) (*Company, error) {
	company, err := s.repo.GetByID(ctx, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, ErrCompanyNotFound
		case errors.Is(err, ErrTimeout):
			return nil, fmt.Errorf("%w: timed out fetching company", ErrServiceInternal)
		default:
			s.logger.Error("failed to get company", "company_id", id, "error", err)
			return nil, ErrServiceInternal
		}
	}

	return company, nil
}

// List is the API list/filter path.
func (s *Service) List(ctx context.Context, filter CompanyListFilter) ([]Company, error) {
	if filter.Limit <= 0 || filter.Limit > 100 {
		filter.Limit = 50
	}

	companies, err := s.repo.List(ctx, filter)
	if err != nil {
		switch {
		case errors.Is(err, ErrTimeout):
			return nil, fmt.Errorf("%w: timed out listing companies", ErrServiceInternal)
		default:
			s.logger.Error("failed to list companies", "filter", filter, "error", err)
			return nil, ErrServiceInternal
		}
	}

	return companies, nil
}

// Delete removes a company record.
func (s *Service) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return ErrCompanyNotFound
		case errors.Is(err, ErrTimeout):
			return fmt.Errorf("%w: timed out deleting company", ErrServiceInternal)
		default:
			s.logger.Error("failed to delete company", "company_id", id, "error", err)
			return ErrServiceInternal
		}
	}

	return nil
}

// FindOrCreate resolves a company by the strongest available identity and
// creates it when no existing record matches.
func (s *Service) FindOrCreate(ctx context.Context, input *Company) (*Company, error) {
	filter := buildFindOrCreateFilter(input)

	if filter != nil {
		companies, err := s.List(ctx, *filter)
		if err != nil {
			return nil, err
		}
		if len(companies) > 0 {
			return &companies[0], nil
		}
	}

	company := &Company{
		Name:       input.Name,
		LogoURL:    input.LogoURL,
		Source:     input.Source,
		ExternalID: input.ExternalID,
		Domain:     input.Domain,
	}

	if err := s.repo.Create(ctx, company); err != nil {
		switch {
		case errors.Is(err, ErrConflict):
			if filter == nil {
				return nil, ErrCompanyAlreadyExists
			}

			companies, listErr := s.List(ctx, *filter)
			if listErr != nil {
				return nil, listErr
			}
			if len(companies) > 0 {
				return &companies[0], nil
			}

			s.logger.Error("company create conflicted but existing record could not be resolved",
				"name", input.Name,
				"source", input.Source,
				"external_id", input.ExternalID,
				"domain", input.Domain,
			)
			return nil, ErrServiceInternal
		case errors.Is(err, ErrTimeout):
			return nil, fmt.Errorf("%w: timed out saving company", ErrServiceInternal)
		default:
			s.logger.Error("failed to find or create company",
				"name", input.Name,
				"source", input.Source,
				"external_id", input.ExternalID,
				"domain", input.Domain,
				"error", err,
			)
			return nil, ErrServiceInternal
		}
	}

	return company, nil
}

func buildFindOrCreateFilter(input *Company) *CompanyListFilter {
	if input == nil {
		return nil
	}

	filter := CompanyListFilter{Limit: 1}

	switch {
	case input.Source != "" && input.ExternalID != "":
		filter.Source = &input.Source
		filter.ExternalID = &input.ExternalID
		return &filter
	case input.Domain != "":
		filter.Domain = &input.Domain
		return &filter
	case input.Name != "":
		filter.Name = &input.Name
		return &filter
	default:
		return nil
	}
}
