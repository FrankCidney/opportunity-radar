package jobs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lib/pq"
)

type PostgresRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

func NewPostgresRepository(db *sql.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgQueryCancelled      = "57014"
)

func (r *PostgresRepository) mapError(op string, err error) error {
	// Context cancellation is not a DB error - surface it directly so
	// the service can distinguish a client disconnect from a real failure
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", op, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", op, context.DeadlineExceeded)
	}

	// No rows found is not a Postgres error. It just means the record doesn't exist
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case pgUniqueViolation:
			// pqErr.Constraint tells you on which unique field you tried to insert
			//  a duplicate value. Useful if a table has multiple fiels set to unique.
			// Log it here before discarding the raw pq.Error,
			// since it won't propagate further.
			r.logger.Warn("unique constraint violation",
				"op", op,
				"constraint", pqErr.Constraint,
				"detail", pqErr.Detail,
			)
			return fmt.Errorf("%s: %w", op, ErrConflict)

		case pgForeignKeyViolation:
			r.logger.Warn("foreign key violation",
				"op", op,
				"constraint", pqErr.Constraint,
				"detail", pqErr.Detail,
			)
			return fmt.Errorf("%s: %w", op, ErrReferenceNotFound)

		case pgQueryCancelled:
			return fmt.Errorf("%s: %w", op, ErrTimeout)
		}

		// Unhandled Postgres error - log the full pq.Error while we still have it.
		r.logger.Error("unhandled postgres error",
			"op", op,
			"code", pqErr.Code,
			"message", pqErr.Message,
			"detail", pqErr.Detail,
			"constraint", pqErr.Constraint,
		)
		return fmt.Errorf("%s: %w", op, ErrInternal)
	}

	// Non-Postgres error (connection failure, driver bug, etc.)
	r.logger.Error("unexpected database error",
		"op", op,
		"error", err,
	)
	return fmt.Errorf("%s: %w", op, ErrInternal)
}

func (r *PostgresRepository) Create(ctx context.Context, job *Job) error {
	const op = "jobs.PostgresRepository.Create"
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

	err := r.db.QueryRowContext(ctx, query,
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
	if err != nil {
		return r.mapError(op, err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*Job, error) {
	const op = "jobs.PostgresRepository.GetByID"

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
	if err != nil {
		return nil, r.mapError(op, err)
	}

	return &job, nil
}

func (r *PostgresRepository) Update(ctx context.Context, job *Job) error {
	const op = "jobs.PostgresRepository.Update"

	query := `
		UPDATE jobs
		SET
			company_id = $1,
			title = $2,
			description = $3,
			location = $4,
			url = $5,
			source = $6,
			posted_at = $7,
			application_deadline = $8,
			score = $9,
			status = $10,
			updated_at = $11
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
		return r.mapError(op, err)
	}

	// WARNING: Not every database or database driver supports getting RowsAffected from result (an sql.Result)
	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError(op, err)
	}

	if rows == 0 {
		return r.mapError(op, sql.ErrNoRows)
	}

	return nil
}

func (r *PostgresRepository) Delete(ctx context.Context, id int64) error {
	const op = "jobs.PostgresRepository.Delete"

	query := `DELETE FROM jobs WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return r.mapError(op, err)
	}

	// TODO: Figure out what kinds of errors could come up here, and whether there are any that should be explicitly handled (i.e., they affect business rules)
	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError(op, err)
	}
	if rows == 0 {
		return r.mapError(op, sql.ErrNoRows)
	}

	return nil
}

func (r *PostgresRepository) List(ctx context.Context, filter JobListFilter) ([]Job, error) {
	const op = "jobs.PostgresRepository.List"

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
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, r.mapError(op, err)
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
			return nil, r.mapError(op, err)
		}

		jobs = append(jobs, job)
	}

	return jobs, r.mapError(op, rows.Err())
}
