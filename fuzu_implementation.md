# Fuzu Implementation Plan

## Purpose

This file is the implementation handoff for adding `Fuzu` as a new local scraper source in `opportunity-radar`.

It is written to be usable in a fresh chat without relying on prior conversation context.

The immediate goal is:

- add a Kenya-local `Fuzu` scraper
- keep the scraper aligned with the current ingest architecture
- preserve the codebase pattern where the scraper only fetches and extracts source data, while normalization, company resolution, scoring, dedupe, and persistence stay in shared layers

## Current Project Context

`opportunity-radar` is a single-user, self-hosted Go application that:

- scrapes job opportunities from configured sources
- normalizes them into a shared internal shape
- resolves or creates companies
- scores jobs against persisted user preferences
- stores them in PostgreSQL
- runs scheduled ingest and digest workflows from the same Go process

Relevant wiring today:

- [cmd/app/main.go](/home/frawuor/projects/personal/opportunity-radar/cmd/app/main.go)
- [internal/ingest/service.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/service.go)
- [internal/ingest/pipeline.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/pipeline.go)
- [internal/ingest/scraper.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/scraper.go)
- [internal/ingest/normalize/default.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/normalize/default.go)
- [internal/ingest/normalize/overrides.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/normalize/overrides.go)

The existing concrete scrapers are:

- [internal/scrapers/remotive/scraper.go](/home/frawuor/projects/personal/opportunity-radar/internal/scrapers/remotive/scraper.go)
- [internal/scrapers/brightermonday/scraper.go](/home/frawuor/projects/personal/opportunity-radar/internal/scrapers/brightermonday/scraper.go)

### Current Scraper Integration Flow

The scraper flow to keep in mind when adding `Fuzu` is:

1. `cmd/app/main.go` constructs concrete scraper instances.
2. `main.go` passes those into `ingest.NewService(...)` as `[]ingest.Scraper`.
3. `internal/ingest/service.go` loops over scrapers and runs them one by one.
4. `internal/ingest/pipeline.go` calls `scraper.Scrape(ctx)`.
5. The scraper returns `[]normalize.RawJob`.
6. Shared normalization turns `RawJob` into `NormalizedJob`.
7. The pipeline resolves the company with `CompanyService.FindOrCreate`.
8. The pipeline builds `jobs.Job`.
9. The scorer computes the job score.
10. The job service saves the record and skips duplicates by service-level duplicate handling.

This means the new Fuzu scraper should be responsible only for:

- discovering Fuzu jobs
- extracting source fields
- mapping them into `normalize.RawJob`

It should not do:

- scoring
- persistence
- company matching policy
- scheduler logic

## Why Fuzu

The current focus is adding useful Kenya-local sources for a profile centered on:

- junior or early-career backend work
- Go/backend/PostgreSQL
- engineering-focused startups and technical companies
- remote-friendly or strong engineering culture roles

`Fuzu` was chosen as the next local source because it appears to have:

- meaningful Kenya-local job volume
- public listing pages for software and technical jobs
- detail pages with better job descriptions than the listing pages alone
- enough structure to support a conservative scraper

## Reconnaissance Summary

The safest first implementation shape for `Fuzu` is:

- listing page discovery
- pagination through result pages
- per-job detail page enrichment
- map into `normalize.RawJob`

This should follow the general shape of the existing `BrighterMonday` scraper rather than the simpler single-request `Remotive` scraper.

### Pages Reviewed

The following public pages were reviewed during reconnaissance:

#### Listing pages

- `https://www.fuzu.com/kenya/job/computers-software-development`
- `https://www.fuzu.com/kenya/job/software-engineer`
- `https://www.fuzu.com/kenya/job/computers-software-development/nairobi/full-time`

#### Detail pages

- `https://www.fuzu.com/kenya/jobs/full-stack-developer-laravel-kotlin-an-on-demand-services-platform-connecting-skilled-tradespeople-with-customers`
- `https://www.fuzu.com/kenya/jobs/assistant-software-developer-christian-health-association-of-kenya`
- `https://www.fuzu.com/kenya/jobs/engineer-backend-microservices-safaricom`
- `https://www.fuzu.com/kenya/jobs/devops-platform-network-infrastructure-administrator-speedy-chui-africa-limited`

### What the listing pages appear to expose

From the reviewed listing pages, the scraper can expect to find:

- job title
- location
- link to the job detail page
- pagination controls like `Prev`, page numbers, and `Next`
- sometimes badges like `Only on Fuzu`

Examples seen:

- `Full Stack Developer (Laravel & Kotlin)`
- `Assistant Software Developer`
- `Engineer - Backend Microservices`
- `DevOps Platform & Network Infrastructure Administrator`

### What the detail pages appear to expose

The detail pages are more useful than the listing pages and should likely be treated as the source of truth for most fields.

From the reviewed detail pages, the scraper can often extract:

- company name
- job title
- location
- free-text description
- responsibilities
- qualifications
- tags
- sometimes an explicit posted date like `Posted: Jan 23, 2026`
- sometimes an application deadline like `Apply by: Feb 2, 2026`
- sometimes an `Only on Fuzu` badge

