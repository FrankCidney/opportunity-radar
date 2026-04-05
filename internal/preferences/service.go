package preferences

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type Service struct {
	repo   Repository
	logger *slog.Logger
}

func NewService(repo Repository, logger *slog.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) Get(ctx context.Context) (*Settings, error) {
	settings, err := s.repo.Get(ctx)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			return nil, ErrSettingsNotFound
		case errors.Is(err, ErrTimeout):
			return nil, fmt.Errorf("%w: timed out loading settings", ErrServiceInternal)
		default:
			s.logger.Error("failed to load settings", "error", err)
			return nil, ErrServiceInternal
		}
	}

	return settings, nil
}

func (s *Service) Save(ctx context.Context, settings *Settings) error {
	normalized := normalizeSettings(settings)
	normalized.RecalculateDerivedFields()

	if err := s.repo.Save(ctx, normalized); err != nil {
		switch {
		case errors.Is(err, ErrTimeout):
			return fmt.Errorf("%w: timed out saving settings", ErrServiceInternal)
		default:
			s.logger.Error("failed to save settings", "error", err)
			return ErrServiceInternal
		}
	}

	return nil
}

func (s *Service) Ensure(ctx context.Context, bootstrap *Settings) (*Settings, bool, error) {
	settings, err := s.Get(ctx)
	if err == nil {
		return settings, false, nil
	}
	if !errors.Is(err, ErrSettingsNotFound) {
		return nil, false, err
	}

	if bootstrap == nil {
		return nil, false, ErrSettingsNotFound
	}

	if err := s.Save(ctx, bootstrap); err != nil {
		return nil, false, err
	}

	settings, err = s.Get(ctx)
	if err != nil {
		return nil, true, err
	}

	return settings, true, nil
}

func normalizeSettings(input *Settings) *Settings {
	if input == nil {
		input = &Settings{}
	}

	return &Settings{
		ID:                     input.ID,
		SetupComplete:          input.SetupComplete,
		DesiredRoles:           normalizeStringList(input.DesiredRoles),
		ExperienceLevel:        strings.TrimSpace(input.ExperienceLevel),
		CurrentSkills:          normalizeStringList(input.CurrentSkills),
		GrowthSkills:           normalizeStringList(input.GrowthSkills),
		Locations:              normalizeStringList(input.Locations),
		WorkModes:              normalizeStringList(input.WorkModes),
		AvoidTerms:             normalizeStringList(input.AvoidTerms),
		RoleKeywords:           normalizeStringList(input.RoleKeywords),
		SkillKeywords:          normalizeStringList(input.SkillKeywords),
		PreferredLevelKeywords: normalizeStringList(input.PreferredLevelKeywords),
		PenaltyLevelKeywords:   normalizeStringList(input.PenaltyLevelKeywords),
		PreferredLocationTerms: normalizeStringList(input.PreferredLocationTerms),
		PenaltyLocationTerms:   normalizeStringList(input.PenaltyLocationTerms),
		MismatchKeywords:       normalizeStringList(input.MismatchKeywords),
		DigestEnabled:          input.DigestEnabled,
		DigestRecipient:        strings.TrimSpace(input.DigestRecipient),
		DigestTopN:             effectiveDigestTopN(input.DigestTopN),
		DigestLookback:         effectiveDigestLookback(input.DigestLookback),
		CreatedAt:              input.CreatedAt,
		UpdatedAt:              input.UpdatedAt,
	}
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))

	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	return normalized
}

func effectiveDigestTopN(value int) int {
	if value <= 0 {
		return 10
	}
	if value > 100 {
		return 100
	}
	return value
}

func effectiveDigestLookback(value time.Duration) time.Duration {
	if value <= 0 {
		return 24 * time.Hour
	}
	return value
}
