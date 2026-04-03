package digest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type Sender interface {
	Send(ctx context.Context, message Message) error
}

type LoggingSender struct {
	logger *slog.Logger
}

func NewLoggingSender(logger *slog.Logger) *LoggingSender {
	return &LoggingSender{logger: logger}
}

func (s *LoggingSender) Send(_ context.Context, message Message) error {
	preview := message.TextBody
	if len(preview) > 240 {
		preview = preview[:240] + "..."
	}

	s.logger.Info("digest email prepared",
		"to", message.To,
		"subject", message.Subject,
		"preview", preview,
	)

	return nil
}

type ResendSender struct {
	apiKey     string
	fromEmail  string
	fromName   string
	apiBaseURL string
	client     *http.Client
	logger     *slog.Logger
}

func NewResendSender(apiKey, fromEmail, fromName string, client *http.Client, logger *slog.Logger) *ResendSender {
	if client == nil {
		client = http.DefaultClient
	}

	return &ResendSender{
		apiKey:     strings.TrimSpace(apiKey),
		fromEmail:  strings.TrimSpace(fromEmail),
		fromName:   strings.TrimSpace(fromName),
		apiBaseURL: "https://api.resend.com",
		client:     client,
		logger:     logger,
	}
}

func (s *ResendSender) Send(ctx context.Context, message Message) error {
	if s.apiKey == "" {
		return fmt.Errorf("resend api key is required")
	}
	if s.fromEmail == "" {
		return fmt.Errorf("resend from email is required")
	}
	if strings.TrimSpace(message.To) == "" {
		return fmt.Errorf("message recipient is required")
	}

	payload := resendSendEmailRequest{
		From:    s.formattedFrom(),
		To:      []string{message.To},
		Subject: message.Subject,
		Text:    message.TextBody,
		HTML:    message.HTMLBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling resend email request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.apiBaseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building resend email request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending resend email: %w", err)
	}
	defer resp.Body.Close()

	respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if readErr != nil {
		return fmt.Errorf("reading resend response: %w", readErr)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		s.logger.Error("resend send failed",
			"status_code", resp.StatusCode,
			"response", string(respBody),
		)
		return fmt.Errorf("sending resend email: unexpected status %d", resp.StatusCode)
	}

	s.logger.Info("digest email sent through resend",
		"to", message.To,
		"subject", message.Subject,
	)

	return nil
}

func (s *ResendSender) formattedFrom() string {
	if s.fromName == "" {
		return s.fromEmail
	}
	return fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
}

type resendSendEmailRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text,omitempty"`
	HTML    string   `json:"html,omitempty"`
}
