# CONVI-7582: Persist evaluation status and overall N/A

**Technical status:** Done in Linear; premise superseded; PRs closed; blocker removed
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7582](https://linear.app/cresta/issue/CONVI-7582/persist-evaluation-status-and-overall-na-on-training-simulator)
**Last updated:** 2026-08-31

## Decision update

After discussion with Jack Jee, reporting no longer needs to distinguish an incomplete timeout, a completed all-criteria-N/A evaluation, and a completed failure when each is stored as `score = 0` and `passed = false`. They will be treated as the same failure-equivalent result. Separate persisted overall `evaluation_status` and `not_applicable` fields are therefore not required for this reporting purpose.

Criterion-level N/A is unchanged: exclude each N/A criterion from scoring, then calculate score and pass from any remaining applicable criteria. Only the overall all-criteria-N/A edge case collapses to failure-equivalent.

The supplied Jack Jee excerpt explicitly supports collapsing all-criteria-N/A with failure and not adding a separate all-N/A field. It does not explicitly mention timeout behavior; the timeout collapse is recorded from the stated outcome of the broader discussion. See [the decision record](../decisions/2026-08-27-collapse-overall-zero-false-results.md).

Linear marks the ticket Done. On 2026-08-31 its description was prefixed with the superseding decision and its stale blocking relation to CONVI-7583 was removed. The three implementation PRs were closed on 2026-08-27 with comments explaining the superseded requirement and criterion-level N/A nuance.

## Original objective and impact (superseded)

- **Objective:** Persist the evaluator's `evaluation_status` and overall `not_applicable` value atomically with score, pass, and criterion results on `director.training_simulator_conversation_scores`.
- **Customer/system impact:** Session reporting would distinguish unfinished evaluation, completed all-N/A, passed, and failed results without guessing from score or criterion shapes.
- **Role:** investigated, ticketed, and implemented as draft PRs before the requirement changed

## Scope

**Original in scope (superseded)**

- Add both fields to the conversation-score schema/model and task-run API update path.
- Preserve them through converters and generated contracts.
- Treat legacy rows without status as ambiguous rather than heuristically backfilling them.
- Test pending, in-progress, completed passed, completed failed, completed all-N/A, timeout snapshots, and legacy rows.

**Original non-goals (superseded)**

- Adding fields to `app.chats` or creating a separate result table.
- Treating all-N/A as failed.
- Heuristically rewriting legacy rows.

## Historical understanding

The evaluator returns status and overall N/A, but Director's task-run update persists only score, passed, and criterion results. Director normally writes a completed evaluation, but its polling timeout can write the latest partial snapshot. Without the two explicit fields, unfinished snapshots, genuine completed all-N/A evaluations, and zero-score failures can collapse into indistinguishable stored shapes.

Read-only staging aggregates found 277 conversation runs, 110 conversation scores with null score/pass, and 18 rows whose criteria were all N/A while the row stored score zero and passed false. Ten of those were current latest results and therefore appeared failed.

The product decision accepts that failure-equivalent presentation for the three listed zero/false cases. The staging evidence remains historically valid but no longer establishes a requirement to distinguish them.

## Source Context

- **Repos:** `cresta-proto`, `go-servers`, `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7582`, `/Users/xuanyu.wang/repos/go-servers-convi-7582`, `/Users/xuanyu.wang/repos/director-convi-7582`
- **Branches:** `convi-7582-persist-evaluation-result-contract`, `convi-7582-persist-evaluation-result`, `convi-7582-send-evaluation-result-state`
- **Closed PRs:** [cresta-proto#9656](https://github.com/cresta/cresta-proto/pull/9656#issuecomment-5442846072), [go-servers#31521](https://github.com/cresta/go-servers/pull/31521#issuecomment-5442846041), [director#22107](https://github.com/cresta/director/pull/22107#issuecomment-5442846062)
- **Commits:** `29a3bfbd80`, `7cd3f9f14f`, `a3612f2054`

## Dependencies

- CONVI-7583 and later reporting work must be rebased on the collapsed semantics and must not depend on these fields solely to distinguish the three listed cases.
- Confirm separately how a timed-out partial snapshot with applicable criteria or a nonzero score should be classified; Jack's quoted messages do not address it.

## Validation

- Proto package built successfully with `bazel build //cresta/v1/trainingsimulator:all`.
- Backend tests cover pending, in-progress timeout snapshots, complete passed, complete failed, complete N/A, and legacy missing-state rows, but cannot compile until GitHub Actions generates the new protobuf and GORM fields.
- Director tests cover complete applicable, complete N/A, retry-save, and in-progress timeout payloads, but cannot run in the isolated worktree until the generated web client is available and workspace dependencies are installed.
- Reporting classification and the visible warning for legacy missing status remain in dependent ticket CONVI-7583.

## Timeline

- 2026-08-24 — Created the Linear ticket in Training Simulator / Convo Intelligence Backlog with code, Figma-context, and aggregate staging evidence. No Linear milestone was assigned because the configured project milestones do not correspond to the design's Milestone 1.
- 2026-08-25 — Began implementation from current `origin/main` in isolated proto, backend, and Director worktrees. Evidence: `sessions/2026-08-25/codex-convi-7582-pr.md`.
- 2026-08-25 — Opened source-only draft PRs [cresta-proto#9656](https://github.com/cresta/cresta-proto/pull/9656), [go-servers#31521](https://github.com/cresta/go-servers/pull/31521), and [director#22107](https://github.com/cresta/director/pull/22107) in dependency order. Generated protobuf, GORM, and web-client artifacts are intentionally left to GitHub Actions.
- 2026-08-26 — Expanded all three PR descriptions with the reporting ambiguity being solved, the evaluator-to-persistence workflow, and each PR's responsibility in the series. Evidence: `sessions/2026-08-26/codex-convi-7582-pr-context.md`.
- 2026-08-27 — Product direction changed after discussion with Jack Jee: incomplete timeout, completed all-criteria-N/A, and completed failure represented as zero/false do not need distinct reporting states. Criterion-level N/A remains excluded from scoring. The CONVI-7582 premise is superseded; external ticket/PR cleanup is pending. Evidence: `decisions/2026-08-27-collapse-overall-zero-false-results.md`.
- 2026-08-27 — Closed proto PR #9656, backend PR #31521, and Director PR #22107 with comments explaining the product decision. The Linear ticket was not changed.
- 2026-08-31 — Verified Linear already marked the ticket Done, added the superseding decision to its description, and removed its stale blocker relation from CONVI-7583.
