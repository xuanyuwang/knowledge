# CONVI-7384 Coaching Hub time-semantics investigation

## Context

- Ticket: CONVI-7384
- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7384`
- Branch: `convi-7384-coaching-scorecard-time-parity`
- Related repos: `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/cresta-proto`

## Inputs Reviewed

- Supplied WEEKLY `RetrieveQAScoreStats` request/response: six scorecards, 91.6667%.
- Supplied MONTHLY response: four scorecards, 93.75%.
- Supplied Scorecards-tab `GET /v1/.../scorecards` response: five scorecards with scores 75, 100, 100, 100, 100.
- Linear CONVI-7384 and related INSI-3993.
- Slack threads linked from both tickets.
- Director Coaching Hub scorecard hook and QA request paths.
- `RetrieveQAScoreStats` ClickHouse query construction and Coaching `ListScorecards` proto mapping.

## Runtime Evidence

Latest RCG production `scorecard_d` rows for agent `ee2c13951bd3d924`, use case `cel-retreat`, and template `019d4e5d-b0a7-74ca-add3-3c051fba74bc` show:

- one scorecard with `scorecard_time=2026-05-28`, `submit_time=2026-06-01`, score 100;
- four scorecards with June `scorecard_time`, submitted June 11/25/25/26, scores 100/100/100/75;
- two scorecards with `scorecard_time=2026-07-02`, one submitted July 2 and one July 6, scores 100/75.

Exact memberships:

- WEEKLY Performance range May 31 04:00Z–July 5 04:00Z over `scorecard_time`: six rows, `(100+100+100+75+100+75)/6 = 91.6667%`.
- MONTHLY June range June 1 04:00Z–July 1 04:00Z over `scorecard_time`: four rows, `(100+100+100+75)/4 = 93.75%`.
- Scorecards tab June range over `scorecard_submit_time`: five rows, `(100+100+100+100+75)/5 = 95%`.

## Code Findings

- `RetrieveQAScoreStats` maps the default conversation-start basis to ClickHouse `scorecard_time`.
- Both filtering and time grouping use that mapped column.
- `ListScorecards` is the RPC behind `GET /v1/{parent}/scorecards`.
- Director's Coaching Hub Scorecards tab passes `startSubmitTime` and `endSubmitTime`.
- Template filters use the base template ID, so revision `f9839486` is not excluded by revision matching.

## Hypotheses

1. Frequency-dependent date boundaries change membership.
2. Performance and Scorecards use different time bases.
3. Template revision filtering excludes the early scorecard.
4. ClickHouse projection or dedup loses rows.
5. Monthly truncation drops qualifying rows.

Production evidence confirms 1 and 2, rejects 3 and 4 for this repro, and makes 5 unnecessary. End-to-end instrumentation was added to capture the actual request boundaries and returned buckets before selecting a fix.

## Instrumentation

Temporary logs in `retrieve_qa_score_stats_clickhouse.go` write NDJSON to `/Users/xuanyu.wang/repos/.cursor/debug-407263.log` with session ID `407263`. Logs capture:

- frequency and filter boundaries;
- whether the query uses scorecard time and/or submit time;
- returned time buckets, weighted sums, and counts.

No user IDs, template IDs, criterion IDs, or secrets are logged.

## Next Step

Run the supplied WEEKLY request and the corresponding MONTHLY request against the instrumented backend, then inspect the clean debug log and decide whether intended parity should use submit time, scorecard time, or explicitly differentiated labels/ranges.
