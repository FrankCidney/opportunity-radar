package models

import "time"

type Company struct {
	ID int64 `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
	Website string `db:"website" json:"website,omitempty"`
	LogoURL string `db:"logo_url" json:"logUrl,omitempty"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}