package preferences

import (
	"testing"
	"time"

	"opportunity-radar/internal/scoring"
)

func TestDeriveRoleKeywordsUsesFamiliesAliasesAndFallbackTokens(t *testing.T) {
	got := deriveRoleKeywords([]string{
		"Backend Engineer",
		"Account Executive",
		"Payments Platform Engineer",
	})

	want := []string{
		"backend engineer",
		"account executive",
		"payments platform engineer",
		"backend",
		"backend developer",
		"api",
		"server",
		"services",
		"microservices",
		"software engineer",
		"software developer",
		"developer",
		"engineer",
		"sales",
		"business development",
		"bdr",
		"sdr",
		"platform",
		"platform engineer",
		"infrastructure",
		"devops",
		"site reliability",
		"sre",
	}

	if !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected derived role keywords:\n got: %v\nwant: %v", got, want)
	}
}

func TestDeriveLocationTermsRemoteAddsPenaltiesWithoutBlockingHybrid(t *testing.T) {
	preferred, penalties := deriveLocationTerms(
		[]string{"Remote", "Hybrid"},
		[]string{"Kenya", "Remote"},
	)

	if got, want := preferred, []string{"kenya", "remote", "worldwide", "distributed", "anywhere", "hybrid"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected preferred location terms: got %v want %v", got, want)
	}

	if got, want := penalties, []string{"on-site", "onsite", "in office", "relocation required"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected penalty location terms: got %v want %v", got, want)
	}
}

func TestRecalculateDerivedFieldsBuildsExplainableProfileInputs(t *testing.T) {
	settings := &Settings{
		DesiredRoles:    []string{"Backend Engineer"},
		ExperienceLevel: "Junior / early-career",
		CurrentSkills:   []string{"Go"},
		GrowthSkills:    []string{"Python"},
		Locations:       []string{"Kenya"},
		WorkModes:       []string{"Remote"},
		AvoidTerms:      []string{"Sales"},
	}

	settings.RecalculateDerivedFields()

	if !settings.SetupComplete {
		t.Fatalf("expected setup to be complete")
	}

	profile := BuildScoringProfile(settings)
	want := scoring.Profile{
		RoleKeywords: []string{
			"backend engineer",
			"backend",
			"backend developer",
			"api",
			"server",
			"services",
			"microservices",
			"software engineer",
			"software developer",
			"developer",
			"engineer",
		},
		SkillKeywords:          []string{"go", "python"},
		PreferredLevelKeywords: []string{"junior", "entry level", "entry-level", "graduate", "new grad", "intern", "associate"},
		PenaltyLevelKeywords:   []string{"senior", "staff", "principal", "lead", "manager", "director", "head of"},
		PreferredLocationTerms: []string{"kenya", "remote", "worldwide", "distributed", "anywhere"},
		PenaltyLocationTerms:   []string{"on-site", "onsite", "in office", "relocation required"},
		MismatchKeywords:       []string{"sales"},
	}

	if !profilesEqual(profile, want) {
		t.Fatalf("unexpected scoring profile:\n got: %+v\nwant: %+v", profile, want)
	}
}

func TestIsSetupCompleteRequiresConfiguredFieldsOnly(t *testing.T) {
	settings := &Settings{
		DesiredRoles:    []string{"Backend Engineer"},
		ExperienceLevel: "Junior / early-career",
		Locations:       []string{"Kenya"},
	}

	if IsSetupComplete(settings) {
		t.Fatalf("expected setup to be incomplete without work modes")
	}

	settings.WorkModes = []string{"Remote"}
	if !IsSetupComplete(settings) {
		t.Fatalf("expected setup to be complete once required fields are present")
	}
}

func TestEffectiveDigestLookbackDefaultsWhenUnset(t *testing.T) {
	if got := effectiveDigestLookback(0); got != 24*time.Hour {
		t.Fatalf("unexpected digest lookback default: got %s want 24h", got)
	}
}

func profilesEqual(got scoring.Profile, want scoring.Profile) bool {
	return stringSlicesEqual(got.RoleKeywords, want.RoleKeywords) &&
		stringSlicesEqual(got.SkillKeywords, want.SkillKeywords) &&
		stringSlicesEqual(got.PreferredLevelKeywords, want.PreferredLevelKeywords) &&
		stringSlicesEqual(got.PenaltyLevelKeywords, want.PenaltyLevelKeywords) &&
		stringSlicesEqual(got.PreferredLocationTerms, want.PreferredLocationTerms) &&
		stringSlicesEqual(got.PenaltyLocationTerms, want.PenaltyLocationTerms) &&
		stringSlicesEqual(got.MismatchKeywords, want.MismatchKeywords)
}
