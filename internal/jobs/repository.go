// package jobs

// import (
// 	"context"
// 	"database/sql"

// 	"opportunity-radar/internal/models"
// )

// type JobRepository interface {
// 	Create(ctx context.Context) error
// 	GetByID(ctx context.Context, id int64) (*models.Job)
// 	Update(ctx context.Context, job *models.Job) error
// 	Delete(ctx context.Context, id int64) error
// 	// List(ctx context.Context, filter JobListFilter) ([]models.Job, error)
// }

// type JobRepositoryPostgres struct {
// 	db *sql.DB
// }

// // func (r *JobRepository) New() JobRepository {
// // 	return &JobRepository{
// // 		db: sql.DB,
// // 	}
// // }
