# Weekly Summary - 2026-07-21 to 2026-07-27

**Created:** 2026-07-24
**Timezone:** America/Toronto
**Note:** Week in progress (Friday); evidence through 2026-07-24.

## Progress

### CONVI-7350: Latest template revision for submitted-scorecard authorization
- Diagnosed a Walter-dev `UpdateScorecard` 403 where proactive `EvaluateScorecardsPermissions` returned `allowed: true` but the write path denied against the scorecard's pinned historical revision.
- Implemented the fix: `hasSubmittedScorecardEditPermission` now loads `LatestRevisionName` internally; `UpdateScorecard` and `ResetScorecard` no longer pass the pinned template into authorization.
- Added revision-mismatch regression tests; focused and full update/reset/evaluate coaching suites passed.
- Opened and merged [go-servers #30336](https://github.com/cresta/go-servers/pull/30336).
- **Status:** validating — Walter-dev manual verification pending.
- **Evidence:** `convi-6862-disable-editing-on-submitted-scorecard/work-items/CONVI-7350.md`, `convi-6862-disable-editing-on-submitted-scorecard/log/2026-07-22.md`

### CONVI-7254: HCD monthly QA criterion shows 0%
- Reproduced HCD's May near-zero monthly criterion score from production ClickHouse: 4,747 semantic weight-zero rows stored as `1e-13` dominate three failing unit-weight rows from a brief May 29 weight-1 window.
- Reconstructed and executed the exact `RetrieveQAScoreStats` SQL shape from unit-test golden files; result matches the API response exactly.
- Ruled out time truncation, voicemail filtering, conversion, and cache as causes.
- Recovered production template history: weight `1 → 0 → 1` across May 29–July 1 revisions.
- Produced an interactive calculation-pipeline report and promoted the full investigation to `analytics/deliverables/hcd-mixed-revision-qa-score-investigation.md`.
- **Status:** active — root cause diagnosed; fix not yet selected.
- **Evidence:** `analytics/work-items/CONVI-7254.md`, `analytics/log/2026-07-24.md`

### CONVI-7238: United manager scorecard undercount
- Refined root cause with ClickHouse and PostgreSQL evidence: Emma has 51 submitted/manually scored scorecards — 44 Chat Quality Form rows with negative/N/A aggregate score and 7 Engaging rows with non-negative score.
- Both aggregate and detail requests set `includeNaScored: false`, so they consistently exclude the 44 Chat Quality Form rows and return only the seven Engaging rows.
- Validated score correctness for conversation `84608747-b451fdca-43d8-448f-b35b-9d5bf8f02ac0`; found an independent projection defect where `scorecard_d` remains on an older draft version despite submitted current metadata in Postgres.
- **Status:** active — investigation complete; product decision and fix pending.
- **Evidence:** `analytics/work-items/CONVI-7238.md`, `analytics/log/2026-07-23.md`

### CONVI-7049: CLO outcome moment filter performance review
- Compared generated `RetrieveQAScoreStats` SQL for metadata moments vs newly supported CLO outcome moments from [go-servers #29175](https://github.com/cresta/go-servers/pull/29175).
- Confirmed metadata-only SQL is unchanged by the CLO change.
- Identified performance risk: CLO uses raw `moment_annotation_d` with query-time JSON extraction, while metadata uses the narrower, monthly-partitioned `moment_annotation_mv_by_metadata_d`.
- Identified mixed-filter risk: any CLO group switches all moment groups in the request, including existing metadata filters, back to the raw table.
- Recommended per-group table routing and runtime validation with `EXPLAIN indexes = 1` and query-log metrics.
- **Status:** review complete; implementation/validation not started.
- **Evidence:** `convi-7049-clo-filter/log/2026-07-23.md`, `convi-7049-clo-filter/sessions/2026-07-23/codex-query-structure-performance.md`

### Scorecard Data Sync: Slack monitor reporting ambiguity
- Explained why the Snap Finance monitor reported zero missing scorecards while dispatching a three-scorecard reindex workflow: Slack renders only missing-row statistics, but auto-heal candidates are the union of missing and existing-but-stale scorecards.
- Recommended including stale and total candidate counts in created-job lines so operators do not interpret `0 missing` as `0 needing repair`.
- **Status:** diagnosis complete; monitor UX improvement not yet implemented.
- **Evidence:** `scorecard-data-sync/log/2026-07-22.md`, `scorecard-data-sync/sessions/2026-07-22/codex-slack-missing-vs-reindex.md`

### Active Days behavior guide
- Rewrote the Agent Leaderboard Active Days behavior guide for non-engineering and engineering audiences.
- Clarified message-based vs no-agent-message attribution rules and promoted heading levels for Superhuman paste compatibility.
- **Status:** deliverable ready; manual paste into Superhuman doc pending.
- **Evidence:** `active-days/log/2026-07-22.md`, `active-days/deliverables/agent-leaderboard-active-days-behavior-guide.md`

### Knowledge repo: domain catalog metadata
- Published the four-domain catalog, including eight Analytics and seven Scorecard Workflows subdomains, across the root README, canonical domain model, workspace metadata, and tool adapters.
- **Status:** complete.
- **Evidence:** `workspace/log/2026-07-24.md`, `workspace/sessions/2026-07-24/codex-domain-catalog-metadata.md`

## Plan

### Next-week priorities
1. **CONVI-7254:** Confirm intended criterion-grouped weighting semantics with stakeholders, then implement and validate the agreed aggregation fix against production queries.
2. **CONVI-7238:** Decide whether Manager Leaderboard `Scorecards evaluated` should count submitted scorecards with N/A aggregate scores; if yes, set `includeNaScored: true` and recompute expected counts.
3. **CONVI-7238:** Measure and repair the `scorecard_d` projection gap (starting with scorecard `019f3e97-a65f-74a9-8b70-f7c93f377833`).
4. **CONVI-7350:** Complete Walter-dev verification for the merged submitted-editor authorization fix.
5. **CONVI-7049:** Validate CLO moment filter performance with per-group table routing and query-log evidence before broad rollout.
6. **Active Days:** Paste the prepared Markdown into the Superhuman behavior guide.
7. **Time range filter investigation:** Verify process scorecard `scorecard_time` population and update subdomain READMEs with time-range semantics.

### Follow-ups
- Track Director fail-open behavior on permission-query errors separately from CONVI-7350.
- Improve scorecard-sync-monitor Slack reporting to distinguish missing vs stale candidates.
- Verify Manager Leaderboard frontend daily-row accumulation when aggregate responses contain multiple daily rows per manager.

## Problems

### Open product/semantics decisions
- **CONVI-7254:** Cross-template-revision weighting within criterion-grouped aggregation produces misleading monthly scores when a criterion briefly had weight 1 before reverting to 0. Fix blocked on agreement about intended semantics.
- **CONVI-7238:** `includeNaScored: false` intentionally excludes 44 of 51 scorecards for Emma; unclear whether Manager Leaderboard should count N/A-submitted scorecards.

### Correctness and reliability risks
- **CONVI-7238:** Independent PG→CH projection defect — scorecard `019f3e97-a65f-74a9-8b70-f7c93f377833` has submitted current metadata in Postgres and criterion-level `score_d`, but `scorecard_d` remains on an older draft version across all replicas.
- **CONVI-7049:** CLO outcome moment filters route through the raw `moment_annotation_d` table; a mixed request with both CLO and metadata filters forces all moment groups onto the slower path, creating a performance regression risk.
- **CONVI-7350:** Director still has a secondary fail-open edge where a permission-query error leaves `allowed` undefined and the submitted scorecard form editable.

### Operational ambiguity
- Scorecard-sync-monitor Slack output shows `0 missing` while still dispatching reindex workflows for stale scorecards, which can mislead on-call operators about repair scope.

### Evidence gaps
- CONVI-7350 Walter-dev verification not yet recorded post-merge.
- CONVI-7049 performance risk is code-review based; no production query-log validation yet.
- Active Days Superhuman document not yet updated with the new guide content.
