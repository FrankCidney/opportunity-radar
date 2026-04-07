# Architecture

This document explains how `opportunity-radar` is structured today, why certain decisions were made, where the current edges are, and what likely future directions look like.

It is intentionally different from the main README:
- `README.md` is for setup, deployment, and running the app
- `docs/ARCHITECTURE.md` is for technical reasoning, constraints, and evolution

## Product Model

`opportunity-radar` is currently designed as a self-hosted, single-user application.

That means:
- one deployed app instance is expected to serve one operator
- one PostgreSQL database belongs to that one operator
- deployment configuration is environment-driven
- operator preferences are persisted in the app database
- there is no account system, tenancy model, or shared hosted control plane

This constraint simplifies the system a lot:
- the scheduler can be process-local
- settings do not need per-user isolation logic
- auth and permissions are deferred
- deployment can stay practical and lightweight

## Why A Single Go Service

The app is one Go process that owns:
- the HTTP admin UI
- scheduled ingest runs
- digest generation and delivery
- source scraping
- normalization and scoring

This choice was made for simplicity and operability:
- fewer moving parts
- easier deployment on small hosts
- simpler debugging
- lower coordination overhead than multiple services or workers

At the current project stage, reliability and clarity matter more than high-scale separation.

## Why PostgreSQL

PostgreSQL is the database used by the app.

Why PostgreSQL was chosen:
- strong fit for structured application data
- good support for uniqueness constraints and indexed identity fields
- reliable transactional behavior
- easy local and hosted deployment options
- good long-term path if the app grows

The app uses explicit SQL instead of an ORM.

Why explicit SQL:
- the schema is still small and understandable
- queries are easy to reason about
- repository behavior stays visible and testable
- there is less hidden behavior than with ORM-driven persistence

## Runtime Shape

At startup, the app currently:
1. loads environment-based runtime config
2. opens a PostgreSQL connection
3. applies pending SQL migrations
4. loads or bootstraps persisted app settings
5. builds the scoring profile from settings
6. wires the scraper and ingest pipeline
7. starts the HTTP admin UI
8. runs ingest immediately if scheduler config says to do so
9. continues on the configured schedule

The scheduler, digest, and UI all run inside the same application process.

## High-Level Data Flow

The core ingest flow is:

1. scraper fetches external job data
2. scraper returns `normalize.RawJob`
3. shared normalization converts raw data into a canonical internal shape
4. source-specific normalization cleanup is applied when needed
5. company resolution tries to find or create the associated company
6. a `jobs.Job` is constructed
7. scoring computes a numeric score from the saved operator profile
8. the job is persisted

This keeps concerns separate:
- scraping fetches and extracts source data
- normalization cleans and standardizes it
- scoring decides relevance
- persistence stores the final result

## Package Structure

Important packages today:

- `cmd/app`
  Composition root and runtime startup

- `internal/shared/config`
  Environment-based runtime config loading

- `internal/shared/migrator`
  Startup migration runner that applies SQL migrations automatically

- `internal/ingest`
  Pipeline orchestration and scraper interfaces

- `internal/ingest/normalize`
  Shared and source-specific normalization logic

- `internal/scrapers/remotive`
  Remotive scraper

- `internal/scrapers/brightermonday`
  BrighterMonday scraper

- `internal/scoring`
  Rule-based scoring logic

- `internal/preferences`
  Persisted operator settings and the server-rendered admin UI

- `internal/jobs`
  Job model, repository, and service

- `internal/companies`
  Company model, repository, and service

- `internal/digest`
  Digest selection, rendering, delivery tracking, and sending

- `internal/scheduler`
  Periodic run orchestration

## Why The Pipeline Is Layered

The app intentionally avoids mixing concerns across layers.

For example:
- scraper code does not perform scoring
- scoring logic does not know about HTML
- repositories do not decide business rules
- the UI does not directly manipulate persistence details

This layering matters because scraper behavior is inherently unstable. By containing source-specific fragility inside scraper and normalization packages, the rest of the app remains comparatively stable.

## Scraping Strategy

The app currently has two sources:
- Remotive
- BrighterMonday

### Remotive

Remotive is simpler because it provides an API response that maps more directly into `RawJob`.

### BrighterMonday

BrighterMonday is more complex and currently uses a two-step scrape:

1. listing discovery
2. detail-page enrichment

Why two-step scraping was chosen:
- listing pages expose candidate jobs and pagination
- detail pages contain richer descriptions and metadata
- the richer detail is more useful for scoring and digest quality

The scraper is intentionally conservative:
- bounded pagination
- no high-concurrency crawling
- small delay between requests
- source-specific parsing stays local to the scraper package

## Normalization And Why It Exists

External job data is inconsistent across sources.

Normalization exists to:
- trim and standardize noisy input
- make source fields comparable
- produce a consistent internal job shape
- avoid polluting the rest of the app with source-specific quirks

There is a default normalization path plus source-specific overrides.

This is especially useful for:
- HTML-heavy descriptions
- source-specific metadata formatting
- conservative company identity handling

## Company Identity Strategy

