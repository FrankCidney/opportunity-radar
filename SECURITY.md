# Security

## Current Status

Opportunity Radar implements the Phase 1 account-security foundation: registration,
password hashing, email verification, password recovery, database-backed sessions,
membership-derived workspace identity, CSRF protection, browser security headers,
and basic in-process abuse controls.

Tenant isolation for jobs, companies, preferences, and digests is not complete.
During this transition, only the explicitly bootstrapped legacy owner may access the
existing console. Newly registered workspaces cannot access legacy domain data.

## Reporting a Vulnerability

Do not open a public issue containing exploit details, credentials, personal data,
or other sensitive information.

No private reporting address has been published yet. Before a public multi-user
release, the project owner must add a monitored security contact or enable private
vulnerability reporting on the source-code host. Until then, contact the repository
owner privately through an established channel.

Include:

- the affected version or commit;
- the affected component;
- reproduction steps;
- likely impact;
- any suggested mitigation.

## Current Security Boundaries

- PostgreSQL is the system of record.
- Database and Resend credentials are deployment secrets supplied through
  environment variables.
- The Docker Compose PostgreSQL service is not published to the host by default.
- SQL migrations execute automatically at application startup.
- External scraper and email requests use application-controlled clients.
- The server-rendered admin UI currently assumes one trusted operator.
- Authentication uses opaque, high-entropy cookie tokens. Only SHA-256 token hashes
  are stored.
- Passwords use bcrypt with cost 12 and are never placed in authenticated request
  principals.
- Verification and reset tokens are single-use, hashed, and expiring.
- Password reset revokes all existing sessions.
- State-changing forms use signed double-submit CSRF tokens.
- Cookies are `HttpOnly`, `SameSite=Lax`, and `Secure` in production.
- Token-bearing pages use `Referrer-Policy: no-referrer`.
- Production responses enable HSTS, and form bodies are bounded before parsing.
- Login, registration, verification resend, reset request, and reset submission are
  rate-limited in the single application process.

## Secrets

- Never commit `.env` files, database credentials, Resend API keys, or production
  URLs containing credentials.
- Supply production secrets through the deployment platform's secret management.
- Use separate credentials for development and production.
- Rotate credentials after suspected exposure.
- Avoid logging secrets or authorization headers.

The current example database password is intended only for the isolated local Docker
Compose environment. Do not reuse it for a publicly reachable database.

## Production Deployment Baseline

For the current transitional version:

- terminate HTTPS before traffic reaches the application;
- restrict application access to the intended operator;
- keep PostgreSQL private;
- use a production database password and encrypted database transport where the
  hosting topology requires it;
- protect deployment and email-provider accounts with strong authentication;
- keep recoverable database backups;
- review logs for scraper, database, migration, and email failures;
- deploy only reviewed commits with passing tests.

## Remaining Multi-User Security Work

Before all registered workspaces can use domain features, the application must add
and verify:

- repository-level tenant filtering;
- database-enforced same-tenant relationships;
- negative cross-tenant integration tests;
- tenant-safe background work and digests;
- shared rate limiting before running multiple application replicas;
- automated cleanup/retention for expired sessions and account tokens.

Legacy ownership is safe only through the protected one-time startup bootstrap. The
first public registrant never receives legacy data.

## Data and Privacy

The application stores job-search preferences, collected job and company data, and
an optional digest recipient. It does not yet provide account export, account
deletion, a retention policy, or a published privacy policy.

Those capabilities and policies must be defined before operating a public
multi-user service with real users.

## Supported Versions

The project does not yet publish versioned releases or a formal security-support
window. The current supported state is the latest reviewed deployment maintained by
the repository owner. Add an explicit version policy before external releases.
