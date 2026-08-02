# Coaching Hub & QM Report: Scorecard Time Basis

Date: 2026-07-30
Ticket: [CONVI-7162](https://linear.app/cresta/issue/CONVI-7162/holiday-inn-club-vacations-manager-leaderboard-scorecards-completed)
Repos: `/Users/xuanyu.wang/repos/director-convi-7162`, `/Users/xuanyu.wang/repos/go-servers-convi-7162`

## Question

Does Coaching Hub (and QM Report), which the ticket uses as the ground-truth comparison for Manager Leaderboard “Scorecards Completed,” call APIs that count by **scorecard submit time**?

## Verdict

**Yes.** Both Coaching Hub’s Scorecards column and QM Report’s evaluated-scorecard metrics are submit-time by backend implementation. They do **not** use `RetrieveQAScoreStats` / `ScorecardTimeBasis`. Manager Leaderboard is the outlier API path (QA ClickHouse `scorecard_time` by default before CONVI-7162).

| Surface | Primary RPC(s) for completion-style counts | Time basis |
|---------|--------------------------------------------|------------|
| Coaching Hub “Scorecards” | `CoachingService.RetrieveCoachingOverviews` | `submitted_at` |
| QM Report Evaluations | `AnalyticsService.RetrieveDirectorTaskStats` | `scorecards.submitted_at` |
| QM Report task-cycle drill-down | `AnalyticsService.RetrieveManualQAStats` | `scorecards.submitted_at` |
| Manager Leaderboard “Scorecards Completed” | `AnalyticsService.RetrieveQAScoreStats` | default `scorecard_time` (interaction); CONVI-7162 opts into `scorecard_submit_time` |

---

## Coaching Hub

### FE call path

- Page: `packages/director-app/src/features/coaching-workflow/coaching-hub/CoachingHub.tsx`
- Overview request: `.../coaching-hub/hooks/useRequestForRetrieveRecentActivitiesData.ts`
- Hook: `packages/director-app/src/hooks/coaching/useRecentCoachingActivities.tsx` → `CrestaAPI.coaching.retrieveCoachingOverviews`
- Column: `countScorecardsScored` / `lastScorecardSubmitTime` via `useRecentActivitiesTableColumns` → `GradedScorecardsCell`

FE does **not** send `startTime` / `endTime` for the overview scorecard count; UI tooltip says last ~30 days.

### Backend

`apiserver/internal/coaching/action_retrieve_coaching_overviews.go`:

- Defaults window to ~1 calendar month when times omitted.
- `buildScorecardStatsSubquery` filters:

```sql
customer = ? AND profile = ? AND submitted_at >= ? AND submitted_at <= ?
```

and aggregates `COUNT(*)` / `MAX(submitted_at)`.

### Not used for Hub scorecard counts

- `RetrieveScorecardStats`
- `RetrieveQAScoreStats` (used only for focus-criteria / quintile scores on the same page, not the Scorecards column)
- `scorecardTimeBasis` / `conversationTimeRangeField`

**Confidence:** high.

---

## QM Report

### FE call path

- Page: `packages/director-app/src/features/qa/report/QAReport.tsx`
- Evaluations overview / task cycles: `useQMTaskStats` → `CrestaAPI.insights.retrieveDirectorTaskStats`
- Task-cycle drill-down: `useQAReportData` / `useRetrieveManualQAStats` → `retrieveManualQAStats`

Neither request type exposes `scorecardTimeBasis`. Time range comes from the date picker (`filterByTimeRange`) or evaluation periods (`filterByEvaluationPeriods`).

### Backend

- `insights-server/.../retrieve_qm_task_stats.go`: `WHERE submitted_at >= ? AND submitted_at <= ?`; groups with `DATE_TRUNC(..., scorecards.submitted_at ...)`.
- `insights-server/.../retrieve_manual_qa_stats.go`: `WHERE scorecards.submitted_at >= ...`; groups on `formatted_submit_time` derived from `submitted_at`.

### Not used on QM Report for these metrics

- `RetrieveQAScoreStats`
- `RetrieveScorecardStats`

**Confidence:** high for evaluated / completion-style scorecard counts. Assigned/quota conversation counts may involve other assignment logic (not fully required for this comparison).

---

## Why the ticket comparison fails

Ticket step 4: compare Manager Leaderboard against Coaching Hub or QM Report.

Those surfaces already count by **Postgres `submitted_at`**. Manager Leaderboard (post CONVI-6968) counts via **ClickHouse QA stats** keyed by default to **`scorecard_time`** (conversation start / process interaction time). Same submissions, different day buckets → redistributed / missing daily counts.

CONVI-7162’s Manager-only `ScorecardTimeBasis=SUBMIT_TIME` aligns the Leaderboard QA path with Hub/QM submit-day semantics without changing Hub/QM APIs.

---

## Gaps / caveats

1. Coaching Hub FE “30 days” tooltip vs backend `AddDate(0, -1, 0)` (calendar month).
2. Hub `RetrieveQAScoreStats` for focus criteria still uses default interaction-time basis; that is unrelated to the Scorecards column.
3. QM assigned/quota metrics may not be purely `submitted_at`; evaluated scorecard counts are.
