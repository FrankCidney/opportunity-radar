package preferences

import "time"

type Settings struct {
	ID                     int64
	SetupComplete          bool
	DesiredRoles           []string
	ExperienceLevel        string
	CurrentSkills          []string
	GrowthSkills           []string
	Locations              []string
	WorkModes              []string
	AvoidTerms             []string
	RoleKeywords           []string
	SkillKeywords          []string
	PreferredLevelKeywords []string
	PenaltyLevelKeywords   []string
	PreferredLocationTerms []string
	PenaltyLocationTerms   []string
	MismatchKeywords       []string
	DigestEnabled          bool
	DigestRecipient        string
	DigestTopN             int
	DigestLookback         time.Duration
	CreatedAt              time.Time
	UpdatedAt              time.Time
}
