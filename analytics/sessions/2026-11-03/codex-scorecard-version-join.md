# Scorecard ID and source-version join

- Date: 2026-11-03 (America/Toronto).
- Primary domain: analytics; subdomain: qa-score.
- Primary source repo and checkout: /Users/xuanyu.wang/repos/go-servers, main at 35c364ff69; source inspected read-only.
- Request: explain why the PI conversation query joins parent and criterion rows on update timestamp as well as scorecard ID, and evaluate the supplied asymmetric replication timeline.

## Verified source

- `insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go:202–220` uses `qaScorecardQuery` and joins scorecard_score to filtered_scorecard on both scorecard_id and scorecard_last_update_time. The CTE name scorecard_score does not imply the legacy physical table: qaScoreQuery selects the requested table.
- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go:226–245` selects max(scorecard_last_update_time) per scorecard ID from the parent projection, then applies parent filters. Lines 378–383 explain why latest-version selection must see all-N/A parents; lines 400–401 enforce the same ID-and-version join for metrics.
- `shared/clickhouse/conversations/scorecard_score.go:328–329` writes ScorecardID from parent ResourceID and ScorecardLastUpdateTime from parent UpdatedAt for criterion rows. `shared/clickhouse/conversations/conversation.go:3209–3210` derives the parent fields from the same properties. This is a source version, not ingestion or replica arrival time.
- `shared/clickhouse/testing/schemas/ch_conv_schema.sql:473–519` includes scorecard_id, scorecard_last_update_time and a separate update_time in the score projection; ReplacingMergeTree permits physical duplicates until merges. The query's DISTINCT removes identical projected rows, not distinct source versions.
- `insights-server/internal/analyticsimpl/dedup_optimization_integration_test.go:231–325` documents and tests the all-N/A overwrite regression for both statistics and RetrieveQAConversations. Tests were read, not executed in this explanation-only investigation.

## Explanation

The logical association is already scorecard ID. Timestamp adds the requirement that criterion rows belong to the selected parent version. A plain ID join can match old and new criteria, including stale scores that no longer apply. Independently choosing the latest row per criterion also needs explicit deletion/N/A and snapshot rules; it does not establish that all rows belong to the same parent snapshot.

In the supplied hypothetical timeline, parent version V2 is visible at 09:03 while only score version V1 is visible. ID-and-version equality rejects V1 until matching V2 arrives at 09:06. The affected card contributes no rows; a conversation disappears if no other qualifying card contributes a row. This validates the logical failure window, not actual deployment routing or measured replication times. A version marker is necessary for this query's semantics but does not prove that every child row for that version has arrived.

Removing equality changes the failure from temporarily missing results to potentially stale/mixed results. Candidate designs requiring separate evaluation include publishing a complete denormalized snapshot, marking complete versions and reading only published ones, or retaining/displaying the conversation with an explicit pending score. A last-complete-version fallback requires retained full snapshots and reliable completion metadata; selecting an older surviving criterion row is insufficient, especially for an intentional all-N/A update.

## Limits and actions

- No product code, customer data, external tickets, or deployments changed. No credentials used.
- Historical CONVI-7931 context helped locate the source; this session independently verified join, writer, schema and regression-test logic. No historical customer counts are asserted as current.
- A future change must define freshness versus completeness behavior, verify the actual drawer request's score resource/filters, and test parent-first, child-first, partial-child and all-N/A transitions.
