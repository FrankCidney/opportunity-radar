## Questions to ask after every phase

### “Did you implement only this phase?”

The agent should work incrementally. Be cautious if it attempts authentication, migrations, scoring, scheduling, and digests in one enormous change.

Smaller phases make it easier to:

- review database changes;
- test tenant separation;
- find regressions;
- reverse course before later code depends on a mistake.

### “What tests prove tenant isolation?”

This is the single most important question.

The agent should show tests involving at least two tenants:

```text
Tenant A creates Job A
Tenant B creates Job B

Tenant A can retrieve Job A
Tenant A cannot retrieve Job B

Tenant B can retrieve Job B
Tenant B cannot retrieve Job A
```

The same principle applies to:

- companies;
- preferences;
- scrape runs;
- schedules;
- digest deliveries.

A test with only one user does not prove tenant isolation.

### “Where does the tenant ID come from?”

For a normal authenticated request, the answer should be:

> From the authenticated session and membership stored in the request context.

Bad answers include:

- a hidden form field;
- a query parameter;
- a URL parameter accepted without checking membership;
- a browser cookie containing an unsigned tenant ID.

The browser can eventually request a workspace switch, but the server must verify that the logged-in user belongs to the requested workspace.

### “Do repository queries require tenant ID?”

Repository methods should look like:

```go
Get(ctx, tenantID, jobID)
List(ctx, tenantID, filter)
Save(ctx, tenantID, job)
```

The SQL should include tenant filtering:

```sql
WHERE tenant_id = $1 AND id = $2
```

Be cautious if tenant checks happen only in HTTP handlers. Background workers and future code could bypass those handlers.

### “Can the database prevent cross-tenant relationships?”

It should not merely rely on Go code.

For example, the database should reject this:

```text
Job owned by Tenant A
        ↓
Company owned by Tenant B
```

The plan’s composite foreign key enforces that. This is an important migration-review point.

### “Was existing data safely backfilled?”

Before making `tenant_id` mandatory, the agent should demonstrate that:

- the legacy tenant was created;
- all existing companies belong to it;
- all existing jobs belong to it;
- each job and its company belong to the same tenant;
- existing settings and digest history were preserved;
- no ownership column remains null.

The agent must not assign existing private data to whoever happens to register first.

### “Is Run Once asynchronous?”

The HTTP handler should create a queued database record and return. It should not keep the web request open while scraping websites.

You should be able to see a run move through states such as:

```text
queued → running → completed
                 ↘ partial
                 ↘ failed
```

### “Does a queued run survive restart?”

If Run Once is accepted and the application restarts, the queued database record should remain available for a worker. The database—not an in-memory goroutine—must be the source of truth.

### “Can one tenant trigger unlimited scraping?”

The answer should be no.

Check for:

- one active run per tenant;
- Run Once cooldown;
- fixed frequency options;
- global worker limits;
- per-source rate limits;
- run timeouts;
- pagination limits;
- source kill switches.

User-selected frequency expresses preference; it must not override platform safety limits.

### “Is scoring using the correct tenant’s profile?”

The agent must not retain the current process-global mutable scorer.

Each run should receive an immutable profile for its tenant. Tests should run two tenants with different preferences and prove that the same job receives different relevance scores.

### “What happens after preferences change?”

The expected flow is:

```text
save preferences
      ↓
increment scoring profile version
      ↓
queue tenant-only rescore
      ↓
update that tenant's active jobs
```

Changing Alice’s preferences must never rescore Bob’s jobs.

### “Is freshness separate from stored relevance?”

The current scoring system includes freshness. If freshness remains permanently embedded in a stored score, scores silently become stale every day.

The expected design is:

- store stable relevance;
- use posting date/freshness when ordering results.

### “Are scraping and digest schedules separate?”

They must be independently configurable.

A user should be able to:

```text
Scrape: every 6 hours
Digest: daily
```

or:

```text
Scrape: manual only
Digest: disabled
```

Completing a scrape should not automatically mean sending an email.

### “Can email be sent twice?”

The agent should explain the delivery idempotency rule and show a corresponding database constraint or test. Retrying a failed worker must not generate the same logical digest twice.

## Red flags worth stopping immediately

Pause implementation if you see any of these:

- Jobs or companies are made global/shared.
- A `user_job_scores` table is introduced over global jobs.
- Tenant IDs come directly from form or query input without membership validation.
- Repository methods load tenant-owned records using only their record ID.
- Existing jobs are assigned to the first public registrant.
- A fake or default password is placed in a migration.
- Session or reset tokens are stored in plaintext.
- One global mutable scorer is reused across tenants.
- Run Once performs scraping inside the HTTP request.
- A goroutine or in-memory queue is the only record of pending work.
- The agent edits historical migration files instead of adding new migrations.
- Scraper and digest schedules are treated as the same setting.
- The agent makes a large destructive migration before completing and validating the backfill.
- Tests cover only one tenant.

## A compact phase-review template

After each implementation phase, you can ask the agent:

> Summarize what changed in this phase. Show the migrations, the important interface changes, and the tests proving the phase’s exit criteria. Specifically identify how tenant isolation is enforced in both SQL and Go. List anything deferred, any deviations from `multi-user-plan.md`, and any risks that remain. Do not begin the next phase yet.

That should give you a manageable review checkpoint without requiring you to reread the entire architecture document every time.