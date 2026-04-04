package preferences

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"opportunity-radar/internal/digest"
	"opportunity-radar/internal/scoring"
)

type SettingsGetterSaver interface {
	Get(ctx context.Context) (*Settings, error)
	Save(ctx context.Context, settings *Settings) error
}

type ScoringProfileUpdater interface {
	SetProfile(profile scoring.Profile)
}

type DigestConfigUpdater interface {
	UpdateConfig(config digest.Config)
	CurrentConfig() digest.Config
}

type Handler struct {
	service          SettingsGetterSaver
	scorer           ScoringProfileUpdater
	digestService    DigestConfigUpdater
	logger           *slog.Logger
	resendConfigured bool
}

func NewHandler(
	service SettingsGetterSaver,
	scorer ScoringProfileUpdater,
	digestService DigestConfigUpdater,
	resendConfigured bool,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		service:          service,
		scorer:           scorer,
		digestService:    digestService,
		logger:           logger,
		resendConfigured: resendConfigured,
	}
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to load settings", err)
		return
	}

	page := statusPageData{
		Title:            "Opportunity Radar Settings",
		SetupComplete:    settings.SetupComplete,
		DigestConfig:     h.digestService.CurrentConfig(),
		DigestWarnings:   digestWarnings(settings, h.resendConfigured),
		ResendConfigured: h.resendConfigured,
	}

	h.render(w, homeTemplate, page)
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderProfileForm(w, r, "/setup", "Initial Setup", "")
	case http.MethodPost:
		h.handleProfileSave(w, r, true)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ProfileSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderProfileForm(w, r, "/settings/profile", "Profile Settings", "")
	case http.MethodPost:
		h.handleProfileSave(w, r, false)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) DigestSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderDigestForm(w, r, "")
	case http.MethodPost:
		h.handleDigestSave(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) renderProfileForm(w http.ResponseWriter, r *http.Request, action string, heading string, flash string) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to load profile settings", err)
		return
	}

	h.render(w, profileTemplate, profilePageData{
		Title:              heading,
		Heading:            heading,
		Action:             action,
		Flash:              flash,
		SetupComplete:      settings.SetupComplete,
		RoleKeywords:       strings.Join(settings.RoleKeywords, "\n"),
		SkillKeywords:      strings.Join(settings.SkillKeywords, "\n"),
		PreferredLevels:    strings.Join(settings.PreferredLevelKeywords, "\n"),
		PenaltyLevels:      strings.Join(settings.PenaltyLevelKeywords, "\n"),
		PreferredLocations: strings.Join(settings.PreferredLocationTerms, "\n"),
		PenaltyLocations:   strings.Join(settings.PenaltyLocationTerms, "\n"),
		MismatchKeywords:   strings.Join(settings.MismatchKeywords, "\n"),
	})
}

func (h *Handler) renderDigestForm(w http.ResponseWriter, r *http.Request, flash string) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to load digest settings", err)
		return
	}

	h.render(w, digestTemplate, digestPageData{
		Title:            "Digest Settings",
		Flash:            flash,
		SetupComplete:    settings.SetupComplete,
		DigestEnabled:    settings.DigestEnabled,
		DigestRecipient:  settings.DigestRecipient,
		DigestTopN:       settings.DigestTopN,
		DigestLookback:   settings.DigestLookback.String(),
		Warnings:         digestWarnings(settings, h.resendConfigured),
		ResendConfigured: h.resendConfigured,
	})
}

