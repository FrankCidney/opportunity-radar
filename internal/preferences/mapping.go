package preferences

import (
	"strings"

	"opportunity-radar/internal/scoring"
)

type roleFamilyDefinition struct {
	Terms   []string
	Aliases []string
	Tokens  []string
}

var (
	DigestLookbackOptions = []DigestLookbackOption{
		{Value: "24h", Label: "1 day"},
		{Value: "48h", Label: "2 days"},
		{Value: "72h", Label: "3 days"},
		{Value: "120h", Label: "5 days"},
		{Value: "168h", Label: "7 days"},
	}

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

	EmailLookbackOptions = []string{"24h", "48h", "72h", "120h", "168h"}

	roleFamilyOrder = []string{
		"backend",
		"software-engineering",
		"platform",
		"frontend",
		"fullstack",
		"data",
		"product",
		"design",
		"sales",
		"customer-success",
	}

	roleFamilies = map[string]roleFamilyDefinition{
		"backend": {
			Terms:   []string{"backend", "backend engineer", "backend developer", "api", "server", "services", "microservices"},
			Aliases: []string{"backend engineer", "backend developer", "api engineer", "api developer", "server-side engineer", "golang backend engineer"},
			Tokens:  []string{"backend", "api", "server", "services"},
		},
		"software-engineering": {
			Terms:   []string{"software engineer", "software developer", "developer", "engineer"},
			Aliases: []string{"software engineer", "software developer", "application engineer", "applications engineer"},
			Tokens:  []string{"software", "engineer", "developer"},
		},
		"platform": {
			Terms:   []string{"platform", "platform engineer", "infrastructure", "devops", "site reliability", "sre"},
			Aliases: []string{"platform engineer", "infrastructure engineer", "devops engineer", "site reliability engineer", "sre"},
			Tokens:  []string{"platform", "infrastructure", "devops", "sre", "reliability"},
		},
		"frontend": {
			Terms:   []string{"frontend", "frontend engineer", "frontend developer", "ui", "web"},
			Aliases: []string{"frontend engineer", "frontend developer", "web engineer", "ui engineer"},
			Tokens:  []string{"frontend", "ui", "web"},
		},
		"fullstack": {
			Terms:   []string{"fullstack", "full-stack", "full stack", "fullstack engineer", "full stack engineer"},
			Aliases: []string{"fullstack engineer", "full-stack engineer", "full stack developer"},
			Tokens:  []string{"fullstack", "full-stack", "full stack"},
		},
		"data": {
			Terms:   []string{"data", "data engineer", "analytics engineer", "etl", "pipelines"},
			Aliases: []string{"data engineer", "analytics engineer", "data platform engineer"},
			Tokens:  []string{"data", "analytics", "etl", "pipeline"},
		},
		"product": {
			Terms:   []string{"product manager", "product management", "pm"},
			Aliases: []string{"product manager", "technical product manager"},
			Tokens:  []string{"product"},
		},
		"design": {
			Terms:   []string{"designer", "product designer", "ux", "ui", "ux designer", "ui designer"},
			Aliases: []string{"product designer", "ux designer", "ui designer"},
			Tokens:  []string{"design", "designer", "ux", "ui"},
		},
		"sales": {
			Terms:   []string{"sales", "account executive", "business development", "bdr", "sdr"},
			Aliases: []string{"account executive", "sales development representative", "business development representative"},
			Tokens:  []string{"sales", "account executive", "business development", "bdr", "sdr"},
		},
		"customer-success": {
			Terms:   []string{"customer success", "customer support", "support engineer", "technical support"},
			Aliases: []string{"customer success manager", "support engineer", "technical support engineer"},
			Tokens:  []string{"support", "success", "customer"},
		},
	}
)

type DigestLookbackOption struct {
	Value string
	Label string
}

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
	if len(derived) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(derived)*4)
	result = append(result, derived...)

	for _, role := range derived {
		for _, family := range matchedRoleFamilies(role) {
			result = append(result, roleFamilies[family].Terms...)
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
		return []string{}, []string{}
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

func matchedRoleFamilies(role string) []string {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return nil
	}

	matches := make([]string, 0, 2)

	for _, family := range roleFamilyOrder {
		definition := roleFamilies[family]
		if roleMatchesFamily(role, definition) {
			matches = append(matches, family)
		}
	}

	if len(matches) == 0 {
		return []string{}
	}

	return normalizeStringList(matches)
}

func roleMatchesFamily(role string, definition roleFamilyDefinition) bool {
	for _, alias := range definition.Aliases {
		if role == strings.ToLower(strings.TrimSpace(alias)) {
			return true
		}
	}

	for _, token := range definition.Tokens {
		normalized := strings.TrimSpace(strings.ToLower(token))
		if normalized == "" {
			continue
		}
		if strings.Contains(role, normalized) {
			return true
		}
	}

	return false
}
