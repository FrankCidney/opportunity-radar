# Security

## Current Status

Opportunity Radar is currently a self-hosted, single-user application. It does not
yet implement accounts, authentication, authorization, tenant isolation, or
internet-facing abuse controls.

Until those controls are implemented, do not expose the current application directly
to untrusted users or the public internet. Restrict access at the network, hosting,
VPN, or reverse-proxy layer.

The planned multi-user security model is documented under `docs/dev/`, but planned
controls must not be assumed to exist.

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

For the current single-user version:

- terminate HTTPS before traffic reaches the application;
- restrict application access to the intended operator;
- keep PostgreSQL private;
- use a production database password and encrypted database transport where the
  hosting topology requires it;
- protect deployment and email-provider accounts with strong authentication;
- keep recoverable database backups;
- review logs for scraper, database, migration, and email failures;
- deploy only reviewed commits with passing tests.

## Multi-User Security Work

Authentication and tenant security are not implemented yet. Before real multi-user
use, the application must add and verify:

- password hashing;
- hashed, expiring sessions and one-time tokens;
- secure cookie attributes;
- CSRF protection;
- normalized account identity;
- email verification and password reset;
- login, registration, and expensive-action rate limits;
- repository-level tenant filtering;
- database-enforced same-tenant relationships;
- negative cross-tenant integration tests;
- safe ownership of legacy data.

This section should be replaced with the implemented controls as Phase 1 and later
phases land.

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
