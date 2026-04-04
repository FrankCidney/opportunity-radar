package preferences

import "errors"

var (
	ErrSettingsNotFound = errors.New("settings not found")
	ErrServiceInternal  = errors.New("preferences service internal")
)
