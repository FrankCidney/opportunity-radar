package main

import (
	"context"
	"fmt"
	"log"

	"opportunity-radar/internal/scrapers/remotive"
	"opportunity-radar/internal/shared/config"
	"opportunity-radar/internal/shared/logger"
)

func main() {
	// Load config
	cfg := config.Load()
	
	// Initialize structured logger
	logr := logger.New(cfg.Env)
	
	scraper := remotive.NewScraper(logr)

	raws, err := scraper.Scrape(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("jobs fetched:", len(raws))
	fmt.Println("title:", raws[0].Title)
	fmt.Println("company name:", raws[0].Company)
	fmt.Println("url:", raws[0].URL)
	fmt.Println("location:", raws[0].Location)
	fmt.Println("description:", raws[0].Description)
}
