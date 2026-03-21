package scoring

import "opportunity-radar/internal/jobs"

type Scorer interface {
	Score(job *jobs.Job) float64
}