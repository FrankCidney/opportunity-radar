package main

import (
	"context"
	"errors"
	"testing"
)

func TestCombinedRunEligibilityRequiresSetupAndVerifiedLegacyOwner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		setup  bool
		legacy bool
		want   bool
	}{
		{name: "both ready", setup: true, legacy: true, want: true},
		{name: "setup incomplete", setup: false, legacy: true, want: false},
		{name: "legacy owner unavailable", setup: true, legacy: false, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			checker := combinedRunEligibility{
				setup:  stubEligibility{allowed: tt.setup},
				legacy: stubLegacyReadiness{ready: tt.legacy},
			}
			got, err := checker.CanRun(context.Background())
			if err != nil {
				t.Fatalf("CanRun() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("CanRun() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCombinedRunEligibilityStopsOnSetupError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	legacy := &countingLegacyReadiness{ready: true}
	checker := combinedRunEligibility{
		setup:  stubEligibility{err: wantErr},
		legacy: legacy,
	}

	_, err := checker.CanRun(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("CanRun() error = %v, want %v", err, wantErr)
	}
	if legacy.calls != 0 {
		t.Fatal("legacy readiness was queried after setup error")
	}
}

type stubEligibility struct {
	allowed bool
	err     error
}

func (s stubEligibility) CanRun(context.Context) (bool, error) {
	return s.allowed, s.err
}

type stubLegacyReadiness struct {
	ready bool
}

func (s stubLegacyReadiness) LegacyTenantReady(context.Context) (bool, error) {
	return s.ready, nil
}

type countingLegacyReadiness struct {
	ready bool
	calls int
}

func (s *countingLegacyReadiness) LegacyTenantReady(context.Context) (bool, error) {
	s.calls++
	return s.ready, nil
}
