package companies

import "time"

type Company struct {
	ID int64 `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
	LogoURL string `db:"logo_url" json:"logoUrl,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
	Source string `db:"source" json:"source"`
	ExternalID string `db:"external_id" json:"externalId,omitempty"`
	Domain string `db:"domain" json:"domain,omitempty"`
}
