# Current State

## Overview

`opportunity-radar` is currently a single Go service focused on ingesting job data, normalizing it, associating jobs with companies, scoring jobs, and persisting the results in PostgreSQL.

The project is still early, but the core ingest path is now taking shape:

- one scraper implementation exists: `remotive`
- raw jobs are normalized into internal models
- companies are resolved or created before jobs are saved
- jobs are scored with a weighted profile-driven rule-based scorer
- jobs and companies both have repository and service layers
- `cmd/app` can now run ingest on startup, continue on a daily scheduler, send a daily digest of top-scored jobs through Resend when configured, and serve a minimal admin/settings HTTP surface

## What Exists Today

### Application Composition

The composition root is [main.go](/home/francis/projects/my-repos/opportunity-radar/cmd/app/main.go).

It currently wires together:

- config loading
- structured logging
- PostgreSQL connection
- persisted app preferences/settings
- `companies` Postgres repository and service
- `jobs` Postgres repository and service
- ingest pipeline
- `remotive` scraper
- ingest service
- digest service
- digest runner/orchestrator
- scheduler
- admin HTTP handlers/routes
- signal-based shutdown context

The current app entrypoint now:

- builds the service graph
- creates a root context tied to `SIGINT` / `SIGTERM`
- starts a minimal admin HTTP server in the same process
- runs one ingest cycle immediately on startup by default
- runs the digest workflow after ingest in the same scheduled cycle
- continues periodic ingest runs every 24 hours by default
- shuts down cleanly when the process is stopped

The current runtime now also:

- bootstraps a persisted `app_settings` row on first run when one does not yet exist
- builds the scorer from persisted settings instead of hardcoded values alone
- builds digest runtime config from persisted settings
- keeps the admin/settings surface available even when the scheduler is disabled

The current run order for one scheduler cycle is:

1. scheduler triggers a run
2. runner executes ingest
3. runner executes the daily digest workflow

### Ingest Pipeline

The ingest flow lives mainly in:

- [pipeline.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/pipeline.go)
- [service.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/service.go)
- [default.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/normalize/default.go)
- [overrides.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/normalize/overrides.go)
- [description.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/normalize/description.go)
- [scraper.go](/home/francis/projects/my-repos/opportunity-radar/internal/ingest/scraper.go)

Current behavior:

1. A scraper returns `[]RawJob`.
2. The normalizer parses and trims the data into a `NormalizedJob`.
3. The pipeline asks the company service to `FindOrCreate` the company.
4. The pipeline builds a `jobs.Job`.
5. The scorer computes a score from keyword matches.
6. The job service saves the job.

Pipeline behavior is intentionally resilient:

- if normalization fails for a record, that record is skipped
- if company resolution fails, the job is still allowed through with a sentinel `company_id = 0` assumption
- if saving a job fails because it already exists, the job is skipped
- one bad scraper run does not stop the whole ingest service

### Company Normalization and Matching

This area was recently improved.

In the `internal/ingest/normalize` package:

- company names are normalized to a simpler fallback key
- names are lowercased
- punctuation is stripped
- whitespace is collapsed
- common company suffixes like `inc`, `llc`, `ltd`, `corp`, and `company` are removed
- the default normalizer is now conservative about company identity and does not infer company `domain` or `external_id` from job-level fields
- source-specific overrides are applied after shared normalization
- Remotive descriptions are converted from HTML to readable plain text
- Remotive descriptions can prepend source-specific metadata like `job_type` and `salary`
- company logos can flow through normalization when the source provides them

Example:

- `"Google Inc."` becomes `"google"`

In [service.go](/home/francis/projects/my-repos/opportunity-radar/internal/companies/service.go), `FindOrCreate` now checks for an existing company in sequence:

1. `source + external_id`
2. `domain`
3. `name`

This is a safer default for a multi-source ingest pipeline than stopping after the first available identity signal.

One important caveat remains:

- when a source does not provide trustworthy company-level identifiers, matching falls back to normalized name
- this is much safer than inferring from job URLs, but it is still an imperfect heuristic

## Domain Packages

### Jobs

The `internal/jobs` package is the most developed package right now.

It includes:

- `Job` model and status enum
- repository interface
- Postgres repository
- repository error mapping
- service layer
- service-level sentinel errors

Current service capabilities:

- `Save`
- `GetByID`
- `List`
- `Archive`
- `UpdateScore`

Important job behavior:

- `Save` defaults new jobs to `StatusActive`
- duplicate jobs are identified by `source + url`
- archived jobs are handled as a status transition, not deletion

### Companies

The `internal/companies` package now mirrors the same structure as `jobs`.

It includes:

