# Fuzu Scraper Implementation Notes

## Overview

This document explains how the Fuzu scraper integrates into the opportunity-radar codebase.

## Scraper Integration Flow

The Fuzu scraper follows the established pattern for scrapers in this codebase:

```
cmd/app/main.go
    ↓ (constructs scrapers)
ingest.NewService(pipeline, []ingest.Scraper{...}, logr)
    ↓
service.RunAll(ctx) → pipeline.RunAll(ctx)
    ↓
pipeline calls scraper.Scrape(ctx) for each scraper
    ↓
Scraper returns []normalize.RawJob
    ↓
normalize.Normalize() converts RawJob → NormalizedJob
    ↓
CompanyService.FindOrCreate() resolves company
    ↓
jobs.Job created with NormalizedJob + company
    ↓
Scorer computes score
    ↓
jobs.Service.Save() persists record (skips duplicates)
```

## How Fuzu Scraper Works

### Listing Page Discovery

The Fuzu scraper uses JSON-LD structured data for parsing listing pages:

- Fuzu exposes an `ItemList` JSON-LD block with `itemListElement` containing job entries
- Each entry has `name` (title) and `url` (detail page link)
- Falls back to HTML parsing if JSON-LD is missing

Example from Fuzu listing pages:
```json
{
  "@context": "http://schema.org",
  "@type": "ItemList",
  "itemListElement": [
    {"@type": "ListItem", "name": "Backend Developer", "url": "/jobs/backend-developer-abc123"}
  ]
}
```

### Detail Page Parsing

Fuzu detail pages expose rich `JobPosting` JSON-LD data:

- `identifier.value` - Stable numeric external ID
- `datePosted` - ISO date format (e.g., "2026-06-05")
- `title` - Job title
- `hiringOrganization.name` - Company name
- `jobLocation.address.addressLocality` - Location (city)
- `employmentType` - Job type (FULL_TIME, etc.)
- `industry` - Industry sector
- `skills` - Required skills
- `description` - HTML description (stripped to text)
- `validThrough` - Application deadline

### Pagination

- Follows `rel="next"` and `aria-label="Next"` links
- Also handles numeric pagination with `page=N` query params
- Bounded to `MaxPagesPerPath` (default 3) to prevent runaway scraping

### URL Handling

- Detail page URLs (`/jobs/...` not `/job/...`) are the canonical job links
- External ID is derived from URL slug as fallback if JSON-LD ID is missing
- All relative URLs are resolved against the appropriate base URL

## Adding a New Scraper

To add another scraper source:

1. Create `internal/scrapers/<source>/scraper.go` with:
   - `Config` struct with BaseURL, ListingPaths, MaxPagesPerPath, RequestDelay
   - `Scraper` struct with base fields (baseURL, listingPaths, maxPages, client, logger, requestMu, lastRequestAt)
   - `NewScraper(cfg Config, logger *slog.Logger) *Scraper`
   - `Source() string` returning stable source ID
   - `Scrape(ctx context.Context) ([]normalize.RawJob, error)`

2. Follow the pattern:
   - Use JSON-LD extraction if the source provides structured data
   - Fall back to HTML parsing for resilience
   - Use the shared `walk`, `attr`, `cleanText`, `textLines`, etc. helpers
   - Handle pagination conservatively

3. Add tests in `internal/scrapers/<source>/scraper_test.go`:
   - Test JSON-LD parsing
   - Test HTML fallback
   - Test pagination
   - Test bad status handling
   - Test defaults

4. Wire into `cmd/app/main.go`:
   - Add import
   - Construct scraper
   - Add to `[]ingest.Scraper` slice

5. If source-specific description normalization is needed, update:
   - `internal/ingest/normalize/overrides.go` - add case in `applySourceOverrides`

## Fuzu-Specific Decisions

- **Source ID**: "fuzu" - stable identifier used for dedupe and normalization overrides
- **Listing path**: `/kenya/job/computers-software-development` - focuses on tech jobs in Kenya
- **Max pages**: 3 - conservative to avoid overloading the site
- **Request delay**: 1 second - polite scraping behavior
- **External ID**: Prefer JSON-LD `identifier.value`, fallback to URL slug
- **PostedAt**: Prefer explicit `datePosted`, fallback to relative date parsing, then current time
- **Description**: Strip HTML tags, keep all content (requirements/qualifications are descriptive)

## Files Changed

- `internal/scrapers/fuzu/scraper.go` - Main scraper implementation
- `internal/scrapers/fuzu/scraper_test.go` - Test suite
- `cmd/app/main.go` - Wire Fuzu scraper into ingest service