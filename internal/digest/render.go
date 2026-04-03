package digest

import (
	"fmt"
	"html"
	"strings"
)

func RenderMessage(recipient string, items []JobDigestItem) Message {
	subject := fmt.Sprintf("Opportunity Radar Daily Digest: %d job(s)", len(items))

	var text strings.Builder
	text.WriteString("Here are your top job matches for today:\n\n")

	var htmlBody strings.Builder
	htmlBody.WriteString("<html><body>")
	htmlBody.WriteString("<h1>Opportunity Radar Daily Digest</h1>")
	htmlBody.WriteString("<p>Here are your top job matches for today.</p>")
	htmlBody.WriteString("<ol>")

	for i, item := range items {
		companyName := item.CompanyName
		if companyName == "" {
			companyName = "Unknown company"
		}

		text.WriteString(fmt.Sprintf("%d. %s - %s\n", i+1, item.Title, companyName))
		if item.Location != "" {
			text.WriteString(fmt.Sprintf("   Location: %s\n", item.Location))
		}
		text.WriteString(fmt.Sprintf("   Score: %.0f\n", item.Score))
		text.WriteString(fmt.Sprintf("   Source: %s\n", item.Source))
		text.WriteString(fmt.Sprintf("   URL: %s\n\n", item.URL))

		htmlBody.WriteString("<li>")
		htmlBody.WriteString(fmt.Sprintf("<p><strong>%s</strong> - %s</p>",
			html.EscapeString(item.Title),
			html.EscapeString(companyName),
		))
		htmlBody.WriteString("<ul>")
		if item.Location != "" {
			htmlBody.WriteString(fmt.Sprintf("<li>Location: %s</li>", html.EscapeString(item.Location)))
		}
		htmlBody.WriteString(fmt.Sprintf("<li>Score: %.0f</li>", item.Score))
		htmlBody.WriteString(fmt.Sprintf("<li>Source: %s</li>", html.EscapeString(item.Source)))
		htmlBody.WriteString(fmt.Sprintf(`<li><a href="%s">Open job</a></li>`, html.EscapeString(item.URL)))
		htmlBody.WriteString("</ul>")
		htmlBody.WriteString("</li>")
	}

	htmlBody.WriteString("</ol>")
	htmlBody.WriteString("</body></html>")

	return Message{
		To:       recipient,
		Subject:  subject,
		TextBody: text.String(),
		HTMLBody: htmlBody.String(),
	}
}
