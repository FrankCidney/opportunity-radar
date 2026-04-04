package scoring

import (
	"math"
	"strings"
	"sync"
	"time"

	"opportunity-radar/internal/jobs"
)

type Profile struct {
	RoleKeywords           []string
	SkillKeywords          []string
	PreferredLevelKeywords []string
	PenaltyLevelKeywords   []string
	PreferredLocationTerms []string
	PenaltyLocationTerms   []string
	MismatchKeywords       []string
}

type RulesScorer struct {
	mu      sync.RWMutex
	profile Profile
	now     func() time.Time
}

func NewRulesScorer(profile Profile) *RulesScorer {
	return &RulesScorer{
		profile: profile,
		now:     time.Now,
	}
}

func newRulesScorerWithClock(profile Profile, now func() time.Time) *RulesScorer {
	return &RulesScorer{
		profile: profile,
		now:     now,
	}
}

func (s *RulesScorer) Score(job *jobs.Job) float64 {
	s.mu.RLock()
	profile := s.profile
	nowFn := s.now
	s.mu.RUnlock()

	title := normalizeText(job.Title)
	description := normalizeText(job.Description)
	location := normalizeText(job.Location)

	score := 0.0

	score += weightedMatches(title, description, profile.RoleKeywords, 24, 8)
	score += weightedMatches(title, description, profile.SkillKeywords, 14, 5)
	score += weightedMatches(title, description, profile.PreferredLevelKeywords, 16, 5)
	score += locationMatches(location, description, profile.PreferredLocationTerms, 12, 4)

	score -= weightedMatches(title, description, profile.PenaltyLevelKeywords, 18, 7)
	score -= locationMatches(location, description, profile.PenaltyLocationTerms, 14, 5)
	score -= weightedMatches(title, description, profile.MismatchKeywords, 16, 6)

	score += freshnessScore(job.PostedAt, nowFn())

	return math.Max(score, 0)
}

func (s *RulesScorer) SetProfile(profile Profile) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.profile = profile
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}

	return " " + strings.Join(strings.Fields(value), " ") + " "
}

func weightedMatches(title string, description string, terms []string, titleWeight float64, descriptionWeight float64) float64 {
	score := 0.0

	for _, term := range terms {
		normalized := normalizeText(term)
		if normalized == "" {
			continue
		}

		if strings.Contains(title, normalized) {
			score += titleWeight
			continue
		}

		if strings.Contains(description, normalized) {
			score += descriptionWeight
		}
	}

	return score
}

func locationMatches(location string, description string, terms []string, locationWeight float64, descriptionWeight float64) float64 {
	score := 0.0

	for _, term := range terms {
		normalized := normalizeText(term)
		if normalized == "" {
			continue
		}

		if strings.Contains(location, normalized) {
			score += locationWeight
			continue
		}

		if strings.Contains(description, normalized) {
			score += descriptionWeight
		}
	}

	return score
}

func freshnessScore(postedAt time.Time, now time.Time) float64 {
	if postedAt.IsZero() {
		return 0
	}

	if postedAt.After(now) {
		return 0
	}

	age := now.Sub(postedAt)

	switch {
	case age <= 3*24*time.Hour:
		return 15
	case age <= 7*24*time.Hour:
		return 10
	case age <= 14*24*time.Hour:
		return 5
	case age <= 30*24*time.Hour:
		return 2
	default:
		return 0
	}
}
