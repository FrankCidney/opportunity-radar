package scoring

import (
	"testing"
	"time"

	"opportunity-radar/internal/jobs"
)

func TestRulesScorerPrefersRelevantJuniorRemoteBackendJobs(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	scorer := newRulesScorerWithClock(testProfile(), func() time.Time { return now })

	relevant := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Build APIs in Go and Postgres for a distributed product team.",
		Location:    "Remote",
		PostedAt:    now.Add(-48 * time.Hour),
	}

	mismatch := &jobs.Job{
		Title:       "Senior Customer Success Manager",
		Description: "Own onboarding, renewals, and customer relationships for SaaS accounts.",
		Location:    "Remote",
		PostedAt:    now.Add(-48 * time.Hour),
	}

	if scorer.Score(relevant) <= scorer.Score(mismatch) {
		t.Fatalf("expected relevant backend job to score higher")
	}
}

func TestRulesScorerWeightsTitleMatchesAboveDescriptionOnlyMatches(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	scorer := newRulesScorerWithClock(testProfile(), func() time.Time { return now })

	titleMatch := &jobs.Job{
		Title:       "Backend Specialist",
		Description: "Help build internal tools.",
		Location:    "Remote",
		PostedAt:    now.Add(-5 * 24 * time.Hour),
	}

	descriptionMatch := &jobs.Job{
		Title:       "Product Specialist",
		Description: "You will work with backend systems.",
		Location:    "Remote",
		PostedAt:    now.Add(-5 * 24 * time.Hour),
	}

	if scorer.Score(titleMatch) <= scorer.Score(descriptionMatch) {
		t.Fatalf("expected title match to score higher than description-only match")
	}
}

func TestRulesScorerRewardsFreshnessWhenOtherSignalsAreEqual(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	scorer := newRulesScorerWithClock(testProfile(), func() time.Time { return now })

	fresh := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Go, Postgres, and REST APIs.",
		Location:    "Remote",
		PostedAt:    now.Add(-2 * 24 * time.Hour),
	}

	stale := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Go, Postgres, and REST APIs.",
		Location:    "Remote",
		PostedAt:    now.Add(-21 * 24 * time.Hour),
	}

	if scorer.Score(fresh) <= scorer.Score(stale) {
		t.Fatalf("expected fresher job to score higher")
	}
}

func TestRulesScorerAppliesLocationPenalty(t *testing.T) {
	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	scorer := newRulesScorerWithClock(testProfile(), func() time.Time { return now })

	remote := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Go, Postgres, and Docker.",
		Location:    "Remote",
		PostedAt:    now.Add(-4 * 24 * time.Hour),
	}

	onsite := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Go, Postgres, and Docker.",
		Location:    "On-site in Nairobi",
		PostedAt:    now.Add(-4 * 24 * time.Hour),
	}

	if scorer.Score(remote) <= scorer.Score(onsite) {
		t.Fatalf("expected remote job to score higher than on-site job")
	}
}

func testProfile() Profile {
	return Profile{
		RoleKeywords: []string{
			"backend",
			"software engineer",
			"engineer",
			"developer",
			"api",
		},
		SkillKeywords: []string{
			"go",
			"golang",
			"postgres",
			"docker",
			"rest",
		},
		PreferredLevelKeywords: []string{
			"junior",
			"entry level",
			"associate",
			"intern",
		},
		PenaltyLevelKeywords: []string{
			"senior",
			"staff",
			"principal",
			"lead",
			"manager",
		},
		PreferredLocationTerms: []string{
			"remote",
			"distributed",
		},
		PenaltyLocationTerms: []string{
			"on-site",
			"onsite",
			"in office",
		},
		MismatchKeywords: []string{
			"sales",
			"customer success",
			"marketing",
			"designer",
		},
	}
}
