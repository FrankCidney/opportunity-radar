package main

import (
	"database/sql"
	"opportunity-radar/internal/ingest"
	"opportunity-radar/internal/jobs"
	"opportunity-radar/internal/scoring"
	"opportunity-radar/internal/scrapers/remotive"
	"opportunity-radar/internal/shared/config"
	"opportunity-radar/internal/shared/logger"
	"os"
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

	jobsRepo := jobs.NewPostgresRepository(sqlDB, logr)
	jobsService := jobs.NewService(jobsRepo, logr)
	companyService := &ingest.StubCompanyService{}

	scorer := scoring.NewRulesScorer([]string{"go", "golang", "backend", "remote"})
	pipeline := ingest.NewPipeline(scorer, jobsService, companyService, logr)

	remotiveScraper := remotive.NewScraper(logr)

	ingest.NewService(pipeline, []ingest.Scraper{remotiveScraper}, logr)
}
