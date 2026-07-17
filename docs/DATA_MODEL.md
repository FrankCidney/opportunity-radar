# Data Model

## Status

This document describes the current transitional PostgreSQL model. Identity and
authentication tables exist, while jobs, companies, settings, and digests remain on
the legacy single-operator schema until Phase 2.

The schema is defined by ordered SQL files in `migrations/`. Explicit SQL
repositories in `internal/` read and write the records.

## Current Relationships

```text
users
    ├── sessions
    ├── account_tokens
    └── tenant_memberships ── tenants

app_settings (one legacy row)

companies
    └── jobs

digest_deliveries
```

There are currently no durable scrape-run or rescore-run records.

## Users, Tenants, and Memberships

`users` stores normalized unique email identity, a bcrypt password hash, verification
state, disabled state, and timestamps.

`tenants` is the workspace ownership boundary. Normal registration creates one user,
one non-legacy tenant, and one owner membership transactionally.

`tenant_memberships` connects users to tenants with an `owner` or `member` role.
Team-management behavior is not implemented yet.

An existing deployment may have one `is_legacy = TRUE` tenant. Its ownership is
created only through protected bootstrap configuration.

## Sessions and Account Tokens

`sessions` stores:

- user identity;
- a unique 32-byte SHA-256 token hash;
- expiry and activity timestamps.

The usable opaque token exists only in the browser cookie.

`account_tokens` stores hashed, expiring, single-use tokens for:

- `verify_email`;
- `reset_password`.

Password reset consumes its token, updates the password hash, and deletes all
sessions in one transaction.

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

All current job/company/settings/digest data still belongs implicitly to the legacy
operator. Only the verified legacy membership can reach it through HTTP, but the
rows do not yet contain `tenant_id`.

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
policy yet. Expired session and account-token rows are rejected by queries but do
not yet have an automated cleanup task.

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
