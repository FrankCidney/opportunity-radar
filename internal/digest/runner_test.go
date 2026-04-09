package digest

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestRunnerSkipsWhenSetupIsIncomplete(t *testing.T) {
	t.Parallel()

	ingestRunner := &stubIngestRunner{}
	runner := NewRunner(ingestRunner, stubEligibilityChecker{canRun: false}, nil, testRunnerLogger())

	if err := runner.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll() error = %v", err)
	}

	if ingestRunner.called {
		t.Fatal("expected ingest runner not to be called")
	}
	if got := runner.LastSummary(); got != "Run skipped because setup is incomplete." {
		t.Fatalf("unexpected summary: got %q", got)
	}
}

func TestRunnerRunsWhenSetupIsComplete(t *testing.T) {
	t.Parallel()

	ingestRunner := &stubIngestRunner{}
	runner := NewRunner(ingestRunner, stubEligibilityChecker{canRun: true}, nil, testRunnerLogger())

	if err := runner.RunAll(context.Background()); err != nil {
		t.Fatalf("RunAll() error = %v", err)
	}

	if !ingestRunner.called {
		t.Fatal("expected ingest runner to be called")
	}
	if got := runner.LastSummary(); got != "Run completed." {
		t.Fatalf("unexpected summary: got %q", got)
	}
}

func TestRunnerReturnsEligibilityError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("boom")
	ingestRunner := &stubIngestRunner{}
	runner := NewRunner(ingestRunner, stubEligibilityChecker{err: wantErr}, nil, testRunnerLogger())

	err := runner.RunAll(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if ingestRunner.called {
		t.Fatal("expected ingest runner not to be called")
	}
	if got := runner.LastSummary(); got != "Run failed before ingest." {
		t.Fatalf("unexpected summary: got %q", got)
	}
}

func testRunnerLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type stubIngestRunner struct {
	called bool
}

func (r *stubIngestRunner) RunAll(_ context.Context) error {
	r.called = true
	return nil
}

type stubEligibilityChecker struct {
	canRun bool
	err    error
}

func (c stubEligibilityChecker) CanRun(_ context.Context) (bool, error) {
	if c.err != nil {
		return false, c.err
	}
	return c.canRun, nil
}
