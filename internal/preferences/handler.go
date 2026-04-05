package preferences

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"opportunity-radar/internal/digest"
	"opportunity-radar/internal/runcontrol"
	"opportunity-radar/internal/scoring"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

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

type RunController interface {
	RunNow(ctx context.Context) error
	Status() runcontrol.Status
}

type Handler struct {
	service          SettingsGetterSaver
	scorer           ScoringProfileUpdater
	digestService    DigestConfigUpdater
	runController    RunController
	logger           *slog.Logger
	resendConfigured bool
	schedulerEnabled bool
	scheduleLabel    string
	templates        *template.Template
	staticHandler    http.Handler
}

func NewHandler(
	service SettingsGetterSaver,
	scorer ScoringProfileUpdater,
	digestService DigestConfigUpdater,
	runController RunController,
	resendConfigured bool,
	schedulerEnabled bool,
	scheduleLabel string,
	logger *slog.Logger,
) *Handler {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"joinLines":      joinLines,
		"textareaLines":  textareaLines,
		"contains":       contains,
		"formatLookback": formatLookback,
	}).ParseFS(templatesFS, "templates/*.html"))

	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}

	return &Handler{
		service:          service,
		scorer:           scorer,
		digestService:    digestService,
		runController:    runController,
		logger:           logger,
		resendConfigured: resendConfigured,
		schedulerEnabled: schedulerEnabled,
		scheduleLabel:    scheduleLabel,
		templates:        tmpl,
		staticHandler:    http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))),
	}
}

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}

	if !settings.SetupComplete {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	h.render(w, "index.html", pageData{
		Title:                "Opportunity Radar",
		ActiveNav:            "home",
		State:                settings,
		Flash:                r.URL.Query().Get("flash"),
		Warnings:             statusWarnings(settings, h.schedulerEnabled, h.resendConfigured),
		SetupReminder:        optionalSetupReminder(settings),
		SchedulerEnabled:     h.schedulerEnabled,
		ScheduleLabel:        h.scheduleLabel,
		ResendConfigured:     h.resendConfigured,
		RunStatus:            h.runStatus(),
		ExperienceOptions:    ExperienceOptions,
		WorkModeOptions:      WorkModeOptions,
		LocationOptions:      LocationOptions,
		EmailLookbackOptions: EmailLookbackOptions,
	})
}

func (h *Handler) Setup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := h.loadSettings(r.Context())
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
			return
		}
		h.render(w, "setup.html", pageData{
			Title:                "Set Up Opportunity Radar",
			ActiveNav:            "onboarding",
			State:                settings,
			Flash:                r.URL.Query().Get("flash"),
			Warnings:             onboardingWarnings(settings),
			SchedulerEnabled:     h.schedulerEnabled,
			ScheduleLabel:        h.scheduleLabel,
			ResendConfigured:     h.resendConfigured,
			RunStatus:            h.runStatus(),
			ExperienceOptions:    ExperienceOptions,
			WorkModeOptions:      WorkModeOptions,
			LocationOptions:      LocationOptions,
			EmailLookbackOptions: EmailLookbackOptions,
		})
	case http.MethodPost:
		h.handleSetupSave(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) ProfileSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}

	if !settings.SetupComplete {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	h.render(w, "profile.html", pageData{
		Title:                "Profile",
		ActiveNav:            "profile",
		State:                settings,
		Flash:                r.URL.Query().Get("flash"),
		Warnings:             []string{"Changes here affect future scoring runs. Existing saved jobs are not automatically rescored."},
		SetupReminder:        optionalSetupReminder(settings),
		SchedulerEnabled:     h.schedulerEnabled,
		ScheduleLabel:        h.scheduleLabel,
		ResendConfigured:     h.resendConfigured,
		RunStatus:            h.runStatus(),
		ExperienceOptions:    ExperienceOptions,
		WorkModeOptions:      WorkModeOptions,
		LocationOptions:      LocationOptions,
		EmailLookbackOptions: EmailLookbackOptions,
	})
}

