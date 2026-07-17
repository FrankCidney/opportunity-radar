Your preferences point toward a different architecture from the current multi-user plan. The plan assumes globally shared jobs and companies; I recommend replacing that with user-owned data and treating each scrape as a tenant-scoped operation.

For now, a “tenant” can map one-to-one to a user. However, it is worth keeping `tenant_id` conceptually separate from `user_id`, even if the first implementation makes them equivalent. That leaves room for teams or organizations later without another major data migration.

## Recommended product decisions

### Registration and authentication

Registration should be open, but “open” should not mean completely unguarded.

Recommended initial flow:

1. User registers with email and password.
2. The account is created in an unverified state.
3. A verification email is sent.
4. The user verifies their address.
5. They complete profile setup and configure scraping.
6. Automated runs and email digests become available.

Email verification is valuable because it:

- prevents mistyped digest addresses;
- makes password recovery possible;
- reduces throwaway and automated accounts;
- protects your email-provider reputation;
- gives you a trustworthy account identifier.

I would allow a newly registered user to log in before verification, but restrict expensive or externally visible actions such as scheduled scraping and email delivery until verification is complete.

Other authentication choices I recommend:

- Normalize emails before uniqueness checks, usually lowercase plus whitespace trimming.
- Use Argon2id or bcrypt for passwords.
- Store only a hash of session tokens in the database, not the usable token itself.
- Use `Secure`, `HttpOnly`, and `SameSite=Lax` session cookies.
- Rotate the session after login and password changes.
- Add basic rate limiting to registration, login, password reset, and Run Once.
- Build password-reset support alongside verification; both use essentially the same one-time-token infrastructure.

CAPTCHA can wait until abuse appears. Rate limits and verified-email requirements are a better first layer.

### Outgoing email

Use one application-owned email provider account and API key, configured through deployment secrets. Do not ask users to provide Resend or SMTP credentials.

Per-user credentials create substantial complexity:

- secrets must be encrypted and safely managed;
- users must configure sender domains;
- delivery errors become difficult to support;
- the product gains multiple inconsistent sending identities;
- credential compromise becomes a platform concern.

The application should send from one verified product address, for example:

```text
Opportunity Radar <updates@your-domain.example>
```

The destination can default to the account email. You may still want a separate `digest_email` field so users can direct job alerts elsewhere without changing their login identity. A changed digest address should be verified before use.

Email needs fall into two categories:

- Transactional: verification, password reset, security notices.
- Product email: job digests and possibly scrape completion/failure notices.

They can initially share the same provider and API key, but should have separate templates and delivery records.

Useful data to retain:

- provider message ID;
- message type;
- recipient;
- send status and failure reason;
- created/sent timestamps;
- digest period or logical idempotency key.

You should also process bounces and suppress repeated sends to permanently failing addresses eventually. That does not need to block the first backend implementation.

### Jobs and companies

Jobs and companies should be tenant-owned.

The core relationship should look approximately like:

```text
tenant
 ├── users
 ├── preferences
 ├── scrape schedules
 ├── scrape runs
 ├── companies
 │    └── jobs
 └── digest deliveries
```

Every repository operation involving tenant-owned data must receive a tenant identity. Tenant filtering should not be left to handlers or callers as an optional convention.

This changes several parts of the existing plan:

- `companies` gets `tenant_id`.
- `jobs` gets `tenant_id`, either directly or transitively through company ownership. I favor adding it directly for clear filtering and stronger constraints.
- Source uniqueness becomes tenant-scoped, such as `UNIQUE (tenant_id, source, url)`.
- Company identity matching happens only inside the tenant.
- Digest queries always require `tenant_id`.
- Scrape runs record which tenant requested the work.

A job discovered independently by two users will exist twice. That costs more storage, but it gives the semantics you want and avoids complicated shared-data ownership rules. PostgreSQL storage is unlikely to be the limiting factor here; scraper traffic is the more important concern.

The existing `user_job_scores` join table is therefore unnecessary if each job belongs to exactly one tenant and currently has one score. The score can remain attached to the tenant-owned job or live in a one-to-one `job_scores` table if you want scoring history/versioning.

### Push versus pull scoring

Push scoring is the better fit for this application, especially after making jobs tenant-specific.

With tenant-owned jobs, ingestion becomes:

1. Load the tenant’s current scoring profile.
2. Scrape and normalize a job.
3. resolve/create a company within that tenant.
4. Score the job.
5. Persist the job and its score transactionally.

Advantages:

- Fast listing, sorting, filtering, pagination, and digest generation.
- Scoring failures occur during a controlled background operation.
- Stored results are explainable and inspectable.
- Query cost does not rise every time a user loads a page.
- You do not need to score every job for every user because each ingest run already has one tenant context.

The primary disadvantage is stale scores after preferences change. Solve that with an asynchronous tenant-scoped rescore operation:

```text
preferences updated
        ↓
increment profile/scoring version
        ↓
enqueue tenant rescore
        ↓
recalculate that tenant’s active jobs
```

Store a `scoring_version` or `profile_version` with each result. That lets the backend identify jobs scored using old preferences and prevents ambiguity while rescoring is in progress.

A pure pull model would avoid rescore jobs, but it makes every job-list and digest query CPU-heavy and complicates database sorting and pagination. You would often have to load many jobs into memory, score them, sort them, and only then select a page. That becomes increasingly unpleasant as job history grows.

