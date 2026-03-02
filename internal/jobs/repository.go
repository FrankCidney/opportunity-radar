package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
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

type PostgresRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) Repository {
	return &PostgresRepository{
		db: db,
	}
}

func (r *PostgresRepository) Create(ctx context.Context, job *Job) error {
	// Create the query
	query := `
	INSERT INTO jobs (
		company_id, title, description, location, 
		url, source, posted_at, application_deadline,
		score, status, created_at, updated_at
	)
	VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12
	)
	RETURNING id
	`

	now := time.Now().UTC()
	job.CreatedAt = now
	job.UpdatedAt = now

	return r.db.QueryRowContext(ctx, query,
		job.CompanyID,
		job.Title,
		job.Description,
		job.Location,
		job.URL,
		job.Source,
		job.PostedAt,
		job.ApplicationDeadline,
		job.Score,
		job.Status,
		job.CreatedAt,
		job.UpdatedAt,
	).Scan(&job.ID)
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*Job, error) {
	query := `
	SELECT
		id, company_id, title, description, location,
		url, source, posted_at, application_deadline, score, 
		status, created_at, updated_at
	FROM jobs
	WHERE id = $1
	`
	var job Job
	
	err := r.db.QueryRowContext(
		ctx,
		query,
		id,
	).Scan(
		&job.ID,
		&job.CompanyID,
		&job.Title,
		&job.Description,
		&job.Location,
		&job.URL,
		&job.Source,
		&job.PostedAt,
		&job.ApplicationDeadline,
		&job.Score,
		&job.Status,
		&job.CreatedAt,
		&job.UpdatedAt,
	)

	// if err == sql.ErrNoRows {
	// 	return nil, err
	// }

	if err != nil {
		return nil, err
	}

	return &job, nil
}

func (r *PostgresRepository) Update(ctx context.Context, job *Job) error {
	query := `
		UPDATE jobs
		SET
			company_id = $1,
			title = $2,
			description = $3,
			location = $4,
			url = $5,
			souce = $6,
			posted_at = $7,
			application_deadline = $8,
			score = $9,
			status = $10,
			updated_at = $11,
		WHERE id = $12
	`

	job.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(
		ctx,
		query,
		job.CompanyID,
		job.Title,
		job.Description,
		job.Location,
		job.URL,
		job.Source,
		job.PostedAt,
		job.ApplicationDeadline,
		job.Score,
		job.Status,
		job.UpdatedAt,
		job.ID,
	)

	if err != nil {
		return err
	}

	// WARNING: Not every database or database drive supports getting RowsAffected from result (an sql.Result)
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	query := `DELETE FROM jobs WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	
	return nil
}

func (r *PostgresRepository) List(ctx context.Context, filter JobListFilter) ([]Job, error) {
	baseQuery := `
		SELECT
			id, company_id, title, description, location,
			url, source, posted_at, application_deadline, score, 
			status, created_at, updated_at
		FROM jobs
	`

	var conditions []string
	var args []interface{}
	argPos := 1

	if filter.CompanyID != nil {
		conditions = append(conditions, fmt.Sprintf("company_id = $%d", argPos))
		args = append(args, filter.CompanyID)
		argPos++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argPos))
		args = append(args, filter.Status)
		argPos++
	}

	query := baseQuery

	if len(conditions) > 0 {
		query += "WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY posted_at DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argPos)
		args = append(args, filter.Limit)
		argPos++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argPos)
		args = append(args, filter.Limit)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		var jobs []Job

		for rows.Next() {
			var job Job

			if err := rows.Scan(
				&job.ID,
				&job.CompanyID,
				&job.Title,
				&job.Description,
				&job.Location,
				&job.URL,
				&job.Source,
				&job.PostedAt,
				&job.ApplicationDeadline,
				&job.Score,
				&job.Status,
				&job.CreatedAt,
				&job.UpdatedAt,
			); err != nil {
				return nil, err
			}

			jobs = append(jobs, job)
		}

		return jobs, rows.Err()
}