func (h *Handler) ProfileEdit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := h.loadSettings(r.Context())
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
			return
		}
		h.render(w, "profile_edit.html", pageData{
			Title:                "Edit Profile",
			ActiveNav:            "profile",
			State:                settings,
			Flash:                r.URL.Query().Get("flash"),
			SchedulerEnabled:     h.schedulerEnabled,
			ScheduleLabel:        h.scheduleLabel,
			ResendConfigured:     h.resendConfigured,
			RunStatus:            h.runStatus(),
			ExperienceOptions:    ExperienceOptions,
			WorkModeOptions:      WorkModeOptions,
			LocationOptions:      LocationOptions,
			EmailLookbackOptions: EmailLookbackOptions,
		})
	case http.MethodPost:
		h.handleProfileSave(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) DigestSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		settings, err := h.loadSettings(r.Context())
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
			return
		}
		if !settings.SetupComplete {
			http.Redirect(w, r, "/setup", http.StatusSeeOther)
			return
		}
		h.render(w, "notifications.html", pageData{
			Title:                "Email Updates",
			ActiveNav:            "notifications",
			State:                settings,
			Flash:                r.URL.Query().Get("flash"),
			Warnings:             digestWarnings(settings, h.schedulerEnabled, h.resendConfigured),
			SchedulerEnabled:     h.schedulerEnabled,
			ScheduleLabel:        h.scheduleLabel,
			ResendConfigured:     h.resendConfigured,
			RunStatus:            h.runStatus(),
			ExperienceOptions:    ExperienceOptions,
			WorkModeOptions:      WorkModeOptions,
			LocationOptions:      LocationOptions,
			EmailLookbackOptions: EmailLookbackOptions,
		})
	case http.MethodPost:
		h.handleDigestSave(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) RunOnce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}
	if !settings.SetupComplete {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}

	err = h.runController.RunNow(r.Context())
	switch {
	case err == nil:
		http.Redirect(w, r, "/?flash=Manual+run+started.", http.StatusSeeOther)
	case errors.Is(err, runcontrol.ErrRunInProgress):
		http.Redirect(w, r, "/?flash=A+run+is+already+in+progress.+Try+again+in+a+moment.", http.StatusSeeOther)
	default:
		h.logger.Error("manual run failed", "error", err)
		http.Redirect(w, r, "/?flash=Manual+run+failed+to+start.", http.StatusSeeOther)
	}
}

func (h *Handler) Static(w http.ResponseWriter, r *http.Request) {
	h.staticHandler.ServeHTTP(w, r)
}

func (h *Handler) handleSetupSave(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid form submission", err)
		return
	}

	populateProfileFromForm(settings, r)
	settings.DigestRecipient = strings.TrimSpace(r.FormValue("email_destination"))
	settings.RecalculateDerivedFields()

	missingRequired := missingRequiredSetupFields(settings)
	if err := h.service.Save(r.Context(), settings); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save settings", err)
		return
	}

	h.scorer.SetProfile(BuildScoringProfile(settings))
	if settings.SetupComplete {
		if len(missingRecommendedSetupFields(settings)) == 0 {
			http.Redirect(w, r, "/settings/profile?flash=Setup+complete.+You+can+edit+these+settings+any+time.", http.StatusSeeOther)
			return
		}

		http.Redirect(w, r, "/settings/profile", http.StatusSeeOther)
		return
	}

	flash := "Setup saved. Finish the required fields to unlock the home page."
	if len(missingRequired) > 0 {
		flash = "Setup saved, but some required fields are still missing."
	}

	warnings := onboardingWarnings(settings)
	if len(missingRequired) > 0 {
		warnings = append(warnings, "Still required: "+strings.Join(missingRequired, ", ")+".")
	}

	h.render(w, "setup.html", pageData{
		Title:                "Set Up Opportunity Radar",
		ActiveNav:            "onboarding",
		State:                settings,
		Flash:                flash,
		Warnings:             warnings,
		SchedulerEnabled:     h.schedulerEnabled,
		ScheduleLabel:        h.scheduleLabel,
		ResendConfigured:     h.resendConfigured,
		RunStatus:            h.runStatus(),
		ExperienceOptions:    ExperienceOptions,
		WorkModeOptions:      WorkModeOptions,
		LocationOptions:      LocationOptions,
		EmailLookbackOptions: EmailLookbackOptions,
	})
}

func (h *Handler) handleProfileSave(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid form submission", err)
		return
	}

	populateProfileFromForm(settings, r)
	settings.RecalculateDerivedFields()
	if err := h.service.Save(r.Context(), settings); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save profile settings", err)
		return
	}

	h.scorer.SetProfile(BuildScoringProfile(settings))

	if !settings.SetupComplete {
		http.Redirect(w, r, "/setup?flash=Your+changes+left+setup+incomplete.+Please+finish+the+required+fields.", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings/profile?flash=Profile+updated.", http.StatusSeeOther)
}

func (h *Handler) handleDigestSave(w http.ResponseWriter, r *http.Request) {
	settings, err := h.loadSettings(r.Context())
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to load settings", err)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid form submission", err)
		return
	}

	topN, err := strconv.Atoi(strings.TrimSpace(r.FormValue("email_top_n")))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "email update count must be a valid integer", err)
		return
	}

	lookback, err := time.ParseDuration(strings.TrimSpace(r.FormValue("email_lookback")))
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "email lookback must be a valid duration", err)
		return
	}

	settings.DigestEnabled = r.FormValue("email_updates_enabled") == "on"
	settings.DigestRecipient = strings.TrimSpace(r.FormValue("email_destination"))
	settings.DigestTopN = topN
	settings.DigestLookback = lookback

	if err := h.service.Save(r.Context(), settings); err != nil {
		h.writeError(w, http.StatusInternalServerError, "failed to save email update settings", err)
		return
	}

	h.digestService.UpdateConfig(digest.Config{
		Enabled:   settings.DigestEnabled,
		Recipient: settings.DigestRecipient,
		TopN:      settings.DigestTopN,
		Lookback:  settings.DigestLookback,
	})

	http.Redirect(w, r, "/settings/digest?flash=Email+update+settings+saved.", http.StatusSeeOther)
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

