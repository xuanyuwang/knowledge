# CONVI-6808: Behavior Annotation Analysis

## Root Cause Found

SD: criteria use **behavior triggers** (not metadata triggers as initially assumed). The scoring pipeline looks up annotations by `behavior_id`, but only **3 of 241 conversations** for Christian Nelson have annotations with the correct `behavior_id`. The remaining 238 produce `NOT_APPLICABLE`.

## The Lookup Mismatch

### How behavior scoring works

1. `calculateEvidencesForSdxMomentTrigger()` (`autoqa_scoring.go:177`) receives annotations from `MomentAnnotationsByBehaviorName[trigger.ResourceName]`
2. If no annotations → empty evidence list → `determineOutcome()` returns `NOT_APPLICABLE`
3. `autoqa_mapper.go:87-93`: `NOT_APPLICABLE` + `autoQaConfig.NotApplicable == nil` → `not_applicable=true`, no score stored

### How annotations are indexed

`autoqa_dao.go:186-203`: `findMomentAnnotations()` loads all annotations, then `Add()` indexes them:
- ALL annotations → `MomentAnnotationsByMomentName[momentName]`
- ONLY annotations with `behavior_id` set → `MomentAnnotationsByBehaviorName[behaviorName]`

### The gap

| Lookup method | Matching conversations | Annotations |
|---------------|----------------------|-------------|
| By `behavior_id` (current scoring path) | **3** | 30 |
| By `moment_template_id` (broader) | **151** | 452 |

148 conversations have annotations with the correct `moment_template_id` but NOT the correct `behavior_id`. These are invisible to the scoring pipeline.

## Annotation Breakdown for Christian Nelson (last 30 days)

### By behavior_id and moment_template

| Moment Template | Behavior (display_name) | behavior_id | Annotations | Conversations |
|----------------|------------------------|-------------|-------------|---------------|
| `01966db6` | SD: Disclose Cancellation Fee | `01966db5` (SD:) | 6 | 3 |
| `01966de2` | SD: Confirm Appointment | `0ee53963` (SD:) | 6 | 3 |
| `019731c1` | SD: Verifies Pricing | `e3e1f459` (SD:) | 6 | 3 |
| `1d70b062` | **Explain ROR** | `bbe24255` (non-SD) | **206** | **103** |
| `1d70b062` | SD: Explain ROR | `b5382146` (SD:) | 6 | 3 |
| `b4ff5b69` | SD: Explain Pest Coverage | `0560dfd4` (SD:) | 6 | 3 |

### Key observations

1. **"Explain ROR" collision**: Moment template `1d70b062` is shared by 3 behaviors: "Right of Rescission" (status=3/inactive), "Explain ROR" (`bbe24255`), and "SD: Explain ROR" (`b5382146`). The annotation pipeline assigns most annotations to "Explain ROR" (`bbe24255`), not "SD: Explain ROR" (`b5382146`). The scoring criterion references `b5382146`, so it only finds 3 conversations.

2. **Other 4 SD: criteria**: Each has only 1 behavior per moment template (no collision), yet only 3 conversations have annotations with `behavior_id` set. There are ~100+ conversations with SDX annotations (`adherence_type=0`) for these moment templates, but they lack `behavior_id` and have `adherence_type=UNSPECIFIED` — these produce no scoring evidence even if found.

3. **All 5 SD: criteria**: Exactly 3 conversations have annotations with the correct SD: behavior_id. These are the same 3 conversations across all criteria. The remaining 238 conversations get `NOT_APPLICABLE`.

## Why NOT_DETECTED is not the outcome

For behavior triggers, `calculateEvidencesForSdxMomentTrigger()` returns **empty** `[]*AutoScoringEvidence{}` when no annotations exist (line 182-183). This differs from moment and metadata triggers which return a `NOT_DETECTED` evidence:

| Trigger type | No annotations | Outcome |
|-------------|---------------|---------|
| Moment | Returns `NOT_DETECTED` evidence | Scored as 0 |
| Metadata | Returns `NOT_DETECTED` evidence | Scored as 0 |
| **Behavior** | Returns empty evidence list | **NOT_APPLICABLE** |

This is by design in the current code. Whether the behavior trigger SHOULD produce `NOT_DETECTED` instead of `NOT_APPLICABLE` when no annotations exist is a product decision.

## Relationship to PRs 26913/27170/27187

**Not the root cause.** These PRs only affected ClickHouse write paths:

| PR | Merged | Impact | Affects scoring? |
|----|--------|--------|-----------------|
| 26913 | Apr 21 | Removed historic analytics, broke CH writes | No — PG scoring untouched |
| 27170 | Apr 23 | Fixed CH score row percentage values | No — PG scoring untouched |
| 27187 | Apr 23 | Restored CH writes on async/reindex/batch paths | No — PG scoring untouched |

The PG `director.scores` data (238/241 `not_applicable=true`) is created by the autoQA scoring pipeline, which was not modified by any of these PRs.

## Relationship to other recent changes

| Commit | Date | Change | Affects Greenix? |
|--------|------|--------|-----------------|
| `97ae6acb` (CONVI-6491) | Apr 16 | Scored N/A support in `autoqa_mapper.go` | No — only changes path when `autoQaConfig.NotApplicable != nil`; SD: criteria have nil |
| `15243d3eca` | Apr 22 | `FilterContextForMessages` behavior annotation gating | No — only affects email per-message/per-agent scoring; Greenix is voice → uses `createPerConversationAutoscoringResults` with full context |
| `389d02c88a` | Recent | Score all agents between SDX and DDX/DNX for email | No — email-only |

## Historical comparison

| Period | Agent conversations | With behavior annotations |
|--------|-------------------|--------------------------|
| 30-60 days ago | 37 | 1 (2.7%) |
| Last 30 days | 241 | 3 (1.2%) |

Behavior annotation coverage has always been ~1-3% for this agent. **This is NOT a recent regression.**

## Root cause conclusion

The annotation pipeline (orchestrator/AI service) is not populating `behavior_id` on most behavior-related moment annotations. The scoring pipeline correctly looks up by `behavior_id`, but finds annotations for only 3 conversations. The missing link is upstream: **why does the annotation pipeline create SDX annotations for these moment templates without setting the `behavior_id`?**

## Recommended next steps

1. **Annotation pipeline investigation**: Why do SDX annotations (`adherence_type=0, type=26`) for these moment templates lack `behavior_id`? The orchestrator team should investigate how behavior annotations are created and when `behavior_id` is populated vs left null.

2. **"Explain ROR" behavior collision**: Even when `behavior_id` IS set, moment template `1d70b062` has 103 conversations tagged with `bbe24255` ("Explain ROR") vs only 3 with `b5382146` ("SD: Explain ROR"). The annotation pipeline appears to prefer the older behavior. This is a separate issue.

3. **Product decision**: Should behavior triggers produce `NOT_DETECTED` (scored 0) instead of `NOT_APPLICABLE` (excluded) when no behavior annotations exist? Currently behavior triggers return empty evidence → N/A, while moment/metadata triggers return `NOT_DETECTED` evidence → scored.
