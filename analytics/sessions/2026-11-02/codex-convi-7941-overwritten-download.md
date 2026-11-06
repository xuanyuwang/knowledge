# CONVI-7941 overwritten download investigation

- Date: 2026-11-02 (America/Toronto)
- Primary source repo for this session: /Users/xuanyu.wang/repos/director
- Source context: main checkout, read-only investigation; no source edits.
- Knowledge domain: analytics; subdomain: performance-insights.
- Ticket: https://linear.app/cresta/issue/CONVI-7941/download-from-pi-on-of-overwrittens-not-wokring
- Ticket is Triage, assigned to Xuanyu Wang, with a one-sentence description and no comments/attachments.
- User authorized testing via the existing Walter Dev Chrome tab. Initial view: xuanyu na score template, This week / Weekly, target audience 00, exclude voicemail Yes; zero scorecards.
- Investigating displayed Number of overwritten counts, CSV exports, and detail downloads.

## Database discovery

The user did not know a template with nonzero overwrites and explicitly authorized checking the database. Applied `connect-customer-app-db` and `connect-customer-clickhouse` skills. Resolved staging placement from current config, read the `voice-staging_dev` AWS profile and its `cresta` SSO entry before explicit profile use, and used the skills' secret-safe launchers. AWS SSO refreshed successfully. PostgreSQL access used `cresta-cli --read-only`; all database queries were SELECT, SHOW, or DESCRIBE. No secrets were saved.

ClickHouse connection: `clickhouse-conversations.voice-staging.internal.cresta.ai:9440`, database `cresta_walter_dev`. Only this database matched Walter on the cluster. The legacy `scorecard_score` family stops in May 2024 and is not suitable for current discovery. Current data is in the `score` / `score_d_read` family.

Discovery query (candidate search, not final API-equivalent population):

```sql
SELECT scorecard_template_id,
       uniqExact(score_id) AS overwritten_criteria,
       uniqExact(scorecard_id) AS scorecards,
       max(scorecard_time), groupUniqArray(usecase_id)
FROM score_d_read
WHERE manually_scored AND ai_scored AND scorecard_time >= '2026-05-01'
GROUP BY scorecard_template_id
ORDER BY overwritten_criteria DESC LIMIT 15;
```

Thirteen candidates found. `0199ba42-d3af-76b9-95db-5611ef8d15e3` had six criterion IDs across three scorecards with newest scorecard time October 6. A follow-up FINAL query confirmed six criterion rows, including the October 6 card `01a10e87-02c8-7a16-96e7-761466263226` and posit criterion `01a01bbd-ec9b-7601-9411-35ca23583c6a`; stored `ai_value=-1`, `numeric_value=0`, `not_applicable=false`, voicemail false.

PostgreSQL: account/cluster `voice-staging`, database `walter-dev`, schema `director`. `SELECT DISTINCT resource_id,title,type,status FROM director.scorecard_templates WHERE resource_id IN (...)` mapped candidates to:

- `0199ba42-d3af-76b9-95db-5611ef8d15e3`: 06.10.25
- `4464b401-2b42-4ecc-915e-0400691be5ea`: 1702455694878 Copy
- `019b9821-290a-754b-9cf5-774762079850`: multiselect for dropdown
- `019ef552-40a9-76ff-aea3-77e3b4906356`: Test auto NA score - Eleanore

Template revisions produce multiple rows per resource ID; DISTINCT was used for discovery. No database changes or reindexing.

## Browser reproduction and correction

Selected **06.10.25**, **Last 30 days / Weekly**, cleared initial audience **00**, retained **Exclude voicemail: Yes**, **Calculation: Criteria adherence**. Set all three table configurations to Number of overwritten. Progression shows total 1 and October 4 bucket 1 for Mathias Test Requirement-Behavior and posit; All criteria shows one unique overwritten conversation.

For posit, clicked its October 4 count → popover shows October 4–10 and one overwritten conversation → Download scorecard → Export, Include comments checked.

