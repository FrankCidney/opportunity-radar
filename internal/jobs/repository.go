package jobs

import (
	"context"
	"time"
)

type JobSort string

const (
	JobSortPostedAtDesc JobSort = "posted_at_desc"
	JobSortScoreDesc    JobSort = "score_desc"
)

type JobListFilter struct {
	CompanyID    *int64
	Status       *JobStatus
	CreatedAfter *time.Time
	Limit        int
	Offset       int
	SortBy       JobSort
}

type Repository interface {
	Create(ctx context.Context, job *Job) error
	GetByID(ctx context.Context, id int64) (*Job, error)
	Update(ctx context.Context, job *Job) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, filter JobListFilter) ([]Job, error)
}
