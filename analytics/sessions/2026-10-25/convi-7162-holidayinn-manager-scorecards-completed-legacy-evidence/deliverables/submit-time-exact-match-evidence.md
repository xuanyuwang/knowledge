# CONVI-7162 Evidence: Submit-Time Restores Exact Scorecard Counts

**Date verified:** 2026-07-02
**Ticket:** [CONVI-7162](https://linear.app/cresta/issue/CONVI-7162/holiday-inn-club-vacations-manager-leaderboard-scorecards-completed)
**Status of product fix:** not shipped yet
**Purpose:** Concrete production evidence that filtering/grouping Manager Leaderboard `Scorecards Completed` by `scorecard_submit_time` restores exact daily submission counts for the primary repro.

Raw investigation notes: `sessions/2026-07-02/codex-investigation.md`

---

## Hypothesis

Manager Leaderboard currently counts submitted scorecards using QA APIs timed by `scorecard_time` (conversation start for conversation scorecards).

Customer expectation is:

> how many scorecards this manager completed/submitted each day

**Hypothesis:** If the same scorecard set is filtered and grouped by `scorecard_submit_time` instead of `scorecard_time`, daily counts will exactly match actual submissions (PG `submitted_at`).

---

## Scope of verification

| Field | Value |
|---|---|
| Customer / profile | `holidayinn` / `transfers-voice` |
| CH database | `holidayinn_transfers_voice` |
| Manager / submitter | Cliff Hawker / `9d654376ad4f1cdc` |
| Template | Ride Along Template (`f00391f9-c9f8-4bd4-885a-3bcad260817c`, revision `9be08011`) |
| Window | `2026-06-15 00:00` through `2026-06-29 00:00` America/New_York |
| Method | Read-only PG + ClickHouse queries |

This verifies the **time-basis hypothesis with production data**. It is not a post-deploy UI/API confirmation of a shipped fix.

---

## Result summary

| Check | Result |
|---|---|
| PG submissions | Exactly **2 per ET weekday** for 10 weekdays (20 total) |
| CH row presence | All **20** exact PG scorecards exist in `scorecard_d FINAL` |
| CH by `scorecard_submit_time` | Exact match to PG: **2 per weekday** |
| CH by `scorecard_time` | Redistributes counts; reproduces ticket Leaderboard pattern |
| Missing-row / backfill needed for this sample? | **No** |

**Conclusion:** For this repro, switching Manager `Scorecards Completed` from `scorecard_time` to `scorecard_submit_time` would restore exact daily counts.

---

## 1. Source of truth: PG submitted counts

PG `director.scorecards` filtered by:

- `submitter_user_id = '9d654376ad4f1cdc'`
- Ride Along Template
- `submitted_at` in the ET window above

| Submitted Day ET | PG Submitted Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 2 |
| 2026-06-17 | 2 |
| 2026-06-18 | 2 |
| 2026-06-19 | 2 |
| 2026-06-22 | 2 |
| 2026-06-23 | 2 |
| 2026-06-24 | 2 |
| 2026-06-25 | 2 |
| 2026-06-26 | 2 |
| **Total** | **20** |

Expected Leaderboard prior week (Mon–Fri 6/15–6/19): **2, 2, 2, 2, 2** (10 total).
Expected Mon–Tue 6/22–6/23: **2, 2**.

---

## 2. ClickHouse sync check

Queried `holidayinn_transfers_voice.scorecard_d FINAL` for the exact 20 PG `resource_id` values:

| Metric | Value |
|---|---:|
| Distinct scorecards present | 20 |
| Physical rows | 20 |

So the Cliff Hawker mismatch is **not** explained by missing CH rows.

Per-scorecard timestamps also show CH `scorecard_submit_time` aligns with PG `submitted_at`, while `scorecard_time` often equals an earlier conversation day. Examples:

| Submitted ET | Scorecard Time ET | Scorecard ID |
|---|---|---|
| 2026-06-15 18:42 | 2026-06-14 15:49 | `019ecd72-28c5-7253-a649-6c7456dc8c32` |
| 2026-06-15 18:45 | 2026-06-14 15:17 | `019ecd75-4f91-77fd-9bb0-472e36a90a1a` |
| 2026-06-19 16:55 | 2026-06-17 18:56 | `019ee1a8-5dd3-751b-8d4b-af8aba77b436` |
| 2026-06-19 16:59 | 2026-06-18 17:53 | `019ee1ac-f352-753b-a288-975033d0dbc3` |
| 2026-06-22 18:20 | 2026-06-20 16:44 | `019ef16a-c0e2-75e2-8bc6-5b22e0b32508` |
| 2026-06-22 18:22 | 2026-06-20 16:27 | `019ef16c-6f33-722a-b12c-58100edd3cd6` |

Full 20-row mapping is in `sessions/2026-07-02/codex-investigation.md`.

---

## 3. Hypothesis test: group the same CH rows two ways

### A. Group by `scorecard_submit_time` (proposed fix basis)

| Submit Day ET | Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 2 |
| 2026-06-17 | 2 |
| 2026-06-18 | 2 |
| 2026-06-19 | 2 |
| 2026-06-22 | 2 |
| 2026-06-23 | 2 |
| 2026-06-24 | 2 |
| 2026-06-25 | 2 |
| 2026-06-26 | 2 |

**Exact match to PG.**

### B. Group by `scorecard_time` (current QA Leaderboard basis)

| Scorecard Time Day ET | Count |
|---|---:|
| 2026-06-14 | 2 |
| 2026-06-15 | 2 |
| 2026-06-16 | 3 |
| 2026-06-17 | 1 |
| 2026-06-18 | 2 |
| 2026-06-20 | 2 |
| 2026-06-22 | 1 |
| 2026-06-23 | 2 |
| 2026-06-24 | 2 |
| 2026-06-25 | 3 |

Counts are redistributed across conversation days.

---

## 4. Emulation of the broken Leaderboard week

Query semantics matching current Manager QA path for prior week 2026-06-15 through 2026-06-21 ET:

- time filter/group on `scorecard_time`
- Cliff as submitter
- Ride Along Template
- submitted status

| Leaderboard Day ET | Count | Ticket reported |
|---|---:|---|
| Mon 2026-06-15 | 2 | 2 |
| Tue 2026-06-16 | 3 | 3 |
| Wed 2026-06-17 | 1 | 1 |
| Thu 2026-06-18 | 2 | 2 |
| Fri 2026-06-19 | 0 | 0 |

Exact match to the customer-reported broken pattern. Friday’s two submissions are attributed to earlier conversation days by `scorecard_time`.

Same 10 scorecards by submit day:

| Submit Day ET | Count |
|---|---:|
| 2026-06-15 | 2 |
| 2026-06-16 | 2 |
| 2026-06-17 | 2 |
| 2026-06-18 | 2 |
| 2026-06-19 | 2 |

### Current-week Mon/Tue sample

Four scorecards submitted on 2026-06-22 and 2026-06-23 ET:

| Basis | 6/20 | 6/22 | 6/23 |
|---|---:|---:|---:|
| by `scorecard_submit_time` | 0 | 2 | 2 |
| by `scorecard_time` | 2 | 1 | 1 |

This explains the ticket’s “only 1 each day” on Monday/Tuesday despite 2 submissions each day: two Monday submissions were attached to Saturday conversations.

---

## 5. Side-by-side proof for the ticket windows

### Prior week Mon–Fri (6/15–6/19)

| Day | Expected (submit) | Current Leaderboard (`scorecard_time`) | Submit-time result |
|---|---:|---:|---:|
| Mon | 2 | 2 | 2 |
| Tue | 2 | 3 | 2 |
| Wed | 2 | 1 | 2 |
| Thu | 2 | 2 | 2 |
| Fri | 2 | 0 | 2 |
| **Total** | **10** | **8** | **10** |

### Current week Mon–Tue (6/22–6/23)

| Day | Expected (submit) | Current Leaderboard (`scorecard_time`) | Submit-time result |
|---|---:|---:|---:|
| Mon | 2 | 1 | 2 |
| Tue | 2 | 1 | 2 |

---

## 6. What this does and does not prove

### Proved

- For Cliff Hawker / `holidayinn-transfers-voice` / Ride Along, using `scorecard_submit_time` yields exact daily submission counts.
- Using `scorecard_time` fully explains the ticket’s redistributed/missing daily counts.
- This sample is not a CH missing-row problem.

### Not proved here

- End-to-end that a deployed API/UI change already fixed production (no fix shipped yet).
- That every other affected Holiday Inn manager/profile has the same exclusive root cause; other profiles may still need separate sync checks.
- That every QA scorecard consumer should switch to submit time; only Manager completed-count semantics were validated against customer expectation.

---

## 7. Implication for the fix

Recommended product behavior for Manager `Scorecards Completed`:

1. Keep QA APIs (`RetrieveQAScoreStats` aggregate + `RetrieveQAConversations` drawer).
2. Add an explicit request time basis for scorecard submit time.
3. Have Manager Leaderboard set that basis for both aggregate and drawer requests.
4. Leave ordinary QA conversation-start/end semantics as default for other callers.

Until that ships, production Manager counts for this metric remain on `scorecard_time` and can redistribute submissions across days.

---

## Related artifacts

- Ticket: CONVI-7162
- Investigation session: `sessions/2026-07-02/codex-investigation.md`
- Project README: `README.md`
- Prior design context: `/Users/xuanyu.wang/repos/knowledge/convi-6968-schwab-leaderboard-launch`
