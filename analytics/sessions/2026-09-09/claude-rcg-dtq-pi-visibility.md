# Session: RCG DTQ PI visibility — "template intermittently not showing data to different users"

- Date: 2026-09-09 (updated same evening after viewer-account correction and log forensics; root cause confirmed 2026-09-10 via HAR)
- Tool: Claude Code
- Source: Slack #convo-intelligence thread [p1788990592932079](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1788990592932079) (Andra Harders)
- Ticket: [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after) (RCG tracking and validation ticket)
- Databases: RCG prod app DB `rcg-us-east-1` (us-east-1-prod), RCG auth DB `rcg` (auth-us-east-1-prod), RCG ClickHouse `rcg_us_east_1` — all read-only via the connect-customer-* skills. Logs via groundcover (`groundcover` backend, cluster `us-east-1-prod`).
- Method: the `investigate-analytics-data-issue` skill (coaching-qm-skills#3), first live run

## Report

RCG users see an empty Performance Insights table for template "Customer Xperience Center - Social Care DTQ - V1" while Kath Cas (kcas@rccl.com) sees data with "the same filters" in usecase CEL Customer Xperience. QM Report shows scores exist. Incognito makes no difference. Described as "intermittently not showing data to different users." A parallel RCG PI latency incident (30s+ page loads, leaders on a training call affected) was reported the same day in the same channel; Zendesk P2 #23745 reports the same for HCD.

## Findings

1. **Template family**: 69 template instances share this title, one per usecase (mass-duplicated 2026-07-01; more 2026-09-03). The instance with data is `019dbd42-2ee4-7684-84f1-faf442fdc2c4` (cel-customer-xperience, active): 190 scorecards, all process scorecards (empty conversation_id), 100% manually scored and submitted, 12 evaluated agents, latest 2026-09-04. A new same-titled pair created 2026-09-03 17:31 in cel-co-outbound-sales (`01a06853-38a9…`) has zero scorecards. Within cel-customer-xperience itself there are three exact-title instances: `019dbd42` (data), `019d5078` (active, zero scorecards ever), `019d5079` (inactive).
2. **ClickHouse is healthy**: 186 rows / 185 distinct scorecards on `019dbd42`, all `manually_scored`, 178 with submit times, `usecase_id = cel-customer-xperience` on every row, newest (Sep 2–4) present and fresh. Not a sync outage.
3. **Viewer accounts (auth DB)**: Andra has two RCG auth entries — `aharders@rccl.com` (roles {AGENT}) and `andra.harders@cresta.ai` (all roles, type 3). She reports she was using the **Cresta account**. Kath Cas has manager-plus roles {CI_ADMIN, QA_ADMIN, QA_SPECIALIST, OPERA_ADMIN, MANAGER_2ND, 18, 19}.
4. **Groundcover log forensics (her session `ae16aa4d…` on /director/insights/performance, 21:34–21:55 UTC)**:
   - `ParseUserFilterForAnalytics`: `isACLEnabled=true isRootAccess=true`, `FinalUsers length=10031 ShouldQueryAllUsers=true` — her requests are **unscoped** (all users). ACL and the agent-only hypothesis are both dead for her account.
   - Zero `there is no scorecard template found` warnings for RCG → her template filter resolved to a real template (not the empty-instance early return).
   - Her RetrieveQAScoreStats queries ran 2–7s each, ~20+ in parallel; one `context canceled` at 21:34:09 (client aborted 103ms in, 2m deadline remaining), one ES failure at 21:17. Session totals: 8 error, 16 warning events.
   - RCG-wide 19:00–22:00 UTC: 77 RetrieveQAScoreStats errors (all sampled ones `context canceled` — client aborts, not server timeouts), 218 RetrieveHintStats, 96 RetrieveAdherences, 39 RetrieveConversationMessages errors.
5. **Production query shapes** (from error logs): (a) `conversation_d` scan bounded by `conversation_end_time` + `agent_user_id IN (SELECT user_id FROM agent_filter)` + `usecase_id IN (?)`; (b) `scorecard_d` bounded by `scorecard_time` + `agent_user_id IN (agent_filter)` + `usecase_id IN (?)` + `scorecard_template_id IN (?)` — the process-template path (no conversation join). Template ids are placeholder parameters, so `system.query_log` text cannot reveal which instance a request used (also, query_log is per-node behind the proxy).

## Root cause — CONFIRMED (HAR evidence, 2026-09-10)

Andra's HAR capture (`~/Downloads/rcg.cresta.com.har`, 20 `qaScoreStats:retrieve` entries) settles it:

- Every request uses the **correct** template `customers/rcg/profiles/us-east-1/scorecardTemplates/019dbd42-2ee4-7684-84f1-faf442fdc2c4`, correct usecase `cel-customer-xperience`, no user/group/status filters, `includeNaScored: true`, `filterToAgentsOnly: false` — and every response is 200 with `totalScorecardCount: 0`.
- Windows were August and July 2026, which **contain** CH data (51 in-window scorecards with scorecard_time Aug 23–29) — so this is not a time-window empty.
- Every request carries `conversationTimeRangeField: TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`.

The chain: director `usePerformanceFilters.tsx:112-113` defaults `dateRangeTarget` to `CONVERSATION_ENDED_AT` since **[CONVI-7586] / director PR #22057, merged 2026-08-26** ("Set date range filter default to close date"). The process-template filter-stripping in the same hook does **not** reset `dateRangeTarget`, and `useQAScoreStatsRequestParams.ts:42` forwards it as `conversationTimeRangeField`. In `retrieve_qa_score_stats_clickhouse.go:510-511 + 63-64`, ENDED_AT sets `needsConversationEndTime=true`, building a `conversation_end_time`-bounded `conversation` CTE and an **INNER JOIN `scorecard.conversation_id = conversation.conversation_id`**. All DTQ process scorecards have `conversation_id = ''` → the join eliminates every row → `totalScorecardCount=0`.

ClickHouse reproduction (bit-for-bit): 51 scorecards in the August window → **0** after the conversation join; `empty_conversation_ids = 51`.

**Per-user "intermittency"**: users whose persisted localStorage filter state (`performance-page-v2`) predates the default (or otherwise lacks/overrides `dateRangeTarget`) send no ENDED_AT field → no join → data visible (Kath). Fresh or reset sessions get the new default → empty (Andra, including incognito). Onset ~early September matches the Aug 26 deploy.

Rejected with evidence:
- ACL scoping for Andra — `isRootAccess=true`, FinalUsers=10031.
- Agent-only status forcing — flagd `enableScorecardPublish` prod targeting is on only for cng/wyndham, not rcg.
- Sync gaps / missing CH rows — 51 in-window rows present; data fresh.
- Usecase_id mismatch — all rows carry cel-customer-xperience.
- Wrong template instance — HAR proves every request used the data-bearing instance.
- Time-window emptiness — August window provably contains rows.
- "No template found" early return — zero warnings; FE cache — server-side.

The canceled-query/2-7s-latency observations are a real but **separate** RCG PI performance incident (same-day thread, ZenDesk #23745 for HCD); they cause slowness, not the empty DTQ table.

## Fix direction

- FE (minimal): when the selected template is process type, omit/reset `dateRangeTarget` (add to the process-template filter reset in `usePerformanceFilters`).
- BE (robust): skip the conversation CTE/join when the resolved templates are process-only, or left-join and pass rows with empty conversation_id.

## Secondary observations

- Process scorecards' `scorecard_time` is the evaluated date (day-truncated midnight ET), up to ~2 weeks before `scorecard_submit_time`; recent-window PI views can legitimately look empty.
- 8 of 185 CH rows have stale zero-epoch `scorecard_submit_time` while PG shows submitted (e.g. scorecard `01a06475…`): the known stale-field class the sync monitor cannot detect. 185 vs 190 PG delta unexplained (PG count not yet filtered for expected-zero classes).
- QM Report showing scores is a different surface; not comparable without normalizing predicates.

## Next actions

1. Confirm with Andra: (a) does the table show empty or never finish loading; (b) the exact template resource id in the network request (DevTools → RetrieveQAScoreStats → `filterByAttribute.scorecardTemplates`), and whether the dropdown entry she picks carries an "(inactive)" suffix or which usecase is selected. `user_history_correlation_id` (req-…) in the logs can also pin the request payload via the history system.
2. Treat the empty-table reports as part of the RCG PI latency incident until (1) says otherwise: the cancellation storm and error counts are the measurable signal.
3. Longer term: duplicate-template hygiene (69 same-titled instances across usecases; new pair created Sep 3) and surfacing usecase/template identity in the dropdown label.
4. Optionally: targeted reindex for the 8 stale-submit-time scorecards.
