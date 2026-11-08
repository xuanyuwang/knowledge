# CONVI-7931: compare the 12-card loss with the recent sync issue

- Date: 2026-11-04 (America/Toronto).
- Primary domain: analytics; subdomain: qa-score; canonical work item CONVI-7931.
- Primary source repo: /Users/xuanyu.wang/repos/go-servers, main checkout, read-only investigation.
- User asks whether the 12 missing Oportun cards are caused by the recent documented delayed ClickHouse writes that leave no matching parent/criterion join.

## Hypothesis and checks

Compare issue 7 (asymmetric projection visibility) and issue 6 (older projections arriving later) in scorecard-data-sync/deliverables/scorecard-races-corrected-recommendations.md against the saved October 29 ID/version set. Recheck the 12 live parent/criterion rows, including projection update_time separately from source scorecard_last_update_time. No customer data mutation or repair.

## Starting evidence

All 12 saved parent source versions are older than criterion versions, and criterion versions equal PostgreSQL updated_at. This is the reverse of the specific parent-first / criteria-late example in issue 7. Same equality join failure does not establish the original delivery cause. The timestamps differ by approximately 0.34–0.50 seconds but are source-version timestamps, not measured ingestion delays. October 29's direct stored-row reproduction gave 326 from 338 IDs.

## Live result

On November 4, read-only queries of `scorecard_d` and `score_d`, both raw and FINAL, found the exact same parent/criterion source versions for all 12 IDs as October 29. Each criterion projection contains 38 distinct criterion IDs, and raw versus FINAL returned the same one source version per card/table. These are surviving versions on the replicas selected by the Distributed query, not a retained history of all writes.

Every parent has an older source version but a later projection `update_time` than its criteria. The parent build times are 11.779–1654.746 milliseconds after the criterion build times. Source-version gaps are 339.341–504.044 milliseconds. Neither field directly measures network delivery/replica arrival time.

Example `019f37af-2221-706e-bd13-76985737143e` (all July 6, 2026):

| Field | Parent | Criteria |
| --- | --- | --- |
| Source `scorecard_last_update_time` | 13:47:32.452418 | 13:47:32.919140 |
| Projection `update_time` | 13:47:32.947027 | 13:47:32.928947 |

The stale parent projection was built 18.080 ms after the newer criterion projection. Its source version does not equal the criterion version, so the strict analytics join rejects it. The same source mismatch was observed in October and remains visible in November; ordinary short-lived propagation delay is insufficient to explain this persisted state.

## Comparison with the reviewed issues

- **Issue 7:** same immediate query failure—no matching `(scorecard_id, source version)`—but a different observed direction and duration. The described example has a new parent waiting for new criteria; these 12 have new criteria paired with stale parents months after the source timestamps.
- **Issue 6:** stronger match to the observed data. The old parent carries a later projection build time, and the replacement engine uses projection `update_time`, allowing an older source snapshot to survive. This is evidence consistent with an overlapping/stale write; it does not prove the original two-worker order or that a newer parent was ever successfully written. A failed/partial write can also leave mismatched tables.
- **Conclusion:** confirmed durable parent/criterion version mismatch causing the 12-card omission; not established as issue 7's transient replication window. Issue 6's stale projection ordering is the leading recorded mechanism to investigate. Do not call the 0.34–0.50-second source gap measured ClickHouse lag.
- **Fix implication:** a latest-common-version fallback cannot recover these IDs from the raw versions observed here: parent has V1, criteria have V2, intersection empty. Reconstructing coherent projections is needed; removing version equality risks mixing snapshots/all-N/A regressions. No repair performed.

## Source evidence

### Replica cross-check

`clusterAllReplicas('conversations', oportun_us_west_2.scorecard/score)` returned 72 grouped rows: 12 IDs × 2 tables × 3 nodes. For each ID, all three nodes have the exact same source versions as the October 29 evidence; each node has one parent and 38 criteria. Nodes: chi-conversations-conversations-0-0-0, chi-conversations-conversations-0-1-0, chi-conversations-conversations-0-2-0. This rules out a single lagging read replica as an explanation of the observed mismatch. SQL/result saved in `convi-7931-all-replicas.sql` and `convi-7931-all-replicas-result.txt`.

