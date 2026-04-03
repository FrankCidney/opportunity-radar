package digest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/lib/pq"
)

type PostgresRepository struct {
	db     *sql.DB
	logger *slog.Logger
}

const (
	pgUniqueViolation = "23505"
	pgQueryCancelled  = "57014"
)

func NewPostgresRepository(db *sql.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}

func (r *PostgresRepository) mapError(op string, err error) error {
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s: %w", op, context.Canceled)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: %w", op, context.DeadlineExceeded)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		switch pqErr.Code {
		case pgUniqueViolation:
			r.logger.Warn("unique constraint violation",
				"op", op,
				"constraint", pqErr.Constraint,
				"detail", pqErr.Detail,
			)
			return fmt.Errorf("%s: %w", op, ErrConflict)
		case pgQueryCancelled:
			return fmt.Errorf("%s: %w", op, ErrTimeout)
		}

		r.logger.Error("unhandled postgres error",
			"op", op,
			"code", pqErr.Code,
			"message", pqErr.Message,
			"detail", pqErr.Detail,
			"constraint", pqErr.Constraint,
		)
		return fmt.Errorf("%s: %w", op, ErrInternal)
	}

	r.logger.Error("unexpected database error",
		"op", op,
		"error", err,
	)
	return fmt.Errorf("%s: %w", op, ErrInternal)
}

func (r *PostgresRepository) Create(ctx context.Context, delivery *Delivery) error {
	const op = "digest.PostgresRepository.Create"

	query := `
	INSERT INTO digest_deliveries (
		recipient, digest_date, job_count, subject, sent_at, created_at
	)
	VALUES (
		$1, $2, $3, $4, $5, $6
	)
	RETURNING id
	`

	now := time.Now().UTC()
	delivery.SentAt = now
	delivery.CreatedAt = now

	err := r.db.QueryRowContext(ctx, query,
		delivery.Recipient,
		delivery.DigestDate.Format(time.DateOnly),
		delivery.JobCount,
		delivery.Subject,
		delivery.SentAt,
		delivery.CreatedAt,
	).Scan(&delivery.ID)
	if err != nil {
		return r.mapError(op, err)
	}

	return nil
}

func (r *PostgresRepository) GetByRecipientAndDate(ctx context.Context, recipient string, digestDate string) (*Delivery, error) {
	const op = "digest.PostgresRepository.GetByRecipientAndDate"

	query := `
	SELECT
		id, recipient, digest_date, job_count, subject, sent_at, created_at
	FROM digest_deliveries
	WHERE recipient = $1 AND digest_date = $2
	`

	var delivery Delivery
	err := r.db.QueryRowContext(ctx, query, recipient, digestDate).Scan(
		&delivery.ID,
		&delivery.Recipient,
		&delivery.DigestDate,
		&delivery.JobCount,
		&delivery.Subject,
		&delivery.SentAt,
		&delivery.CreatedAt,
	)
	if err != nil {
		return nil, r.mapError(op, err)
	}

	return &delivery, nil
}
