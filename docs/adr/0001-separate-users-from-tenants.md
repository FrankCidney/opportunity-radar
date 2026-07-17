# ADR-0001: Separate users from tenants

- Status: Accepted
- Date: 2026-07-18
- Deciders: project owner and implementer

## Context

A user is a login identity. The data owner may initially look identical because each
registration creates one personal workspace, but future organizations may contain
multiple users and one user may belong to multiple organizations.

Attaching jobs and preferences directly to a user would make organization support
require another ownership migration.

## Decision

Represent login identities as `users`, ownership boundaries as `tenants`, and access
as `tenant_memberships`.

Normal registration transactionally creates:

- one user;
- one personal tenant;
- one owner membership;
- one session;
- one email-verification token.

The product may call a tenant a “workspace” in user-facing text.

## Alternatives Considered

### Attach all data to `user_id`

Simpler initially, but conflates people with data ownership and complicates future
organizations.

### Build organizations only when needed

Avoids one table today but requires a later ownership migration across every
tenant-owned domain.

### Share all domain records globally

Reduces duplicate rows but conflicts with the product requirement that users own
independent companies and jobs.

## Consequences

- Repository and database ownership can consistently use `tenant_id`.
- Future team membership does not require changing domain ownership.
- Authentication must resolve an authorized membership, not trust a tenant ID from
  the browser.
- Phase 1 has a safe transitional holding page because domain tables are not yet
  tenant-scoped.

## Implementation Notes

- Public registration never claims the legacy workspace.
- Existing operator data requires explicit protected bootstrap.
- Tenant-aware domain repositories and same-tenant foreign keys arrive in Phase 2.
