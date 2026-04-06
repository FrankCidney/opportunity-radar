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

### Stage 4 Sub-Steps And Remaining Integration Work

#### 4A. Real UI Integration
- integrate the chosen `ui-prototype/preview` layout into the real app
- preserve the current backend route responsibilities while replacing the placeholder Stage 3 presentation layer
- keep the visual direction from the preview prototype as the styling baseline

#### 4B. Define What "Setup Complete" Means
- decide which onboarding fields are required for setup completion
- make this explicit in both the backend and the UI
- avoid a vague or inconsistent `SetupComplete` state

Important current decision:
- not every field needs to be mandatory immediately during prototype work
- but the real app should eventually have a clear rule for when onboarding is complete

#### 4C. Map Friendly UI Inputs To Real App Settings
- decide how the user-friendly UI controls map to the persisted settings model and scoring inputs
- confirm what stays as a simple UI control versus what becomes first-class persisted data

This especially affects:
- experience level
- work mode selection
- country/location selection
- user-entered role/skill intent

#### 4D. Surface Runtime Status Clearly
- show scheduler status in the real UI
- show whether setup is complete
- show whether email updates are enabled
- show whether email delivery is configured or log-only

Important current product decision:
- scheduler status should be visible
- scheduler enable/disable should remain deployment-controlled, not a normal UI toggle

#### 4E. Define The Real `Run Now` UX
- add a manual `Run Now` action in the real UI
- make clear that it should trigger the same cycle as scheduled execution: ingest first, then email updates
- show that using `Run Now` does not turn the scheduler off or alter future scheduled behavior

Important follow-up questions to resolve during implementation:
- what happens if a scheduled run is already in progress
- how success/failure/progress should be communicated back to the user
- whether the first version needs a queued/running/completed state in the UI

#### 4F. Call Out Current Scoring Limitation In The UI
- changing profile settings updates future scoring runs
- existing persisted jobs are not automatically rescored

The UI should avoid implying that profile edits instantly rerank historical jobs unless or until a real rescore feature exists.

Current status:
- Stage 4 is now substantially implemented
- the real app uses the `ui-prototype/preview` layout and styling baseline
- the real app now has onboarding, profile summary/editing, email update settings, and a real `Run Once` action
- `/` redirects to `/setup` until required onboarding fields are complete
- required fields are currently:
  - at least one target role
  - one experience level selection
  - at least one location selection
  - at least one work mode selection
- scheduler status is visible in the real UI
- scheduler enable/disable remains deployment-controlled
- manual runs and scheduled runs now share a single-run guard
- if email updates are enabled and no matching jobs are found, the app now still sends a status email saying so

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

### Stage 5 Sub-Steps And Translation Decisions

#### 5A. Translate Friendly Inputs Into Scoring Inputs
- add a deterministic mapping layer from UI-facing choices into the scoring profile the backend actually uses
- keep the user experience high-level while the scorer continues to work with explicit profile terms

Examples:
- a role choice like `backend engineer` can expand into backend-related role keywords
- an experience level choice like `Junior / early-career` can expand into preferred level and penalty level terms
- a work mode choice like `Remote` can map into preferred location/work-mode terms
- country selections can map into preferred location terms

#### 5B. Curate And Constrain Input Options
- define the first controlled option sets for:
  - experience level
  - work mode
  - country/location selection
  - email lookback options

This keeps the first version understandable and aligned with the current small-source ingest reality.

#### 5C. Keep The Translation Layer Explainable
- the deterministic mapping should remain inspectable and testable
- the saved structured profile remains the source of truth
- future UI explainability should be able to point back to these mappings

#### 5D. Prepare For Later Rescoring Or Explainability
- keep the translation layer designed so it can later support:
  - a future rescore pass for already-saved jobs
  - scoring explainability
  - optional LLM-assisted profile generation

Current status:
- Stage 5 has partially started as part of Stage 4 integration
- the app now stores friendly UI-facing settings and derives scorer-facing fields from them
- current deterministic mappings already exist for:
  - desired roles -> role keywords
  - experience level -> preferred and penalty level keywords
  - work modes and locations -> preferred and penalty location terms
  - avoid terms -> mismatch keywords

What still remains for Stage 5:
- refine and improve the translation heuristics
- decide how broad or conservative role/skill/location expansions should be
- make the translation layer easier to explain in the UI later
- add more focused tests around the mappings themselves

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

## Current Env Settings That Moved Into Persisted UI-Editable Settings

These operator preferences are now owned by persisted app settings and the real UI instead of env config:

- digest recipient
- digest enabled
- digest top N
- digest lookback

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

## Where We Are Now

Current stage:
- Stage 4 is implemented in the real app
- Stage 5 has begun, but is not complete

What was completed recently:
- real onboarding and settings UI integrated into the app
- preview styling/layout adopted as the real UI baseline
- required-field-based `SetupComplete` behavior
- live profile and email-update editing
- real `Run Once` action
- shared run coordination and basic run status reporting
- "no new jobs" email update behavior
- first-pass deterministic mapping from friendly UI inputs into scorer-facing fields

## Recommended Next Step

Continue with Stage 5:

- refine the deterministic translation layer
- add direct tests for mapping behavior
- review whether the current role/skill/location expansions are too broad or too narrow
- improve how the UI explains what profile changes affect

After that, the most natural follow-up is Stage 7 explainability:

- show why a job scored well or poorly
- surface matched and penalized signals in the UI
- make the translation and scoring behavior easier for the user to trust and tune
