# CONVI-7931 Oportun process-scorecard count investigation

Date: 2026-10-29. Investigation only; no product or customer-data changes.

## Request and tracking

- User requested ticket creation first, then investigation of https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793224226297459.
- Created https://linear.app/cresta/issue/CONVI-7931, Todo / Medium / Customer Issue, assigned to Xuanyu Wang; linked Slack, Zendesk #24673, CE-8103 and CONVI-7674.
- Report: QA-NOA-BO-OPS-ZEUS APP VERIFICATION V1.1, July 1–31, QM 338 / 99.6%, PI 275 / 100%. Year and exact customer request are not yet verified.
- Primary repos: Director request/display paths and go-servers analytics; config for placement; knowledge records on main checkout.

## Initial evidence

- Prior CONVI-7667 now resolves in Linear to CE-8103, Done.
- GitHub verifies Director #22772 merged September 15 as f740a0fa253a6c304516502ac933387f09a1eb2c.
- Historical work item established July 2026 PG and CH populations of 338 for template 94e6b75d-b1eb-4e41-a477-1d070d96fa9c, revision c0139eb5, use case backoffice-processes. This is historical evidence, to refresh.
- Prior fix clears a conversation-ended-at target for standalone process scorecards. It explains zero results, not yet this partial undercount.

## Investigation plan

Verify production build and normalization, refresh PG/CH populations, inspect filters/user eligibility and score aggregation, and reproduce the 275/338 difference using exact ID sets if access permits.

## Verified investigation — October 29, 2026

The earlier Director fix is in Oportun's currently served build. The HTML serves release_director_2026-10-29-0e665dc; its commit 0e665dc37b0af0a590b776a4d48f09295e98edae contains PR #22772's merge f740a0fa253a6c304516502ac933387f09a1eb2c. Current process-filter normalization clears the conversation date target. This verifies today's build, not the build used in the original report.

For template 94e6b75d-b1eb-4e41-a477-1d070d96fa9c / revision c0139eb5 / backoffice-processes, using July 2026 UTC [July 1, August 1):

| Check | Result |
| --- | --- |
| PostgreSQL submitted scorecards | 338 |
| ClickHouse distinct parent IDs | 338; exact same ID set as PostgreSQL |
| Scorecards with valid criterion rows | 338 |
| Scorecards surviving the analytics ID + source-version join | 326 |
| Live PI, daily grouping | 326 / 100% |
| Same join, weekly-expanded [June 28, August 2) | 335 |
| Live PI, weekly grouping | 335 / 100% |

**Confirmed current defect:** 12 ClickHouse parent rows have source timestamps approximately 0.3–0.5 seconds older than both PostgreSQL updated_at and their newest criterion rows. RetrieveQAScoreStats joins scorecard ID AND scorecard_last_update_time, so those 12 disappear despite being present in both tables. A direct stored-row SQL reproduction returns 326, matching the daily UI. Example 019f37af-2221-706e-bd13-76985737143e: parent 2026-07-06 13:47:32.452418; criteria and PostgreSQL 2026-07-06 13:47:32.919140.

There is one duplicate parent ID across shards (339 returned FINAL rows, 338 distinct IDs); deduplication avoids treating it as an extra scorecard. The 12 source-version mismatches, rather than missing parent IDs, explain the reproduced daily undercount.

**Score semantics:** PostgreSQL's mean whole-card score is 99.61065088757397%, matching reported QM 99.6%. The live PI selection is Criteria adherence: weighted criterion aggregation is 99.7070884592853% on the 326 joined cards and renders as 100%. Including all 338 cards' valid criteria gives 99.72074839430327%. These are different calculations and display precision; the displayed 99.6% versus 100% alone does not establish a scoring defect.

### Affected scorecard IDs

019f37af-2221-706e-bd13-76985737143e
019f398f-5298-758a-b507-43bbc3165582
019f3e6e-d53b-718a-9da9-14523fc9111e
019f3e73-15a4-774c-ae53-df4983df1d5e
019f43aa-9361-77bd-86f8-34871cb9f298
019f484f-f0e2-7778-b8fa-e376f5c7ee87
019f4913-bac2-7354-8a04-e8b55a7b7dbb
019f4d8c-e801-7009-8ac1-45dafe7b474b
019f4dc2-4b3f-778e-bbed-06fbdc0ae301
019f5da7-271d-730f-a781-715a8a52fb0e
019fb9cf-d0ef-764d-980f-ff58464a43d7
019fb9d3-1a98-7762-ba5c-37bf1343dda6

### Remaining work

