package jobs

import (
	"context"
)

type JobListFilter struct {
	CompanyID *int64
	Status *JobStatus
	Limit int
	Offset int
}

type Repository interface {
	Create(ctx context.Context, job *Job) error
	GetByID(ctx context.Context, id int64) (*Job, error)
	Update(ctx context.Context, job *Job) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter JobListFilter) ([]Job, error)
}
