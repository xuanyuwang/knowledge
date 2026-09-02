# Codex Session: Milestone 1 Remaining Work

> **Historical snapshot:** Superseded by the 2026-08-27 zero/false decision and the 2026-08-31 revalidation in `codex-milestone-1-current-gaps.md`. CONVI-7582 is Done/superseded with closed PRs; CONVI-7583 is In Progress with proto/backend worktrees; the only new response datum for the verified frontend deltas is `AgentPerformanceEntry.attempt_count`.

## Current State

- CONVI-7582 is In Progress with three PRs: [cresta-proto#9656](https://github.com/cresta/cresta-proto/pull/9656), [go-servers#31521](https://github.com/cresta/go-servers/pull/31521), and [director#22107](https://github.com/cresta/director/pull/22107).
- Proto validation is otherwise green, but its cross-repo `go generate` check fails against `go-servers/main` because the new fields require the companion converter.
- Backend CI currently fails because neither the generated protobuf fields nor generated nullable GORM model fields are present yet.
- Director typecheck/production build currently fails until the generated web client exposes the new fields; the PR remains draft.
- CONVI-7583 and CONVI-7584 are both Backlog and have no implementation PRs.

## Remaining Milestone 1 Work

1. Finish the CONVI-7582 generation/release/dependency sequence, run its backend/frontend tests, obtain review, and merge in compatible order.
2. Implement CONVI-7583: assignment-rooted bounded reads, zero-run coverage, all-attempt count, deterministic latest-attempt-first selection, conversation/quiz result classification, correct denominators, warnings, authorization, and telemetry.
3. Resolve the completed-all-N/A filter/presentation decision and reporting persona/row-scope authorization decision.
4. Implement CONVI-7584: consume backend state without re-derivation, show attempts/N-A/warnings/unavailable aggregates/latest activity, preserve routing, and add focused tests.
5. Complete staging reconciliation, authorization checks, >1,000-run/safety-bound tests, latency/count telemetry validation, and Design verification.

Lesson- and module-level reporting remain outside Milestone 1.
