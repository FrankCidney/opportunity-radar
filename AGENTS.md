# Agent Guidelines

- Treat this as a production application with real users. Prioritize security,
  privacy, correctness, performance, accessibility, and clear failure handling.
- Follow `docs/ARCHITECTURE.md` and the active implementation plan. Preserve package
  boundaries and make tenant isolation explicit in repositories, SQL, and tests.
- Never store plaintext passwords, session tokens, one-time tokens, or provider
  secrets. Validate untrusted input and use secure defaults.
- Add safe, staged migrations; do not edit applied migrations or perform destructive
  changes before backfill and validation.
- Add proportionate tests, including negative and cross-tenant cases. Run the full
  suite before declaring a unit complete.
- Keep HTTP handlers thin, background work durable, database transactions short,
  and external calls bounded by timeouts and platform limits.
- Extend the existing server-rendered UI and shared CSS patterns. Keep new screens
  visually consistent, responsive, accessible, and usable with clear errors.
- Treat documentation as part of implementation, not optional cleanup. Update it in
  the same contained unit as the behavior it describes:
  - `README.md` for setup, configuration, deployment, and first-use changes.
  - `docs/ARCHITECTURE.md` for architecture that is actually implemented.
  - `SECURITY.md` for authentication, authorization, secrets, privacy, security
    assumptions, and vulnerability-reporting guidance.
  - `docs/OPERATIONS.md` for migrations, backups/restores, workers, monitoring,
    failure recovery, maintenance, and production runbooks.
  - `docs/DATA_MODEL.md` for ownership, tables, invariants, lifecycle, retention,
    export, and deletion behavior.
  - `docs/USER_GUIDE.md` for durable user-facing workflows.
  - `docs/adr/` for consequential decisions and their reasoning/tradeoffs.
- Create a durable document when its subject first becomes real; do not document
  planned behavior as implemented. Keep temporary implementation notes in
  `docs/dev/`, and ensure completed work is reflected in the durable docs before a
  phase is declared done.
- Commit complete, passing units with conventional messages; preserve unrelated
  worktree changes.
