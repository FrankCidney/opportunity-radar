# Agents Guide

## Project Summary

`opportunity-radar` is a Go service for collecting, normalizing, scoring, and storing job opportunities and company leads.

The product goal is practical rather than generic:

- find junior or early-career backend/generalist roles, especially Kenya and remote
- surface small online-first companies worth outreach even if they are not actively hiring
- reduce search time by turning noisy external sources into a smaller ranked set of opportunities

This is currently a single Go application with PostgreSQL persistence, SQL migrations, a lightweight ingest pipeline, and room for future HTTP/UI work.

Right now, `cmd/app` runs a production-style in-process scheduler that checks persisted onboarding state before automatic runs, skips scheduled work until required setup fields are complete, then runs ingest followed by the daily digest workflow and continues daily by default. The digest can send through Resend when configured and falls back to logging otherwise. The app also now has a real server-rendered admin UI for onboarding, profile editing, email update settings, reset/clear actions, and manual runs. The current near-term implementation focus is adding richer ingest coverage, starting with a Kenya-local `Fuzu` scraper as the next planned source.

The current product direction is intentionally single-tenant and self-hosted:

- one deployed app instance is expected to serve one user/operator
- configuration is environment-driven rather than stored in a user/preferences system
- the near-term packaging goal is "clone repo, fill `.env`, run one command"
- multi-user SaaS concerns are explicitly deferred for now

## Repo Layout

- `cmd/app`: application composition root
- `cmd/test`: small manual test harness
- `internal/ingest`: raw input pipeline, scraper interfaces, normalization, orchestration
- `internal/jobs`: job domain model, repository, service, repository errors, service errors
- `internal/companies`: company domain model, repository, service, repository errors, service errors
- `internal/scoring`: scoring interfaces and rule-based implementation
- `internal/digest`: daily digest selection, rendering, send tracking, and orchestration
- `internal/scheduler`: periodic execution and runtime coordination
- `internal/scrapers/remotive`: concrete scraper implementation
- `internal/scrapers/brightermonday`: concrete scraper implementation
- `internal/shared/config`: environment/config loading
- `internal/shared/logger`: `slog` setup
- `migrations`: schema evolution

## Architectural Shape

The codebase is trending toward small layered packages with explicit dependencies:

1. Scrapers fetch external data and return `normalize.RawJob`.
2. The normalizer converts `RawJob` into an internal `NormalizedJob`.
3. The ingest pipeline asks a company service to resolve or create the company.
4. The pipeline builds a `jobs.Job`, scores it, and saves it through the job service.
5. Repositories handle storage concerns and map DB/driver failures into repository-level sentinel errors.
6. Services translate those lower-level errors into business-meaningful service errors and log unexpected failures.

The important design bias is interface-driven composition:

- consumers depend on small interfaces, not concrete implementations
- concrete Postgres repositories are wired in at the application boundary
- services encapsulate business rules and error translation
- swapping implementations should mostly happen at construction time, not through package-level globals

`internal/ingest` is the clearest example of this. It depends only on:

- `JobService` with `Save`
- `CompanyService` with `FindOrCreate`
- `scoring.Scorer`
- `Scraper`

That makes the ingest pipeline easy to test and easy to rewire later.

`internal/scheduler` follows the same idea. It depends on a narrow runner interface with `RunAll(context.Context) error`, leaving ingest details inside `internal/ingest` and runtime wiring inside `cmd/app`.

`internal/digest` is the current application-level follow-up workflow after ingest. It owns digest-specific behavior such as selecting recent top-scored jobs, rendering content, recording delivery state, and hiding the email sender behind an interface.

`internal/preferences` now owns the real operator-facing settings model plus the server-rendered admin UI. It persists friendly UI inputs such as desired roles, experience level, locations, work modes, and email-update settings, and it derives the lower-level scoring profile from those fields.

`internal/runcontrol` now owns shared run coordination and run status. Both scheduled runs and manual `Run Once` actions go through the same single-run guard so overlapping runs are skipped rather than queued.

## Dependency Direction

Prefer this dependency direction when adding features:

- shared utilities -> domain packages -> orchestration/composition
- repositories -> services -> handlers/routes
- concrete implementations should satisfy interfaces owned by the consuming package when practical

Avoid letting orchestration packages reach into database details directly.

## Current Domain Pattern

The `jobs` and `companies` packages should follow the same internal structure:

- `model.go`: domain structs and enums
- `repository.go`: repository interface plus list filters
- `repository_errors.go`: infrastructure/persistence sentinel errors
- `postgres.go`: Postgres-backed implementation
- `service.go`: business logic and translation layer
- `service_errors.go`: business-facing sentinel errors
- `handler.go` / `routes.go`: HTTP layer, currently mostly stubbed

When extending one of these packages, preserve that split unless there is a strong reason not to.

## Error Handling Conventions

This repo uses layered error meaning:

