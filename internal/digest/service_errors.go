package digest

import "errors"

var (
	ErrDigestAlreadySent = errors.New("digest already sent")
	ErrServiceInternal   = errors.New("digest service internal error")
)
