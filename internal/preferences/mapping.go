package preferences

import (
	"strings"

	"opportunity-radar/internal/scoring"
)

var (
	ExperienceOptions = []string{
		"Junior / early-career",
		"Mid-level",
		"Senior",
		"Lead / manager",
	}

	WorkModeOptions = []string{
		"Remote",
		"Hybrid",
		"On-site",
	}

	LocationOptions = []string{
		"Remote",
		"Kenya",
		"Uganda",
		"Tanzania",
		"Rwanda",
		"South Africa",
		"Nigeria",
		"Ghana",
		"United Kingdom",
		"United States",
	}

	EmailLookbackOptions = []string{"24h", "48h", "72h"}
)

func (s *Settings) RecalculateDerivedFields() {
	if s == nil {
		return
	}

	s.SetupComplete = IsSetupComplete(s)
	s.RoleKeywords = deriveRoleKeywords(s.DesiredRoles)
	s.SkillKeywords = deriveSkillKeywords(s.CurrentSkills, s.GrowthSkills)
	s.PreferredLevelKeywords, s.PenaltyLevelKeywords = deriveLevelKeywords(s.ExperienceLevel)
	s.PreferredLocationTerms, s.PenaltyLocationTerms = deriveLocationTerms(s.WorkModes, s.Locations)
	s.MismatchKeywords = normalizeStringList(s.AvoidTerms)
}

func IsSetupComplete(s *Settings) bool {
	if s == nil {
		return false
	}

	return len(normalizeStringList(s.DesiredRoles)) > 0 &&
		strings.TrimSpace(s.ExperienceLevel) != "" &&
		len(normalizeStringList(s.Locations)) > 0 &&
		len(normalizeStringList(s.WorkModes)) > 0
}

func (s *Settings) PartialSetup() bool {
	if s == nil {
		return false
	}

	return len(s.DesiredRoles) > 0 ||
		strings.TrimSpace(s.ExperienceLevel) != "" ||
		len(s.CurrentSkills) > 0 ||
		len(s.GrowthSkills) > 0 ||
		len(s.Locations) > 0 ||
		len(s.WorkModes) > 0 ||
		len(s.AvoidTerms) > 0 ||
		strings.TrimSpace(s.DigestRecipient) != ""
}

func BuildScoringProfile(s *Settings) scoring.Profile {
	if s == nil {
		return scoring.Profile{}
	}

	return scoring.Profile{
		RoleKeywords:           s.RoleKeywords,
		SkillKeywords:          s.SkillKeywords,
		PreferredLevelKeywords: s.PreferredLevelKeywords,
		PenaltyLevelKeywords:   s.PenaltyLevelKeywords,
		PreferredLocationTerms: s.PreferredLocationTerms,
		PenaltyLocationTerms:   s.PenaltyLocationTerms,
		MismatchKeywords:       s.MismatchKeywords,
	}
}

func deriveRoleKeywords(roles []string) []string {
	derived := normalizeStringList(roles)

	roleExpansions := map[string][]string{
		"backend engineer":   {"backend", "software engineer", "engineer", "api", "platform"},
		"software engineer":  {"software engineer", "engineer", "developer"},
		"platform engineer":  {"platform", "engineer", "infrastructure"},
		"backend developer":  {"backend", "developer", "software developer"},
		"software developer": {"software developer", "developer"},
	}

	result := make([]string, 0, len(derived)*3)
	result = append(result, derived...)
	for _, role := range derived {
		if expansions, ok := roleExpansions[role]; ok {
			result = append(result, expansions...)
		}
	}

	return normalizeStringList(result)
}

func deriveSkillKeywords(currentSkills []string, growthSkills []string) []string {
	result := make([]string, 0, len(currentSkills)+len(growthSkills))
	result = append(result, normalizeStringList(currentSkills)...)
	result = append(result, normalizeStringList(growthSkills)...)
	return normalizeStringList(result)
}

func deriveLevelKeywords(level string) ([]string, []string) {
	switch strings.TrimSpace(level) {
	case "Junior / early-career":
		return []string{"junior", "entry level", "entry-level", "graduate", "new grad", "intern", "associate"},
			[]string{"senior", "staff", "principal", "lead", "manager", "director", "head of"}
	case "Mid-level":
		return []string{"mid", "mid-level", "intermediate"},
			[]string{"staff", "principal", "director", "head of"}
	case "Senior":
		return []string{"senior", "staff"},
			[]string{"intern", "entry level", "entry-level"}
	case "Lead / manager":
		return []string{"lead", "manager", "head of", "director"},
			[]string{"intern", "entry level", "entry-level", "junior", "associate"}
	default:
		return nil, nil
	}
}

func deriveLocationTerms(workModes []string, locations []string) ([]string, []string) {
	preferred := make([]string, 0, len(workModes)+len(locations))
	preferred = append(preferred, normalizeStringList(locations)...)

	workModeMap := map[string][]string{
		"remote":  {"remote", "worldwide", "distributed", "anywhere"},
		"hybrid":  {"hybrid"},
		"on-site": {"on-site", "onsite", "in office"},
	}

	normalizedModes := normalizeStringList(workModes)
	for _, mode := range normalizedModes {
		preferred = append(preferred, workModeMap[mode]...)
	}

	var penalties []string
	if containsString(normalizedModes, "remote") && !containsString(normalizedModes, "on-site") {
		penalties = append(penalties, "on-site", "onsite", "in office", "relocation required")
	}

	return normalizeStringList(preferred), normalizeStringList(penalties)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