- repository errors answer: "what happened in persistence?"
- service errors answer: "what does this mean to the caller?"

Typical repository errors:

- `ErrNotFound`
- `ErrConflict`
- `ErrReferenceNotFound` in packages that need foreign-key semantics
- `ErrTimeout`
- `ErrInternal`

Typical service errors:

- `ErrJobNotFound`, `ErrJobAlreadyExists`
- `ErrCompanyNotFound`, `ErrCompanyAlreadyExists`
- `ErrServiceInternal`

Guidelines:

- repositories should wrap sentinel errors with operation context using `fmt.Errorf("%s: %w", op, err)`
- services should use `errors.Is` to translate repository errors
- services should log unexpected errors with structured fields
- handlers should eventually map service errors to HTTP responses
- do not leak raw DB errors out of the repository layer

## Service Layer Conventions

Service responsibilities in this repo:

- apply business defaults
- enforce simple business rules
- translate repository errors into service-level meaning
- log unexpected failures
- keep the public API small and explicit

Examples already present:

- `jobs.Service.Save` sets status to `StatusActive` before create
- `jobs.Service.List` caps/defaults list limits
- `jobs.Service.Archive` enforces status transition rules
- `companies.Service.FindOrCreate` resolves identity using the strongest available filter before creating

When adding a new use case, prefer putting policy in the service layer rather than in handlers or repository code.

## Repository Conventions

Repository implementations are intentionally explicit SQL, not ORM-driven.

Patterns to preserve:

- define an `op` string per method for error wrapping/logging
- map Postgres and context errors in one place (`mapError`)
- stamp `CreatedAt` / `UpdatedAt` in repository create/update paths
- use `RowsAffected` to detect missing rows on update/delete
- keep list query construction simple and readable with filter structs

If you add another persistence backend later, it should satisfy the same repository interface and preserve the sentinel error semantics.

## Coding Style

The code style is simple, direct, and explicit:

- small structs with constructor functions like `NewService`, `NewPostgresRepository`, `NewScraper`
- constructor injection over package globals
- `context.Context` is passed through I/O and orchestration paths
- `log/slog` for structured logging
- comments are used mainly to explain intent, not every line
- explicit field assignment is preferred when creating domain values
- the project currently values readability over clever abstractions

When editing, prefer:

- straightforward control flow
- low ceremony interfaces
- narrow, package-local abstractions
- naming that reflects domain intent rather than framework jargon

When editing the new UI/settings flow specifically, prefer:

- storing friendly, user-facing intent in the settings model
- deriving scorer-specific keyword buckets from that friendly input
- keeping scheduler control deployment-owned and only exposing scheduler status in the UI
- keeping the home page honest: if setup is incomplete, redirect to onboarding rather than showing a misleading dashboard

## UI Styling Baseline

For future UI work, treat `ui-prototype/preview` as the current visual baseline unless the user explicitly asks to change direction.

The styling direction there is:

- clean, modern, product-like rather than scaffold-like
- light theme with soft slate surfaces and indigo accents
- Inter-style sans-serif typography
- restrained card-based layout with generous spacing
- simple tab-like navigation and pill/chip patterns
- subtle shadows, rounded corners, and clear section hierarchy
- plain-language labels such as `Email Updates` instead of more internal/backend wording where possible
- responsive layout that works on desktop and mobile without heavy frontend tooling

When implementing or refining the real app UI, preserve the feel of that prototype:

- calm and readable
- polished but not flashy
- clear status visibility
- simple forms with good defaults
- obvious hover/focus states for buttons, links, tabs, and form controls

Unless the user asks otherwise, future UI integration work should aim to carry that same visual language into the real app rather than reverting to the older `ui-prototype/stage4` look.

## Data Flow Notes

Ingest currently looks like this:

- scraper fetches remote jobs
- shared normalization trims/parses values and builds conservative company data
- source-specific normalization overrides are applied afterward when needed
- pipeline calls `CompanyService.FindOrCreate`
- pipeline creates a `jobs.Job`
- scorer computes a numeric score from a derived scoring profile
- job service persists the record, skipping duplicates via service error handling

This means company identity is currently derived from:

1. `source + external_id` if available
2. `domain`
3. `name`

That ordering is intentional, but the default normalizer no longer infers company `external_id` or `domain` from job-level fields. Source-specific overrides should only populate those fields when the source provides trustworthy company-level data.

Current Remotive-specific behavior worth knowing:

- the scraper now fetches the broad Remotive public jobs feed rather than hardcoding `software-dev`
- descriptions are converted from HTML to readable plain text
- `job_type` and `salary` are prepended to the description when present instead of getting their own DB columns
- `company_logo` is captured and mapped into `companies.logo_url`

Current BrighterMonday-specific behavior worth knowing:

- the scraper is a two-step HTML scraper: listing discovery plus detail-page enrichment
- pagination is bounded and requests are intentionally conservative
- source-specific description cleanup is handled through normalization overrides

