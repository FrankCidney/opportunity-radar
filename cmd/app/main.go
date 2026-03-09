package main

import (
	"database/sql"
	"opportunity-radar/internal/jobs"
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
		logr.Error("error setting up db: %v", err)
		os.Exit(1)
	}

	err = sqlDB.Ping()
	if err != nil {
		logr.Error("failed to connect to db: %v", err)
		os.Exit(1)
	}

	jobRepo := jobs.NewPostgresRepository(sqlDB, logr)
}