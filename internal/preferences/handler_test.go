package preferences

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"opportunity-radar/internal/digest"
	"opportunity-radar/internal/scoring"
)

func TestSetupPostSavesSettingsAndUpdatesScorer(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			RoleKeywords:           []string{"backend"},
			SkillKeywords:          []string{"go"},
			PreferredLevelKeywords: []string{"junior"},
			DigestTopN:             10,
			DigestLookback:         24 * time.Hour,
		},
	}
	scorer := &stubScorerUpdater{}
	digestService := &stubDigestUpdater{}
	handler := NewHandler(service, scorer, digestService, true, slog.New(slog.NewTextHandler(io.Discard, nil)))

	form := url.Values{}
	form.Set("role_keywords", "backend\napi")
	form.Set("skill_keywords", "go\ngolang")
	form.Set("preferred_level_keywords", "junior")
	form.Set("penalty_level_keywords", "senior")
	form.Set("preferred_location_terms", "remote")
	form.Set("penalty_location_terms", "onsite")
	form.Set("mismatch_keywords", "sales")

	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if service.settings == nil || !service.settings.SetupComplete {
		t.Fatalf("expected setup to be marked complete")
	}
	if len(scorer.profile.RoleKeywords) != 2 {
		t.Fatalf("expected scorer profile to be updated")
	}
}

func TestDigestSettingsPageShowsWarnings(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DigestEnabled:   true,
			DigestRecipient: "",
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, false, slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/settings/digest", nil)
	rec := httptest.NewRecorder()

	handler.DigestSettings(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Digest is enabled, but no recipient email is set.") {
		t.Fatalf("expected missing recipient warning")
	}
	if !strings.Contains(body, "Digests will be logged instead of emailed.") {
		t.Fatalf("expected resend warning")
	}
}

func TestDigestPostSavesSettingsAndUpdatesService(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DigestEnabled:   false,
			DigestRecipient: "",
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	digestService := &stubDigestUpdater{}
	handler := NewHandler(service, &stubScorerUpdater{}, digestService, true, slog.New(slog.NewTextHandler(io.Discard, nil)))

	form := url.Values{}
	form.Set("digest_enabled", "on")
	form.Set("digest_recipient", "user@example.com")
	form.Set("digest_top_n", "15")
	form.Set("digest_lookback", "48h")

	req := httptest.NewRequest(http.MethodPost, "/settings/digest", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.DigestSettings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if got := digestService.config.Recipient; got != "user@example.com" {
		t.Fatalf("unexpected digest recipient: got %q", got)
	}
	if got := digestService.config.TopN; got != 15 {
		t.Fatalf("unexpected digest top n: got %d", got)
	}
}

type handlerStubService struct {
	settings *Settings
}

func (s *handlerStubService) Get(ctx context.Context) (*Settings, error) {
	if s.settings == nil {
		return nil, ErrSettingsNotFound
	}
	cloned := *s.settings
	return &cloned, nil
}

func (s *handlerStubService) Save(ctx context.Context, settings *Settings) error {
	cloned := *settings
	s.settings = &cloned
	return nil
}

type stubScorerUpdater struct {
	profile scoring.Profile
}

func (s *stubScorerUpdater) SetProfile(profile scoring.Profile) {
	s.profile = profile
}

type stubDigestUpdater struct {
	config digest.Config
}

func (s *stubDigestUpdater) UpdateConfig(config digest.Config) {
	s.config = config
}

func (s *stubDigestUpdater) CurrentConfig() digest.Config {
	return s.config
}
