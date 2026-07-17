# ADR-0002: Use opaque database-backed sessions

- Status: Accepted
- Date: 2026-07-18
- Deciders: project owner and implementer

## Context

The multi-user application needs revocable login sessions, password-reset session
invalidation, account disabling, and server-controlled workspace membership.

## Decision

Use cryptographically random opaque session tokens in `HttpOnly` cookies. Store only
SHA-256 token hashes in PostgreSQL.

Resolve the user and authorized membership from PostgreSQL for authenticated
requests. Password reset deletes all sessions for the affected user.

## Alternatives Considered

### Signed self-contained tokens

They reduce session lookups but make immediate revocation, account disabling, and
membership changes more complicated.

### Plaintext database tokens

They simplify lookup but turn a database read leak into immediately usable login
credentials.

## Consequences

- Sessions can be revoked immediately.
- Database access is required to authenticate requests.
- Indexed token hashes keep lookup bounded.
- Expired rows require a future cleanup policy even though they are rejected during
  authentication.

## Implementation Notes

- Raw tokens contain 32 random bytes and are base64url encoded.
- Cookie flags are `HttpOnly`, `SameSite=Lax`, and `Secure` in production.
- Passwords use bcrypt and password hashes are excluded from request principals.
- Verification/reset tokens follow the same raw-token/hash-storage boundary and are
  single-use and expiring.
