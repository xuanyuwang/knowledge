# CONVI-7402 follow-up: `RetrieveConversationStats` value-or-missing errors

Date: 2026-09-17  
Primary source repo: `/Users/xuanyu.wang/repos/go-servers-convi-7402`  
Branch/worktree context: `convi-7402-bswift-users-getting-qa-score-stats-errors` at local `a9cf40e6c6`; PR #31335 merged to `main` as `1566a38` on 2026-09-15  
Related read-only source: `/Users/xuanyu.wang/repos/director`

## Input

- HAR: `/Users/xuanyu.wang/repos/bswift-us-east-1.cresta.com.har`
- SHA-256: `6e9bb41dae82128ed2df41223d19a3ec55a80a4f4a6db99841e4ba5e8e3407c1`
- Size: 6,779,730 bytes
- Current Linear ticket and GitHub PR were inspected read-only. The follow-up issue was created and verified through the Linear MCP.

## Verdict

The original QA fix is working, but the same Performance Insights filter also reaches `RetrieveConversationStats`, which PR #31335 intentionally left unsupported. The merged shared-parser guard now returns a clear HTTP 400 instead of silently lowering the OR into an impossible include-AND-exclude predicate.

This is not the previously noted PostgreSQL-to-ClickHouse scorecard sync gap. The response is deterministic input validation before a ClickHouse query runs.

## HAR evidence

- 33 entries total: 29 HTTP 200 and 4 HTTP 400.
- All four failures are `POST .../conversationStats:retrieve`.
- Every failure returns code 3 / `INVALID_ARGUMENT` with: `value-or-missing metadata filters are not supported for this ClickHouse endpoint`.
- The failing metadata group includes and excludes the same Team moment ID `01999ca3-50a3-7b00-be0d-9d5563bb3b66`, with selected string value `POD 1`. This is the intended `Team = POD 1 OR Team is missing` shape.
- The separate voicemail exclusion is valid and unrelated.
- Two distinct time windows fail: 2026-07-26 through 2026-08-05 and the preceding comparison window 2026-07-15 through 2026-07-25. Each is attempted twice, accounting for the four errors.

## Request-to-source trace

1. Director `AverageHandleTimeInsightsMetric` derives the page metadata moment groups and includes them in both the current-period and delta `useConversationStats` requests.
2. `RetrieveConversationStats` always selects the ClickHouse implementation.
3. `readConversationStatsFromClickhouse` calls the shared `parseClickhouseFilter` before SQL construction.
4. `validateNoMetadataMomentOverlap` sees the same metadata moment ID on both sides and returns the exact HAR error.
5. Therefore no ClickHouse query runs and cache/sync state cannot cause this 400.

The visible production page is consistent with this split: QA score and conversation-volume surfaces render, while Average Handle Time shows `--`.

## Why CI passed

This behavior is explicitly asserted by `TestParseClickhouseFilter_RejectsValueOrMissingMetadataFilter`. The PR fixed OR semantics only in `parseMomentConditionsForQAAttribute` and the two QA query builders. The shared parser used by non-QA endpoints was changed from a silent zero-row hazard to a deliberate error, so tests correctly passed without covering the full Performance Insights page request fan-out.

## Blast radius

Seventeen production ClickHouse files call `parseClickhouseFilter`, including conversation, agent, manager, assistance, hint, guided-workflow, scorecard, suggestion, summarization, knowledge-assist, knowledge-base, live-assist, note-taking, smart-compose, adherence, and customer-snapshot stats. The HAR confirms only `RetrieveConversationStats`; do not claim the others are currently failing without endpoint-specific captures. They share the same explicit rejection if they receive an overlapping metadata group.

## Recommended follow-up

Created tightly linked backend regression [CONVI-7706](https://linear.app/cresta/issue/CONVI-7706/regressionbswift-retrieveconversationstats-rejects-team-value-no-value) for `RetrieveConversationStats`; this is the same UI contract and was introduced by the merged fix's defensive shared-parser change.

Implement value-or-missing OR semantics in the conversation-stats moment query path:

- selected-value branch: preserve current latest-metadata-value behavior, including `allowMatchingStaleMetadataValues`;
- missing branch: absence of any annotation for the same metadata template in the applicable time window;
- left join both branches and require `selected value exists OR metadata annotation is absent`;
- keep unrelated include and exclusion groups AND-ed;
- add request-to-SQL and real ClickHouse fixtures for current/delta windows, value-only, missing-only, value-plus-missing, stale/latest values, and the voicemail exclusion.

Keep the explicit rejection for other shared-parser callers until each query builder can represent the OR correctly. A frontend workaround that drops Team metadata from AHT would make the displayed metric inconsistent with the selected page filters.

## Actions and credentials

- Created CONVI-7706 through the Linear MCP with High priority and a `related to` relationship to CONVI-7402; verified the saved title, description, team, priority, URL, and relationship through the same MCP.
- No product code, PR, or production state was changed.
- No AWS, Okta, Azure, or SSH credentials were read or used.
- No browser automation was used for the Linear mutation.
