package main

import (
	"context"
	"fmt"
	"html"
	"net/url"
	"strings"

	"opportunity-radar/internal/digest"
)

type accountEmailNotifier struct {
	sender        digest.Sender
	publicBaseURL string
}

func (n accountEmailNotifier) SendVerification(
	ctx context.Context,
	email string,
	rawToken string,
) error {
	link := n.link("/verify-email", rawToken)
	return n.sender.Send(ctx, digest.Message{
		To:       email,
		Subject:  "Verify your Opportunity Radar email",
		TextBody: "Confirm your email address:\n\n" + link + "\n\nIf you did not create this account, you can ignore this message.",
		HTMLBody: fmt.Sprintf(
			`<p>Confirm your email address for Opportunity Radar.</p><p><a href="%s">Confirm email</a></p><p>If you did not create this account, you can ignore this message.</p>`,
			html.EscapeString(link),
		),
	})
}

func (n accountEmailNotifier) SendPasswordReset(
	ctx context.Context,
	email string,
	rawToken string,
) error {
	link := n.link("/reset-password", rawToken)
	return n.sender.Send(ctx, digest.Message{
		To:       email,
		Subject:  "Reset your Opportunity Radar password",
		TextBody: "Reset your password:\n\n" + link + "\n\nIf you did not request this change, you can ignore this message.",
		HTMLBody: fmt.Sprintf(
			`<p>A password reset was requested for your Opportunity Radar account.</p><p><a href="%s">Choose a new password</a></p><p>If you did not request this change, you can ignore this message.</p>`,
			html.EscapeString(link),
		),
	})
}

func (n accountEmailNotifier) link(path, rawToken string) string {
	return strings.TrimRight(n.publicBaseURL, "/") +
		path + "?token=" + url.QueryEscape(rawToken)
}
