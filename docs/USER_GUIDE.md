# User Guide

## Status

This guide describes the current self-hosted, single-user application. Registration,
login, organizations/workspaces, and personal schedules are not implemented yet.

## First-Time Setup

1. Deploy and open Opportunity Radar using the instructions in `README.md`.
2. Complete the setup form.
3. Enter the roles you want.
4. Select your experience level.
5. Add current and growth skills.
6. Add preferred locations and work modes.
7. Add terms that should reduce a job's relevance.
8. Save the profile.

Automatic runs are skipped until required setup is complete.

## Profile and Scoring

Opportunity Radar uses a deterministic rule-based scorer. It considers:

- desired roles;
- experience level;
- current and growth skills;
- preferred locations;
- work modes;
- avoid or mismatch terms;
- job freshness.

Higher-scored jobs appear first where score ordering is used.

Changing the profile affects jobs ingested after the change. Existing jobs are not
currently rescored automatically.

## Running Scrapers

The deployed application controls the automatic interval through environment
configuration.

Use **Run Once** when you want to trigger the configured scrapers immediately. The
current Run Once operation executes through the application process; it is not yet a
durable background request.

Scrapers rely on third-party pages and APIs. A source may temporarily fail, return
no jobs, rate-limit the application, or change its page structure.

## Jobs and Companies

The application stores normalized jobs and their associated companies in
PostgreSQL.

Company identity is resolved conservatively using source identity, domain, and then
normalized name. All data belongs to the one operator of the deployed instance.

Jobs may be active or archived. Deleting a company also deletes its associated jobs.

## Email Updates

Email updates require the deployment operator to configure Resend sender
credentials. Without complete sender configuration, the application uses a logging
sender rather than sending real email.

In the application:

1. Open email/digest settings.
2. Enter the destination email address.
3. Choose the number of jobs to include.
4. Choose the lookback period.
5. Enable email updates.
6. Save.

The current digest runs after ingestion and sends at most one update per recipient
per UTC date. It can send a status update even when no recent jobs are found.

## Troubleshooting

### Automatic runs are skipped

Confirm setup is complete and the deployment scheduler is enabled.

### Run Once is already running

The current application allows one ingest run at a time. Wait for it to complete and
review the application logs.

### No email arrives

Confirm:

- email updates are enabled;
- a recipient is saved;
- `RESEND_API_KEY` and `RESEND_FROM_EMAIL` are configured;
- the sender identity is accepted by Resend;
- application logs do not report a provider error.

### A source returns no jobs

The source may have no matching/current results, may be unavailable, or may have
changed its response/page structure. Review source-specific logs.

## Multi-User Features

The following are planned but unavailable:

- open registration and login;
- email verification and password reset;
- isolated workspaces/tenants;
- user-controlled sources and scraping schedules;
- durable Run Once status;
- tenant-specific jobs, companies, scores, and digests.

This section should be replaced with user instructions as those features become
implemented.
