# CONVI-7601 dedicated module statistics contract

Date: 2026-08-28
Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
Worktree: `/Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting`
Branch: `codex/milestone-3-module-reporting`
PR: [cresta-proto#9676](https://github.com/cresta/cresta-proto/pull/9676)

## Objective

Replace the draft `ListTrainingModules` statistics extension with the dedicated,
batched `RetrieveTrainingSimulatorModuleStats` reporting API while preserving the
pre-PR content-list contract.

## Inputs reviewed

- `decisions/2026-08-28-dedicated-lesson-module-stats-apis.md`
- `decisions/2026-08-27-collapse-overall-zero-false-results.md`
- All 12 PR review threads, PR-level review bodies, issue comments, and the current
  PR description on #9676.
- The branch's three-dot diff against `origin/main`, the existing
  `RetrieveTrainingSimulatorTaskStats` contract, validation conventions, and
  repository generation/formatting guidance.

## Review reconciliation

- Retain and adapt the precise naming suggestions for assignment counts,
  applicable scores, and scored-result counts.
- Avoid proto3 optional scalar presence for the average; pair a normal `double`
  with `applicable_score_count` so Director does not receive a synthetic oneof.
- Give criteria their own result-specific applicable denominator rather than the
  module assignment denominator.
- Preserve deterministic failed-result-first criterion ordering.
- Keep overall zero/false/N/A outcomes failure-equivalent and remove the obsolete
  missing-result-status signal.
- Keep `BUILD.bazel` Gazelle-owned and generated clients out of the handwritten
  source change.

### Adopted

- `passed_count` / `total_count` → `passed_assignment_count` /
  `total_assignment_count`; both counts are assignment-rooted facts.
- `average_score` / `scored_count` → `average_applicable_score` /
  `applicable_score_count`; the count communicates both the denominator and
  average availability.
- Criterion ordering remains failed-result count descending with stable string
  tie-breakers.
- Gazelle owns the one-line `BUILD.bazel` addition; rerunning Gazelle made no
  further change.

### Adapted

- Jack's passed/total advice remains correct at module grain, but criterion grain
  now uses `passed_result_count` / `applicable_result_count`. Reusing assignment
  totals would misstate criterion pass rates because criterion-level N/A is
  excluded.
- The optional-scalar concern is addressed without another N/A boolean:
  `average_applicable_score` is non-optional and
  `applicable_score_count == 0` means unavailable. A real zero average remains
  distinguishable when the count is positive, and Director avoids the synthetic
  oneof generated for proto3 optional scalars.
- The generated-client comment remains operationally relevant for the new RPC,
  but generated clients are intentionally not hand-edited in this PR; repository
  automation generates them after merge.
- The earlier filter-placement concern is resolved structurally: `time_range`
  exists only on the dedicated reporting request, while `ListTrainingModules`
  is byte-for-byte restored to its pre-PR request/response shape.

### Not adopted

- Group/team filters remain out of scope because the product does not currently
  support them on this surface.
- `agent_user_names` was initially adopted from review naming guidance, then
  removed after a focused Figma/history check confirmed that the module report
  has neither module-by-agent grouping nor an assignee-population filter.
- CodeRabbit thread `r3885167462` correctly noted that generated PGV validators
  will not enforce the documented parent/name constraints. The suggestion to
  add `validate.rules` was intentionally not adopted per the reviewed contract
  direction: this PR introduces no generated-validator policy, and the future
  handler must enforce the documented constraints. Replied with that rationale
  and resolved the thread.
- Returning grouped raw task-run details is not adopted: it would move aggregation
  semantics and over-fetching into clients, while the accepted decision calls for
  a dedicated aggregate reporting API.
- Separate module failed/N/A counts are not added. The accepted reporting decision
  treats overall zero/false/N/A as failure-equivalent; the contract exposes passed
  and total assignment counts. Criterion-level N/A remains meaningful and is
  excluded through the result-specific applicable denominator.

## Implementation outcome

- Added `RetrieveTrainingSimulatorModuleStats` with the read-only GET binding and
  only `ADMIN`, `SUPER_ADMIN`, and `QA_ADMIN` roles.
- Added the bounded, ordered request with only an optional time range and the
  aligned zero-entry response contract. The request has no agent selector. Its
  bounds, uniqueness, and nonempty-name requirements are documented rather than
  expressed through PR-added `validate.rules` annotations.
- Restored `ListTrainingModules` exactly to `origin/main`, including removal of
  its AIP-132 suppression and PR-added `OPTIONAL` annotations.
- Renumbered the unmerged draft cleanly: module `applicable_score_count` is field
  7, and criterion `applicable_result_count` is field 5. No tag contains the
  draft introduction commit; only the two open reporting branches contain it.
- Removed `has_missing_result_status`; persisted evaluation status is not a
  reporting prerequisite under the accepted collapsed-result decision.
- Added criterion `non_applicable_result_count` as field 6 after product
  follow-up. It counts explicit completed criterion N/A results, remains outside
  `applicable_result_count`, and does not affect pass-rate calculation or the
  failed-result ordering key.

## Validation

- `bazel run //:gazelle` — passed; no new `BUILD.bazel` change.
- `buf lint` — passed with only the repository's existing `DEFAULT` category
  deprecation warning.
- `buf build` — passed.
- `bazel build //cresta/v1/trainingsimulator:all` — passed.
- `git diff --check` — passed.
- Re-ran the required Gazelle, full Buf lint/build, targeted Bazel build, and
  whitespace checks after removing `agent_user_names`; all passed. Bazel's
  generated workspace symlink had to be removed before Buf so the checkout was
  not traversed twice. Local `clang-format` is unavailable, but this follow-up
  deletes one complete, already-formatted field block and changes no retained
  proto formatting.
- Package-scoped `buf lint --path cresta/v1/trainingsimulator` and `buf build
  --path cresta/v1/trainingsimulator` also passed.
- `mage -v apiLint` could not execute: the required private generator image was
  absent locally and ECR returned 403. No AWS login was attempted because no AWS
  credential was individually cleared. The new RPC retains only the same
  `core::0131::synonyms` suppression used by the adjacent custom Retrieve stats
  RPC; it is necessary because `Retrieve` is an established custom reporting
  verb rather than the linter's preferred standard verb.

## Downstream impact

- The unmerged lesson proto branch reuses `TrainingSimulatorModuleStats` and must
  adopt the new field names/criterion denominator when it is reworked around its
  own dedicated API.
- The prepared module backend and Director worktrees still implement the obsolete
  `ListTrainingModules(include_stats)` transport and older draft fields. They must
  be reworked after this contract is approved; they are not released consumers.
- No generated client was edited. The eventual generated Director client will
  expose a non-optional numeric average plus count, avoiding the optional-scalar
  oneof issue raised in review.

## Module-by-agent grouping check

- Re-inspected the current Figma `Reporting` section at node `13108:21741`.
  The module table has one row per module and shows module-level values such as
  average score and lesson count. Its detail drawer shows one aggregate pass
  fraction, average score, active-lesson count, and criterion/question pass
  fractions for the selected module. No frame presents statistics grouped by
  `(module, agent)` or returns one statistics row per agent.
- The adjacent CSV annotation does list agent name and username, but describes
  exported underlying result rows for a single module, not a grouped statistics
  response or module-drawer visualization.
- Historical design notes use `(task, lesson, module, agent)` as the internal
  assignment/latest-result selection grain. The public module API aggregates
  those facts across the reporting population; this internal grain does not
  justify a public agent selector or response grouping key.
- Older product language about an eventual module drill-down by agent is a
  possible future interaction. It is not represented in the current Figma or
  the current module-statistics contract.

## Criterion identity rationale

- Removed `behavior_name` after confirming that the response's
  `training_module_name` plus `criterion_id` already identifies the criterion.
  Behavior provenance is not required by the current reporting UI. Because the
  message is unreleased, renumbered its remaining fields contiguously and changed
  the stable ordering tie-breakers to `display_name`, then `criterion_id`.

## External state

- Local source commit: `97873e3108`. The
  branch's eight PR-only commits, including the earlier
  `69b92025dbbdb447d8c8790a0b8d5b2ca1b1af21`, were collapsed into this single
  commit; the focused Figma review then removed the draft agent selector before
  local handoff.
- Force-pushed the final locally approved amend to
  `origin/codex/milestone-3-module-reporting`. Local and remote heads now both
  resolve to `97873e3108a0cc1366f1bcd3868c0c802103b7c6`.
- PR description edits and remaining review replies/resolutions are still
  pending; this push updated only the branch history and code.

## Credentials used

- GitHub CLI account `xuanyuwang`, after reading its configured credential entry
  and confirming it carried no restriction marking. Used only to read #9676.
