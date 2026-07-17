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
- Update durable documentation when behavior, architecture, configuration,
  operations, security assumptions, or user workflows change.
- Commit complete, passing units with conventional messages; preserve unrelated
  worktree changes.
