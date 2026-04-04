package main

import (
	"embed"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

type prototypeState struct {
	Key                     string
	SetupComplete           bool
	PartialSetup            bool
	Roles                   string
	ExperienceLevel         string
	CurrentSkills           string
	GrowthSkills            string
	Locations               []string
	WorkModes               []string
	Avoid                   string
	EmailUpdatesEnabled     bool
	EmailDestination        string
	EmailTopN               int
	EmailLookback           string
	SchedulerEnabled        bool
	ScheduleLabel           string
	EmailDeliveryConfigured bool
	LastRunAt               string
	LastDigestAt            string
}

type app struct {
	templates *template.Template
	states    map[string]*prototypeState
}

type pageData struct {
	Title                string
	ActiveNav            string
	State                *prototypeState
	Flash                string
	Warnings             []string
	Scenarios            []scenarioLink
	CurrentPath          string
	ExperienceOptions    []string
	WorkModeOptions      []string
	LocationOptions      []string
	EmailLookbackOptions []string
}

type scenarioLink struct {
	Key   string
	Label string
}

func main() {
	tmpl := template.Must(template.New("").Funcs(template.FuncMap{
		"splitLines":  splitLines,
		"joinLines":   joinLines,
		"contains":    contains,
		"containsInt": containsInt,
	}).ParseFS(templatesFS, "templates/*.html"))

	staticRoot, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatal(err)
	}

	app := &app{
		templates: tmpl,
		states: map[string]*prototypeState{
			"first_run": {
				Key:                     "first_run",
				SetupComplete:           false,
				PartialSetup:            false,
				ExperienceLevel:         "Junior / early-career",
				WorkModes:               []string{"Remote"},
				Locations:               []string{"Remote"},
				EmailUpdatesEnabled:     true,
				EmailTopN:               10,
				EmailLookback:           "24h",
				SchedulerEnabled:        true,
				ScheduleLabel:           "Every 24 hours",
				EmailDeliveryConfigured: false,
				LastRunAt:               "No runs yet",
				LastDigestAt:            "No updates sent yet",
			},
			"partial_setup": {
				Key:                     "partial_setup",
				SetupComplete:           false,
				PartialSetup:            true,
				Roles:                   "Backend engineer\nSoftware engineer",
				ExperienceLevel:         "Junior / early-career",
				CurrentSkills:           "Go\nPostgres",
				GrowthSkills:            "Docker\nDistributed systems",
				Locations:               []string{"Remote", "Kenya"},
				WorkModes:               []string{"Remote", "Hybrid"},
				Avoid:                   "Senior\nManager\nSales",
				EmailUpdatesEnabled:     true,
				EmailDestination:        "francis@example.com",
				EmailTopN:               10,
				EmailLookback:           "24h",
				SchedulerEnabled:        true,
				ScheduleLabel:           "Every 24 hours",
				EmailDeliveryConfigured: false,
				LastRunAt:               "Today at 09:10",
				LastDigestAt:            "No updates sent yet",
			},
			"configured": {
				Key:                     "configured",
				SetupComplete:           true,
				PartialSetup:            false,
				Roles:                   "Backend engineer\nSoftware engineer",
				ExperienceLevel:         "Junior / early-career",
				CurrentSkills:           "Go\nPostgres\nDocker",
				GrowthSkills:            "Python\nAI/ML",
				Locations:               []string{"Remote", "Kenya"},
				WorkModes:               []string{"Remote", "Hybrid"},
				Avoid:                   "Senior\nManager\nSales",
				EmailUpdatesEnabled:     true,
				EmailDestination:        "francis@example.com",
				EmailTopN:               10,
				EmailLookback:           "24h",
				SchedulerEnabled:        true,
				ScheduleLabel:           "Every 24 hours",
				EmailDeliveryConfigured: false,
				LastRunAt:               "Today at 07:30",
				LastDigestAt:            "Today at 07:32",
			},
			"configured_scheduler_off": {
				Key:                     "configured_scheduler_off",
				SetupComplete:           true,
				PartialSetup:            false,
				Roles:                   "Backend engineer\nSoftware engineer",
				ExperienceLevel:         "Junior / early-career",
				CurrentSkills:           "Go\nPostgres\nDocker",
				GrowthSkills:            "Python\nAI/ML",
				Locations:               []string{"Remote", "Kenya"},
				WorkModes:               []string{"Remote"},
				Avoid:                   "Senior\nManager",
				EmailUpdatesEnabled:     true,
				EmailDestination:        "francis@example.com",
				EmailTopN:               10,
				EmailLookback:           "24h",
				SchedulerEnabled:        false,
				ScheduleLabel:           "Automatic runs are turned off",
				EmailDeliveryConfigured: true,
				LastRunAt:               "Yesterday at 18:40",
				LastDigestAt:            "Yesterday at 18:41",
			},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticRoot))))
	mux.HandleFunc("/", app.handleHome)
	mux.HandleFunc("/onboarding", app.handleOnboarding)
	mux.HandleFunc("/profile", app.handleProfile)
	mux.HandleFunc("/profile/edit", app.handleProfileEdit)
	mux.HandleFunc("/notifications", app.handleNotifications)
	mux.HandleFunc("/run-once", app.handleRunOnce)

	addr := ":8090"
	log.Printf("Stage 4 UI prototype listening on http://localhost%s\n", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func (a *app) handleHome(w http.ResponseWriter, r *http.Request) {
	state := a.currentState(r)
	if !state.SetupComplete {
		http.Redirect(w, r, addScenario("/onboarding", state.Key), http.StatusSeeOther)
		return
	}

	data := a.newPageData("Opportunity Radar", "home", "/", state, r)
	data.Warnings = statusWarnings(state)
	a.render(w, "index.html", data)
}

func (a *app) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	state := a.currentState(r)
	if r.Method == http.MethodPost {
		state.SetupComplete = true
		state.PartialSetup = false
		copyFormValues(state, r)
		http.Redirect(w, r, addScenario("/profile?flash=Setup+complete.+You+can+edit+these+settings+any+time.", state.Key), http.StatusSeeOther)
		return
	}

	data := a.newPageData("Set Up Opportunity Radar", "onboarding", "/onboarding", state, r)
	data.Warnings = onboardingWarnings(state)
	a.render(w, "setup.html", data)
}

