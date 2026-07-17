# Operations

## Status

This runbook describes the application as it operates today. Opportunity Radar uses
one Go service, one PostgreSQL database, database-backed authentication, and one
process-local scheduler. Multi-user background queues and workers are not yet
implemented.

## Deployment Topology

Supported repository configurations:

- local/self-hosted deployment with Docker Compose;
- Railway deployment using the root `Dockerfile` and `railway.json`.

The application process owns:

- the HTTP server;
- startup migrations;
- scheduled and manual ingestion;
- scraping and normalization;
- scoring;
- digest generation and delivery.

Run one application replica. The current process-local scheduler and run coordinator
are not designed for multiple replicas.

## Required Services

- PostgreSQL 16 or a compatible supported PostgreSQL service.
- The application container/binary.
- Optional Resend credentials for real email delivery.

## Configuration

Current environment variables are documented in `README.md` and `.env.example`.

Required:

- `DATABASE_URL`

Runtime:

- `ENV`
- `PORT`
- `SCHEDULER_ENABLED`
- `SCHEDULER_INTERVAL`
- `SCHEDULER_RUN_ON_START`
- `SCHEDULER_RUN_TIMEOUT`

Optional email delivery:

- `RESEND_API_KEY`
- `RESEND_FROM_EMAIL`
- `RESEND_FROM_NAME`
- `REGISTRATION_ENABLED`
- `TRUST_PROXY_HEADERS`
- `PUBLIC_BASE_URL`
- `AUTH_CSRF_KEY`
- `AUTH_SESSION_TTL`
- `AUTH_VERIFICATION_TTL`
- `AUTH_PASSWORD_RESET_TTL`

Existing deployments may temporarily set both:

- `BOOTSTRAP_ADMIN_EMAIL`
- `BOOTSTRAP_ADMIN_PASSWORD`

Leave `TRUST_PROXY_HEADERS=false` unless direct application traffic is restricted to
a trusted reverse proxy that controls `X-Forwarded-For`/`X-Real-IP`. Railway
deployments should enable it so rate limits use the real client address rather than
one shared proxy address.

Digest recipient, lookback, and top-N are persisted through the application UI.

## Startup

On startup the application:

1. loads environment configuration;
2. connects to PostgreSQL;
3. discovers and applies pending migrations;
4. creates or verifies authentication services;
5. optionally claims an unclaimed legacy workspace;
6. loads or creates the single legacy settings record;
7. constructs the scorer and ingest pipeline;
8. starts the HTTP server with authentication, CSRF, and security headers;
9. optionally runs ingestion when setup and verified legacy ownership allow it;
10. starts the process-local schedule.

A database or migration failure prevents a healthy startup. Do not bypass migration
errors by manually marking a migration applied unless the schema has been verified.

## Existing Deployment Bootstrap

The identity migration creates an unclaimed legacy workspace only when existing
settings, companies, jobs, or digest records are present. It does not create a user
or default password.

To claim it:

1. choose the existing operator's email and a unique password of 12–72 characters;
2. set `BOOTSTRAP_ADMIN_EMAIL` and `BOOTSTRAP_ADMIN_PASSWORD` through protected
   deployment secrets;
3. deploy once;
4. confirm the log records creation of the legacy owner without printing the
   password;
5. remove both bootstrap variables immediately;
6. sign in with the bootstrap account.

The bootstrap account is marked verified because possession of deployment secrets
is the ownership proof. Bootstrap fails rather than attaching legacy data to an
already registered public account. The first public registrant can never claim the
legacy workspace.

## Migrations

Migration files live in `migrations/` and are applied in filename/version order.

Operational rules:

- Back up the database before a destructive or high-risk migration.
- Add new migrations; do not edit migrations already applied to an environment.
- Review both schema changes and data backfills.
- Test fresh-database and existing-data upgrade paths.
- Keep application/schema compatibility in mind during deployment.
- Verify post-migration constraints and row counts before cleanup.

The migrator records applied versions in `schema_migrations` and wraps each
migration plus its version record in one transaction.

## Routine Operation

### Docker Compose

Start or update:

```bash
docker compose up --build -d
```

Inspect status and logs:

```bash
docker compose ps
docker compose logs -f app
```

Stop without deleting the database volume:

```bash
docker compose down
```

Do not add `--volumes` unless permanent local database deletion is intended.

### Railway

Railway normally builds and deploys the connected Git commit. Confirm:

- PostgreSQL is healthy;
- required variables are present;
- migrations completed;
- the HTTP service remains running;
- automatic ingestion is not unexpectedly skipped;
- email configuration is recognized.

## Health and Verification

There is no dedicated production health endpoint, metrics system, tracing, or
alerting integration yet.

After deployment:

1. check startup and migration logs;
2. open the application;
3. confirm saved settings load;
4. trigger Run Once when safe;
5. confirm scraper summaries and stored jobs;
6. verify digest behavior if enabled;
7. restart the service and confirm persisted data remains.

Dedicated liveness/readiness checks and production metrics should be added as the
multi-user runtime is implemented.

## Backups and Restore

No automated backup policy is included in the repository.

Production operators must configure PostgreSQL backups through their hosting
provider or a reviewed `pg_dump`/restore process. A production policy should define:

- backup frequency;
- retention period;
- encryption and access control;
- off-site or provider-level redundancy;
- restore testing frequency;
- acceptable recovery point and recovery time.

A backup is not considered reliable until a restore has been tested. Document the
environment-specific restore procedure alongside deployment secrets, outside the
repository when it contains sensitive infrastructure details.

## Failure Scenarios

### Migration failure

- Keep the failed deployment stopped.
- Read the complete migration error.
- Determine whether PostgreSQL rolled the transaction back.
- Compare the actual schema with the migration's expected preconditions.
- Restore from backup if a destructive partial external operation occurred.
- Fix forward with a reviewed migration; do not rewrite an applied migration.

### Scraper failure or zero results

- Inspect the source-specific logs.
- Confirm the external source is reachable.
- Check whether HTML/API structure changed.
- Avoid unbounded retries that may trigger source blocking.
- Validate parser changes with fixtures and tests before deployment.

### Email failure

- Confirm Resend configuration is present.
- Verify the sender identity with the provider.
- Review provider and application errors without exposing API keys.
- Remember that missing sender configuration intentionally selects the logging
  sender.

### Missed scheduled run

The current scheduler is process-local. Downtime can cause missed work, and missed
runs are not durably queued. Restore service availability and use Run Once when
appropriate.

### Database connectivity failure

- Verify database service health and network access.
- Confirm `DATABASE_URL` without printing credentials.
- Check connection limits and provider incidents.
- Do not repeatedly restart into a failing migration without investigation.

## Maintenance Gaps

The following production capabilities do not exist yet:

- authenticated operational access;
- durable scrape and digest queues;
- multi-replica coordination;
- dedicated health endpoints;
- metrics, tracing, and alerting;
- automated expired session/token cleanup;
- run-history retention policies;
- documented disaster-recovery objectives.

Update this document as each capability becomes implemented.
