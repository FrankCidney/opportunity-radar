package runcontrol

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

var ErrRunInProgress = errors.New("run already in progress")

type Runner interface {
	RunAll(ctx context.Context) error
}

type SummaryProvider interface {
	LastSummary() string
}

type Status struct {
	Running         bool
	LastStartedAt   time.Time
	LastCompletedAt time.Time
	LastState       string
	LastSummary     string
}

type Coordinator struct {
	runner Runner
	logger *slog.Logger

	runInProgress atomic.Bool

	mu     sync.RWMutex
	status Status
}

func New(runner Runner, logger *slog.Logger) *Coordinator {
	return &Coordinator{
		runner: runner,
		logger: logger,
		status: Status{LastState: "idle"},
	}
}

func (c *Coordinator) RunAll(ctx context.Context) error {
	if !c.runInProgress.CompareAndSwap(false, true) {
		return ErrRunInProgress
	}
	defer c.runInProgress.Store(false)

	startedAt := time.Now().UTC()
	c.mu.Lock()
	c.status.Running = true
	c.status.LastStartedAt = startedAt
	c.status.LastState = "running"
	c.status.LastSummary = "Run in progress."
	c.mu.Unlock()

	err := c.runner.RunAll(ctx)
	completedAt := time.Now().UTC()

	c.mu.Lock()
	defer c.mu.Unlock()

	c.status.Running = false
	c.status.LastCompletedAt = completedAt

	if err != nil {
		c.status.LastState = "failed"
		c.status.LastSummary = "Run failed."
		return err
	}

	c.status.LastState = "completed"
	c.status.LastSummary = "Run completed."
	if provider, ok := c.runner.(SummaryProvider); ok {
		if summary := provider.LastSummary(); summary != "" {
			c.status.LastSummary = summary
		}
	}

	return nil
}

func (c *Coordinator) RunNow(ctx context.Context) error {
	return c.RunAll(ctx)
}

func (c *Coordinator) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}