- Trace how these parent versions became stale and verify deployed write/reindex behavior. Current source rereads PostgreSQL after commit and derives both table versions from the same scorecard object; the historical failure mechanism is not yet proven.
- Check sync-monitor deployment, configured scan window, auto-heal enablement/allowlist, threshold and run logs. Current source already detects stale source timestamps; source support does not prove Oportun's historical July rows were scanned or repaired.
- Prepare a targeted reindex of the 12 cards, verify all 338 survive the strict join afterward, and validate PI under the same daily UTC filters. No reindex or customer-data mutation has been performed.
- Preserve the strict version join's all-N/A overwrite protection when considering a code fix; simply removing the timestamp predicate can mix revisions.
- The original 275 result / 63 missing IDs remains unverified. Its exact year, timezone, filters, user context and request/response are unavailable. User requested proceeding with available evidence. Today's verified defect explains 12 missing cards, not the entire reported 63.

Read-only investigation only; no product changes or customer-data mutations. Evidence includes scoped PostgreSQL, auth and ClickHouse reads, current served Director build, source tracing, live UI comparisons and saved SQL/ID-version artifacts.

## Local evidence and source trace

- `convi-7931-july-pg-parents.json`: 338 PostgreSQL parent records, scoped IDs/timestamps/score fields.
- `convi-7931-july-ch-parents.json`: 339 FINAL rows / 338 distinct IDs. Comparing newest parent versions gives identical ID, agent and whole-card score populations to PostgreSQL.
- `convi-7931-july-ch-versions.json`: parent and criterion source versions for all 338 IDs.
- `convi-7931-stale-parent-versions.csv`: exact 12 mismatched IDs with parent, criterion and PostgreSQL versions.
- `convi-7931-latest-parent-join.sql` and `convi-7931-latest-parent-join-result.txt`: direct raw-table ID/source-version join, 326 and 0.997070884592853. DISTINCT criterion projection avoids physical duplicates. This isolates the join rather than claiming a captured customer RPC.
- `convi-7931-weekly-join-result.txt`: same query with June 28–August 2, 335 and 0.9971823048746126.
- `convi-7931-july-auth-users.txt`: scoped ID/state/role lookup. Of 59 affected agents, 58 active, one inactive with 6 cards; five have dual roles. UI weekly count becomes 329 with deactivated users excluded, 310 with Agents only. Both toggles restored off and final grouping daily.

Source paths relative to `/Users/xuanyu.wang/repos`:

- `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`: lines 190–249 latest-parent selection; 365–373 weighted criteria vs whole-card calculations; 378–383 all-N/A overwrite rationale; 400–401 strict ID/source-version join.
- `go-servers/shared/clickhouse/conversations/conversation.go:3210` and `scorecard_score.go:329`: both versions derive from PostgreSQL UpdatedAt.
- `go-servers/apiserver/internal/coaching/action_create_scorecard.go` around 685–810: async post-commit work rereads write-replica scorecard and derives criteria and parent from it; `action_update_scorecard.go` around 260 schedules after transaction commit.
- `go-servers/cron/task-runner/tasks/scorecard-sync-monitor/task.go`: 398–450 classifies stale/missing candidates; 504–540 reads parent metadata; 615–620 compares microsecond-normalized PG/CH timestamps; 623–674 limits automatic reindex by enablement, allowlist, unfiltered runs and threshold. The 0.3–0.5-second mismatches are not normalization noise. Deployed backend/monitor image, configuration, run coverage and repair history were not established.
- Served Director `0e665dc37b0af0a590b776a4d48f09295e98edae`, `packages/director-app/src/components/insights/hooks/performance-filters/usePerformanceFilters.tsx`: ignores cached date target, defaults undefined and clears date target for processes (line 211). Merge ancestry verified with git.

## Access and limits

Current config resolved Oportun profile us-west-2, application DB oportun on us-west-2-prod, ClickHouse oportun_us_west_2 on clickhouse-conversations.us-west-2-prod.internal.cresta.ai, and auth DB oportun on auth-prod/chat-prod. Used dedicated connection skills; manually inspected the AWS us-west-2-prod_dev SSO profile and shared cresta session before selecting it. PostgreSQL/auth used self-authenticating cresta-cli. No passwords or connection URIs were printed or persisted. Reads only; interactive ClickHouse session closed.

Browser was authenticated as Xuanyu, not the affected customer user. Date range must apply by clicking outside the picker; closing through the opener discards the pending selection. A speculative Pacific-time query produced 328 (Central 338), which did not explain weekly 335; the expanded weekly UTC join did explain it. Original affected-session payload/HAR unavailable; user explicitly said continue with available evidence. Zendesk could not be read through Glean (NO_PERMISSION_OR_DELETED). Current source was inspected at go-servers 35c364ff69; backend deployment identity was not verified.

No product-code tests were needed for this read-only investigation. Verification consists of direct SQL, exact ID/version comparisons, source/build ancestry and live UI daily/weekly checks. Knowledge edits and ticket update are the only authored changes.
