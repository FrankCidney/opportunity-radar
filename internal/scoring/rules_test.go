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

func TestRulesScorerScoreIsStableSignalsPlusFreshness(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 4, 12, 0, 0, 0, time.UTC)
	profile := Profile{
		RoleKeywords:           []string{"backend"},
		SkillKeywords:          []string{"go"},
		PreferredLevelKeywords: []string{"junior"},
		PreferredLocationTerms: []string{"remote"},
	}
	scorer := newRulesScorerWithClock(profile, func() time.Time { return now })
	job := &jobs.Job{
		Title:       "Junior Backend Engineer",
		Description: "Build services in Go.",
		Location:    "Remote",
		PostedAt:    now.Add(-48 * time.Hour),
	}

	withoutPostedAt := *job
	withoutPostedAt.PostedAt = time.Time{}

	stableScore := scorer.Score(&withoutPostedAt)
	got := scorer.Score(job)
	want := stableScore + freshnessScore(job.PostedAt, now)

	if got != want {
		t.Fatalf("Score() = %v, want stable score %v + freshness %v = %v",
			got, stableScore, freshnessScore(job.PostedAt, now), want)
	}
}

func TestFreshnessScoreAgeBands(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		postedAt time.Time
		want     float64
	}{
		{name: "missing date", postedAt: time.Time{}, want: 0},
		{name: "future date", postedAt: now.Add(time.Minute), want: 0},
		{name: "three days old", postedAt: now.Add(-3 * 24 * time.Hour), want: 15},
		{name: "seven days old", postedAt: now.Add(-7 * 24 * time.Hour), want: 10},
		{name: "fourteen days old", postedAt: now.Add(-14 * 24 * time.Hour), want: 5},
		{name: "thirty days old", postedAt: now.Add(-30 * 24 * time.Hour), want: 2},
		{name: "older than thirty days", postedAt: now.Add(-31 * 24 * time.Hour), want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := freshnessScore(tt.postedAt, now); got != tt.want {
				t.Fatalf("freshnessScore() = %v, want %v", got, tt.want)
			}
		})
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
