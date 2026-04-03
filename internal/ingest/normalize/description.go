package normalize

import (
	"regexp"
	"strings"
	"unicode"

	"jaytaylor.com/html2text"
)

var bulletPrefixes = []string{
	"●",
	"•",
	"◦",
	"▪",
	"▫",
	"■",
	"□",
	"-",
	"–",
	"—",
}

var whitespacePattern = regexp.MustCompile(`[\t\f\v ]+`)

func normalizeRemotiveDescription(raw RawJob) string {
	text, err := html2text.FromString(raw.Description, html2text.Options{})
	if err != nil {
		text = raw.Description
	}

	text = cleanPlainTextDescription(text)

	var metadata []string
	if raw.JobType != "" {
		metadata = append(metadata, "Job type: "+humanizeUnderscoreText(raw.JobType))
	}
	if raw.Salary != "" {
		metadata = append(metadata, "Salary: "+strings.TrimSpace(raw.Salary))
	}

	if len(metadata) == 0 {
		return text
	}
	if text == "" {
		return strings.Join(metadata, "\n")
	}

	return strings.Join(metadata, "\n") + "\n\n" + text
}

func cleanPlainTextDescription(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	raw = strings.ReplaceAll(raw, "\u00a0", " ")

	lines := strings.Split(raw, "\n")
	cleaned := make([]string, 0, len(lines))
	lastWasBlank := true

	for _, line := range lines {
		normalized := normalizeDescriptionLine(line)
		if normalized == "" {
			if !lastWasBlank {
				cleaned = append(cleaned, "")
			}
			lastWasBlank = true
			continue
		}

		cleaned = append(cleaned, normalized)
		lastWasBlank = false
	}

	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func normalizeDescriptionLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}

	line = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) && r != '\n' {
			return ' '
		}
		return r
	}, line)
	line = whitespacePattern.ReplaceAllString(line, " ")

	if strings.HasPrefix(line, "* ") {
		content := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if content == "" {
			return ""
		}
		return "- " + content
	}

	line = stripWrappedAsterisks(line)

	for _, prefix := range bulletPrefixes {
		if strings.HasPrefix(line, prefix) {
			content := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if content == "" {
				return ""
			}
			return "- " + content
		}
	}

	return line
}

func stripWrappedAsterisks(line string) string {
	if len(line) < 2 {
		return line
	}

	if strings.HasPrefix(line, "*") && strings.HasSuffix(line, "*") {
		inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "*"), "*"))
		if inner != "" && !strings.Contains(inner, "*") {
			return inner
		}
	}

	return line
}

func humanizeUnderscoreText(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "_", " "))
	if value == "" {
		return ""
	}

	parts := strings.Fields(strings.ToLower(value))
	for i, part := range parts {
		runes := []rune(part)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}

	return strings.Join(parts, " ")
}
