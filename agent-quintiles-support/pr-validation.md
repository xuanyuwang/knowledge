# PR Validation Against Requirements

**Created:** 2026-02-19

Validates existing PRs and in-progress work against `requirements.md`.

---

## PRs Reviewed

| PR | Repo | State | Description |
|----|------|-------|-------------|
| [cresta-proto #7874](https://github.com/cresta/cresta-proto/pull/7874) | cresta-proto | **MERGED** | Add `QuintileRank` enum + `quintile_rank` field to `QAScoreGroupBy` |
| [go-servers #25795](https://github.com/cresta/go-servers/pull/25795) | go-servers | **OPEN** (approved, CI green) | `ScoreToQuintileRank` + `setQuintileRankForPerAgentScores` + tests |
| (no PR yet) | director | Working tree changes on `feature/agent-quintiles` | Type layer + Agent Leaderboard column |
| (no PR yet) | config | — | Feature flag `enableQuintileRank` |

---

## 1. cresta-proto #7874 — MERGED ✅

**Requirement:** "divide agents into quintiles based on each agent's QA score" + "only categorize agents when grouped by agents"

**What it does:**
- Added `enum QuintileRank` (UNSPECIFIED=0, RANK_1=1 through RANK_5=5) to `qa_stats.proto`
- Added `QuintileRank quintile_rank = 7 [(google.api.field_behavior) = OUTPUT_ONLY]` to `QAScoreGroupBy`
- Score bands documented: 1=80+, 2=60–79, 3=40–59, 4=20–39, 5=0–19

**Verdict:** ✅ Correct. OUTPUT_ONLY is appropriate (BE computes it). Enum values and bands match requirements. Generated types in `@cresta/web-client@2.0.534` confirmed.

**No issues found.**

---

## 2. go-servers #25795 — OPEN ⚠️

**Requirement:** "divide agents into quintiles based on each agent's QA score for the template" + "only categorize agents when grouped by agents, do NOT support team quintiles"

### What it does

- `ScoreToQuintileRank(score float32)` — maps 0–1 score to `QuintileRank` enum via score bands
- `setQuintileRankForPerAgentScores(response)` — sets `QuintileRank` on scores where `GroupedBy.User != nil`
- Called in the `QA_ATTRIBUTE_TYPE_AGENT` path in `retrieveQAScoreStatsInternal` (Postgres path only)
- 14 tests: boundary cases, multi-agent all bands, nil safety, no-leakage to AGENT_TIER path

### Validation Results

| Check | Status | Detail |
|-------|--------|--------|
| Score bands correct (80+→1, 60–79→2, etc.) | ✅ | `ScoreToQuintileRank` thresholds match requirements |
| Only per-agent rows get quintile | ✅ | Guards on `GroupedBy.User != nil` |
| Not applied to AGENT_TIER grouping | ✅ | `setQuintileRankForPerAgentScores` is only called in the AGENT path; `TestAggregateTopAgentsResponse_NoQuintileRankLeakage` test confirms AGENT_TIER rows stay UNSPECIFIED |
| Nil safety | ✅ | Handles nil response, nil QaScoreResult, empty scores |
| Test coverage | ✅ | 10 boundary cases + 3 integration tests |
| **ClickHouse path** | ⚠️ **MISSING** | `retrieve_qa_score_stats_clickhouse.go` builds per-agent scores (`GroupedBy.User` set at line 719 for `QA_ATTRIBUTE_TYPE_AGENT`) but never sets `QuintileRank`. See issue #1 below. |
| Unrelated change | ℹ️ Minor | `voice-integration/script/translator-tts-sim/cmd/sim.go` — `outBase` → `absOutBase` rename. From an earlier lint-fix commit on the branch (`9fa75e6`). Harmless but unrelated to quintiles. |

### Issue #1: ClickHouse path missing quintile rank (medium severity)

**File:** `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`

In `convertCHResponseToQaScoreStatsResponse` (line 677–700), per-agent scores are assembled:
```go
scores = append(scores, &analyticspb.QAScore{
    GroupedBy: convertQaGroupByForQaScoreStatsRow(row, customerID, groupByAttributes, usernameToUserMap),
    Score:     safeDivideFloat(row.weightedPercentageSum, row.weightSum),
    ...
})
```

`convertQaGroupByForQaScoreStatsRow` sets `GroupedBy.User` when the attribute is `QA_ATTRIBUTE_TYPE_AGENT` (line 718–719) but does NOT set `QuintileRank`.

**Impact:** Customers using the ClickHouse analytics path will get `QUINTILE_RANK_UNSPECIFIED` for all agents. FE will show "–" in the quintile column for these customers.

**Fix options:**
1. **Preferred:** After the loop that builds `scores` in `convertCHResponseToQaScoreStatsResponse`, call `setQuintileRankForPerAgentScores(response)` (same function already written for the Postgres path).
2. Alternatively, set `QuintileRank` inline in `convertQaGroupByForQaScoreStatsRow` when the attribute is AGENT — but this requires passing the score value into that function.

**Recommendation:** Add `setQuintileRankForPerAgentScores(resp)` call in the ClickHouse path before returning. Add a test for the ClickHouse path if integration tests exist.

---

## 3. Director (working tree, no PR) — ⚠️ Multiple Issues

FE changes exist on the `feature/agent-quintiles` branch in `~/repos/director-quintiles` (5 files modified, not pushed as a PR).

### What it does

- `apiTypes.ts`: Added `QuintileRank` import + `quintileRank?: QuintileRank` to `QAScoreGroupBy`
- `transformersQAI.ts`: Added `quintileRank: groupedBy?.quintileRank` passthrough
- `types.ts`: Added `quintileRank?: QuintileRank` to `LeaderboardRow`
- `useVisibleColumnsForLeaderboards.tsx`: Added `'quintileRank'` to `alwaysVisible`
- `AgentLeaderboard.tsx`: Added `row.quintileRank` assignment + column definition

### Validation Results

| Check | Status | Detail |
|-------|--------|--------|
| Type layer (apiTypes, transformer) | ✅ | Correctly threads `quintileRank` from web-client through internal types |
| `LeaderboardRow` field | ✅ | Propagates to `AgentLeaderboardRow` and `TeamLeaderboardRow` via inheritance |
| Row assignment | ✅ | `row.quintileRank = groupResult.groupedBy?.quintileRank` in QA score loop |
| Always visible | ✅ | Added to `alwaysVisible` array |
| **Column position** | ❌ **WRONG** | Currently after Performance group (line 490). Requirements say: "after Live Assist". Should be after `liveAssistColumnGroup` (line 524), before `outcomeMetricsGroup` (line 529). |
| **Cell display format** | ❌ **WRONG** | Currently `Q${QuintileRankNumber[value]}` → "Q1"–"Q5". Requirements say: plain number 1–5. |
| **Feature flag guard** | ❌ **MISSING** | No `useFeatureFlag('enableQuintileRank')` gating. Column and visibility are unconditional. |
| **Icon on agent name** | ❌ **MISSING** | No `QuintileRankIcon` component. Requirements: gold/silver/bronze icon for Q1/Q2/Q3 next to agent name. |
| **Agent LB per metric** | ❌ **MISSING** | No changes to `AgentLeaderboardByMetric.tsx`. Requirements: icon on name column. |
| **Performance page tables** | ❌ **MISSING** | No changes to `LeaderboardByScorecardTemplateItem` or `LeaderboardPerCriterion`. Requirements: quintile column + icon on both. |
| **Coaching Hub** | ❌ **MISSING** | No changes to `AgentDetailsCell.tsx` or `RecentCoachingActivities.tsx`. Requirements: icon with tooltip "Xth quintile based on last 7 days". |

### Issue #2: Column position wrong

**Current:** After Performance group → AHT → Assistance → Engagement → Live Assist → Outcome Metrics
**Required:** After Live Assist, before Outcome Metrics

**Fix:** Move the `if (visibleColumns.has('quintileRank'))` block from line 490 to after `liveAssistColumnGroup` push (line 527).

### Issue #3: Cell display format wrong

**Current:** `Q${QuintileRankNumber[value]}` → "Q1", "Q2", etc.
**Required:** Plain number: `1`, `2`, `3`, `4`, `5`

**Fix:** Change to `String(QuintileRankNumber[value])` or just `QuintileRankNumber[value]`.

### Issue #4: No feature flag

All quintile UI must be gated behind `enableQuintileRank`. Currently unconditional.

**Fix:**
- Read flag: `const enableQuintileRank = useFeatureFlag('enableQuintileRank');`
- Gate column: `if (enableQuintileRank && visibleColumns.has('quintileRank'))`
- Gate `alwaysVisible` inclusion conditionally
- Gate icon rendering

### Issues #5–#8: Missing pages/components

See implementation plan sections 2.1 (shared icon), 2.3–2.6 for the remaining work. These are not bugs in existing code — they're unimplemented requirements.

---

## 4. Config (no PR, no work started)

Feature flag `enableQuintileRank` needs to be added to `config/src/CustomerConfig.ts`. No work has been done yet. See implementation plan section 2.8.

---

## Summary

| PR/Work | Verdict | Action Items |
|---------|---------|-------------|
| cresta-proto #7874 | ✅ Good to go | Already merged |
| go-servers #25795 | ⚠️ Missing ClickHouse path | Add `setQuintileRankForPerAgentScores` call in ClickHouse path before merging |
| director (WIP) | ⚠️ Multiple issues | Fix column position, cell display; add feature flag guard; then implement remaining pages |
| config | ❌ Not started | Add `enableQuintileRank` flag |

### Priority order for fixes

1. **go-servers #25795**: Add ClickHouse path support (blocks correct FE behavior for some customers)
2. **config**: Add feature flag (can be done in parallel, blocks FE gating)
3. **director**: Fix column position + display format + add flag guard (quick corrections)
4. **director**: Implement remaining pages (Performance, Coaching Hub, icons)
