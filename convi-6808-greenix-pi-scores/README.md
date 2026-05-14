# CONVI-6808: Greenix PI Scores Dropped / Metadata Criteria N/A

**Status**: Root cause identified — annotation pipeline issue  
**Customer**: Greenix  
**Ticket**: [CONVI-6808](https://linear.app/cresta/issue/CONVI-6808)  
**Affected since**: ~2026-05-07  
**Namespace**: greenix / us-east-1

## Problem

Greenix reports a sudden, significant drop in Performance Insights scores across all batches (Sales, Loyalty, Support, Collections) starting approximately May 7-8. Metadata-driven criteria are resolving as N/A for agents. No Opera rule changes were made by the customer.

Confirmed affected agent: Christian Nelson (agent_user_id: `7f37358cfb505c53`, Sales batch, IS Compliance template `bf0afbf8-2e1e-4448-9783-bd3b5f8a6ad1`).

## Investigation Summary

### float_weight=0 Hypothesis — RULED OUT

Compared to CONVI-6753 (Home Care Delivered). In Greenix's `score_d` table, every row with `float_weight=0` also has `percentage_value=-1`. Zero `scored_zero_fw` across all days. **Not the same root cause.**

### PG `historic.scorecard_scores` — STALE (Expected)

PG stopped receiving data after 2026-04-28 (25 rows that day, total 36M rows). This is expected due to the historic analytics removal — CH is now written to directly via `BuildScoreRowsFromDirectorScores`.

### ClickHouse Data Quality — ALL STABLE

| Check | Result |
|-------|--------|
| Overall scoring rate | 68-71% daily, stable |
| Per-usecase weighted avg | Stable across all 4 batches |
| Per-criterion scoring rate | No criterion dropped |
| Per-criterion score values | Stable |
| Template revisions | Unchanged (same revision all May) |
| Metadata/moment volumes | Stable |
| Global metadata criteria scoring | 22-28%, stable |

### Christian Nelson Deep Dive

**IS Compliance template (`bf0afbf8`)** — Sales batch:

| Metric | April | May 4-8 |
|--------|-------|---------|
| Scorecards/day | 11-19 | 8-13 |
| Avg scorecard score | 68-93 | 50-87 |
| Conversations/week | 43-70 | 50 |

Score values did drop for this specific agent in May, but the data **exists** in both `score_d` and `scorecard_d`.

**Metadata "SD:" criteria** (SD: Explain Pest Coverage, SD: Verifies Pricing, etc.):
- Only **2 conversations** matched metadata conditions in 30 days (Apr 22 + Apr 25)
- This is **NOT a regression** — same criteria had 0-1 matches/month in March too
- Global metadata criteria scoring rate is stable at 22-28%

### Most Recent Week N/A in Screenshot — RESOLVED (Transient)

The screenshot showed the entire most recent week column as N/A for all criteria, including non-metadata ones. However, API verification on May 9 confirmed:

- **Per-criterion API response** (`groupBy: [CRITERION, TIME_RANGE]`) for Apr 5 - May 10 **includes May 3 data**: 4 criteria with valid scores, `weightSum > 0`, ~134 total scorecards
- **Aggregate API response** (`groupBy: [TIME_RANGE]`) also returned May 3 data: 50 conversations, score 0.78
- Frontend date key alignment verified: column key and data key both produce `"2026-05-03"` — no mismatch

**The N/A issue was transient.** Most likely cause: data processing lag in `conversation_d` around May 7-8. The PI query JOINs `score_d` with `conversation_d` for voicemail/duration filters; if `conversation_d` lagged behind, the JOIN produced zero results for the most recent period. It has since caught up.

### PI Architecture Notes

PI makes two separate `RetrieveQAScoreStats` calls (`PerformanceProgression.tsx`):
- **Per-criterion** (line 120): `groupBy: [CRITERION, TIME_RANGE]` — populates individual criterion rows and drives chapter column population
- **Whole-template** (line 156): `groupBy: [TIME_RANGE]` — "All criteria" aggregate stats

Both use the same `filtersState.submitDateRange`. Chapter rows (`createColumnsForChapter`) only iterate dates present in CHILDREN rows; if per-criterion returns no data for a week, ALL rows show N/A.

### Tables PI Queries

From `insights-server/internal/analyticsimpl/common_clickhouse.go`:
- `scoreTable` → `score_d` (default for QA Score Stats)
- `scorecardTable` → `scorecard_d` (for scorecard-level view)
- `scorecardScoreTable` → `scorecard_score_d` (defined but appears unused in main QA queries)

**`scorecard_score` and `scorecard_score_d` tables are completely empty** (0 rows, latest timestamp 1970-01-01). Not used by PI queries — `score_d` is the default table.

## Root Cause: Behavior Annotation Pipeline

**Root cause identified.** The SD: criteria use **behavior triggers** (not metadata triggers as initially assumed). The scoring pipeline looks up annotations by `behavior_id`, but the annotation pipeline does not populate `behavior_id` on most annotations.

### The Lookup Mismatch

| Lookup method | Conversations with annotations | Annotations |
|---------------|-------------------------------|-------------|
| By `behavior_id` (scoring path) | **3** | 30 |
| By `moment_template_id` (broader) | **151** | 452 |

148 conversations have annotations with the correct `moment_template_id` but NOT the correct `behavior_id`. These are invisible to the scoring pipeline.

### How it works

1. `calculateEvidencesForSdxMomentTrigger()` (`autoqa_scoring.go:177`) receives annotations via `MomentAnnotationsByBehaviorName[trigger.ResourceName]`
2. `autoqa_dao.go:186-203`: `Add()` only indexes annotations by behavior name when `behavior_id` is set
3. No annotations found → empty evidence list → `determineOutcome()` returns `NOT_APPLICABLE`
4. `autoqa_mapper.go:87-93`: `NOT_APPLICABLE` + `autoQaConfig.NotApplicable == nil` → `not_applicable=true`, no score stored

### "Explain ROR" behavior collision

Moment template `1d70b062` is shared by 3 behaviors:
- "Explain ROR" (`bbe24255`) — 103 conversations annotated
- "SD: Explain ROR" (`b5382146`) — 3 conversations annotated
- "Right of Rescission" (inactive)

The SD: criterion references `b5382146`, but most annotations go to `bbe24255`.

### Not a regression

| Period | Agent conversations | With behavior annotations |
|--------|-------------------|--------------------------|
| 30-60 days ago | 37 | 1 (2.7%) |
| Last 30 days | 241 | 3 (1.2%) |

Behavior annotation coverage has always been ~1-3%. The annotation pipeline has never reliably populated `behavior_id` for these SD: criteria.

### Broader annotation data (all agents)

Across all Greenix agents, the annotation pipeline IS creating annotations with `behavior_id` set for these behaviors:

| behavior_id | Conversations | Annotations |
|-------------|--------------|-------------|
| `0560dfd4` (SD: Explain Pest Coverage) | 3,714 | 7,428 |
| `01966db5` (SD: Disclose Cancellation Fee) | 3,091 | 6,182 |
| `0ee53963` (SD: Confirm Appointment) | 3,091 | 6,182 |
| `b5382146` (SD: Explain ROR) | 3,091 | 6,182 |
| `e3e1f459` (SD: Verifies Pricing) | 3,091 | 6,182 |

So behavior annotations DO exist broadly, but Christian Nelson's conversations are rarely among them. This may indicate an agent-specific or conversation-routing issue rather than a global pipeline failure.

### PRs 26913/27170/27187 — NOT related

These PRs only affected ClickHouse write paths (historic analytics removal + restoration). PG scoring pipeline was not modified. See `deliverables/behavior-annotation-analysis.md` for full analysis.

### Recent scoring code changes — NOT related

| Commit | Change | Affects Greenix? |
|--------|--------|-----------------|
| `97ae6acb` (CONVI-6491) | Scored N/A support | No — SD: criteria have `NotApplicable == nil` |
| `15243d3eca` | `FilterContextForMessages` gating | No — email-only; Greenix is voice |
| `389d02c88a` | Score all agents for email | No — email-only |

## Other Investigated Issues

1. **Most recent week N/A in screenshot**: **Transient, now resolved.** API verification on May 9 confirmed valid data for May 3 week. Likely caused by `conversation_d` processing lag around May 7-8.
2. **Score drop for specific agent**: Christian Nelson's May scores (50-65 avg) are lower than April (70-93 avg). Natural variation, not a data issue.
3. **float_weight=0 hypothesis**: Ruled out — not the same root cause as CONVI-6753.

## Next Steps

- [ ] **Annotation pipeline investigation**: Why does Christian Nelson (and most agents) have so few behavior annotations with `behavior_id` set, when the global counts show 3,000+ conversations per behavior? Is this a conversation routing, agent assignment, or annotation creation timing issue?
- [ ] **"Explain ROR" behavior collision**: Even when `behavior_id` IS set, annotations prefer `bbe24255` ("Explain ROR") over `b5382146` ("SD: Explain ROR"). The orchestrator team should investigate the behavior selection logic.
- [ ] **Product decision**: Should behavior triggers produce `NOT_DETECTED` (scored 0) instead of `NOT_APPLICABLE` (excluded) when no behavior annotations exist?
- [ ] Confirm with the customer that the "most recent week N/A" transient issue is resolved
