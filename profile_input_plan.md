# Profile Input Plan

## Goal

Replace hardcoded scoring preferences and user-facing digest settings with a persisted, editable single-user profile and settings system, introduced in safe stages.

This plan is intentionally incremental so the project can implement, test, and validate one layer at a time instead of building storage, HTTP, UI, and LLM features all at once.

## Product Direction

The long-term user experience should be:

- on first run, the app can detect whether setup is complete
- the user can visit a small admin/setup UI
- the user can describe what kinds of opportunities they want in plain language
- the app stores a structured profile that the scorer can use
- the user can later revisit settings and update preferences as goals change
- digest settings can also be edited in the app instead of being locked into env vars

The operator should not need to think in terms of raw keyword lists unless they want to.

## Stage 1: Define The Persisted Model

Goal:
- introduce a persisted model for profile and user-editable settings

Scope:
- define the data shape for scoring preferences
- define the data shape for digest preferences
- define setup/onboarding state

Suggested stored fields:
- preferred roles
- preferred skills
- preferred levels
- preferred locations
- avoid or penalty terms
- digest recipient email
- digest enabled flag
- digest top N
- digest lookback
- created at
- updated at

Notes:
- keep this single-tenant and simple
- one deployed app instance is expected to have one operator profile/settings set
- prefer app-owned persisted settings over env vars for user-facing preferences

## Stage 2: Make Runtime Read From Persisted Preferences

Goal:
- remove dependence on hardcoded profile values in `cmd/app`

Scope:
- add repository/service support for reading and writing app preferences
- build the scorer from persisted settings at startup
- if no preferences exist yet, the app should enter a setup-required state

Notes:
- no UI is required yet
- this stage separates the question of "where preferences come from" from "how the user edits them"
- this stage should be fully testable through repository/service and startup wiring tests

## Stage 3: Add A Minimal HTTP/Admin Surface

Goal:
- create an in-app place where the user can complete setup and later edit settings

Scope:
- add a small HTTP server inside the existing app
- expose a minimal admin surface for setup and editing

Suggested endpoints:
- `GET /setup`
- `POST /setup`
- `GET /settings/profile`
- `POST /settings/profile`
- `GET /settings/digest`
- `POST /settings/digest`

Notes:
- this fits the repo direction toward future HTTP/UI work
- the server should run in the same app process as the scheduler
- if setup is incomplete, `/setup` should be the primary entry point

## Stage 4: Build The First UI

Goal:
- let users express preferences in human terms instead of raw keywords

Scope:
- create a simple onboarding/settings UI
- collect both structured inputs and plain-language intent
- persist the resulting profile/settings

Suggested setup fields:
- what roles are you looking for
- what experience level are you at
- what skills do you already use
- what skills do you want to grow into
- which locations are okay
- remote, hybrid, or on-site preference
- what should be avoided
- digest email destination

Important:
- the same UI must support later editing
- a user should be able to revisit settings after the app has been running for days or weeks
- changing preferences should affect future scoring runs without requiring redeploy or env changes

Example future scenario:
- the user starts focused on junior Go backend roles
- later the user learns Python and AI
- the user updates profile settings in the app
- future ingest cycles score AI/Python roles appropriately

## Stage 5: Add Deterministic Profile Translation

Goal:
- reduce the burden of manually inventing keywords

Scope:
- add a translation layer that converts high-level user intent into structured scoring profile fields
- use curated mappings first

Examples:
- `backend` can expand into backend-related role terms
- `junior` can expand into early-career level terms
- `remote` can map into preferred location/work-mode terms

Notes:
- start deterministic before introducing LLM dependence
- keep the structured profile as the source of truth

## Stage 6: Add Optional LLM-Assisted Profile Generation

Goal:
- allow the user to describe goals in natural language and receive a proposed profile

Scope:
- user enters a plain-language description
- LLM proposes a structured scoring profile
- UI presents the generated result for review and editing
- user confirms before saving

Important:
- LLM output should be advisory, not the persisted truth by itself
- the saved structured profile remains the source of truth
- users must be able to inspect and edit generated results

## Stage 7: Add Explainability And Feedback

Goal:
- make ranking behavior easier to trust and tune

Scope:
- show why a job scored well or poorly
- expose matched and penalized signals in the UI

Examples:
- matched: backend, go, remote, junior
- penalized: senior, manager

Notes:
- explainability will make profile editing much easier later
- this is especially useful once LLM-assisted profile generation exists

## Current Env Settings To Move Into Persisted UI-Editable Settings

These are better treated as operator preferences than deployment config:

- `DIGEST_TO_EMAIL`
- `DIGEST_ENABLED`
- `DIGEST_TOP_N`
- `DIGEST_LOOKBACK`

These should eventually be editable in the app UI after initial setup.

## Current Env Settings To Keep As Deployment/Infrastructure Config

These should remain env/config driven:

- `DATABASE_URL`
- `RESEND_API_KEY`
- `RESEND_FROM_EMAIL`
- `RESEND_FROM_NAME`
- `SCHEDULER_ENABLED`
- `SCHEDULER_INTERVAL`
- `SCHEDULER_RUN_TIMEOUT`
- environment/logging settings

## Suggested Implementation Order

1. Persisted preferences/settings model and migration
2. Preferences repository/service
3. Startup wiring reads scorer profile from DB instead of hardcoded values
4. Minimal HTTP server
5. First-run setup page
6. Editable profile and digest settings pages
7. Deterministic profile expansion
8. Optional LLM-assisted generation
9. Scoring explainability

## Testing Strategy By Stage

### Stage 1
- migration tests
- repository/service tests for reading and writing profile/settings

### Stage 2
- startup wiring tests that prove the scorer is built from persisted preferences
- fallback/setup-required behavior tests when preferences do not exist

### Stage 3
- handler and route tests for setup and settings endpoints

### Stage 4
- manual first-run setup test
- manual edit-after-setup test
- persistence verification after UI changes

### Stage 5
- unit tests for deterministic profile expansion logic

### Stage 6
- parsing and validation tests for LLM-generated profile proposals
- fallback behavior tests when generation fails

### Stage 7
- scoring explanation tests to ensure displayed reasons match actual scoring logic

## Recommended Next Step

Start with Stages 1 and 2 only:

- define and persist profile/settings
- move scorer construction off hardcoded values
- prepare digest settings to move out of env-backed operator preferences

That gives the project a solid, testable foundation before introducing HTTP, UI, or LLM work.
