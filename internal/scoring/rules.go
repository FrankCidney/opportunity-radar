package scoring

import (
	"opportunity-radar/internal/jobs"
	"strings"
)

type RulesScorer struct {
	keywords []string
}

func NewRulesScorer(keywords []string) *RulesScorer {
	return &RulesScorer{keywords: keywords}
}

// TODO: Review scoring logic. Maybe find a better way to score.
func (s *RulesScorer) Score(job *jobs.Job) float64 {
	score := 0
	text := strings.ToLower(job.Title + " " + job.Description)
	for _, kw := range s.keywords {
		if strings.Contains(text, kw) {
			score += 10
		}
	}

	return float64(score)
}