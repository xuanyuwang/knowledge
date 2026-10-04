# CONVI-7706: RetrieveConversationStats value-or-missing regression

**Status:** in review
**Primary domain:** analytics
**Primary subdomain:** performance-insights
**Official ticket:** [CONVI-7706](https://linear.app/cresta/issue/CONVI-7706/regressionbswift-retrieveconversationstats-rejects-team-value-no-value)
**Last updated:** 2026-09-25

## Objective and Impact

- **Objective:** Support metadata value-plus-`(no value)` OR semantics in the ClickHouse `RetrieveConversationStats` path without enabling the shape for other shared-parser callers.
- **Customer/system impact:** Bswift Performance Insights Average Handle Time currently shows `--` because current and comparison-window requests return HTTP 400.
- **Role:** diagnosed and implemented

## Scope

**In scope**

- Recognize the supported same-template metadata value-or-missing group before the shared parser rejects it.
- Generate `selected latest value OR metadata annotation absent` in the conversation-stats query.
- Preserve `allowMatchingStaleMetadataValues` and unrelated filters such as voicemail exclusion.

**Non-goals**

- Supporting value-or-missing in other `parseClickhouseFilter` callers.
- Changing the frontend filter encoding or the existing QA implementations.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7706`
- **Branches:** `xw/convi-7706-conversation-stats-value-or-missing`, based on `origin/main` at `cda4a39f9a`
- **PRs/commits:** [go-servers PR #32440](https://github.com/cresta/go-servers/pull/32440); implementation `e7704f6637`; golden SQL coverage `b1568e1d72`

## Current Understanding

The local fix extracts only the exact metadata value-or-missing group accepted by the existing QA parser, sends all remaining filters through unchanged shared parsing, and adds two left-joined branches to the conversation-stats query. The selected-value branch retains latest-value behavior unless `allowMatchingStaleMetadataValues` is enabled; the existence branch establishes whether the metadata template is absent. Their final predicate is `selected value exists OR metadata annotation is absent`. Other shared-parser endpoints retain the explicit unsupported-filter error.

## Findings and Decisions

- Reused the existing validated value-or-missing parser, parameterized by raw versus metadata-view storage.
- Kept conversation stats on `moment_annotation_d`, matching its existing value-only and missing-only behavior.
- Kept each value-or-missing group AND-ed with other groups and preserved the independent voicemail exclusion.
- A follow-up [shared-caller audit](../sessions/2026-09-17/codex-value-or-missing-api-audit.md) found ten additional Assistance Insights RPCs structurally reachable with the same filter. Nine already support value-only/missing-only through `generateMetadataFilters` and need OR parity; `RetrieveSmartComposeStats` silently ignores ordinary metadata groups and needs full metadata support first.

## Blockers and Dependencies

- No implementation blocker.
- Required CI passes; code-owner review remains.
- The committed-query coverage gap is resolved by `b1568e1d72`: full-RPC golden SQL cases now cover latest-value and stale-enabled value-or-missing behavior.

## Validation and Rollout

- Focused Go tests pass for the new conversation-stats path, existing conversation-stats suite, shared rejection, and QA overlap parsing/query behavior.
- Existing `RetrieveConversationStats` golden SQL tests pass unchanged, which confirms ordinary metadata-filter behavior did not drift. They do not cover the new overlap shape.
- Added two full-RPC cases to `RetrieveConversationStatsTestSuite`, with independent golden SQL files for latest-value and stale-enabled query shapes. The direct structural test remains for focused parser/query-builder assertions.
- Executed both committed golden SQL shapes against the real `voice-staging` conversations ClickHouse database `cresta_walter_dev` using Walter Dev data. The synthetic fixture literals execute successfully but return no rows, as expected; substituting a real September 25–27 window, metadata template, and real string values produced non-empty aggregates in both modes.
- With real `user_hangup` + `transfer` values, both modes returned 52 aggregate rows, summed conversation count 1,173, and 44,255 handle-time seconds. With the narrower real `transfer` + `agent_hangup` values, both returned 50 rows, summed conversation count 698, and 11,892 handle-time seconds. This validates execution of both SQL shapes but the sampled data does not contain a stale-only difference for those selections.
- `bazel run //:gazelle` completed without generated changes.
- `git diff --check` passes.
- Local ClickHouse fixtures confirmed latest mode returns selected plus missing conversations, excludes a nonmatching latest value, and stale-enabled mode additionally accepts an older matching value.
- Production validation is pending deployment.

## Next Actions

1. Let CI complete for golden-test commit `b1568e1d72` and obtain code-owner approval.
2. After deployment, confirm both current and comparison-window Bswift `RetrieveConversationStats` calls return 200 and AHT renders.
3. Implement the confirmed Assistance Insights parity cluster under CONVI-7708; keep Smart Compose metadata-filter correctness separate.

## Timeline

- 2026-09-17 — Diagnosed the post-CONVI-7402 HAR regression, created CONVI-7706, and committed the focused conversation-stats fix as `891796f7d8`. Evidence: `log/2026-09-17.md`, `sessions/2026-09-17/codex-convi-7706-fix.md`.
- 2026-09-17 — Rebased the focused commit onto current `origin/main` as `e7704f6637`, reran focused tests and Gazelle, and opened go-servers PR #32440. Linear automatically moved CONVI-7706 to In Progress and attached the PR.

- 2026-09-17 — Completed [PR #32440 semantic review](../deliverables/convi-7706-pr-32440-semantic-review.md): no actionable correctness findings; 176 local ClickHouse checks pass. Reproduced the ES BasicRequest CI failure on both head and exact base. No product changes or external review submission.
- 2026-09-25 — Rechecked PR head `f1257181a0` and current CI. Required CI passes. The PR adds no golden SQL and its new test manually checks query fragments rather than the RPC-generated query, so full-RPC value-or-missing golden coverage is recommended before merge.
- 2026-09-25 — Added, committed, and pushed two full-RPC golden SQL cases as `b1568e1d72`: default latest-value selection and `AllowMatchingStaleMetadataValues=true`. The focused `TestRetrieveConversationStats` suite and Gazelle pass; broader analytics tests were intentionally not run.
- 2026-09-25 — Ran both golden SQL files on `voice-staging` for `cresta/walter-dev` with their fixture arguments and with real recent metadata arguments. Both query branches execute and return non-empty aggregates with real values; sampled latest/stale totals are equal, so no claim of observed stale-only behavior is made. Evidence: `sessions/2026-09-25/codex-convi-7706-staging-golden-sql-validation.md`.
