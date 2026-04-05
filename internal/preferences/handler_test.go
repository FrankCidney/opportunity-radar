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
	"opportunity-radar/internal/runcontrol"
	"opportunity-radar/internal/scoring"
)

func TestSetupPostSavesSettingsAndUpdatesScorer(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			DigestTopN:     10,
			DigestLookback: 24 * time.Hour,
		},
	}
	scorer := &stubScorerUpdater{}
	digestService := &stubDigestUpdater{}
	handler := NewHandler(service, scorer, digestService, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	form := url.Values{}
	form.Set("roles", "backend engineer\nsoftware engineer")
	form.Set("experience_level", "Junior / early-career")
	form.Add("locations", "Remote")
	form.Add("work_modes", "Remote")
	form.Set("current_skills", "go\ngolang")
	form.Set("avoid", "sales")
	form.Set("email_destination", "me@example.com")

	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want 303", rec.Code)
	}
	if service.settings == nil || !service.settings.SetupComplete {
		t.Fatalf("expected setup to be marked complete")
	}
	if len(scorer.profile.RoleKeywords) == 0 {
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
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, &stubRunController{}, false, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/settings/digest", nil)
	rec := httptest.NewRecorder()

	handler.DigestSettings(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Email updates are enabled, but no destination email is set yet.") {
		t.Fatalf("expected missing recipient warning")
	}
	if !strings.Contains(body, "Updates will be logged instead of emailed.") {
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
	handler := NewHandler(service, &stubScorerUpdater{}, digestService, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	form := url.Values{}
	form.Set("email_updates_enabled", "on")
	form.Set("email_destination", "user@example.com")
	form.Set("email_top_n", "15")
	form.Set("email_lookback", "48h")

	req := httptest.NewRequest(http.MethodPost, "/settings/digest", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.DigestSettings(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want 303", rec.Code)
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

type stubRunController struct{}

func (s *stubRunController) RunNow(ctx context.Context) error {
	return nil
}

func (s *stubRunController) Status() runcontrol.Status {
	return runcontrol.Status{}
}