func (h *Handler) handleProfileSave(w http.ResponseWriter, r *http.Request, markSetupComplete bool) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to load profile settings", err)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid form submission", err)
		return
	}

	settings.RoleKeywords = parseList(r.FormValue("role_keywords"))
	settings.SkillKeywords = parseList(r.FormValue("skill_keywords"))
	settings.PreferredLevelKeywords = parseList(r.FormValue("preferred_level_keywords"))
	settings.PenaltyLevelKeywords = parseList(r.FormValue("penalty_level_keywords"))
	settings.PreferredLocationTerms = parseList(r.FormValue("preferred_location_terms"))
	settings.PenaltyLocationTerms = parseList(r.FormValue("penalty_location_terms"))
	settings.MismatchKeywords = parseList(r.FormValue("mismatch_keywords"))
	if markSetupComplete {
		settings.SetupComplete = true
	}

	if err := h.service.Save(r.Context(), settings); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to save profile settings", err)
		return
	}

	h.scorer.SetProfile(scoring.Profile{
		RoleKeywords:           settings.RoleKeywords,
		SkillKeywords:          settings.SkillKeywords,
		PreferredLevelKeywords: settings.PreferredLevelKeywords,
		PenaltyLevelKeywords:   settings.PenaltyLevelKeywords,
		PreferredLocationTerms: settings.PreferredLocationTerms,
		PenaltyLocationTerms:   settings.PenaltyLocationTerms,
		MismatchKeywords:       settings.MismatchKeywords,
	})

	message := "Profile settings saved."
	if markSetupComplete {
		message = "Setup complete. Profile settings saved."
	}

	h.renderProfileForm(w, r, pathForProfile(markSetupComplete), pageHeading(markSetupComplete), message)
}

func (h *Handler) handleDigestSave(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to load digest settings", err)
		return
	}

	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid form submission", err)
		return
	}

	topN, err := strconv.Atoi(strings.TrimSpace(r.FormValue("digest_top_n")))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Digest Top N must be a valid integer", err)
		return
	}

	lookback, err := time.ParseDuration(strings.TrimSpace(r.FormValue("digest_lookback")))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "Digest lookback must be a valid duration", err)
		return
	}

	settings.DigestEnabled = r.FormValue("digest_enabled") == "on"
	settings.DigestRecipient = strings.TrimSpace(r.FormValue("digest_recipient"))
	settings.DigestTopN = topN
	settings.DigestLookback = lookback

	if err := h.service.Save(r.Context(), settings); err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to save digest settings", err)
		return
	}

	h.digestService.UpdateConfig(digest.Config{
		Enabled:   settings.DigestEnabled,
		Recipient: settings.DigestRecipient,
		TopN:      settings.DigestTopN,
		Lookback:  settings.DigestLookback,
	})

	h.renderDigestForm(w, r, "Digest settings saved.")
}

func (h *Handler) loadSettings(ctx context.Context) (*Settings, error) {
	settings, err := h.service.Get(ctx)
	if err != nil {
		if errors.Is(err, ErrSettingsNotFound) {
			return &Settings{}, nil
		}
		return nil, err
	}

	return settings, nil
}

