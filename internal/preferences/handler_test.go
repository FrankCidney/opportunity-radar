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

func TestSetupPostUsesNonCompleteFlashWhenRecommendedFieldsAreMissing(t *testing.T) {
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
	form.Set("roles", "backend engineer")
	form.Set("experience_level", "Junior / early-career")
	form.Add("locations", "Remote")
	form.Add("work_modes", "Remote")

	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("unexpected status: got %d want 303", rec.Code)
	}

	location := rec.Header().Get("Location")
	if location != "/settings/profile" {
		t.Fatalf("unexpected redirect location: %q", location)
	}
}

func TestSetupPostAllowsPartialSaveAndKeepsSetupIncomplete(t *testing.T) {
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
	form.Set("experience_level", "Junior / early-career")
	form.Set("email_destination", "me@example.com")

	req := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.Setup(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if service.settings == nil {
		t.Fatalf("expected settings to be saved")
	}
	if service.settings.SetupComplete {
		t.Fatalf("expected setup to remain incomplete")
	}
	if got := service.settings.ExperienceLevel; got != "Junior / early-career" {
		t.Fatalf("unexpected experience level: got %q", got)
	}
	if got := service.settings.RoleKeywords; len(got) != 0 {
		t.Fatalf("expected empty role keywords for partial save, got %v", got)
	}
	if !strings.Contains(rec.Body.String(), "Setup saved, but some required fields are still missing.") {
		t.Fatalf("expected partial setup message in response body")
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

func TestHomeShowsSetupReminderWhenRecommendedFieldsAreMissing(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DesiredRoles:    []string{"backend engineer"},
			ExperienceLevel: "Junior / early-career",
			Locations:       []string{"remote"},
			WorkModes:       []string{"remote"},
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Home(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if !strings.Contains(body, "Setup is almost done") {
		t.Fatalf("expected setup reminder banner")
	}
	if !strings.Contains(body, "Complete Setup") {
		t.Fatalf("expected complete setup action")
	}
	if !strings.Contains(body, "Still missing: current skills, growth skills, avoid terms, email destination") {
		t.Fatalf("expected missing recommended fields summary")
	}
	if !strings.Contains(body, "<strong>Incomplete</strong>") {
		t.Fatalf("expected setup status card to show incomplete")
	}
}

func TestHomeHidesSetupReminderWhenRecommendedFieldsAreFilled(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DesiredRoles:    []string{"backend engineer"},
			ExperienceLevel: "Junior / early-career",
			CurrentSkills:   []string{"go"},
			GrowthSkills:    []string{"python"},
			Locations:       []string{"remote"},
			WorkModes:       []string{"remote"},
			AvoidTerms:      []string{"sales"},
			DigestRecipient: "me@example.com",
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Home(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if strings.Contains(body, "Setup is almost done") {
		t.Fatalf("did not expect setup reminder banner")
	}
}

func TestProfileShowsIncompleteMessagingWhenRecommendedFieldsAreMissing(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DesiredRoles:    []string{"backend engineer"},
			ExperienceLevel: "Junior / early-career",
			Locations:       []string{"remote"},
			WorkModes:       []string{"remote"},
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/settings/profile", nil)
	rec := httptest.NewRecorder()

	handler.ProfileSettings(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status: got %d want 200", rec.Code)
	}
	if !strings.Contains(body, "Setup is almost done") {
		t.Fatalf("expected profile page to show reminder-aware heading")
	}
	if !strings.Contains(body, "Complete Setup") {
		t.Fatalf("expected complete setup action on profile page")
	}
	if strings.Contains(body, "Setup is complete") {
		t.Fatalf("did not expect fully complete messaging while recommended fields are missing")
	}
}

func TestHomeFormatsLookbackWindowWithoutMinutesAndSeconds(t *testing.T) {
	service := &handlerStubService{
		settings: &Settings{
			SetupComplete:   true,
			DesiredRoles:    []string{"backend engineer"},
			ExperienceLevel: "Junior / early-career",
			CurrentSkills:   []string{"go"},
			GrowthSkills:    []string{"python"},
			Locations:       []string{"remote"},
			WorkModes:       []string{"remote"},
			AvoidTerms:      []string{"sales"},
			DigestRecipient: "me@example.com",
			DigestTopN:      10,
			DigestLookback:  24 * time.Hour,
		},
	}
	handler := NewHandler(service, &stubScorerUpdater{}, &stubDigestUpdater{}, &stubRunController{}, true, true, "Every 24h", slog.New(slog.NewTextHandler(io.Discard, nil)))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.Home(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "<dd>24h</dd>") {
		t.Fatalf("expected formatted 24h lookback, got body %q", body)
	}
	if strings.Contains(body, "24h0m0s") {
		t.Fatalf("did not expect raw duration string")
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
