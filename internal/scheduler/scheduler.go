package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"opportunity-radar/internal/runcontrol"
)

type Runner interface {
	RunAll(ctx context.Context) error
}

type Config struct {
	Interval   time.Duration
	RunOnStart bool
	RunTimeout time.Duration
}

type Scheduler struct {
	runner        Runner
	logger        *slog.Logger
	interval      time.Duration
	runOnStart    bool
	runTimeout    time.Duration
	runInProgress atomic.Bool
}

func New(runner Runner, cfg Config, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		runner:     runner,
		logger:     logger,
		interval:   cfg.Interval,
		runOnStart: cfg.RunOnStart,
		runTimeout: cfg.RunTimeout,
	}
}

func (s *Scheduler) Run(ctx context.Context) error {
	if s.interval <= 0 {
		return fmt.Errorf("scheduler interval must be greater than zero")
	}

	s.logger.Info("scheduler starting",
		"interval", s.interval,
		"run_on_start", s.runOnStart,
		"run_timeout", s.runTimeout,
	)

	if s.runOnStart {
		s.runOnce(ctx, "startup")
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopping", "reason", ctx.Err())
			return nil
		case <-ticker.C:
			s.runOnce(ctx, "ticker")
		}
	}
}

func (s *Scheduler) runOnce(parentCtx context.Context, trigger string) {
	if !s.runInProgress.CompareAndSwap(false, true) {
		s.logger.Warn("scheduler tick skipped; run already in progress", "trigger", trigger)
		return
	}
	defer s.runInProgress.Store(false)

	runCtx := parentCtx
	cancel := func() {}
	if s.runTimeout > 0 {
		runCtx, cancel = context.WithTimeout(parentCtx, s.runTimeout)
	}
	defer cancel()

	startedAt := time.Now()
	s.logger.Info("scheduler run starting", "trigger", trigger, "started_at", startedAt.UTC())

	if err := s.runner.RunAll(runCtx); err != nil {
		if errors.Is(err, runcontrol.ErrRunInProgress) {
			s.logger.Warn("scheduler tick skipped; run already in progress", "trigger", trigger)
			return
		}
		s.logger.Error("scheduler run failed",
			"trigger", trigger,
			"duration", time.Since(startedAt),
			"error", err,
		)
		return
	}

	s.logger.Info("scheduler run complete",
		"trigger", trigger,
		"duration", time.Since(startedAt),
	)
}
