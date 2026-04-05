package preferences

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

const settingsRowID int64 = 1

func NewPostgresRepository(db *sql.DB, logger *slog.Logger) *PostgresRepository {
	return &PostgresRepository{
		db:     db,
		logger: logger,
	}
}

func (r *PostgresRepository) Get(ctx context.Context) (*Settings, error) {
	const op = "preferences.PostgresRepository.Get"

	query := `
	SELECT
		id,
		setup_complete,
		desired_roles,
		experience_level,
		current_skills,
		growth_skills,
		locations,
		work_modes,
		avoid_terms,
		role_keywords,
		skill_keywords,
		preferred_level_keywords,
		penalty_level_keywords,
		preferred_location_terms,
		penalty_location_terms,
		mismatch_keywords,
		digest_enabled,
		digest_recipient,
		digest_top_n,
		digest_lookback_seconds,
		created_at,
		updated_at
	FROM app_settings
	WHERE id = $1
	`

	var (
		settings              Settings
		digestLookbackSeconds int64
	)

	err := r.db.QueryRowContext(ctx, query, settingsRowID).Scan(
		&settings.ID,
		&settings.SetupComplete,
		pq.Array(&settings.DesiredRoles),
		&settings.ExperienceLevel,
		pq.Array(&settings.CurrentSkills),
		pq.Array(&settings.GrowthSkills),
		pq.Array(&settings.Locations),
		pq.Array(&settings.WorkModes),
		pq.Array(&settings.AvoidTerms),
		pq.Array(&settings.RoleKeywords),
		pq.Array(&settings.SkillKeywords),
		pq.Array(&settings.PreferredLevelKeywords),
		pq.Array(&settings.PenaltyLevelKeywords),
		pq.Array(&settings.PreferredLocationTerms),
		pq.Array(&settings.PenaltyLocationTerms),
		pq.Array(&settings.MismatchKeywords),
		&settings.DigestEnabled,
		&settings.DigestRecipient,
		&settings.DigestTopN,
		&digestLookbackSeconds,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if err != nil {
		return nil, r.mapError(op, err)
	}

	settings.DigestLookback = time.Duration(digestLookbackSeconds) * time.Second

	return &settings, nil
}

func (r *PostgresRepository) Save(ctx context.Context, settings *Settings) error {
	const op = "preferences.PostgresRepository.Save"

	query := `
	INSERT INTO app_settings (
		id,
		setup_complete,
		desired_roles,
		experience_level,
		current_skills,
		growth_skills,
		locations,
		work_modes,
		avoid_terms,
		role_keywords,
		skill_keywords,
		preferred_level_keywords,
		penalty_level_keywords,
		preferred_location_terms,
		penalty_location_terms,
		mismatch_keywords,
		digest_enabled,
		digest_recipient,
		digest_top_n,
		digest_lookback_seconds,
		created_at,
		updated_at
	)
	VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22
	)
	ON CONFLICT (id) DO UPDATE SET
		setup_complete = EXCLUDED.setup_complete,
		desired_roles = EXCLUDED.desired_roles,
		experience_level = EXCLUDED.experience_level,
		current_skills = EXCLUDED.current_skills,
		growth_skills = EXCLUDED.growth_skills,
		locations = EXCLUDED.locations,
		work_modes = EXCLUDED.work_modes,
		avoid_terms = EXCLUDED.avoid_terms,
		role_keywords = EXCLUDED.role_keywords,
		skill_keywords = EXCLUDED.skill_keywords,
		preferred_level_keywords = EXCLUDED.preferred_level_keywords,
		penalty_level_keywords = EXCLUDED.penalty_level_keywords,
		preferred_location_terms = EXCLUDED.preferred_location_terms,
		penalty_location_terms = EXCLUDED.penalty_location_terms,
		mismatch_keywords = EXCLUDED.mismatch_keywords,
		digest_enabled = EXCLUDED.digest_enabled,
		digest_recipient = EXCLUDED.digest_recipient,
		digest_top_n = EXCLUDED.digest_top_n,
		digest_lookback_seconds = EXCLUDED.digest_lookback_seconds,
		updated_at = EXCLUDED.updated_at
	RETURNING id, created_at, updated_at
	`

	now := time.Now().UTC()
	if settings.ID == 0 {
		settings.ID = settingsRowID
	}
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now

	err := r.db.QueryRowContext(
		ctx,
		query,
		settings.ID,
		settings.SetupComplete,
		pq.Array(settings.DesiredRoles),
		settings.ExperienceLevel,
		pq.Array(settings.CurrentSkills),
		pq.Array(settings.GrowthSkills),
		pq.Array(settings.Locations),
		pq.Array(settings.WorkModes),
		pq.Array(settings.AvoidTerms),
		pq.Array(settings.RoleKeywords),
		pq.Array(settings.SkillKeywords),
		pq.Array(settings.PreferredLevelKeywords),
		pq.Array(settings.PenaltyLevelKeywords),
		pq.Array(settings.PreferredLocationTerms),
		pq.Array(settings.PenaltyLocationTerms),
		pq.Array(settings.MismatchKeywords),
		settings.DigestEnabled,
		settings.DigestRecipient,
		settings.DigestTopN,
		int64(settings.DigestLookback/time.Second),
		settings.CreatedAt,
		settings.UpdatedAt,
	).Scan(&settings.ID, &settings.CreatedAt, &settings.UpdatedAt)
	if err != nil {
		return r.mapError(op, err)
	}

	return nil
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
