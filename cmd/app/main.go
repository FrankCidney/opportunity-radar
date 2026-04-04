package main

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/digest"
	"opportunity-radar/internal/ingest"
	"opportunity-radar/internal/jobs"
	"opportunity-radar/internal/preferences"
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

	scorer := scoring.NewRulesScorer(toScoringProfile(settings))
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
	runner := digest.NewRunner(ingestService, digestService, logr)

	if !cfg.SchedulerEnabled {
		logr.Info("scheduler disabled; running ingest once")
		if err := runner.RunAll(ctx); err != nil {
			logr.Error("ingest run failed", "error", err)
			os.Exit(1)
		}
		return
	}

	sched := scheduler.New(runner, scheduler.Config{
		Interval:   cfg.SchedulerInterval,
		RunOnStart: cfg.SchedulerRunOnStart,
		RunTimeout: cfg.SchedulerRunTimeout,
	}, logr)

	if err := sched.Run(ctx); err != nil {
		logr.Error("scheduler failed", "error", err)
		os.Exit(1)
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

func defaultSettingsBootstrap(cfg config.Config) *preferences.Settings {
	return &preferences.Settings{
		SetupComplete: false,
		RoleKeywords: []string{
			"backend",
			"back end",
			"software engineer",
			"software developer",
			"developer",
			"engineer",
			"api",
			"platform",
		},
		SkillKeywords: []string{
			"go",
			"golang",
			"postgres",
			"sql",
			"docker",
			"kubernetes",
			"microservices",
			"distributed systems",
			"grpc",
			"rest",
		},
		PreferredLevelKeywords: []string{
			"junior",
			"entry level",
			"entry-level",
			"graduate",
			"new grad",
			"intern",
			"associate",
		},
		PenaltyLevelKeywords: []string{
			"senior",
			"staff",
			"principal",
			"lead",
			"manager",
			"director",
			"head of",
		},
		PreferredLocationTerms: []string{
			"remote",
			"anywhere",
			"worldwide",
			"distributed",
		},
		PenaltyLocationTerms: []string{
			"on-site",
			"onsite",
			"in office",
			"relocation required",
		},
		MismatchKeywords: []string{
			"sales",
			"account executive",
			"customer success",
			"recruiter",
			"marketing",
			"designer",
		},
		DigestEnabled:   cfg.DigestEnabled,
		DigestRecipient: cfg.DigestToEmail,
		DigestTopN:      cfg.DigestTopN,
		DigestLookback:  cfg.DigestLookback,
	}
}

func toScoringProfile(settings *preferences.Settings) scoring.Profile {
	if settings == nil {
		return scoring.Profile{}
	}

	return scoring.Profile{
		RoleKeywords:           settings.RoleKeywords,
		SkillKeywords:          settings.SkillKeywords,
		PreferredLevelKeywords: settings.PreferredLevelKeywords,
		PenaltyLevelKeywords:   settings.PenaltyLevelKeywords,
		PreferredLocationTerms: settings.PreferredLocationTerms,
		PenaltyLocationTerms:   settings.PenaltyLocationTerms,
		MismatchKeywords:       settings.MismatchKeywords,
	}
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
