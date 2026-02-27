package jobs

import "time"

type JobStatus string

const (
	StatusActive   JobStatus = "active"
	StatusArchived JobStatus = "archived"
)

type Job struct {
	ID                  int64      `db:"id" json:"id"`
	CompanyID           int64      `db:"company_id" json:"companyId"`
	Title               string     `db:"title" json:"title"`
	Description         string     `db:"description" json:"description"`
	Location            string     `db:"location" json:"location"`
	URL                 string     `db:"url" json:"url"`
	Source              string     `db:"source" json:"source"`
	PostedAt            time.Time  `db:"posted_at" json:"postedAt"`
	ApplicationDeadline *time.Time `db:"application_deadline" json:"applicationDeadline"`
	Score               float64    `db:"score" json:"score"`
	Status              JobStatus  `db:"status" json:"status"`
	CreatedAt           time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updatedAt"`
}