func (h *Handler) render(w http.ResponseWriter, tmpl string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	t, err := template.New("page").Parse(tmpl)
	if err != nil {
		h.logger.Error("failed to parse template", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if err := t.Execute(w, data); err != nil {
		h.logger.Error("failed to render template", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message, "error", err)
	http.Error(w, fmt.Sprintf("%s: %v", message, err), status)
}

func parseList(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, ",", "\n")

	parts := strings.Split(value, "\n")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		result = append(result, part)
	}

	return result
}

func digestWarnings(settings *Settings, resendConfigured bool) []string {
	warnings := []string{}

	if settings.DigestEnabled && strings.TrimSpace(settings.DigestRecipient) == "" {
		warnings = append(warnings, "Digest is enabled, but no recipient email is set.")
	}
	if settings.DigestEnabled && !resendConfigured {
		warnings = append(warnings, "Digest email delivery is not configured. Digests will be logged instead of emailed.")
	}

	return warnings
}

func pathForProfile(markSetupComplete bool) string {
	if markSetupComplete {
		return "/setup"
	}
	return "/settings/profile"
}

func pageHeading(markSetupComplete bool) string {
	if markSetupComplete {
		return "Initial Setup"
	}
	return "Profile Settings"
}

type statusPageData struct {
	Title            string
	SetupComplete    bool
	DigestConfig     digest.Config
	DigestWarnings   []string
	ResendConfigured bool
}

type profilePageData struct {
	Title              string
	Heading            string
	Action             string
	Flash              string
	SetupComplete      bool
	RoleKeywords       string
	SkillKeywords      string
	PreferredLevels    string
	PenaltyLevels      string
	PreferredLocations string
	PenaltyLocations   string
	MismatchKeywords   string
}

type digestPageData struct {
	Title            string
	Flash            string
	SetupComplete    bool
	DigestEnabled    bool
	DigestRecipient  string
	DigestTopN       int
	DigestLookback   string
	Warnings         []string
	ResendConfigured bool
}

const homeTemplate = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body>
<h1>{{.Title}}</h1>
{{if .SetupComplete}}<p>Setup is complete.</p>{{else}}<p>Setup is not complete yet. Visit <a href="/setup">/setup</a> to finish configuring the app.</p>{{end}}
<p><a href="/setup">Setup</a> | <a href="/settings/profile">Profile Settings</a> | <a href="/settings/digest">Digest Settings</a></p>
<h2>Digest Status</h2>
<p>Enabled: {{.DigestConfig.Enabled}}</p>
<p>Recipient: {{if .DigestConfig.Recipient}}{{.DigestConfig.Recipient}}{{else}}Not set{{end}}</p>
<p>Email delivery configured: {{.ResendConfigured}}</p>
{{if .DigestWarnings}}
<h3>Warnings</h3>
<ul>{{range .DigestWarnings}}<li>{{.}}</li>{{end}}</ul>
{{end}}
</body>
</html>`

const profileTemplate = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body>
<h1>{{.Heading}}</h1>
<p><a href="/">Home</a> | <a href="/settings/profile">Profile Settings</a> | <a href="/settings/digest">Digest Settings</a></p>
{{if .Flash}}<p><strong>{{.Flash}}</strong></p>{{end}}
{{if .SetupComplete}}<p>Setup is complete.</p>{{else}}<p>Setup is not complete yet. Saving this form will complete the profile setup step.</p>{{end}}
<form method="post" action="{{.Action}}">
<p><label>Role keywords<br><textarea name="role_keywords" rows="8" cols="40">{{.RoleKeywords}}</textarea></label></p>
<p><label>Skill keywords<br><textarea name="skill_keywords" rows="8" cols="40">{{.SkillKeywords}}</textarea></label></p>
<p><label>Preferred level keywords<br><textarea name="preferred_level_keywords" rows="6" cols="40">{{.PreferredLevels}}</textarea></label></p>
<p><label>Penalty level keywords<br><textarea name="penalty_level_keywords" rows="6" cols="40">{{.PenaltyLevels}}</textarea></label></p>
<p><label>Preferred location terms<br><textarea name="preferred_location_terms" rows="6" cols="40">{{.PreferredLocations}}</textarea></label></p>
<p><label>Penalty location terms<br><textarea name="penalty_location_terms" rows="6" cols="40">{{.PenaltyLocations}}</textarea></label></p>
<p><label>Mismatch keywords<br><textarea name="mismatch_keywords" rows="6" cols="40">{{.MismatchKeywords}}</textarea></label></p>
<p><button type="submit">Save Profile Settings</button></p>
</form>
</body>
</html>`

const digestTemplate = `<!doctype html>
<html lang="en">
<head><meta charset="utf-8"><title>{{.Title}}</title></head>
<body>
<h1>{{.Title}}</h1>
<p><a href="/">Home</a> | <a href="/settings/profile">Profile Settings</a> | <a href="/settings/digest">Digest Settings</a></p>
{{if .Flash}}<p><strong>{{.Flash}}</strong></p>{{end}}
{{if .Warnings}}
<h2>Warnings</h2>
<ul>{{range .Warnings}}<li>{{.}}</li>{{end}}</ul>
{{end}}
<form method="post" action="/settings/digest">
<p><label><input type="checkbox" name="digest_enabled" {{if .DigestEnabled}}checked{{end}}> Enable digest</label></p>
<p><label>Recipient email<br><input type="email" name="digest_recipient" value="{{.DigestRecipient}}" size="40"></label></p>
<p><label>Top N jobs<br><input type="number" name="digest_top_n" value="{{.DigestTopN}}" min="1" max="100"></label></p>
<p><label>Lookback duration<br><input type="text" name="digest_lookback" value="{{.DigestLookback}}"></label></p>
<p><button type="submit">Save Digest Settings</button></p>
</form>
</body>
</html>`
