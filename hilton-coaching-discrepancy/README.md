# Hilton Coaching Report Discrepancies

**Created:** 2026-01-07
**Updated:** 2026-02-12
**Customer:** Hilton (hilton-voice.cresta.com), Alaska Air (also affected by CONVI-6242)
**Usecase:** HGV Package Sales (`voice-hgv-marketing`)

## Overview

Investigation into coaching session count discrepancies between Manager Leaderboard and Coaching Report, plus related UI bugs. Also includes the `cron-label-conversations` stale labels fix (CONVI-6242) affecting Agent Leaderboard "Active Days". Spans four Linear tickets across backend, frontend, and ClickHouse schema.

## Tickets

| Ticket | Title | Status | Summary |
|--------|-------|--------|---------|
| [CONVI-5921](https://linear.app/cresta/issue/CONVI-5921) | HGV Coaching Report vs Manager Leaderboard discrepancies | Released | Soft-deleted session, side panel bug, UI efficiency filter |
| [CONVI-5968](https://linear.app/cresta/issue/CONVI-5968) | Coaching Report not showing sessions for agent Victor Adeyemo | Released | Sessions without focus criteria not shown |
| [CONVI-6097](https://linear.app/cresta/issue/CONVI-6097) | HGV Coaching Report vs Manager Leaderboard discrepancies (deactivated users) | **In Progress** | `RetrieveCoachingSessionStats` doesn't support `excludeDeactivatedUsers` |
| [CONVI-6242](https://linear.app/cresta/issue/CONVI-6242) | Agent Leaderboard "Active Days" shows 0 (stale labels) | Merged | `cron-label-conversations` labels open conversations; stale `agent_user_id` after re-assignment |

## Bugs Found

### Bug 1: Soft-deleted session counted differently across APIs

**Status:** Not a bug - expected behavior
**Ticket:** CONVI-5921

One session (`019b955d-22da-7471-8e62-f2e8783a0f89`) was soft-deleted 8 seconds after submission.
- `RetrieveCoachingEfficiencyStats` filters `deleted_at IS NULL` - returns 5 sessions
- `RetrieveCoachingSessionStats` has no such filter - returns 6 sessions

**Fix:** go-servers [PR #25032](https://github.com/cresta/go-servers/pull/25032) - exclude soft-deleted sessions from `RetrieveCoachingSessionStats`

---

### Bug 2: UI shows 0 sessions despite API returning 5

**Status:** Fixed
**Ticket:** CONVI-5921

**Root cause:** `useCoachingReportLeaderboardPerMetricData.tsx:115` filters out ALL sessions with `isEfficiencyScoreInvalid: true`, regardless of the selected metric. When metric is "# of sessions", sessions should be counted regardless of efficiency validity.

```typescript
// BEFORE (buggy) - filters ALL metrics
stats.forEach((stat) => {
  if (stat.isEfficiencyScoreInvalid) { return; }
});

// AFTER (fixed) - only filter for efficiency metrics
stats.forEach((stat) => {
  if (stat.isEfficiencyScoreInvalid &&
      selectedMetric !== CoachingReportLeaderboardHeatMapValueTypes.NUM_SESSIONS) {
    return;
  }
});
```

**Fix:** director [PR #15934](https://github.com/cresta/director/pull/15934)

---

### Bug 3: "Total" column shows "N/A" instead of session count sum

**Status:** Fixed
**Ticket:** CONVI-5921

**Root cause:** Two layers:
1. **Inconsistent data sources** - `CoachingReport.tsx:270-278` uses hardcoded `CRITERION_CATEGORY_PERFORMANCE` filter for the total column, while daily columns use a dynamic criterion category
2. **Same invalid efficiency filter** as Bug 2

**Fix:** Same PR as Bug 2

---

### Bug 4: Side panel hides efficiency for sessions with empty `focus_criteria_ids`

**Status:** Fixed
**Ticket:** CONVI-5921

**Root cause:** `utils.ts:165-170` checks `focusCriteria.length > 0` before displaying efficiency. The backend uses COALESCE fallback to coaching plan targets for sessions with empty `focus_criteria_ids` and returns valid efficiency - frontend should trust the backend value.

```typescript
// BEFORE (buggy) - discards valid efficiency when focus criteria empty
coachingEfficiency: focusCriteria.length > 0
  ? stats.find(...)?.coachingEfficiency
  : undefined,

// AFTER (fixed) - always use backend value
coachingEfficiency: stats.find(...)?.coachingEfficiency,
```

**Fix:** director [PR #15934](https://github.com/cresta/director/pull/15934)

---

### Bug 5: Sessions without focus criteria not shown in Coaching Report

**Status:** Fixed
**Ticket:** CONVI-5968

**Root cause:** Backend didn't include sessions with empty `focus_criteria_ids` when `include_sessions_without_focus_criteria` wasn't set. Added proto field + backend/frontend support.

**Fixes:**
- cresta-proto [PR #7681](https://github.com/cresta/cresta-proto/pull/7681)
- go-servers [PR #25066](https://github.com/cresta/go-servers/pull/25066)
- director [PR #15953](https://github.com/cresta/director/pull/15953)

---

### Bug 6: `RetrieveCoachingSessionStats` doesn't filter deactivated users (CURRENT WORK)

**Status:** In Progress
**Ticket:** CONVI-6097

**Root cause:** When `excludeDeactivatedUsers: true` is set in the request:
- `RetrieveCoachingEfficiencyStats` correctly filters sessions for deactivated agents
- `RetrieveCoachingSessionStats` ignores this flag entirely

**Example:** Agent `c48f434fc3325c3d` (Juana Isabel Munoz) has `active_state = 2` (INACTIVE). Her coaching session `019aef9c-765f-76be-a308-c55549031474` appears in the Manager Leaderboard (3 sessions) but not in the Coaching Report (2 sessions).

| API | Respects `excludeDeactivatedUsers` | Sessions for Carl (Dec 5) |
|-----|-----------------------------------|---------------------------|
| `RetrieveCoachingEfficiencyStats` (Coaching Report) | Yes | 2 |
| `RetrieveCoachingSessionStats` (Manager Leaderboard) | No | 3 |

**Fix needed:** Add `excludeDeactivatedUsers` filter support to `RetrieveCoachingSessionStats` in `insights-server/internal/analyticsimpl/retrieve_coaching_session_stats.go`.

---

### Bug 7: `cron-label-conversations` writes stale labels for re-assigned conversations

**Status:** Merged
**Ticket:** CONVI-6242

**Root cause:** The cron filters by `chats.created_at`, labeling conversations while still open. If re-assigned afterward, the ClickHouse row has the wrong `agent_user_id`/`usecase_id`.

**Impact:** Agent Leaderboard "Active Days" shows 0 instead of 1 (e.g., Corinne Harmon / Alaska Air).

**Fix:** Filter by `ended_at` — only process closed conversations where agent/usecase are stable. Since a conversation can only end once, each conversation is written exactly once with final values. No ClickHouse schema change needed.

**PR:** go-servers [PR #25706](https://github.com/cresta/go-servers/pull/25706) — `ended_at` filter, watermark update, event window fix

**Details:** See [convi-6242-stale-labels-fix.md](convi-6242-stale-labels-fix.md)

## Key Concepts

### `isEfficiencyScoreInvalid` flag
Set to `true` when efficiency can't be calculated (session too recent, insufficient conversations). Does NOT mean the session is invalid - `totalNumberOfSessions` is always populated regardless of this flag.

### `focus_criteria_ids` COALESCE fallback
Sessions with empty `focus_criteria_ids` fall back to the coaching plan's targets via:
```sql
COALESCE(
    NULLIF(split_part(focus_criteria_template_criterion_id, '/', 2), ''),
    t.criterion_or_chapter_id
)
```
The frontend should trust backend efficiency values without re-validating criteria existence.

### User active states
From `cresta-proto`: `ACTIVE_STATE_UNSPECIFIED = 0`, `ACTIVE = 1`, `INACTIVE = 2`, `MIGRATED = 3`

## Reference Documents

| Document | Description |
|----------|-------------|
| [reference/coaching-efficiency-api.md](reference/coaching-efficiency-api.md) | Complete `RetrieveCoachingEfficiencyStats` API behavior, data flow, edge cases |
| [reference/database-model.md](reference/database-model.md) | Database schema, table relationships, join strategy |
| [convi-6242-stale-labels-fix.md](convi-6242-stale-labels-fix.md) | CONVI-6242 full fix plan: code changes, migration SQL, rollout steps |

## Log History

| Date | Summary |
|------|---------|
| 2026-01-07 | Initial investigation of session count discrepancy (6 vs 5) |
| 2026-01-08 | Found UI bugs (0 sessions displayed, N/A total, side panel) |
| 2026-01-09 | Documented DB model, completed frontend fixes (CONVI-5921) |
| 2026-01-22 | Bug 4 root cause corrected: deactivated user filtering, not UNNEST issue |
| 2026-01-25 | CONVI-6097 confirmed: `RetrieveCoachingSessionStats` needs `excludeDeactivatedUsers` |
| 2026-02-12 | Reorganized documents; CONVI-6242 fix: go-servers [#25706](https://github.com/cresta/go-servers/pull/25706) (no schema migration needed) |
