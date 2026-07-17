package main

import (
	"context"
	"strings"
	"testing"

	"opportunity-radar/internal/digest"
)

func TestAccountEmailNotifierBuildsEscapedPublicLinks(t *testing.T) {
	t.Parallel()

	sender := &capturingDigestSender{}
	notifier := accountEmailNotifier{
		sender:        sender,
		publicBaseURL: "https://radar.example.com/",
	}
	if err := notifier.SendVerification(
		context.Background(),
		"person@example.com",
		"token/with+reserved?",
	); err != nil {
		t.Fatalf("SendVerification() error = %v", err)
	}

	wantLink := "https://radar.example.com/verify-email?token=token%2Fwith%2Breserved%3F"
	if !strings.Contains(sender.message.TextBody, wantLink) {
		t.Fatalf("text body = %q, want escaped link %q", sender.message.TextBody, wantLink)
	}
	if sender.message.To != "person@example.com" {
		t.Fatalf("recipient = %q", sender.message.To)
	}
}

type capturingDigestSender struct {
	message digest.Message
}

func (s *capturingDigestSender) Send(_ context.Context, message digest.Message) error {
	s.message = message
	return nil
}
