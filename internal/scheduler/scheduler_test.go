package scheduler

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"
)

func TestRunExecutesImmediatelyWhenRunOnStart(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{}
	scheduler := New(runner, Config{
		Interval:   time.Hour,
		RunOnStart: true,
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := scheduler.Run(ctx); err != nil {
			t.Errorf("Run() error = %v", err)
		}
	}()

	waitForRuns(t, runner, 1, time.Second)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
}

func TestRunExecutesOnTicker(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{}
	scheduler := New(runner, Config{
		Interval:   25 * time.Millisecond,
		RunOnStart: false,
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := scheduler.Run(ctx); err != nil {
			t.Errorf("Run() error = %v", err)
		}
	}()

	waitForRuns(t, runner, 1, time.Second)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
}

func TestRunSkipsOverlappingTicks(t *testing.T) {
	t.Parallel()

	block := make(chan struct{})
	runner := &stubRunner{
		runStarted: make(chan struct{}, 1),
		runFunc: func(ctx context.Context) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-block:
				return nil
			}
		},
	}

	scheduler := New(runner, Config{
		Interval:   20 * time.Millisecond,
		RunOnStart: true,
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		defer close(done)
		if err := scheduler.Run(ctx); err != nil {
			t.Errorf("Run() error = %v", err)
		}
	}()

	select {
	case <-runner.runStarted:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not start the first run")
	}

	time.Sleep(90 * time.Millisecond)
	if got := runner.Count(); got != 1 {
		t.Fatalf("expected exactly one in-flight run, got %d", got)
	}

	close(block)
	waitForRuns(t, runner, 1, time.Second)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("scheduler did not stop after context cancellation")
	}
}

func TestRunAppliesRunTimeout(t *testing.T) {
	t.Parallel()

	runner := &stubRunner{
		runFunc: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	scheduler := New(runner, Config{
		Interval:   time.Hour,
		RunOnStart: true,
		RunTimeout: 30 * time.Millisecond,
	}, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	err := runSchedulerForSingleStartupRun(ctx, scheduler)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("runSchedulerForSingleStartupRun() error = %v", err)
	}

	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected timeout-bound run to finish quickly, took %v", elapsed)
	}
}

func runSchedulerForSingleStartupRun(ctx context.Context, scheduler *Scheduler) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- scheduler.Run(runCtx)
	}()

	time.Sleep(80 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		return err
	case <-time.After(time.Second):
		return context.DeadlineExceeded
	}
}

func waitForRuns(t *testing.T, runner *stubRunner, want int, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if runner.Count() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %d runs; got %d", want, runner.Count())
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubRunner struct {
	mu         sync.Mutex
	count      int
	runFunc    func(ctx context.Context) error
	runStarted chan struct{}
}

func (r *stubRunner) RunAll(ctx context.Context) error {
	r.mu.Lock()
	r.count++
	runFunc := r.runFunc
	runStarted := r.runStarted
	r.mu.Unlock()

	if runStarted != nil {
		select {
		case runStarted <- struct{}{}:
		default:
		}
	}

	if runFunc != nil {
		return runFunc(ctx)
	}

	return nil
}

func (r *stubRunner) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}