- `Company` model
- repository interface
- Postgres repository
- repository error mapping
- service layer
- service-level sentinel errors

Current service capabilities:

- `Save`
- `GetByID`
- `List`
- `Delete`
- `FindOrCreate`

Current company identity strategy:

- strongest match: `source + external_id`
- cross-source fallback: `domain`
- weakest fallback: exact normalized `name`

In practice, the default normalizer now only populates company identity fields that are truly company-level data. Source-specific overrides can enrich `external_id`, `domain`, or `logo_url` later when a source provides reliable values.

## Persistence

Persistence is PostgreSQL-based and implemented with explicit SQL, not an ORM.

Relevant files:

- [postgres.go](/home/francis/projects/my-repos/opportunity-radar/internal/jobs/postgres.go)
- [postgres.go](/home/francis/projects/my-repos/opportunity-radar/internal/companies/postgres.go)
- [postgres.go](/home/francis/projects/my-repos/opportunity-radar/internal/digest/postgres.go)
- [migrations](/home/francis/projects/my-repos/opportunity-radar/migrations)

Current schema support includes:

- `companies` table
- `jobs` table
- `digest_deliveries` table
- `app_settings` table
- uniqueness on job `source + url`
- newer company fields: `source`, `external_id`, and `domain`
- uniqueness on digest delivery `recipient + digest_date`

Repository behavior is structured consistently:

- DB and driver errors are mapped to sentinel repository errors
- services translate those into business-meaningful service errors
- timestamps are managed in repository create/update paths

## Scoring

Scoring is now more intentional than the original flat keyword counter, but still deterministic and rule-based.

In [rules.go](/home/francis/projects/my-repos/opportunity-radar/internal/scoring/rules.go):

- scoring is profile-driven rather than a single flat keyword list
- title matches are weighted more heavily than description matches
- scoring considers role fit, skill fit, level fit, location fit, mismatch penalties, and freshness
- the scorer can be updated at runtime when profile settings change through the admin UI

This is still a heuristic scorer, but it is much closer to the current product goal of surfacing the most relevant opportunities from noisy inputs.

## Scrapers

There is currently one implemented scraper:

- [scraper.go](/home/francis/projects/my-repos/opportunity-radar/internal/scrapers/remotive/scraper.go)

The scraper:

- calls the Remotive API
- parses the JSON response
- converts response items into `normalize.RawJob`
- currently captures company logo, job type, salary, HTML description, and other core job fields

This establishes the current scraper contract and pattern for future source integrations.

## Shared Utilities

Shared infrastructure currently includes:

- [config.go](/home/francis/projects/my-repos/opportunity-radar/internal/shared/config/config.go) for env-based config
- [logger.go](/home/francis/projects/my-repos/opportunity-radar/internal/shared/logger/logger.go) for `slog` setup
- [scheduler.go](/home/francis/projects/my-repos/opportunity-radar/internal/scheduler/scheduler.go) for periodic ingest execution
- [service.go](/home/francis/projects/my-repos/opportunity-radar/internal/digest/service.go) for daily digest selection and send tracking
- [runner.go](/home/francis/projects/my-repos/opportunity-radar/internal/digest/runner.go) for sequencing ingest before digest
- [sender.go](/home/francis/projects/my-repos/opportunity-radar/internal/digest/sender.go) for digest sender implementations, including Resend and logging fallback
- [service.go](/home/francis/projects/my-repos/opportunity-radar/internal/preferences/service.go) for persisted app settings
- [handler.go](/home/francis/projects/my-repos/opportunity-radar/internal/preferences/handler.go) for the minimal admin/settings surface

Scheduler config is now environment-driven. Current settings include:

- `SCHEDULER_ENABLED` to switch between continuous scheduling and one-shot mode
- `SCHEDULER_INTERVAL` with a default of `24h`
- `SCHEDULER_RUN_ON_START` with a default of `true`
- `SCHEDULER_RUN_TIMEOUT` with a default of `30m`

Digest delivery infrastructure is still partially environment-driven. Current settings include:

- `RESEND_API_KEY`
- `RESEND_FROM_EMAIL`
- `RESEND_FROM_NAME`

Operator-facing digest preferences are now persisted in `app_settings` and used at runtime:

- digest enabled/disabled
- digest recipient
- digest top N
- digest lookback

Those digest preference values are still bootstrapped from env on first run if no `app_settings` row exists yet, but after bootstrap the app uses the persisted values.

Profile/scoring preferences are also persisted in `app_settings`, including:

- role keywords
- skill keywords
- preferred level keywords
- penalty level keywords
- preferred location terms
- penalty location terms
- mismatch keywords

Config loading behavior is now cleaner than before:

