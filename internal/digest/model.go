package digest

import "time"

type Delivery struct {
	ID         int64
	Recipient  string
	DigestDate time.Time
	JobCount   int
	Subject    string
	SentAt     time.Time
	CreatedAt  time.Time
}

type JobDigestItem struct {
	Title       string
	CompanyName string
	Location    string
	URL         string
	Source      string
	Score       float64
	PostedAt    time.Time
}

type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}
