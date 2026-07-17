# Architecture Decision Records

Architecture Decision Records (ADRs) preserve consequential technical and product
decisions, including the context, alternatives, tradeoffs, and consequences.

`docs/ARCHITECTURE.md` describes the architecture that currently exists. ADRs explain
why a particular durable decision was made.

## Records

- [ADR-0001: Separate users from tenants](0001-separate-users-from-tenants.md)
- [ADR-0002: Use opaque database-backed sessions](0002-opaque-database-backed-sessions.md)

Create ADRs as consequential decisions become implemented. Planned decisions may be
drafted, but they must be clearly marked `Proposed` and must not be described as
implemented.

## Naming

Use sequential filenames:

```text
0001-short-decision-title.md
0002-another-decision.md
```

## Status Values

- `Proposed`
- `Accepted`
- `Superseded`
- `Deprecated`

When superseding an ADR, link the old and replacement records in both directions.

## Template

```markdown
# ADR-NNNN: Decision title

- Status: Proposed
- Date: YYYY-MM-DD
- Deciders: project owner and implementers

## Context

What problem or constraint requires a decision?

## Decision

What was decided?

## Alternatives Considered

What credible alternatives were evaluated?

## Consequences

What becomes easier, harder, safer, or more expensive?

## Implementation Notes

What invariants or boundaries must implementations preserve?
```

Likely multi-user ADR subjects include:

- separating users from tenants/workspaces;
- tenant-owned jobs and companies;
- database-backed durable background work;
- push relevance scoring with query-time freshness;
- application-owned outgoing email credentials.