- missing or invalid env values return errors from config loading instead of panicking
- startup logs config failures and exits cleanly

The codebase consistently uses:

- constructor injection
- `context.Context`
- `log/slog`
- package-local interfaces where a consumer only needs a narrow contract

## App Behaviour

This section tracks the current runtime behavior of the app, especially the boundary between deployment-controlled behavior and user-editable preferences.

### Runtime Shape

The app now has two long-lived concerns running in the same process:

- the ingest/digest runtime
- a minimal admin HTTP server for setup and settings

The HTTP/admin surface currently exposes:

- `/`
- `/setup`
- `/settings/profile`
- `/settings/digest`

The admin pages are intentionally simple and server-rendered for now so they can later be replaced or enhanced by a more frontend-focused implementation without needing another backend redesign.

### Normal Run Modes

There are currently two runtime modes:

1. Scheduler-enabled mode
- controlled by `SCHEDULER_ENABLED=true`
- the app runs continuously
- the scheduler executes the normal cycle on its configured interval
- one cycle currently means: ingest first, then digest
- the admin HTTP server is also available while the scheduler runs

2. Scheduler-disabled mode
- controlled by `SCHEDULER_ENABLED=false`
- the app runs one ingest/digest cycle at startup
- after that one cycle, the app keeps the admin HTTP server alive instead of exiting immediately
- this is useful for setup, maintenance, and manual inspection, but it does not schedule future automatic runs

### What The User Can Change In-App

The user can currently change these in the admin UI:

- scoring/profile preferences
- digest enabled/disabled
- digest recipient email
- digest top N
- digest lookback

These changes are persisted in `app_settings`.

Profile and digest settings are also live-updated in memory:

- profile changes update the running scorer for future ingest runs
- digest changes update the running digest service for future digest runs

This means the user does not need to restart the app to make future scheduled cycles use updated settings.

One important caveat:

- changing profile settings does not automatically rescore jobs that are already stored in the database
- updated scoring applies to future ingest runs, not retroactive re-ranking of old jobs

### What Remains Deployment-Controlled

These are currently treated as deployment/runtime concerns and should not be normal user-editable UI settings:

- `DATABASE_URL`
- `RESEND_API_KEY`
- `RESEND_FROM_EMAIL`
- `RESEND_FROM_NAME`
- `SCHEDULER_ENABLED`
- `SCHEDULER_INTERVAL`
- `SCHEDULER_RUN_ON_START`
- `SCHEDULER_RUN_TIMEOUT`

Current product direction:

- scheduler status should be visible to the user
- scheduler enable/disable should remain deployment-controlled rather than being toggled in the UI

### Digest Behavior

Current digest behavior:

- if digest is disabled, the digest workflow logs and skips
- if digest is enabled but no recipient email is set, the digest workflow warns and skips
- if Resend is not configured, digest sending falls back to the logging sender instead of email delivery

The admin UI should make those states visible to the user.

Current user-visible digest warnings in the admin surface include:

- digest enabled but recipient missing
- digest enabled but Resend not configured, so digest output will only be logged

Digest selection behavior:

- digest uses the configured `TopN` as a maximum, not a requirement
- if fewer than `TopN` jobs are available, only the available jobs are included
- if no jobs are available in the lookback window, the digest is skipped rather than sending an empty digest

Digest duplicate-send behavior:

- sent digests are recorded in `digest_deliveries`
- duplicate sends for the same recipient and UTC day are skipped
- this protects against repeated sends during multiple runs on the same day

### Scheduler Visibility And Future UX

One known UX gap remains:

- if the scheduler is disabled, the current admin UI does not yet clearly warn the user that automatic future runs are off

This matters because a user could enable digest or update profile settings and reasonably expect future automated behavior even when `SCHEDULER_ENABLED=false`.

Planned direction:

- scheduler status should be explicitly shown in the UI
- if the scheduler is off, the UI should warn that settings are saved but automatic runs will not happen

### Manual Run Direction

Current decision direction:

- a future `Run Now` action is desirable
- this should likely trigger the same cycle as the scheduler: ingest then digest
- this is especially useful after a user changes profile or digest preferences and wants immediate effect

Current design decision:

- do not expose scheduler on/off as a normal UI control
- do expose scheduler status in the UI
- do add a future manual run action in the UI

That keeps deployment/runtime control separate from user preference editing while still giving the user a practical way to act immediately.

## Architectural Direction

The project is moving toward a clean layered structure:

- repositories own persistence concerns
- services own business meaning and translation of repository errors
- orchestration packages depend on small interfaces instead of concrete implementations
- concrete implementations are wired at the application boundary

This pattern is already visible in the ingest pipeline and in the jobs/companies packages.