Company resolution currently prefers stronger signals first:

1. `source + external_id`
2. `domain`
3. normalized company name

This is intentionally conservative.

Why:
- scraper-provided company identity is often incomplete or unreliable
- job-level URLs should not be treated as company identity
- a weak match is better as a last resort than as a default

This strategy is still heuristic-based and not perfect.

## Scoring Model

Scoring is currently rule-based, deterministic, and profile-driven.

It uses saved operator preferences such as:
- desired roles
- experience level
- locations
- work modes
- current skills
- growth skills
- avoid terms

Why rule-based scoring was chosen first:
- transparent behavior
- easier debugging
- easy to tune without model infrastructure
- no inference costs
- good enough for the current product stage

This is a practical starting point, not the end state.

## Settings Model

The app mixes two kinds of configuration on purpose:

### Environment config

Used for deployment/runtime concerns:
- database connection
- port
- scheduler behavior
- email sender credentials

### Persisted settings

Used for operator intent:
- role preferences
- experience level
- locations and work modes
- digest recipient
- digest lookback and top-N

Why this split exists:
- deployment concerns belong to infrastructure
- search preferences belong to the operator and should be editable in the UI

## UI Approach

The UI is server-rendered HTML.

Why:
- simple to deploy
- no frontend build pipeline required to run the app
- fast enough for the current needs
- suitable for an admin/operator console

This keeps the product focused on workflow and clarity rather than frontend complexity.

## Deployment Model

The app is now structured for:
- Docker Compose local/self-hosted deployment
- future Fly.io deployment using the same codebase

Why this model:
- one repository
- one binary
- one database
- straightforward update path

The app now applies migrations automatically on startup, which makes both deployment and future updates much easier.

## Current Constraints And Limitations

### Scraper fragility

Scrapers are the most fragile part of the system.

Examples:
- page structure changes can break HTML parsing
- source field labels can change
- job listings can move behind anti-bot systems
- public pages can disappear or be rate-limited

This is expected. Scrapers are adapters over systems we do not control.

### Source coverage is still small

Only two job sources currently exist.

That means:
- job coverage is still limited
- source outages affect result volume materially
- quality depends heavily on those few sources

### Scheduler is process-local

The scheduler lives inside the app process.

That is fine for the current single-user deployment model, but it means:
- missed uptime means missed runs
- the app is not using an external durable job queue
- long-running or blocked downstream calls can affect run timing

### No multi-user support

The app is not designed for multiple users today.

Missing pieces include:
- authentication
- user identity
- tenant-aware settings and jobs
- per-user digests
- authorization boundaries

### No retroactive rescoring

When profile settings change, existing jobs are not automatically rescored.

That is a conscious simplification for now.

### Limited observability

The app has logging, but not a full operational observability stack.

Things not yet present:
- metrics
- tracing
- alerting integrations
- structured run health dashboards

## Operational Risks

The most likely breakage points are:
- scraper parsing changes
- email provider misconfiguration
- database connectivity issues
- scheduler configuration mistakes
- deployment environments sleeping or stopping the process

For this reason, the app benefits from:
- clear logs
- bounded scraper behavior
- startup migration safety
- simple deployment topology

## Why There Is No LLM Layer Yet

An LLM-driven layer is a plausible future addition, but it is not the current foundation.

Why it is deferred:
- the current scoring needs transparency more than sophistication
- deterministic heuristics are cheaper and easier to debug
- the app still needs broader source coverage before more advanced ranking is the main bottleneck

That said, an LLM could become useful later for:
- friendlier onboarding input interpretation
- richer role and skill extraction
- summarizing long descriptions
- semantic ranking supplements
- better mismatch detection

If introduced, it should likely augment the rule-based system first rather than replace it all at once.

## Likely Future Improvements

Reasonable future directions include:

- adding more scrapers
  More job boards, ATS feeds, or other opportunity sources

- improving scraper resilience
  Better parser diagnostics, clearer zero-results warnings, and safer fallback behavior

- expanding scoring
  Better weighting, fresher ranking signals, and eventually semantic or LLM-assisted relevance

- improving onboarding UX
  Friendlier free-text inputs that translate into profile settings and scoring signals

- adding company discovery beyond active job posts
  Startup/product directories or company signal sources

- improving observability
  Metrics, structured run health, and better operator feedback

- supporting more deployment targets
  Still self-hosted, but easier across different providers

- exploring multi-user support later
  Only if the product direction changes enough to justify the added complexity

## Non-Goals For Now

Things intentionally not optimized right now:

- high-scale scraping
- generic scraping infrastructure for every possible source
- public multi-tenant SaaS
- distributed workers
- heavy frontend architecture
- machine-learning-first ranking

These are all possible someday, but they are not required to make the current app useful.

## Guiding Principle

The current architecture favors:
- clear boundaries
- low deployment friction
- practical self-hosting
- explainable scoring
- small, explicit building blocks

That bias is intentional.

At this stage, the app does not need to be the most abstract or the most scalable version of itself. It needs to be understandable, dependable, and easy to keep evolving.