### Important reconnaissance conclusions

1. `Fuzu` should be implemented as a two-step scraper.
   The listing pages are good for discovery, but the detail pages are much better for description quality and metadata extraction.

2. The first implementation should start from one listing path only.
   Recommended starting path:
   `https://www.fuzu.com/kenya/job/computers-software-development`

3. Search-engine-discovered Fuzu URLs looked noisy in places.
   During reconnaissance, some search results appeared to resolve to mismatched or confusing content. The scraper should not rely on external search results. It should traverse Fuzu directly from known listing pages and only follow on-site job links.

4. Some fields are clearly available, while others need a conservative fallback strategy.

## Recommended Scope For Version 1

### Include in version 1

- one listing path:
  `/kenya/job/computers-software-development`
- pagination support
- detail-page fetch for each discovered job
- conservative dedupe by canonical detail page URL
- extract enough fields for a useful `normalize.RawJob`
- tests covering parse behavior and failure cases

### Defer from version 1 unless trivial

- multiple Fuzu listing-path variants
- advanced company-logo extraction if the signal is weak
- rich job-type inference if the page does not expose it cleanly
- deadline persistence beyond optional `RawData`
- aggressive filtering inside the scraper based on role keywords

## Expected Field Mapping

The new scraper should aim to populate these `normalize.RawJob` fields:

- `Source`: `"fuzu"`
- `Title`
- `Company`
- `ExternalID`
- `Location`
- `JobType`
- `Salary`
- `Description`
- `URL`
- `PostedAt`
- `RawData`

### Fields likely safe to populate

- `Source`
- `Title`
- `Company`
- `Location`
- `URL`
- `Description`

### Fields that need careful extraction or fallback

- `ExternalID`
  If no structured ID is exposed, derive it from the canonical URL path slug.

- `PostedAt`
  Prefer explicit `Posted: ...` text from the detail page when available.
  If not available, decide during implementation whether to:
  - skip the job if date is required and no safe value exists, or
  - derive a conservative fallback only if there is a clearly parseable relative date on the listing page.

- `JobType`
  Populate only if Fuzu exposes it clearly. Otherwise leave empty.

- `Salary`
  Populate only if clearly visible and trustworthy.

- `RawData`
  Good candidates:
  - `apply_by`
  - `only_on_fuzu`
  - `tags`
  - `seniority`
  - any listing-page metadata that might be useful later

## Implementation Checklist

### 1. Create the scraper package

Add:

- `internal/scrapers/fuzu/scraper.go`
- `internal/scrapers/fuzu/scraper_test.go`

Follow the existing scraper package pattern.

### 2. Define scraper config and defaults

Create a `Config` type if needed, likely with:

- `BaseURL`
- `ListingPaths`
- `MaxPagesPerPath`
- `RequestDelay`

Reason:

- this matches the `BrighterMonday` style
- it keeps the Fuzu scraper testable
- it gives room for later path expansion

### 3. Implement constructor pattern

Follow the same basic pattern used by existing scrapers:

- exported `NewScraper(...)`
- internal `newScraper(...)` for testability
- default HTTP client with timeout
- default logger fallback if needed

### 4. Implement `Source() string`

Return:

- `"fuzu"`

Keep the source ID stable because it is part of downstream identity and persistence behavior.

### 5. Implement listing discovery

The scraper should:

- request the configured listing page
- parse listing entries
- extract detail page URLs
- capture any useful listing metadata
- follow pagination conservatively
- dedupe discovered URLs

Recommended starting path:

- `/kenya/job/computers-software-development`

### 6. Implement detail-page parsing

For each discovered detail URL:

- fetch the page
- parse the title
- parse company
- parse location
- parse description
- parse responsibilities and qualifications if present
- combine those sections into one clean description string if needed
- parse posted date if present
- optionally parse apply-by date into `RawData`

### 7. Decide canonical URL handling

Use the detail page URL as the job URL.

Normalize or resolve relative links so the final `URL` is absolute and stable.

### 8. Decide external ID derivation

If Fuzu does not expose a separate stable ID:

- derive `ExternalID` from the URL path slug

For example, from:

- `/kenya/jobs/assistant-software-developer-christian-health-association-of-kenya`

the slug itself may be the safest fallback ID.

### 9. Map to `normalize.RawJob`

Build `normalize.RawJob` with only trustworthy data.

Do not invent company-level identifiers unless Fuzu clearly exposes them.

### 10. Add source-specific normalization only if needed

Check whether Fuzu descriptions need cleanup beyond default normalization.

If they do:

- update [internal/ingest/normalize/overrides.go](/home/frawuor/projects/personal/opportunity-radar/internal/ingest/normalize/overrides.go)
- add a Fuzu-specific description normalizer if necessary

If not:

- do not add an override just for symmetry

### 11. Wire the scraper into `main`

Update [cmd/app/main.go](/home/frawuor/projects/personal/opportunity-radar/cmd/app/main.go):

