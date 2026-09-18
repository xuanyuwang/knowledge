# CONVI-7402 PR #31335 review-comment triage

**PR:** https://github.com/cresta/go-servers/pull/31335
**Domain:** analytics (`qa-score`)
**Source repo / branch:** `/Users/xuanyu.wang/repos/go-servers-convi-7402` / `convi-7402-bswift-users-getting-qa-score-stats-errors`
**Reviewed head:** `035bcedd9e` (`unify tests`)

## Review state

The PR is open, out of date with `main`, awaiting one code-owner approval, and merge-blocked by three unresolved human review threads. GitHub reports 17 successful checks, 9 skipped checks, and the code-owner check waiting for approval.

## Actionable findings

1. **Fix submit-time filtering before merge (medium, correctness).** `readQaScoreStatsFromClickhouse` deliberately removes the conversation time range when `TimeRangeFilterTarget` is submit time, but still gives `req.FilterByTimeRange` to `parseMomentConditionsForQAAttribute`. The overlap CTEs therefore filter annotations by conversation start/end time while scorecards are selected by submit time. A scorecard submitted inside the requested window for a conversation outside it can be incorrectly classified as missing the metadata annotation. Use the same submit-time treatment for the value/existence CTEs and add a regression covering an out-of-window conversation with an in-window scorecard submission.
2. **Alias the QA-conversations overlap CTE output (low, defensive consistency).** Rename `t1.conversation_id` to an indexed output such as `conversation_id_<n>` and update the join/predicate references, matching the sibling QA-score-stats builder. Even if fully qualified references avoid an immediate ambiguity, this is a small consistency fix that removes the review concern and makes multi-group joins safer. Add a multi-group SQL-builder regression, ideally including a normal include plus an overlap group.
3. **Make the supported semantic scope explicit (medium, product/API decision).** The parser accepts exactly one included and one excluded `CONVERSATION_METADATA` moment with the same ID. Elasticsearch supports broader mixed groups, including multiple moments, different IDs, and conversation outcomes. For this customer fix, keep the PR narrowly scoped unless product behavior requires full parity, but document the limitation and create a follow-up ticket with explicit expected semantics and tests. Do not silently claim general Elasticsearch parity in the PR description.
4. **Track `parseClickhouseFilter` parity separately (medium, correctness across other endpoints).** Conversation Stats, Agent Stats, and other endpoints using this helper still AND the selected-value include with the missing-value exclusion for an overlapping metadata group, yielding zero rows. Because fixing the shared helper broadens an already large PR, create a dedicated follow-up and link it before merging; raise priority if the same saved filter is used on those production surfaces.
5. **Apply cheap current-code cleanup.** Add the required `// Given`, `// When`, and `// Then` markers to the new parser and query tests. The earlier slice-backing-array concern is already addressed at current head by allocating a new combined slice in both builders. The duplicate top-level query test is also already addressed by commit `035bcedd9e`.
6. **Defer optional refactors unless required by the reviewer.** Extracting the overlap parser branch and duplicated CTE construction into helpers would improve maintainability, but it should follow correctness fixes and can be split out to avoid more churn in a PR already above the repository's size warning threshold.

## Suggested completion sequence

1. Implement and test the submit-time fix.
2. Apply the alias and Given/When/Then cleanup; add execution-oriented or multi-group coverage for the alias concern.
3. Reply to the three human threads with the fix or explicit scope decision, then resolve them.
4. Create and link follow-ups for broader Elasticsearch parity and shared `parseClickhouseFilter` behavior.
5. Update the branch from `main`, rerun focused parser/query tests plus the analytics integration suite where ClickHouse/Postgres are available, push, and re-request code-owner review.

## Non-actions

- The resolved CodeRabbit duplicate-test thread needs no further work.
- CodeRabbit's unavailable Linear/Glean integration warning is review-tool configuration noise, not a product-code blocker.
- Splitting the existing PR solely to satisfy the size bot is optional; documenting why the SQL golden coverage is large is sufficient if the change stays cohesive.

## Implementation (2026-09-08)

Each change was committed before starting the next:

1. `33340674eb` — `[CONVI-7402] Align moment filters with submit time`: submit-time QA score stats now omit conversation-time bounds from metadata/outcome moment filters; added a no-conversation-window parser regression.
2. `64a3998849` — `[CONVI-7402] Alias overlap conversation IDs`: added indexed output aliases to QA-conversations overlap CTEs and updated the existing multi-group SQL golden.
3. `440f030a5f` — `[CONVI-7402] Clarify supported mixed moment groups`: documented the intentionally narrow same-ID metadata scope and added explicit rejection tests for different-ID metadata and outcome groups.
4. `5e57e89905` — `[CONVI-7402] Reject unsupported shared overlap filters`: stopped shared non-QA ClickHouse endpoints from silently returning zero rows for value-or-missing filters; they now return an explicit unsupported-filter error until the shared query representation can preserve OR groups.
5. `e2e3f38d31` — `[CONVI-7402] Structure overlap parser tests`: added required Given/When/Then sections to the original overlap parser subtests.
6. `e2e36a4847` — `[CONVI-7402] Extract overlap moment parsing`: extracted the nested QA overlap parser branch without changing accepted shapes.
7. `1b2a95bd9a` — `[CONVI-7402] Share QA moment CTE construction`: replaced duplicated QA score/conversation latest-value CTE construction with one parameterized helper.

Final validation passed: combined focused Go tests covering overlap parsers, the shared-helper rejection, QA score-stats SQL, and both QA retrieval suites; `bazel run //:gazelle` was run before every commit and produced no BUILD changes. All seven commits were pushed to the PR branch, whose remote head is now `1b2a95bd9a21c2453cd70609ebd20c36f0365ed7`; the source worktree is clean and synchronized with the remote.