## What Is Still Incomplete

Several pieces are present only as scaffolding or are not fully wired yet.

### Scheduler

The scheduler is now implemented in [scheduler.go](/home/francis/projects/my-repos/opportunity-radar/internal/scheduler/scheduler.go).

Current scheduler behavior:

- runs the ingest service through a small `Runner` interface
- triggers one immediate run on startup by default
- triggers recurring runs every 24 hours by default
- skips a tick if the previous run is still in progress
- can apply a per-run timeout through context
- logs run start, completion, duration, and failure
- stops when the application context is cancelled

### Daily Digest

The daily digest plumbing is now implemented in the `internal/digest` package.

Current digest behavior:

- runs after ingest in the same scheduled cycle
- selects recent active jobs by `created_at`
- ranks digest candidates by score descending, then recency
- enriches jobs with company names when available
- renders both text and HTML digest content
- records sent digests in `digest_deliveries` so one recipient does not get the same day’s digest twice
- sends through Resend when provider config is present
- falls back to a logging sender when Resend is not configured

Current provider behavior:

- real email delivery is implemented through Resend
- local and incomplete-config environments fall back to logging instead of failing hard
- the digest sender remains interface-based, so another provider can still be added later

### HTTP Layer

The following files exist but are mostly stubs:

- [handler.go](/home/francis/projects/my-repos/opportunity-radar/internal/jobs/handler.go)
- [routes.go](/home/francis/projects/my-repos/opportunity-radar/internal/jobs/routes.go)
- [handler.go](/home/francis/projects/my-repos/opportunity-radar/internal/companies/handler.go)
- [routes.go](/home/francis/projects/my-repos/opportunity-radar/internal/companies/routes.go)

There is not yet a real API or UI flow built on top of the services.

### App Runtime

`cmd/app` now sets up the service graph and starts the scheduler by default.

It still does not yet:

- expose HTTP endpoints

Useful runtime commands now live in the `Makefile`:

- `make run` to start the normal scheduler-enabled app
- `make run-once` to run ingest once with scheduling disabled
- `make run-scheduler-smoke` to run the app with a `5s` interval for local verification

### Tests

There are still few tests overall, but there is now focused coverage around Remotive description normalization, scheduler behavior, and digest selection/send tracking behavior.

The code currently passes `go test ./...`, but there is not yet meaningful automated coverage of:

- ingest behavior
- service error translation
- repository behavior
- most normalization edge cases

## Known Issues / Caveats

- Company names are currently stored in normalized form for matching, not preserved separately as a display/original name.
- Company fallback matching by exact normalized name is useful but still imperfect.
- The ingest pipeline assumes a sentinel unknown company record or `company_id = 0` fallback, but that behavior is not yet fully formalized in schema and application design.
- Scheduler shutdown is graceful for `SIGINT` / `SIGTERM`, but not for hard kills or process panics.
- Scheduler cancellation depends on downstream work respecting `context.Context`; a scraper or DB call that ignores cancellation may delay shutdown.
- The scheduler is currently in-process and memory-only: it does not persist last-run state, catch up missed runs after downtime, or coordinate across multiple app instances.
- Email delivery depends on correct Resend configuration and a verified sender identity; without that config the app intentionally falls back to logging mode.
- Digest idempotency is tracked per UTC day and recipient, which is fine for the current single-tenant model but will need revisiting if user time zones or multiple digests per day are introduced.

## Current Operational Picture

Today, the project is best described as:

- ingest core: partially implemented and coherent
- persistence layer: implemented for jobs and companies
- scoring: implemented at a basic level
- scraper support: one source implemented
- scheduler: implemented for single-process daily execution
- daily digest: implemented with persisted send tracking and Resend delivery support
- HTTP/UI: not implemented
- tests: still light overall, but scheduler and digest unit tests now exist

## Hosting Direction

The current hosting direction is intentionally single-tenant and self-hosted.

For now, the expected operating model is:

- one app instance per user/operator
- one Postgres database per app instance
- user-specific behavior controlled through environment variables
- no user table, auth, tenancy layer, or notification preferences system yet

The near-term goal is to make the app easy to clone, configure, and run locally or on a small host with minimal setup. Docker packaging, automatic migrations on startup, and a complete `.env.example` are expected next steps in support of that direction.

## Immediate Next Step

The scheduler, digest plumbing, and first email provider integration are now in place, so the next major feature is likely to be one of:

Natural next implementation areas now look like:

- exposing jobs and companies through handlers/routes
- adding stronger automated coverage for ingest and repository behavior
- formalizing the sentinel unknown-company behavior
- adding more scrapers and richer scoring logic

This file should be updated as those decisions are made and implemented.