Read-only source remains go-servers main `35c364ff69`; deployed backend identity and July writer implementation were not established.

- `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go:226–245,400–401`: choose greatest parent source timestamp, join on ID plus source version.
- `shared/clickhouse/conversations/conversation.go:3209–3216`: parent source marker is PG UpdatedAt; separate projection update_time is `time.Now()`.
- `shared/clickhouse/conversations/scorecard_score.go:255,328–338`: criteria carry parent PG UpdatedAt and separately generated build time.
- `shared/clickhouse/testing/schemas/ch_conv_schema.sql:410–448`: scorecard replacement version is update_time. A fresh production `system.tables` query independently confirmed both `scorecard` and `score` use `ReplicatedReplacingMergeTree(..., update_time)`. The production Distributed tables use `Distributed('conversations_write', 'oportun_us_west_2', 'scorecard'/'score', toUnixTimestamp(scorecard_time))`. The extra all-replica check used the `conversations` cluster and explicitly establishes agreement on its three returned nodes; no unqueried topology claim is made.
- `apiserver/internal/coaching/action_create_scorecard.go:771–797`: current async path constructs/writes criteria then constructs/writes parent. Parent and child writes are separate; one worker's source object supplies both source-version fields, while overlapping workers can finish their table writes in different orders.
- Reviewed reference: `scorecard-data-sync/deliverables/scorecard-races-corrected-recommendations.md:53–73`, issues 6 and 7; November 3 analytical explanation at `analytics/sessions/2026-11-03/codex-scorecard-version-join.md`.

## Evidence files and access

- `convi-7931-live-versions.sql`, `convi-7931-live-versions-result.txt`: fresh read-only raw/FINAL versions and build timestamps.
- `convi-7931-live-comparison.csv`: normalized 12-ID comparison, checked against October 29 CSV with assertions.
- Current config master `4e60169`: `configv3/prod/oportun/us-west-2/config.yaml`; cluster us-west-2-prod; ClickHouse host clickhouse-conversations.us-west-2-prod.internal.cresta.ai; database oportun_us_west_2.
- Used connect-customer-clickhouse skill with AWS SSO profile us-west-2-prod_dev; manually read its own config entry and shared cresta SSO session first. No restricted markings there. No secrets printed/persisted. No new PostgreSQL/auth reads.
- Config fetch encountered case-colliding remote refs. A process-local extra refspec did not override all configured fetches; used a temporary master-only shallow metadata repo instead. Aborted an unnecessarily large unshallowed temporary fetch. Original config checkout/ref settings were not edited.
- Product code and customer data unchanged. No product test suite needed; evidence is scoped SQL, saved-version comparison and source tracing. Original reported 275 / 63-card difference remains unverified and outside what the 12-ID read proves.

Fresh engine query: `SELECT name,engine,engine_full,sorting_key FROM system.tables WHERE database=currentDatabase() AND name IN ('scorecard','score','scorecard_d','score_d') FORMAT JSONEachRow`. Confirmed replacement by build/update time rather than source version; no actual network arrival timestamps or historical superseded parent version were recovered.

CONVI-7931 description updated with the comparison and independently read back; its existing In Progress state was preserved. Knowledge work item, README and November 4 daily log updated, preserving unrelated changes.

## Ticket-summary follow-up

User requested the ticket state that the earlier 275 count most likely came from a temporary visibility window (issue 7), while the 12 persistent missing cards came from stale-write ordering (issue 6). Prepended these as the current assessment in Linear, retaining the confidence limits: no original 275 request-time evidence, and no recovered historical worker sequence. Read back to verify both points and preserve In Progress status. Updated the canonical work item and daily log; no new customer reads or mutations.