Current Fuzu implementation planning worth knowing:

- the next planned local source is `Fuzu`
- reconnaissance suggests Fuzu should also use a two-step `listing page -> detail page` approach
- the preferred first listing path is `https://www.fuzu.com/kenya/job/computers-software-development`
- implementation notes and checklist live in [fuzu_implementation.md](/home/frawuor/projects/personal/opportunity-radar/fuzu_implementation.md)

Scheduler/runtime behavior worth knowing:

- the scheduler owns timing, run coordination, logging, and shutdown behavior only
- the scheduler does not know about scraper-specific details
- by default the app attempts one automatic cycle on startup and then repeats every `24h`
- automatic scheduled runs are skipped until required onboarding fields are complete
- overlapping runs are skipped rather than queued
- each run can be bounded by a configurable timeout
- graceful shutdown is driven from a root context in `cmd/app`
- scheduled runs now share a run coordinator with manual runs from the UI
- if a run is already in progress, the first version does not queue another run

Digest/runtime behavior worth knowing:

- the scheduler does not call ingest directly anymore; it calls a small runner/orchestrator
- one run cycle is currently: ingest first, then daily digest
- the digest selects recent active jobs ordered by score descending
- sent digests are recorded in Postgres to avoid duplicate sends for the same recipient and UTC day
- the digest currently supports Resend as the concrete email provider
- when Resend config is missing, the app intentionally falls back to a logging sender
- digest/email-update preferences are persisted in `app_settings`
- if no matching jobs are found, the app now still sends a "no new jobs" status email rather than silently skipping delivery

Preferences/UI behavior worth knowing:

- onboarding now captures both the optional email destination and the optional "send me email updates" choice
- profile settings can be edited after setup and can now be cleared/reset from the UI
- email update settings can be edited after setup and can now be cleared/reset from the UI
- clearing profile settings makes setup incomplete again and blocks future automatic runs until required fields are filled back in
- clearing email update settings disables digest delivery, clears the recipient, and resets digest defaults

- the real app UI now uses the `ui-prototype/preview` layout and styling as its baseline
- `/` redirects to `/setup` until required onboarding fields are complete
- `SetupComplete` is derived from required fields rather than treated as a free-form flag
- current required fields are: at least one role, one experience level, at least one location, and at least one work mode
- changing profile settings updates future scoring runs but does not automatically rescore already-saved jobs
- current real UI routes are:
  - `/`
  - `/setup`
  - `/settings/profile`
  - `/settings/profile/edit`
  - `/settings/digest`
  - `/run-once`

## Practical Guidelines For Future Agents

- Start by reading the package you are changing and the adjacent package it collaborates with.
- When adding a new behavior, first decide which layer owns it: scraper, normalizer, service, repository, or composition root.
- If you need a new dependency, inject it through constructors instead of reaching for globals.
- If a consumer only needs one or two methods, prefer a small interface over a concrete dependency.
- Mirror existing package patterns before introducing a new one.
- Keep `cmd/app` as the place where concrete implementations are wired together.
- Keep migrations in sync with model and repository expectations.
- Preserve the single-tenant self-hosted assumption unless the user explicitly asks to design for multi-user support.
- Favor environment-driven configuration and packaging-friendly runtime behavior over introducing auth, tenancy, or preference systems prematurely.

## Known Rough Edges

Some parts of the repo are still in-progress:

- parts of the old stub HTTP packages still exist outside the new preferences UI flow
- tests are still light overall even though there is now focused coverage around the Remotive scraper, normalization, scheduler, and digest packages
- formatting and alignment are slightly inconsistent in some files
- comments sometimes describe intent better than naming, which is helpful now but may need cleanup as the code matures
- the new friendly-input translation layer is still heuristic and can likely be improved
- the real UI has been integrated, but there is still room to refine copy, validation, and visual polish

Treat these as signs of an evolving codebase, not reasons to bypass the existing architectural direction.

## Safe Defaults When Contributing

If you are unsure how to implement something, the safest default is:

1. add or update an interface only where a consumer truly needs it
2. keep persistence-specific behavior inside repository implementations
3. keep business meaning inside services
4. use structured logs for unexpected failures
5. return sentinel errors that callers can reliably match with `errors.Is`
6. prefer a small, testable change over a broad refactor

## Good First Files To Read

For quick orientation, start with:

- `README.md`
- `cmd/app/main.go`
- `internal/ingest/pipeline.go`
- `internal/digest/service.go`
- `internal/digest/runner.go`
- `internal/runcontrol/coordinator.go`
- `internal/preferences/model.go`
- `internal/preferences/mapping.go`
- `internal/preferences/handler.go`
- `internal/ingest/normalize/default.go`
- `internal/ingest/normalize/overrides.go`
- `internal/ingest/normalize/description.go`
- `internal/jobs/service.go`
- `internal/jobs/postgres.go`
- `internal/companies/service.go`

Those files show most of the project's current architectural intent.
