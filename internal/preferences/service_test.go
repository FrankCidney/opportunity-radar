package preferences

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

func TestServiceSaveNormalizesSettings(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := service.Save(context.Background(), &Settings{
		DesiredRoles:    []string{" Backend Engineer ", "backend engineer"},
		Locations:       []string{" Remote ", "remote"},
		WorkModes:       []string{" Remote "},
		ExperienceLevel: " Junior / early-career ",
		DigestRecipient: "  test@example.com ",
		DigestTopN:      0,
		DigestLookback:  0,
	})
	if err != nil {
		t.Fatalf("expected save to succeed: %v", err)
	}

	if repo.saved == nil {
		t.Fatalf("expected settings to be saved")
	}

	if got, want := repo.saved.DesiredRoles, []string{"backend engineer"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected desired roles: got %v want %v", got, want)
	}

	if got, want := repo.saved.RoleKeywords, []string{"backend engineer", "backend", "backend developer", "api", "server", "services", "microservices", "software engineer", "software developer", "developer", "engineer"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected role keywords: got %v want %v", got, want)
	}

	if repo.saved.DigestRecipient != "test@example.com" {
		t.Fatalf("unexpected digest recipient: %q", repo.saved.DigestRecipient)
	}

	if repo.saved.DigestTopN != 10 {
		t.Fatalf("unexpected digest top n: got %d want 10", repo.saved.DigestTopN)
	}

	if repo.saved.DigestLookback != 24*time.Hour {
		t.Fatalf("unexpected digest lookback: got %s want 24h", repo.saved.DigestLookback)
	}
}

func TestServiceEnsureBootstrapsWhenMissing(t *testing.T) {
	repo := &stubRepository{getErr: ErrNotFound}
	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	bootstrap := &Settings{
		DesiredRoles:    []string{"backend engineer"},
		Locations:       []string{"remote"},
		WorkModes:       []string{"remote"},
		ExperienceLevel: "Junior / early-career",
		DigestTopN:      5,
		DigestLookback:  12 * time.Hour,
	}

	settings, created, err := service.Ensure(context.Background(), bootstrap)
	if err != nil {
		t.Fatalf("expected ensure to succeed: %v", err)
	}
	if !created {
		t.Fatalf("expected settings to be created")
	}
	if settings == nil {
		t.Fatalf("expected settings result")
	}
	if got, want := settings.DesiredRoles, []string{"backend engineer"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected desired roles: got %v want %v", got, want)
	}
	if got, want := settings.RoleKeywords, []string{"backend engineer", "backend", "backend developer", "api", "server", "services", "microservices", "software engineer", "software developer", "developer", "engineer"}; !stringSlicesEqual(got, want) {
		t.Fatalf("unexpected role keywords: got %v want %v", got, want)
	}
}

func TestServiceSaveUsesEmptySlicesForUnsetDerivedFields(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

	err := service.Save(context.Background(), &Settings{
		ExperienceLevel: "Junior / early-career",
	})
	if err != nil {
		t.Fatalf("expected save to succeed: %v", err)
	}

	if repo.saved == nil {
		t.Fatalf("expected settings to be saved")
	}

	if repo.saved.RoleKeywords == nil {
		t.Fatalf("expected empty role keywords slice, got nil")
	}
	if repo.saved.MismatchKeywords == nil {
		t.Fatalf("expected empty mismatch keywords slice, got nil")
	}
	if len(repo.saved.RoleKeywords) != 0 {
		t.Fatalf("expected no role keywords, got %v", repo.saved.RoleKeywords)
	}
	if len(repo.saved.MismatchKeywords) != 0 {
		t.Fatalf("expected no mismatch keywords, got %v", repo.saved.MismatchKeywords)
	}
}

type stubRepository struct {
	getErr error
	saved  *Settings
}

func (r *stubRepository) Get(ctx context.Context) (*Settings, error) {
	if r.saved != nil {
		return r.saved, nil
	}
	if r.getErr != nil {
		return nil, r.getErr
	}
	return nil, ErrNotFound
}

func (r *stubRepository) Save(ctx context.Context, settings *Settings) error {
	if settings == nil {
		return errors.New("settings must not be nil")
	}
	cloned := *settings
	r.saved = &cloned
	r.getErr = nil
	return nil
}

func stringSlicesEqual(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
