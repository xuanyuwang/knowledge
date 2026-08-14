# CONVI-7435 Bswift PI vs QM / 40% vs N/A

**Date:** 2026-08-05
**Tool:** claude (Cursor)
**Ticket:** [CONVI-7435](https://linear.app/cresta/issue/CONVI-7435)
**Slack:** https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785959396310119
**Primary domain:** analytics / performance-insights
**Source repos:** go-servers, director
**Branch/worktree:** main checkouts (read-only investigation)

## Inputs reviewed

- Slack thread (Tari Mills; Tinglin asked Xuanyu for issue #2)
- Linear CONVI-7435 + embedded screenshots
- ClickHouse `bswift_us_east_1` `scorecard_d` / `score_d` / `conversation`
- Postgres `director.scorecards` / `director.scorecard_templates`
- Code: `RetrieveQAConversations`, `transformQAConversations`, `QAConversationExamplesDrawer`, prior art CONVI-6753

## Key evidence

### Conversation `#712933057605`

| Field | Value |
|---|---|
| UUID | `019fb6c4-e4df-72c7-a23c-6ea3a86ad964` |
| Agent | Breanna Marshall `11c92350e55ab8af` |
| Start | 2026-07-30 21:25:48 UTC (4:25 PM EDT) |
| Usecase | voice-care |

**Scorecards present (PG + CH):**

| Template | Template ID | Scorecard ID | Score | Submitted |
|---|---|---|---|---|
| Automated Heartbeat Quality Scorecard | `0199abb9-fd82-7696-9302-54a27b5e33b6` | `019fb70f-6d8f-7612-a072-67bbd7dc0b2e` | 100 | no (AI) |
| Non-Performance Behaviors Heartbeat Quality Scorecard | `019f4cbf-e16c-720a-9646-6921927a9a55` | `019fb70f-6d73-700c-bf83-acb06b6aa538` | **33.3** (now) | no (AI) |
| Manual Heartbeat Quality Scorecard | `019ce797-39e5-74d9-8037-8345c6032a47` | — | **none** | — |

Opening Manual Heartbeat on the conversation → empty criteria (0 scored) → Performance Score N/A is **correct** for the PG card.

### Screenshot math (smoking gun)

PI drawer for week `07/26/2026–08/01/2026` under Manual Heartbeat showed:

- `#712933057605` → **40%**
- `#712529323779` → **100%** (conv `019fb6c4-e4cc-…`, Manual submitted)
- `#712528445490` → **76%** (conv `019fb19e-ae84-…`, Manual submitted)
- Header: **72.0% on 3 scorecards**

Check: `(40 + 100 + 76) / 3 = 72` exactly.

Current Manual-only CH for that agent/week:

- Only **2** Manual scorecards: 76 and 100 → avg **88%**
- Ticket conversation has **zero** Manual `score_d` / `scorecard_d` rows (also no Manual criterion IDs)

Non-Performance Behaviors scores of exactly **40%** are common for this agent (pattern: 2/5 scored + 1 N/A). Ticket conv Non-Performance is **33.3% now** (2/6, no N/A); raw CH rows were rewritten `2026-08-05 17:38 UTC`, so it may have been 40% at screenshot time. Neighbor same-day Manual conv `019fb6c4-e4cc-…` still has Non-Performance **40%**.

### Issue #1 (weekly dates) — context

- Tari: PI Weekly for Jul 1–31 pulls Jun 28–30; Daily aligns with QM (12 @ ~97%).
- Screenshot drawer range for last July week cell is `07/26–08/01` (extends past Jul 31).
- Consistent with known weekly bucket expansion vs QM honoring exact selected range / submitted Manual evals only.

## Hypotheses

1. **Confirmed (issue #2 root cause):** Manual Heartbeat scorecard for this conversation **existed and was deleted/reset**. CH `system.mutations` shows `DELETE`/`_row_exists=0` for Manual template `019ce797-…` at `scorecard_time=2026-07-30 21:25:48` with scorecard IDs `019fb852-…` (mutated 2026-07-31) and `019fc8e3-…` (mutated 2026-08-03). PG reset clears scores immediately; CH delete is async → PI can show stale CH % while closed conversation shows N/A. After mutation completes, PI no longer lists the conversation under Manual (matches current observation).
2. Screenshot `72%=(76+100+40)/3` is consistent with a Manual card that scored ~40% before reset (or briefly stale CH), not necessarily a Non-Performance template leak.
3. Issue #1: intentional or long-standing weekly calendar-bucket expansion; product/UX alignment with QM TBD.

## Commands / queries run

- CH: scorecard/score for conversation UUID; Manual week filter for agent; all templates in week bucket
- PG: scorecards + template titles; Manual template criterion ID prefixes
- Linear `extract_images` on ticket screenshots

## Decisions

- Scope this session to diagnose issue #2; keep #1 as documented expected-vs-QM semantics question for Tinglin/product.
- Do not patch until live `RetrieveQAConversations` with Manual filter is reproduced for week 07/26–08/01.

## Next steps

1. Reproduce drawer API: Manual template + Breanna + 07/26–08/01 — does `#712933057605` still appear?
2. If yes, capture request (`scorecardTemplates`, `scoreResource`, statuses, criteria) and CH SQL path.
3. If no, treat as data-timing (NP was 40% + leak since fixed, or one-off) and still explain N/A as “no Manual scorecard”.
4. Reply in Slack/Linear with issue #2 finding; ask Tinglin on #1 weekly expansion product intent.
5. Update work item + daily log on conclusion.
