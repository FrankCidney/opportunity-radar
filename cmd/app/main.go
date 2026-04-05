package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/digest"
	"opportunity-radar/internal/ingest"
	"opportunity-radar/internal/jobs"
	"opportunity-radar/internal/preferences"
	"opportunity-radar/internal/runcontrol"
	"opportunity-radar/internal/scheduler"
	"opportunity-radar/internal/scoring"
	"opportunity-radar/internal/scrapers/remotive"
	"opportunity-radar/internal/shared/config"
	"opportunity-radar/internal/shared/logger"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logr := logger.New("development")
		logr.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logr := logger.New(cfg.Env)

	logr.Info("starting application",
		"env", cfg.Env,
	)

	sqlDB, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logr.Error("error setting up db", "error", err)
		os.Exit(1)
	}

	err = sqlDB.Ping()
	if err != nil {
		logr.Error("failed to connect to db", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	companiesRepo := companies.NewPostgresRepository(sqlDB, logr)
	companyService := companies.NewService(companiesRepo, logr)
	jobsRepo := jobs.NewPostgresRepository(sqlDB, logr)
	jobsService := jobs.NewService(jobsRepo, logr)
	digestRepo := digest.NewPostgresRepository(sqlDB, logr)
	preferencesRepo := preferences.NewPostgresRepository(sqlDB, logr)
	preferencesService := preferences.NewService(preferencesRepo, logr)

	settings, settingsCreated, err := preferencesService.Ensure(ctx, defaultSettingsBootstrap(cfg))
	if err != nil {
		logr.Error("failed to load app settings", "error", err)
		os.Exit(1)
	}
	if settingsCreated {
		logr.Info("bootstrapped app settings from default profile and current digest env values",
			"setup_complete", settings.SetupComplete,
		)
	}
	if !settings.SetupComplete {
		logr.Warn("app settings are not marked complete yet; using bootstrap settings until setup UI is added")
	}

	scorer := scoring.NewRulesScorer(preferences.BuildScoringProfile(settings))
	pipeline := ingest.NewPipeline(scorer, jobsService, companyService, logr)

	remotiveScraper := remotive.NewScraper(logr)

	ingestService := ingest.NewService(pipeline, []ingest.Scraper{remotiveScraper}, logr)
	digestSender := buildDigestSender(cfg, logr)
	digestService := digest.NewService(
		digestRepo,
		jobsService,
		companyService,
		digestSender,
		toDigestConfig(settings),
		logr,
	)
	digestRunner := digest.NewRunner(ingestService, digestService, logr)
	runCoordinator := runcontrol.New(digestRunner, logr)
	adminHandler := preferences.NewHandler(
		preferencesService,
		scorer,
		digestService,
		runCoordinator,
		isResendConfigured(cfg),
		cfg.SchedulerEnabled,
		scheduleLabel(cfg),
		logr,
	)
	server := buildHTTPServer(cfg, preferences.Routes(adminHandler))

	serverErrCh := make(chan error, 1)
	go func() {
		logr.Info("http server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErrCh <- err
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logr.Error("http server shutdown failed", "error", err)
		}
	}()

	if !cfg.SchedulerEnabled {
		logr.Info("scheduler disabled; running ingest once and keeping admin server available")
		if err := runCoordinator.RunAll(ctx); err != nil {
			logr.Error("ingest run failed", "error", err)
			os.Exit(1)
		}

		select {
		case <-ctx.Done():
			return
		case err := <-serverErrCh:
			logr.Error("http server failed", "error", err)
			os.Exit(1)
		}
	}

	sched := scheduler.New(runCoordinator, scheduler.Config{
		Interval:   cfg.SchedulerInterval,
		RunOnStart: cfg.SchedulerRunOnStart,
		RunTimeout: cfg.SchedulerRunTimeout,
	}, logr)

	schedulerErrCh := make(chan error, 1)
	go func() {
		schedulerErrCh <- sched.Run(ctx)
	}()

	select {
	case <-ctx.Done():
		return
	case err := <-serverErrCh:
		logr.Error("http server failed", "error", err)
		os.Exit(1)
	case err := <-schedulerErrCh:
		if err != nil {
			logr.Error("scheduler failed", "error", err)
			os.Exit(1)
		}
	}
}

func buildDigestSender(cfg config.Config, logger *slog.Logger) digest.Sender {
	if cfg.ResendAPIKey == "" || cfg.ResendFromEmail == "" {
		logger.Info("resend not fully configured; using logging digest sender")
		return digest.NewLoggingSender(logger)
	}

	logger.Info("using resend digest sender",
		"from_email", cfg.ResendFromEmail,
	)

	return digest.NewResendSender(
		cfg.ResendAPIKey,
		cfg.ResendFromEmail,
		cfg.ResendFromName,
		nil,
		logger,
	)
}

func buildHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func isResendConfigured(cfg config.Config) bool {
	return cfg.ResendAPIKey != "" && cfg.ResendFromEmail != ""
}

func defaultSettingsBootstrap(cfg config.Config) *preferences.Settings {
	settings := &preferences.Settings{
		SetupComplete:   false,
		DesiredRoles:    []string{"backend engineer", "software engineer"},
		ExperienceLevel: "Junior / early-career",
		CurrentSkills:   []string{"go", "postgres", "docker"},
		GrowthSkills:    []string{"python", "ai/ml"},
		Locations:       []string{"remote", "kenya"},
		WorkModes:       []string{"remote", "hybrid"},
		AvoidTerms:      []string{"senior", "manager", "sales"},
		DigestEnabled:   cfg.DigestEnabled,
		DigestRecipient: cfg.DigestToEmail,
		DigestTopN:      cfg.DigestTopN,
		DigestLookback:  cfg.DigestLookback,
	}
	settings.RecalculateDerivedFields()
	settings.SetupComplete = false
	return settings
}

func toDigestConfig(settings *preferences.Settings) digest.Config {
	if settings == nil {
		return digest.Config{}
	}

	return digest.Config{
		Enabled:   settings.DigestEnabled,
		Recipient: settings.DigestRecipient,
		TopN:      settings.DigestTopN,
		Lookback:  settings.DigestLookback,
	}
}

func scheduleLabel(cfg config.Config) string {
	if !cfg.SchedulerEnabled {
		return "Automatic runs are turned off"
	}
	return "Every " + cfg.SchedulerInterval.String()
}
