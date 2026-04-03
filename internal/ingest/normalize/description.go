package normalize

import (
	"strings"

	"jaytaylor.com/html2text"
)

func normalizeRemotiveDescription(raw string) string {
	text, err := html2text.FromString(raw, html2text.Options{})
	if err != nil {
		return strings.TrimSpace(raw)
	}

	return strings.TrimSpace(text)
}
