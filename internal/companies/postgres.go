package companies

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
	pgUniqueViolation = "23505"
	pgQueryCancelled  = "57014"
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
			// a duplicate value. Useful if a table has multiple fields set to unique.
			// Log it here before discarding the raw pq.Error,
			// since it won't propagate further.
			r.logger.Warn("unique constraint violation",
				"op", op,
				"constraint", pqErr.Constraint,
				"detail", pqErr.Detail,
			)
			return fmt.Errorf("%s: %w", op, ErrConflict)

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

func (r *PostgresRepository) Create(ctx context.Context, company *Company) error {
	const op = "companies.PostgresRepository.Create"

	query := `
	INSERT INTO companies (
		name, logo_url, created_at, updated_at, source, external_id, domain
	)
	VALUES (
		$1, $2, $3, $4, $5, $6, $7
	)
	RETURNING id
	`

	now := time.Now().UTC()
	company.CreatedAt = now
	company.UpdatedAt = now

	err := r.db.QueryRowContext(ctx, query,
		company.Name,
		company.LogoURL,
		company.CreatedAt,
		company.UpdatedAt,
		company.Source,
		company.ExternalID,
		company.Domain,
	).Scan(&company.ID)
	if err != nil {
		return r.mapError(op, err)
	}

	return nil
}

func (r *PostgresRepository) GetByID(ctx context.Context, id int64) (*Company, error) {
	const op = "companies.PostgresRepository.GetByID"

	query := `
	SELECT
		id, name, logo_url, created_at, updated_at, source, external_id, domain
	FROM companies
	WHERE id = $1
	`

	var company Company
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&company.ID,
		&company.Name,
		&company.LogoURL,
		&company.CreatedAt,
		&company.UpdatedAt,
		&company.Source,
		&company.ExternalID,
		&company.Domain,
	)
	if err != nil {
		return nil, r.mapError(op, err)
	}

	return &company, nil
}

func (r *PostgresRepository) Update(ctx context.Context, company *Company) error {
	const op = "companies.PostgresRepository.Update"

	query := `
	UPDATE companies
	SET
		name = $1,
		logo_url = $2,
		source = $3,
		external_id = $4,
		domain = $5,
		updated_at = $6
	WHERE id = $7
	`

	company.UpdatedAt = time.Now().UTC()

	result, err := r.db.ExecContext(ctx, query,
		company.Name,
		company.LogoURL,
		company.Source,
		company.ExternalID,
		company.Domain,
		company.UpdatedAt,
		company.ID,
	)
	if err != nil {
		return r.mapError(op, err)
	}

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
	const op = "companies.PostgresRepository.Delete"

	query := `DELETE FROM companies WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return r.mapError(op, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return r.mapError(op, err)
	}
	if rows == 0 {
		return r.mapError(op, sql.ErrNoRows)
	}

	return nil
}

func (r *PostgresRepository) List(ctx context.Context, filter CompanyListFilter) ([]Company, error) {
	const op = "companies.PostgresRepository.List"

	baseQuery := `
		SELECT
			id, name, logo_url, created_at, updated_at, source, external_id, domain
		FROM companies
	`

	var conditions []string
	var args []interface{}
	argPos := 1

	if filter.Source != nil {
		conditions = append(conditions, fmt.Sprintf("source = $%d", argPos))
		args = append(args, *filter.Source)
		argPos++
	}

	if filter.ExternalID != nil {
		conditions = append(conditions, fmt.Sprintf("external_id = $%d", argPos))
		args = append(args, *filter.ExternalID)
		argPos++
	}

	if filter.Domain != nil {
		conditions = append(conditions, fmt.Sprintf("domain = $%d", argPos))
		args = append(args, *filter.Domain)
		argPos++
	}

	if filter.Name != nil {
		conditions = append(conditions, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *filter.Name)
		argPos++
	}

	query := baseQuery
	if len(conditions) > 0 {
		query += "WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

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

	var companies []Company
	for rows.Next() {
		var company Company
		if err := rows.Scan(
			&company.ID,
			&company.Name,
			&company.LogoURL,
			&company.CreatedAt,
			&company.UpdatedAt,
			&company.Source,
			&company.ExternalID,
			&company.Domain,
		); err != nil {
			return nil, r.mapError(op, err)
		}
		companies = append(companies, company)
	}

	if err := rows.Err(); err != nil {
		return nil, r.mapError(op, err)
	}

	return companies, nil
}
