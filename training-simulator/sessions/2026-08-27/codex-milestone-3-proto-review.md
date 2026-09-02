# Milestone 3 module-reporting proto review

**Date:** 2026-08-27
**PR:** [cresta-proto #9676](https://github.com/cresta/cresta-proto/pull/9676)
**Worktree:** `/Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting`

## Outcome

- **Observed behavior:** reviewed all PR comments and GraphQL review threads. Two actionable threads existed: verify the Training Simulator BUILD target through Gazelle and define criterion-stat ordering.
- **Observed behavior:** reran `bazel run //:gazelle`; it completed without a diff. Replied with that evidence and resolved the thread.
- **Confirmed decision:** `TrainingSimulatorModuleStats.criteria` is ordered by failed count descending, followed by display name, behavior name, and criterion ID ascending. Added this contract in commit `df15792d2d`, replied, and resolved the thread.
- **Observed behavior:** the repository formatting bot applied clang-format in commit `0e68aba6b6` without generated source.
- **Observed behavior:** API lint required consistent `OPTIONAL` annotations across `ListTrainingModulesRequest` and an AIP-132 suppression for the intentional parallel `module_stats` field. Preserved a concurrent worktree edit adding the suppression and completed the request annotations in commit `05e4c9b756`.
- **Observed behavior:** Buf lint and the Training Simulator Bazel proto build passed locally. Both CI API-lint paths, Buf generation, Bazel generation, Bazel test, breaking-change checks, dependency lint, and public-surface drift passed after the final edits. Full protobuf lint, clang-format confirmation, security scans, CodeRabbit, and code-owner approval were still pending at the last observation.
- **Observed behavior:** the latest `Check if go-servers PR is required` check passed. An earlier separate go-servers generation run failed on unrelated `RcgLobSpecialChunkingPayload.MaxTotalCharacters` drift; it was not evidence of a module-reporting contract failure and was not changed in this proto-only review task.

## Criterion N/A contract

- **Confirmed decision:** criterion N/A results remain excluded from the criterion pass-rate denominator.
- **Proposed behavior:** retain `passed_count` and `failed_count` for Milestone 3. Their sum is the explicit applicable denominator and directly supports the required passed/failed breakdown.
- **Proposed behavior:** do not replace `failed_count` with a generic `total_count`; that name is ambiguous about whether N/A is included. If N/A prevalence becomes a product metric, add an explicit `not_applicable_count` later as an additive field.

## Credentials used

- Standard GitHub SSH credential for pushes.
- Standard GitHub CLI keychain credential for PR reads, replies, thread resolution, and the `/format` command.

## 2026-08-28 follow-up

- The earlier passed/failed and applicable-only denominator proposal above is superseded.
- **Confirmed decision:** module stats expose `passed_count` and assignment-rooted `total_count`; criterion stats expose `passed_count` and share the parent total. `total_count - passed_count` is intentionally a broad non-pass remainder rather than a failure count, and pass-rate fields are omitted as redundant.
- Implemented in `6b11a998c6`, replied to the two new reviewer threads, and resolved them. Repository API lint passed.