- import `internal/scrapers/fuzu`
- construct a Fuzu scraper instance
- include it in the `[]ingest.Scraper` slice passed to `ingest.NewService(...)`

### 12. Decide whether `cmd/app` needs Fuzu config helpers

BrighterMonday has:

- [cmd/app/brightermonday_config.go](/home/frawuor/projects/personal/opportunity-radar/cmd/app/brightermonday_config.go)

For Fuzu version 1:

- a static listing path is probably enough
- only create a `fuzu_config.go` helper if path derivation from preferences is genuinely needed

### 13. Add tests before trusting the scraper

Model the tests after:

- [internal/scrapers/remotive/scraper_test.go](/home/frawuor/projects/personal/opportunity-radar/internal/scrapers/remotive/scraper_test.go)
- [internal/scrapers/brightermonday/scraper_test.go](/home/frawuor/projects/personal/opportunity-radar/internal/scrapers/brightermonday/scraper_test.go)

At minimum, test:

- listing page parsing
- detail page parsing
- pagination behavior
- bad status handling
- malformed HTML or missing required fields
- URL dedupe behavior
- `ExternalID` derivation
- `PostedAt` parsing

### 14. Update implementation notes during the work

During the actual implementation session, create and maintain:

- `notes.md`

That file should explain:

- how `main.go` wires scrapers
- how the ingest service and pipeline use them
- what a scraper is expected to return
- where normalization fits
- where company resolution, scoring, and persistence happen
- the practical checklist for adding the next scraper after Fuzu

## Detailed Step-By-Step Plan

### Phase 1: Reconfirm live page shape

Before coding, re-check the live Fuzu listing and detail pages to ensure the HTML structure still matches the reconnaissance summary.

Reconfirm at least:

- the listing page contains job links
- the listing page contains pagination
- the detail page contains title, company, location, and body content
- explicit posted dates are present often enough to use

If the page shape changed materially, update this plan first.

### Phase 2: Build parsing helpers in isolation

Implement parsing in a way that can be exercised through tests with fixture-like inline HTML, similar to the BrighterMonday tests.

Suggested internal helpers:

- listing parser
- detail parser
- canonical URL resolver
- external ID derivation helper
- posted date parser

### Phase 3: Implement the networked scrape flow

Flow:

1. fetch listing page
2. parse job candidates and next page
3. repeat until max pages or no next page
4. fetch each detail page
5. parse detail data
6. convert to `normalize.RawJob`
7. return the collected slice

### Phase 4: Keep behavior conservative

Important behavior rules:

- bounded pagination
- modest timeout
- modest request delay if multiple pages are fetched
- no unnecessary concurrency in version 1
- log and skip detail-page failures where reasonable
- avoid one broken page killing the whole source unless the listing request itself fails

### Phase 5: Decide description composition

Fuzu detail pages appear to break content into sections like:

- `Description`
- `Responsibilities`
- `Qualifications`

A likely approach is to combine them into one readable plain-text description, preserving section labels only if they help readability.

The goal is to improve downstream scoring quality without storing messy duplicated content.

### Phase 6: Wire into the app and verify end-to-end

After parser and scraper tests look good:

- wire Fuzu into `cmd/app/main.go`
- run targeted tests
- run broader tests if practical
- manually inspect logs or one controlled run if a safe local validation path exists

### Phase 7: Document what was learned

As part of the implementation session:

- write `notes.md`
- record any Fuzu-specific gotchas
- note which fields were reliable and which were not
- note how to replicate the same process for the next scraper

## Open Decisions To Resolve During Implementation

These should be resolved explicitly while coding:

1. How often does Fuzu expose an explicit `Posted:` date on the detail page?
2. If explicit posted date is missing, what is the fallback policy?
3. Is there a stable company logo signal worth extracting?
4. Is there a clearly exposed job type field worth mapping?
5. Should responsibilities and qualifications be concatenated into `Description` or stored partly in `RawData`?
6. Is a static single listing path enough for version 1, or does a second Fuzu path materially improve coverage?

## Recommended Initial Acceptance Criteria

The first Fuzu implementation should be considered successful if:

- a new `internal/scrapers/fuzu` package exists
- it satisfies `ingest.Scraper`
- it scrapes at least one stable Kenya-local Fuzu listing path
- it follows pagination conservatively
- it enriches jobs from detail pages
- it maps trustworthy data into `normalize.RawJob`
- it is wired into `cmd/app/main.go`
- it has tests covering key parsing and scrape behavior
- `notes.md` is created and documents the code path for future scraper additions

## Short Summary For A Fresh Chat

If starting in a new chat, the task is:

- implement a new Kenya-local `Fuzu` scraper
- use a `listing page -> detail page` strategy
- start from `https://www.fuzu.com/kenya/job/computers-software-development`
- keep the scraper source-specific and return `[]normalize.RawJob`
- wire it into `cmd/app/main.go`
- add tests similar to the existing `BrighterMonday` and `Remotive` scraper tests
- create `notes.md` during implementation to explain the end-to-end scraper integration flow through the codebase