func (a *app) handleProfile(w http.ResponseWriter, r *http.Request) {
	state := a.currentState(r)
	data := a.newPageData("Profile", "profile", "/profile", state, r)
	data.Warnings = statusWarnings(state)
	a.render(w, "profile.html", data)
}

func (a *app) handleProfileEdit(w http.ResponseWriter, r *http.Request) {
	state := a.currentState(r)
	if r.Method == http.MethodPost {
		copyFormValues(state, r)
		http.Redirect(w, r, addScenario("/profile?flash=Profile+updated+for+the+prototype.", state.Key), http.StatusSeeOther)
		return
	}

	data := a.newPageData("Edit Profile", "profile", "/profile/edit", state, r)
	a.render(w, "profile_edit.html", data)
}

func (a *app) handleNotifications(w http.ResponseWriter, r *http.Request) {
	state := a.currentState(r)
	if r.Method == http.MethodPost {
		state.EmailUpdatesEnabled = r.FormValue("email_updates_enabled") == "on"
		state.EmailDestination = strings.TrimSpace(r.FormValue("email_destination"))
		state.EmailLookback = strings.TrimSpace(r.FormValue("email_lookback"))
		state.EmailDeliveryConfigured = r.FormValue("email_delivery_configured") == "configured"

		switch strings.TrimSpace(r.FormValue("email_top_n")) {
		case "5":
			state.EmailTopN = 5
		case "15":
			state.EmailTopN = 15
		default:
			state.EmailTopN = 10
		}

		http.Redirect(w, r, addScenario("/notifications?flash=Email+update+settings+saved+in+the+prototype.", state.Key), http.StatusSeeOther)
		return
	}

	data := a.newPageData("Email Updates", "notifications", "/notifications", state, r)
	data.Warnings = notificationsWarnings(state)
	a.render(w, "notifications.html", data)
}

func (a *app) handleRunOnce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	state := a.currentState(r)
	now := time.Now().Format("Today at 15:04")
	state.LastRunAt = now
	if state.EmailUpdatesEnabled {
		state.LastDigestAt = now
	}

	http.Redirect(w, r, addScenario("/?flash=Manual+run+queued+for+the+prototype.+Automatic+runs+remain+unchanged.", state.Key), http.StatusSeeOther)
}

