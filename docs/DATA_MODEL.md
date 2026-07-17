# Data Model

## Status

This document describes the current single-user PostgreSQL model. The planned
multi-user tenant model is not implemented yet.

The schema is defined by ordered SQL files in `migrations/`. Explicit SQL
repositories in `internal/` read and write the records.

## Current Relationships

```text
app_settings (one row)

companies
    └── jobs

digest_deliveries
```

There are currently no users, tenants, memberships, sessions, account tokens,
scrape-run records, or rescore-run records.

## Companies

`companies` stores normalized company identity:

- numeric ID;
- normalized name;
- logo URL;
- source;
- source-specific external ID;
- domain;
- creation and update timestamps.

Current identity resolution prefers:

1. source plus external ID;
2. domain;
3. normalized name;
4. creation of a new company.

Company identity is global because the application currently assumes one operator.
Deleting a company cascades to its jobs.

## Jobs

`jobs` stores:

- company reference;
- title and description;
- location;
- external URL and source;
- posted time and optional application deadline;
- persisted numeric score;
- `active` or `archived` status;
- creation and update timestamps.

Current uniqueness is `(source, url)` across the database.

The persisted score combines profile relevance and freshness. Existing jobs are not
automatically rescored when preferences change.

## Application Settings

`app_settings` is constrained to one row with ID `1`.

It stores:

- onboarding completion;
- product-facing role, experience, skill, location, work-mode, and avoidance
  preferences;
- derived scoring keyword lists;
- digest enabled state;
- digest recipient;
- digest top-N and lookback.

Deployment configuration such as database credentials, scheduler behavior, and
email-provider credentials is intentionally stored outside PostgreSQL in environment
configuration.

## Digest Deliveries

`digest_deliveries` records:

- recipient;
- UTC digest date;
- job count;
- subject;
- sent and creation timestamps.

The current idempotency constraint is `(recipient, digest_date)`.

The current service sends email before recording delivery. A concurrent delivery
conflict can therefore be detected after a send. The planned delivery lifecycle
must resolve this risk before public multi-user operation.

## Ownership and Isolation

All current domain data belongs implicitly to the single operator of the deployed
instance. There is no row-level ownership or authorization boundary.

Before multi-user operation, tenant-owned tables must gain non-null tenant identity,
tenant-scoped repository methods and indexes, and database-enforced same-tenant
relationships. Until then, the database must not be treated as multi-tenant.

## Lifecycle and Deletion

Current behavior:

- jobs may be archived;
- jobs may be deleted through the jobs domain;
- deleting a company deletes its jobs through the foreign key;
- settings can be reset through existing UI flows;
- digest delivery records persist without an automated retention policy.

There is no account deletion, tenant deletion, user-data export, or formal retention
policy because accounts and tenants do not exist yet.

## Future Documentation Work

As multi-user phases are implemented, update this document with:

- user, tenant, and membership ownership;
- authentication/session/token lifecycles;
- composite tenant foreign keys;
- tenant-scoped uniqueness;
- scrape and rescore run state machines;
- digest delivery state and idempotency;
- retention, export, and deletion behavior;
- migration/backfill rules for legacy data.
