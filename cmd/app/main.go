package main

import (
	"context"
	"database/sql"
	"os"

	"opportunity-radar/internal/companies"
	"opportunity-radar/internal/ingest"
	"opportunity-radar/internal/jobs"
	"opportunity-radar/internal/scoring"
	"opportunity-radar/internal/scrapers/remotive"
	"opportunity-radar/internal/shared/config"
	"opportunity-radar/internal/shared/logger"
)

func main() {
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
	if err := ingestService.RunAll(context.Background()); err != nil {
		logr.Error("ingest run failed", "error", err)
		os.Exit(1)
	}
}