func (h *Handler) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.ExecuteTemplate(w, name, data); err != nil {
		h.logger.Error("failed to render template", "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Error(message, "error", err)
	http.Error(w, message, status)
}

func (h *Handler) runStatus() runcontrol.Status {
	if h.runController == nil {
		return runcontrol.Status{LastState: "idle"}
	}
	return h.runController.Status()
}

func populateProfileFromForm(settings *Settings, r *http.Request) {
	settings.DesiredRoles = parseLines(r.FormValue("roles"))
	settings.ExperienceLevel = strings.TrimSpace(r.FormValue("experience_level"))
	settings.CurrentSkills = parseLines(r.FormValue("current_skills"))
	settings.GrowthSkills = parseLines(r.FormValue("growth_skills"))
	settings.Locations = parseValues(r.Form["locations"])
	settings.WorkModes = parseValues(r.Form["work_modes"])
	settings.AvoidTerms = parseLines(r.FormValue("avoid"))
}

func parseLines(value string) []string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, ",", "\n")
	parts := strings.Split(value, "\n")
	return parseValues(parts)
}

func parseValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func onboardingWarnings(settings *Settings) []string {
	if settings.PartialSetup() {
		return []string{"You already started setup. Your current answers are shown below so you can finish when ready."}
	}
	return nil
}

func missingRequiredSetupFields(settings *Settings) []string {
	missing := []string{}
	if len(normalizeStringList(settings.DesiredRoles)) == 0 {
		missing = append(missing, "roles")
	}
	if strings.TrimSpace(settings.ExperienceLevel) == "" {
		missing = append(missing, "experience level")
	}
	if len(normalizeStringList(settings.Locations)) == 0 {
		missing = append(missing, "at least one location")
	}
	if len(normalizeStringList(settings.WorkModes)) == 0 {
		missing = append(missing, "at least one work mode")
	}
	return missing
}

func missingRecommendedSetupFields(settings *Settings) []string {
	missing := []string{}
	if len(normalizeStringList(settings.CurrentSkills)) == 0 {
		missing = append(missing, "current skills")
	}
	if len(normalizeStringList(settings.GrowthSkills)) == 0 {
		missing = append(missing, "growth skills")
	}
	if len(normalizeStringList(settings.AvoidTerms)) == 0 {
		missing = append(missing, "avoid terms")
	}
	if strings.TrimSpace(settings.DigestRecipient) == "" {
		missing = append(missing, "email destination")
	}
	return missing
}

func optionalSetupReminder(settings *Settings) *setupReminder {
	if settings == nil || !settings.SetupComplete {
		return nil
	}

	missing := missingRecommendedSetupFields(settings)
	if len(missing) == 0 {
		return nil
	}

	return &setupReminder{
		Title:   "Setup is almost done",
		Message: "A few recommended fields are still blank. Fill them in so future runs and updates reflect your full preferences.",
		Missing: missing,
	}
}

func statusWarnings(settings *Settings, schedulerEnabled bool, resendConfigured bool) []string {
	warnings := make([]string, 0, 3)
	if !schedulerEnabled {
		warnings = append(warnings, "Automatic runs are turned off. Settings changes are saved, but nothing will run on a schedule.")
	}
	return append(warnings, digestWarnings(settings, schedulerEnabled, resendConfigured)...)
}

func digestWarnings(settings *Settings, schedulerEnabled bool, resendConfigured bool) []string {
	warnings := []string{}
	if settings.DigestEnabled && strings.TrimSpace(settings.DigestRecipient) == "" {
		warnings = append(warnings, "Email updates are enabled, but no destination email is set yet.")
	}
	if settings.DigestEnabled && !resendConfigured {
		warnings = append(warnings, "Email delivery is not configured. Updates will be logged instead of emailed.")
	}
	if settings.DigestEnabled && !schedulerEnabled {
		warnings = append(warnings, "Email updates are enabled, but automatic runs are turned off. Use Run Once or enable the scheduler in deployment config.")
	}
	return warnings
}

func joinLines(values []string) string {
	if len(values) == 0 {
		return "Not set yet"
	}
	return strings.Join(values, ", ")
}

func textareaLines(values []string) string {
	return strings.Join(values, "\n")
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func formatLookback(value time.Duration) string {
	if value <= 0 {
		return "Not set yet"
	}

	hours := int(value / time.Hour)
	if hours > 0 && value == time.Duration(hours)*time.Hour {
		return strconv.Itoa(hours) + "h"
	}

	return value.String()
}

type pageData struct {
	Title                string
	ActiveNav            string
	State                *Settings
	Flash                string
	Warnings             []string
	SetupReminder        *setupReminder
	SchedulerEnabled     bool
	ScheduleLabel        string
	ResendConfigured     bool
	RunStatus            runcontrol.Status
	ExperienceOptions    []string
	WorkModeOptions      []string
	LocationOptions      []string
	EmailLookbackOptions []string
}

type setupReminder struct {
	Title   string
	Message string
	Missing []string
}
