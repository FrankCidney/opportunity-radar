package main

import (
	"context"
	"database/sql"
	"os"
	"os/signal"
	"syscall"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/ingest"
	"opportunity-radar/internal/jobs"
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
	cfg := config.Load()

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

	scorer := scoring.NewRulesScorer([]string{"go", "golang", "backend", "remote"})
	pipeline := ingest.NewPipeline(scorer, jobsService, companyService, logr)

	remotiveScraper := remotive.NewScraper(logr)

	ingestService := ingest.NewService(pipeline, []ingest.Scraper{remotiveScraper}, logr)

	if !cfg.SchedulerEnabled {
		logr.Info("scheduler disabled; running ingest once")
		if err := ingestService.RunAll(ctx); err != nil {
			logr.Error("ingest run failed", "error", err)
			os.Exit(1)
		}
		return
	}

	sched := scheduler.New(ingestService, scheduler.Config{
		Interval:   cfg.SchedulerInterval,
		RunOnStart: cfg.SchedulerRunOnStart,
		RunTimeout: cfg.SchedulerRunTimeout,
	}, logr)

	if err := sched.Run(ctx); err != nil {
		logr.Error("scheduler failed", "error", err)
		os.Exit(1)
	}
}
