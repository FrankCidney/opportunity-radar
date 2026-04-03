package digest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestResendSenderSendBuildsExpectedRequest(t *testing.T) {
	t.Parallel()

	var gotAuthorization string
	var gotContentType string
	var gotMethod string
	var gotURL string
	var gotPayload resendSendEmailRequest

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			gotAuthorization = r.Header.Get("Authorization")
			gotContentType = r.Header.Get("Content-Type")
			gotMethod = r.Method
			gotURL = r.URL.String()

			body, err := io.ReadAll(r.Body)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(body, &gotPayload); err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"id":"email_123"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	sender := NewResendSender("test-api-key", "digest@example.com", "Opportunity Radar", client, testSenderLogger())
	sender.apiBaseURL = "https://api.resend.test"

	err := sender.Send(context.Background(), Message{
		To:       "me@example.com",
		Subject:  "Top jobs today",
		TextBody: "plain body",
		HTMLBody: "<p>html body</p>",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %s", gotMethod)
	}
	if gotURL != "https://api.resend.test/emails" {
		t.Fatalf("expected resend url, got %s", gotURL)
	}
	if gotAuthorization != "Bearer test-api-key" {
		t.Fatalf("unexpected authorization header %q", gotAuthorization)
	}
	if gotContentType != "application/json" {
		t.Fatalf("unexpected content type %q", gotContentType)
	}
	if gotPayload.From != "Opportunity Radar <digest@example.com>" {
		t.Fatalf("unexpected from value %q", gotPayload.From)
	}
	if len(gotPayload.To) != 1 || gotPayload.To[0] != "me@example.com" {
		t.Fatalf("unexpected to payload %#v", gotPayload.To)
	}
	if gotPayload.Subject != "Top jobs today" {
		t.Fatalf("unexpected subject %q", gotPayload.Subject)
	}
	if gotPayload.Text != "plain body" {
		t.Fatalf("unexpected text payload %q", gotPayload.Text)
	}
	if gotPayload.HTML != "<p>html body</p>" {
		t.Fatalf("unexpected html payload %q", gotPayload.HTML)
	}
}

func TestResendSenderSendReturnsErrorOnNonSuccessStatus(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Body:       io.NopCloser(strings.NewReader("bad request")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	sender := NewResendSender("test-api-key", "digest@example.com", "", client, testSenderLogger())
	sender.apiBaseURL = "https://api.resend.test"

	err := sender.Send(context.Background(), Message{
		To:      "me@example.com",
		Subject: "Top jobs today",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func testSenderLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := f(req)
	if err != nil {
		return nil, fmt.Errorf("round trip failed: %w", err)
	}
	return resp, nil
}
