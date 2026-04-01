package companies

import "context"

type CompanyListFilter struct {
	Source     *string
	ExternalID *string
	Domain     *string
	Name       *string
	Limit      int
	Offset     int
}

type Repository interface {
	Create(ctx context.Context, company *Company) error
	GetByID(ctx context.Context, id int64) (*Company, error)
	Update(ctx context.Context, company *Company) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter CompanyListFilter) ([]Company, error)
}
