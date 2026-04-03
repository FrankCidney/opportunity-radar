package digest

import (
	"context"
	"log/slog"
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