There is one subtle issue in the current scorer: freshness contributes to the stored score. A push-computed freshness score becomes stale as time passes even if preferences do not change. I recommend splitting:

- stable profile relevance, computed and stored during ingestion/rescoring;
- time-dependent freshness/ranking adjustment, applied when selecting jobs.

That is a small hybrid approach, but it preserves fast queries and avoids periodically rescoring everything merely because another day passed. PostgreSQL can apply a simple age adjustment during ordering if necessary.

### User-controlled scraper schedules

Users should own scraper configuration:

- enabled sources;
- automatic scraping enabled/disabled;
- chosen frequency;
- next scheduled run;
- Run Once capability.

However, the scheduler should not create a timer or goroutine per user. Persist schedule state in PostgreSQL and let one dispatcher find due schedules.

Conceptually:

```text
dispatcher checks due schedules
            ↓
claims a schedule/run in PostgreSQL
            ↓
creates a tenant-scoped scrape run
            ↓
worker executes selected sources
            ↓
jobs are normalized, scored, and stored for that tenant
```

This can still live in the single Go service initially. You do not need a separate queue product or worker service yet. But the work should be represented by durable database records rather than only process-local timers.

Useful tables would include:

- `scrape_schedules`: tenant, frequency, enabled flag, next run, last run.
- `tenant_sources`: tenant, source, enabled flag, source-specific settings.
- `scrape_runs`: tenant, trigger type, status, timestamps, result counts, error.
- Optionally `scrape_run_sources`: per-source status and statistics.

`Run Once` should create the same kind of scrape-run record as a scheduled run. It should not run the entire scrape synchronously inside the HTTP request. The request can enqueue or claim the work and return a run ID.

### On-demand scraping tradeoffs

Running scrapers per tenant gives the cleanest product semantics:

- users get results based on their own sources and schedule;
- disabling automation genuinely means no tenant work runs;
- runs are auditable and attributable;
- future source-specific settings fit naturally;
- a failed scrape only affects one tenant’s run.

The costs are significant:

- multiple users may request the same source at nearly the same time;
- external traffic increases with the number of users;
- HTML sources may rate-limit or block the application;
- long scrapes consume server capacity;
- aggressive user schedules could create abuse or cost problems.

For the first version, I recommend true tenant-scoped on-demand runs with guardrails:

- Offer a controlled set of frequencies rather than arbitrary cron expressions.
- Define a minimum interval per source.
- Prevent overlapping runs for the same tenant and source.
- Apply global concurrency limits per scraper.
- Apply source-level rate limits across all tenants.
- Put Run Once behind a cooldown.
- Record failures and use retry backoff.
- Set maximum pagination and run-duration limits.
- Add a platform-wide kill switch for each scraper.

A sensible initial frequency set might be:

- every 6 hours;
- every 12 hours;
- daily;
- every 3 days;
- weekly;
- disabled.

The exact minimum should depend on the source. An API source may tolerate more frequent use than an HTML scraper.

Later, if duplicate traffic becomes a problem, you can introduce a short-lived acquisition cache or coalescing layer. For example, requests for the same source within a brief window could share the fetched raw payload while still independently normalizing and persisting tenant-owned jobs. That optimization preserves user isolation without prematurely turning jobs into global domain records.

### Scheduling and digests should be separate

A user’s scraping frequency and digest frequency are different concepts.

For example:

- scrape every 6 hours;
- send one daily digest;
- scrape manually only;
- send no digest;
- scrape weekly and send a digest only after a successful run.

Keep separate configuration and execution records for scraping and digests. Do not make digest delivery an implicit side effect of every scrape run.

## Revisions needed in the current plan

Before implementation, `multi-user-plan.md` should be rewritten around these decisions:

- Replace global jobs/companies with tenant-owned jobs/companies.
- Introduce tenants, even if membership is initially one user per tenant.
- Remove the “score every new global job for every active user” loop.
- Replace `user_job_scores` with tenant-job scoring and scoring versions.
- Replace the single global scrape interval with persisted per-tenant schedules.
- Add durable scrape-run records and asynchronous Run Once execution.
- Separate scrape scheduling from digest scheduling.
- Add email verification and password-reset tokens.
- Keep provider credentials global, with an optional verified digest recipient.
- Make repository-level tenant scoping an architectural rule.
- Plan for migration of the current single-user data into one initial tenant.

The proposed migration in the document should not be used as written. In addition to encoding the wrong ownership model, it attempts to turn the existing settings row into a foreign-keyed user preference before reliably creating and associating the corresponding user. This transition needs staged migrations with backfilling and validation before old constraints or columns are removed.

## Recommended backend sequence

I would structure the eventual implementation in this order:

1. Establish `tenants`, `users`, memberships, authentication, and email verification.
2. Migrate the existing operator and data into a default tenant.
3. Tenant-scope preferences, companies, jobs, digests, and every repository method.
4. Make scoring tenant-scoped and add scoring/profile versions plus background rescoring.
5. Introduce source configuration, durable scrape runs, and per-tenant schedules.
6. Add a database-backed dispatcher with concurrency and rate controls.
7. Adapt digest generation to tenant identity and its own schedule.
8. Add the minimal server-rendered authentication and run-status UI needed to operate it.

Server-rendered HTML remains entirely viable. Nothing in this backend design requires moving to a JavaScript frontend. The important UI-supporting backend pieces are authenticated request context, tenant-scoped services, asynchronous operation status, and clear run/error records.