func copyFormValues(state *prototypeState, r *http.Request) {
	state.Roles = strings.TrimSpace(r.FormValue("roles"))
	state.ExperienceLevel = strings.TrimSpace(r.FormValue("experience_level"))
	state.CurrentSkills = strings.TrimSpace(r.FormValue("current_skills"))
	state.GrowthSkills = strings.TrimSpace(r.FormValue("growth_skills"))
	state.Avoid = strings.TrimSpace(r.FormValue("avoid"))
	state.Locations = sortedSelection(r.Form["locations"])
	state.WorkModes = sortedSelection(r.Form["work_modes"])
}

func sortedSelection(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	sort.Strings(result)
	if contains(result, "Remote") {
		result = moveRemoteFirst(result)
	}
	return result
}

func moveRemoteFirst(values []string) []string {
	result := make([]string, 0, len(values))
	result = append(result, "Remote")
	for _, value := range values {
		if value == "Remote" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func (a *app) render(w http.ResponseWriter, name string, data pageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := a.templates.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (a *app) currentState(r *http.Request) *prototypeState {
	key := r.URL.Query().Get("scenario")
	if key == "" {
		key = "first_run"
	}
	if state, ok := a.states[key]; ok {
		return state
	}
	return a.states["first_run"]
}

func (a *app) newPageData(title string, active string, currentPath string, state *prototypeState, r *http.Request) pageData {
	return pageData{
		Title:                title,
		ActiveNav:            active,
		State:                state,
		Flash:                r.URL.Query().Get("flash"),
		Warnings:             nil,
		Scenarios:            scenarioLinks(),
		CurrentPath:          currentPath,
		ExperienceOptions:    []string{"Junior / early-career", "Mid-level", "Senior", "Lead / manager"},
		WorkModeOptions:      []string{"Remote", "Hybrid", "On-site"},
		LocationOptions:      []string{"Remote", "Kenya", "Uganda", "Tanzania", "Rwanda", "South Africa", "Nigeria", "Ghana", "United Kingdom", "United States"},
		EmailLookbackOptions: []string{"24h", "48h", "72h"},
	}
}

func scenarioLinks() []scenarioLink {
	return []scenarioLink{
		{Key: "first_run", Label: "First Run"},
		{Key: "partial_setup", Label: "Partial Setup"},
		{Key: "configured", Label: "Configured"},
		{Key: "configured_scheduler_off", Label: "Scheduler Off"},
	}
}

func addScenario(target string, scenario string) string {
	if scenario == "" {
		return target
	}
	if strings.Contains(target, "?") {
		return target + "&scenario=" + scenario
	}
	return target + "?scenario=" + scenario
}

func statusWarnings(state *prototypeState) []string {
	var warnings []string
	if !state.SchedulerEnabled {
		warnings = append(warnings, "Automatic runs are turned off. Settings changes are saved, but nothing will run on a schedule.")
	}
	if state.EmailUpdatesEnabled && strings.TrimSpace(state.EmailDestination) == "" {
		warnings = append(warnings, "Email updates are on, but no destination email is set yet.")
	}
	if state.EmailUpdatesEnabled && !state.EmailDeliveryConfigured {
		warnings = append(warnings, "Email delivery is not configured. Updates would be logged instead of emailed in the real app.")
	}
	return warnings
}

func onboardingWarnings(state *prototypeState) []string {
	if state.PartialSetup {
		return []string{"You already started setup. Your current answers are shown below so you can finish when ready."}
	}
	return nil
}

func notificationsWarnings(state *prototypeState) []string {
	return statusWarnings(state)
}

func (p pageData) Route(target string) string {
	return addScenario(target, p.State.Key)
}

func (p pageData) ScenarioRoute(target string, scenario string) string {
	return addScenario(target, scenario)
}

func (p pageData) IsActive(name string) bool {
	return p.ActiveNav == name
}

func joinLines(values any) string {
	switch typed := values.(type) {
	case string:
		return joinStringLines(typed)
	case []string:
		if len(typed) == 0 {
			return "Not set yet"
		}
		return strings.Join(typed, ", ")
	default:
		return "Not set yet"
	}
}

func joinStringLines(value string) string {
	lines := splitLines(value)
	if len(lines) == 0 {
		return "Not set yet"
	}
	return strings.Join(lines, ", ")
}

func splitLines(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	lines := strings.Split(value, "\n")
	clean := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		clean = append(clean, line)
	}
	return clean
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsInt(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
