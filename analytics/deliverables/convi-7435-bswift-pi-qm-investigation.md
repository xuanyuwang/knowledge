# CONVI-7435: Bswift Performance Insights vs QM / scorecard N/A investigation

**Ticket:** [CONVI-7435](https://linear.app/cresta/issue/CONVI-7435/bswift-performance-insights-and-qm-report-scorecard-discrepancies)
**Zendesk:** [#22972](https://crestasupport.zendesk.com/tickets/22972)
**Slack:** [thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785959396310119)
**Customer:** Bswift (`bswift` / `us-east-1`), usecase `voice-care`
**Agent:** Breanna Marshall (`11c92350e55ab8af`)
**Conversation:** `#712933057605` / `019fb6c4-e4df-72c7-a23c-6ea3a86ad964`
**Template (filter):** Manual Heartbeat Quality Scorecard (`019ce797-39e5-74d9-8037-8345c6032a47`)
**Validated:** 2026-08-05
**Status:** Issue #2 root cause confirmed; Issue #1 open (product/UX)

## Executive Summary

Bswift reported two discrepancies that reduced trust in Cresta as the source of truth for agent performance:

1. **Weekly date-range mismatch** between Performance Insights (PI) and QM Report (12 vs 13 scorecards; ~97% vs ~92%).
2. **PI showed 40%** for conversation `#712933057605` while the closed-conversation Manual Heartbeat scorecard showed **Performance Score N/A** / 0 criteria scored.

**Issue #2 is explained by a Manual Heartbeat scorecard reset with PostgreSQL → ClickHouse lag.**

A Manual Heartbeat scorecard for this conversation **did exist** and was later **deleted from ClickHouse** via the standard async reset/delete path. Closed Conversations reads Postgres (scores cleared immediately → N/A). Performance Insights reads ClickHouse (stale rows remain until mutations complete → numeric %). After catch-up, the conversation **no longer appears** under the Manual Heartbeat PI filter — matching current behavior.

**Issue #1** is a separate semantics/UX question: PI weekly grouping expands to full calendar weeks (e.g. Jun 28–30 and through Aug 1 for a Jul 1–31 selection), while QM honors the selected range and submitted Manual evaluations.

---

## Customer-Visible Symptoms

| Surface | Observation |
|---|---|
| PI (Manual Heartbeat, Weekly, Jul 1–31) | 13 scorecards, ~92% |
| QM Report (same agent/template/range, Weekly) | 12 evaluated, 96.8% |
| PI switched to Daily | Aligns with QM (~12, ~97%) |
| PI conversation drawer (week 07/26–08/01) | `#712933057605` at **40%**; header **72.0% on 3 scorecards** |
| Closed Conversations → Manual Heartbeat | **Performance Score: N/A**, sections N/A, **0 scored** |
| PI Manual filter (recheck after investigation) | Conversation **no longer listed** |

---

## Entities

| Entity | ID |
|---|---|
| Conversation UUID | `019fb6c4-e4df-72c7-a23c-6ea3a86ad964` |
| Platform conversation # | `712933057605` |
| Conversation start (`scorecard_time`) | `2026-07-30 21:25:48` UTC (4:25 PM EDT) |
| Agent user ID | `11c92350e55ab8af` |
| Manual Heartbeat template | `019ce797-39e5-74d9-8037-8345c6032a47` |
| Automated Heartbeat template | `0199abb9-fd82-7696-9302-54a27b5e33b6` |
| Non-Performance Behaviors template | `019f4cbf-e16c-720a-9646-6921927a9a55` |
| ClickHouse DB | `bswift_us_east_1` |

---

## Issue #2 — Root Cause (confirmed)

### What happened

1. A **Manual Heartbeat** scorecard was created/scored for this conversation (scorecard time = conversation start).
2. The scorecard was **reset** (and/or replaced then reset again). Reset clears Postgres scores immediately; ClickHouse deletion is **asynchronous**.
3. While CH still had rows, PI drawer showed a numeric **40%** for this conversation under Manual filter.
4. Closed Conversations opened Manual Heartbeat against empty/cleared PG state → **N/A / 0 scored**.
5. After CH mutations completed, Manual rows disappeared → conversation vanished from Manual PI filter.

### Data path divergence

| Surface | Source of truth | After reset |
|---|---|---|
| Closed Conversations Performance Score | Postgres `director.scorecards` / `director.scores` | Cleared immediately → N/A |
| Performance Insights conversation % | ClickHouse `score` / `scorecard` (+ `*_d` views) via `RetrieveQAConversations` | Stale until `DeleteScoresOnShard` / `DeleteScorecardOnShard` mutations finish |

Code path: `ResetScorecard` → PG delete/nullify → async `DeleteScoresOnShard` / `DeleteScorecardOnShard` issuing:

```sql
DELETE FROM score
WHERE toStartOfHour(scorecard_time) = toStartOfHour(toDateTime64(?, 0))
  AND conversation_id = ?
  AND scorecard_id = ?

DELETE FROM scorecard
WHERE toStartOfHour(scorecard_time) = toStartOfHour(toDateTime64(?, 0))
  AND scorecard_template_id = ?
  AND scorecard_id = ?
```

---

## Evidence

### A. Screenshot arithmetic (PI drawer)

Drawer for week `07/26/2026 – 08/01/2026`, Manual Heartbeat, Breanna Marshall:

| Conversation | Score shown |
|---|---|
| `#712933057605` | **40%** |
| `#712529323779` | **100%** |
| `#712528445490` | **76%** |
| Header | **72.0% on 3 scorecards** |

Check: `(40 + 100 + 76) / 3 = 72` exactly.

Those 100% and 76% rows are real, submitted Manual scorecards still present in CH:

| Conversation UUID | Platform # | Manual score | Submitted |
|---|---|---|---|
| `019fb6c4-e4cc-76cd-908f-9d76b4f7c925` | `712529323779` | 100 | yes |
| `019fb19e-ae84-78a0-81bd-a36220d99ef3` | `712528445490` | 76 | yes |

After Manual deletes for the ticket conversation, Manual-only CH for that agent/week is **only those two** → avg **88%**, not 72%. The third 40% contribution is gone with the deleted Manual card.

### B. Live PG / CH state (2026-08-05 investigation)

**No Manual Heartbeat scorecard** remains for `019fb6c4-e4df-72c7-a23c-6ea3a86ad964`.

Remaining scorecards:

| Template | Scorecard ID | Score | Submitted |
|---|---|---|---|
| Automated Heartbeat | `019fb70f-6d8f-7612-a072-67bbd7dc0b2e` | 100 | no (AI) |
| Non-Performance Behaviors | `019fb70f-6d73-700c-bf83-acb06b6aa538` | 33.3 | no (AI) |

- Zero Manual rows in `score_d` / `scorecard_d` for this conversation.
- Zero Manual criterion IDs (`019ce797%`) on this conversation’s score rows.
- Opening Manual Heartbeat in Closed Conversations → N/A is **correct** for current PG state.

### C. ClickHouse `system.mutations` (definitive)

Successful Lightweight Deletes for **Manual Heartbeat** on this conversation’s `scorecard_time`:

| Mutation time (UTC) | Table | Scorecard ID | Predicate (summary) |
|---|---|---|---|
| **2026-07-31 13:17:24** | `score` | `019fb852-53fe-76d8-a61f-5a1e402be571` | `scorecard_time` hour `2026-07-30 21:25:48` **and** `conversation_id = 019fb6c4-e4df-…` **and** this scorecard_id |
| **2026-07-31 13:17:27** | `scorecard` | `019fb852-53fe-76d8-a61f-5a1e402be571` | same hour **and** `scorecard_template_id = 019ce797-…` (Manual) **and** this scorecard_id |
| **2026-08-03 18:29:16** | `score` | `019fc8e3-197d-703e-80b3-a3a9caae2336` | same conversation + hour + scorecard_id |
| **2026-08-03 18:29:23** | `scorecard` | `019fc8e3-197d-703e-80b3-a3a9caae2336` | same hour + Manual template + scorecard_id |

Interpretation:

- Manual Heartbeat scorecards **existed** for this conversation (at least two generations: `019fb852-…` then `019fc8e3-…`).
- Both were removed via the standard per-scorecard CH delete API used by reset/reindex.
- Two deletes imply reset → recreate → reset again (or equivalent rewrite), not a one-off indexing glitch.

### D. Current PI behavior (post catch-up)

With Manual filter, the conversation **does not appear** on Performance Insights — consistent with mutations `is_done = 1` and no remaining Manual CH rows.

---

## Timeline (Issue #2)

```text
2026-07-30 21:25:48 UTC  Conversation starts (#712933057605)
2026-07-31 13:17 UTC     CH delete Manual scorecard 019fb852-…
2026-08-03 18:29 UTC     CH delete Manual scorecard 019fc8e3-… (second generation)
~ticket screenshots      PI still showed 40% / 72% on 3 (stale CH and/or pre-final catch-up)
2026-08-05               Investigation: no Manual rows live; PI Manual filter no longer lists conv
```

Exact wall-clock of the user-facing reset click vs mutation create_time was not recovered from apiserver audit in this pass; mutation predicates are sufficient to prove Manual rows existed and were deleted.

---

## Issue #1 — Weekly date-range discrepancy (open)

### Observation

| Report | Jul 1–31 + Weekly | Result |
|---|---|---|
| Performance Insights | Expands to calendar weeks | Includes Jun 28–30 (and last week through Aug 1 in drawer); 13 scorecards, ~92% |
| QM Report | Honors selected range / submitted evals | 12 conversations, 96.8% |
| PI Daily | Same selected range | Aligns with QM (~12, ~97%) |

### Assessment

This is **expected given current PI weekly bucketing**, not the same defect as Issue #2. Support guidance until product aligns:

- For parity with QM on a calendar month, use **Daily** (or otherwise avoid Weekly expansion), **or**
- Treat QM as source of truth for submitted Manual evaluation counts in a strict date window.

Full product decision (align PI to selected range vs disclose expanded week bounds in UI) remains open.

---

## Ruled out

| Hypothesis | Why ruled out |
|---|---|
| PI Manual filter permanently leaking Non-Performance 40% | Mutations prove Manual rows existed/deleted; live Manual filter no longer lists the conversation |
| Closed Conversations bug showing N/A incorrectly | PG has no Manual scores; N/A is correct after reset |
| Permanent CH/PG desync requiring backfill | Mutations completed; current CH matches “no Manual scorecard” |

---

## Recommendations

1. **Support / customer:** Issue #2 was a **reset + analytics lag** event, not two permanently different scoring systems. Current Manual PI state is consistent with Closed Conversations (conversation absent / no Manual score).
2. **Optional hardening:** Reduce PI staleness after `ResetScorecard` (faster CH delete, or briefly suppress conversations whose PG Manual scorecard is empty while CH still has rows). Out of scope unless product prioritizes.
3. **Issue #1:** Confirm with product whether weekly expansion is intentional; if yes, surface effective date range in PI UI; if no, clamp weekly groups to the selected range.

---

## References

- Work item: `analytics/work-items/CONVI-7435.md`
- Session notes: `analytics/sessions/2026-08-05/claude-convi-7435-bswift-pi-qm.md`
- Delete implementation: `go-servers/shared/clickhouse/conversations/scorecard_score.go` (`DeleteScoresOnShard`, `DeleteScorecardOnShard`)
- Reset RPC: `go-servers/apiserver/internal/coaching/action_reset_scorecard.go`
- Time-range semantics context: `analytics/work-items/time-range-filter-investigation.md`