The download-event observer timed out after 60 seconds. That was not sufficient evidence that no download occurred: subsequent inspection of Chrome's download history and disk found the file. Browser logs also contained an Insights 401 expired-token error during the investigation; do not attribute the eventual successful export or original ticket to that error without a request trace. Reloaded the page to refresh the session and repeated the detailed export. Both saved files are 830 bytes, each has one identical row. Therefore the refreshed case succeeds; the initial observer timeout is not a reproduced product failure.

Saved CSV fields:

- Agent: voice_transcription_test_agent; team: default.
- Conversation platform ID: `96a81da9-c21f-4fad-809d-bbbb81a9e0fd`.
- Conversation resource ID: `01a10e86-3a0b-7409-9bcc-3763df15029d`.
- Local conversation time: October 5, 2026 at 20:04 (UTC October 6 at 00:04), consistent with the October 4 weekly bucket.
- Original Auto Score: N/A, matching database sentinel -1.
- Overwritten Score: Yes, using the corresponding template revision's label for numeric value 0.
- posit comment column exists but is empty; nonempty comments were not verified.

Verified all table-level files by parsing CSV contents:

1. `performance_progression.zip`: contains `performance_progression.csv` and `performance_progression_by_agent.csv`; each has two data rows, Mathias and posit, October 4–10, Num Opportunities 1 / Num Done 1 / % Done 1.
2. `Agent_leaderboard_by_criteria.csv`: one agent row; Total # of overwritten 1, Mathias 1, posit 1, introductory criterion empty.
3. `Agent Leaderboard by Criterion.csv` with posit selected: one agent row; Total 1, October 4 column 1, other dates empty.

The summary files and individual conversation export are distinct existing download paths. All tested outputs match their displayed populations. Agent-mode tests were performed; Team mode and large/paginated populations were not exercised.

## Source findings

Director checkout is main `34ad89e76d0050eaaf9ec6e8cf1b5148933ca84e`. Fetched `origin/main` at `344235d593`; the detailed export hook is identical, and progression differences are styling only. Served assets identify `main-8d3de48`; deployed code should not be equated with either local SHA.

- `packages/director-app/src/components/insights/qa-insights/hooks/useFiltersForValueType.ts`: overwritten mode selects SCORE_TYPE_OVERRIDDEN and includeNaScored=true.
- `packages/director-app/src/features/insights/performance/performance-progression/useColumnsForPerformanceProgression.tsx`: cells pass template, selected value type, and additional filters into QAICell.
- `packages/director-app/src/components/insights/qa-insights/cell/QAICell.tsx:419`: detailed export receives requestParams.filterByAttribute, selected date interval, qaScores, valueType, and includeComments.
- `packages/director-app/src/hooks/insights/useExportPerformanceInsightCriteria.tsx`: retrieves conversations with the same filter, fetches metadata and revision templates, produces original/overwritten/comment fields, and saves the CSV.
- **Confirmed defect, lines 202–204:** onError has no error argument; it clears notifications and calls `getErrorHandler(...)` without invoking its return value. `packages/director-api/src/services/baseApiErrorHandler.ts:71` confirms this is a function factory. Recommended fix: accept the error and call the returned handler with it. This is independently provable in source but not established as the original report's cause.
- **Separate conditional issue, not reproduced:** `PerformanceProgression.tsx:394` starts summary and by-agent requests independently and generates the ZIP in the by-agent continuation. If the summary finishes later, its file may be absent. Both files were present in the tested ZIP. Do not conflate this with a proven overwritten-specific regression.
- Criteria and time-based leaderboard QAICell callers omit scorecardTemplate and valueType; their existing popovers do not expose the detailed Download scorecard button. Their table-level aggregate downloads work. No evidence this capability changed recently.

Related Linear issues read: CONVI-5972 (released comment export support, Director #16136) and CONVI-5805 (historical count-versus-popover filtering fix, Director #15637). These do not prove the new report's cause.

## Handoff

No product source, customer data, Linear comments, or ticket status were changed. Knowledge records and local downloaded evidence were saved. Browser remains on the populated 06.10.25 / Last 30 days / Weekly example, all three tables in overwritten mode and the final leaderboard selecting posit. Original ticket needs customer/template/date/file specifics for further attribution.
