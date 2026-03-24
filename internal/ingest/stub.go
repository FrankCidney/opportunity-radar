package ingest

import (
    "context"
    "opportunity-radar/internal/companies"
)

type StubCompanyService struct{}

func (s *StubCompanyService) FindOrCreate(ctx context.Context, input companies.Company) (*companies.Company, error) {
    return &companies.Company{ID: 1}, nil
}