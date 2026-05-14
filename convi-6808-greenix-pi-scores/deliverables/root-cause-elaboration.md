# CONVI-6808: Root Cause Elaboration

## Summary

SD: criteria show N/A because the scoring pipeline finds no annotations for most conversations. The scoring pipeline looks up by `behavior_id`, but only 3 of 241 conversations for Christian Nelson have annotations with `behavior_id` set. This is not a recent regression — it has always been this way (~1-3% coverage).

## The Two-Layer Annotation Architecture

Moment annotations for a conversation come from **two separate sources**, and the scoring pipeline only reads one of them.

### Layer 1: Moment Detection (AI Services)

AI services (intents, keywords, NLU models) detect moments in real-time during conversations. These create annotations with:
- `moment_template_id` = set (which moment was detected)
- `behavior_id` = **NOT SET** (null)
- `adherence_type` = `UNSPECIFIED` (0)

These are classified as **non-adherence moments** by the coach builder policy engine (`coach_builder_utils.py:36-41`) because they have no `policy_name` in their metadata:

```python
# coach_builder_utils.py:39-41
if not policy_id:
    non_adherence_moments.append(moment)
    continue
```

These moments serve as **detection signals** — the policy engine uses them as input to determine whether a behavior was performed.

### Layer 2: Behavior Adherence Evaluation (Policy Engine)

The coach builder policy engine (`python-ai-services/policy-engine/`) evaluates behaviors in real-time during conversations. When it determines a behavior outcome, it creates adherence annotations with:
- `moment_template_id` = set
- `behavior_id` = **SET** (from `config_moment.behavior_id`)
- `adherence_type` = `SHOULD_DO_X` (3), `DID_DO_X` (1), or `DID_NOT_DO_X` (2)

Created by `_create_moments_from_config()` (`coach_builder_policy_base_evaluator.py:410-442`):

```python
# coach_builder_policy_base_evaluator.py:422-435
moment = MomentAnnotation(
    moment_template_id=config_moment.moment_template_id,
    behavior_id=config_moment.behavior_id,        # <-- SET from config
    adherence_type=adherence_type,                 # <-- SDX/DDX/DNX
    ...
)
```

### What the Scoring Pipeline Reads

The autoQA scoring pipeline (`autoqa_dao.go:186-203`) indexes annotations into two maps:

```
ALL annotations → MomentAnnotationsByMomentName[momentName]         # Layer 1 + 2
ONLY annotations with behavior_id → MomentAnnotationsByBehaviorName[behaviorName]  # Layer 2 only
```

For **behavior triggers** (all SD: criteria use this type), the scoring code looks up from `MomentAnnotationsByBehaviorName` only:

```go
// autoqa_scoring.go:171-174
case TriggerTypeBehavior:
    annotations := conversationAutoScoringContext.MomentAnnotationsByBehaviorName[trigger.ResourceName]
    return calculateEvidencesForSdxMomentTrigger(annotations, ...)
```

**Layer 1 annotations are invisible to behavior trigger scoring.** Only Layer 2 annotations (with `behavior_id` set) are found.

## The Gap for Christian Nelson

| What | Count | Source |
|------|-------|--------|
| Total conversations (30 days) | 241 | scorecards |
| Conversations with Layer 1 annotations (moment_template_id match) | 151 | app.moment_annotations |
| Conversations with Layer 2 annotations (behavior_id match) | **3** | app.moment_annotations |
| Conversations the scoring pipeline can score | **3** | Layer 2 only |
| Conversations scored NOT_APPLICABLE | 238 | director.scores |

148 conversations have the moment detected (Layer 1) but no behavior evaluation result (Layer 2).

### Why only 3 conversations have Layer 2 annotations

The coach builder policy engine runs **in real-time during the conversation**. For a behavior adherence annotation to exist, the following must all be true:

1. **The behavior policy must be deployed** for the conversation's use case / routing configuration
2. **The policy engine must receive the conversation** for evaluation (routing, capacity)
3. **The policy engine must evaluate the specific behavior** in its coach builder config
4. **The evaluation must produce an outcome** (SDX, DDX, or DNX)

If any of these conditions fails, no Layer 2 annotation is created, and the scoring pipeline finds nothing → NOT_APPLICABLE.

### Global data suggests the pipeline works — for other conversations

| behavior_id | Conversations (all agents) | Annotations |
|-------------|---------------------------|-------------|
| `0560dfd4` (SD: Explain Pest Coverage) | 3,714 | 7,428 |
| `01966db5` (SD: Disclose Cancellation Fee) | 3,091 | 6,182 |
| `0ee53963` (SD: Confirm Appointment) | 3,091 | 6,182 |
| `b5382146` (SD: Explain ROR) | 3,091 | 6,182 |
| `e3e1f459` (SD: Verifies Pricing) | 3,091 | 6,182 |

Each conversation produces exactly 2 annotations per behavior (e.g., SDX + DDX or SDX + DNX). The policy engine IS creating behavior annotations for thousands of conversations globally. But Christian Nelson has only 3.

**This is not a global pipeline failure.** It's either:
- An agent-specific or use-case-specific configuration issue (his conversations route to a pipeline that doesn't run behavior evaluation)
- A timing issue (behavior policies weren't deployed for his use case during most of his conversations)
- A conversation attribute issue (duration, type, language) that excludes his calls from behavior evaluation

### Historical: always low

| Period | Agent conversations | With Layer 2 annotations |
|--------|-------------------|--------------------------|
| 30-60 days ago | 37 | 1 (2.7%) |
| Last 30 days | 241 | 3 (1.2%) |

This pattern has been consistent. The behavior policy has never produced annotations for most of Christian Nelson's conversations.

## The "Explain ROR" Collision (Separate Issue)

Even for the 3 conversations that DO have Layer 2 annotations, the "SD: Explain ROR" criterion has a second problem.

Moment template `1d70b062` is shared by 3 behaviors:

| Behavior | behavior_id | Status | Conversations annotated |
|----------|-------------|--------|------------------------|
| Right of Rescission | (unknown) | Inactive (status=3) | 0 |
| Explain ROR | `bbe24255` | Active | 103 |
| **SD: Explain ROR** | `b5382146` | Active | 3 |

The criterion's trigger references `b5382146` (SD: Explain ROR), but the policy engine assigns most annotations to `bbe24255` (Explain ROR). This is likely because the "Explain ROR" behavior was created first and the policy engine matches to it preferentially.

Even if all conversations had Layer 2 annotations, 103 would be scored by "Explain ROR" and only 3 by "SD: Explain ROR". This is a behavior collision that requires the annotation pipeline to be reconfigured.

## Why NOT_APPLICABLE and Not NOT_DETECTED

Behavior triggers have a unique code path in `calculateEvidencesForSdxMomentTrigger()`:

```go
// autoqa_scoring.go:182-183
if len(annotations) == 0 {
    return []*autoqapb.AutoScoringEvidence{}, nil  // EMPTY list
}
```

This returns an **empty evidence list**, which causes `determineOutcome()` to return the default: `NOT_APPLICABLE`.

By contrast, moment and metadata triggers return a `NOT_DETECTED` evidence when no annotations exist, which becomes a scored 0.

| Trigger type | No annotations → | Evidence | Outcome | Scored? |
|-------------|------------------|----------|---------|---------|
| Moment | `NOT_DETECTED` evidence | Non-empty | `NOT_DETECTED` → 0 | Yes |
| Metadata | `NOT_DETECTED` evidence | Non-empty | `NOT_DETECTED` → 0 | Yes |
| **Behavior** | Empty evidence list | Empty | `NOT_APPLICABLE` | **No** |

**This asymmetry is by design in the current code.** Whether behavior triggers should produce `NOT_DETECTED` instead of `NOT_APPLICABLE` is a product decision.

## Full Data Flow Diagram

```
┌─────────────────────────────────────────────────────────┐
│  Real-time Conversation                                  │
│                                                          │
│  ┌──────────────┐    ┌─────────────────────────────┐    │
│  │ AI Services   │    │ Coach Builder Policy Engine  │    │
│  │ (intents,    │    │ (behavior adherence eval)    │    │
│  │  keywords)   │    │                              │    │
│  └──────┬───────┘    └──────────┬───────────────────┘    │
│         │                       │                        │
│    Layer 1                 Layer 2                       │
│    moment_template_id ✓    moment_template_id ✓         │
│    behavior_id ✗           behavior_id ✓                │
│    adherence_type = 0      adherence_type = SDX/DDX/DNX │
└─────────┬───────────────────────┬────────────────────────┘
          │                       │
          ▼                       ▼
┌─────────────────────────────────────────────────────────┐
│  app.moment_annotations                                  │
│                                                          │
│  Christian Nelson: 151 convos     3 convos               │
│  Global:          many            3,091+ convos          │
└─────────────────────────────────────────────────────────┘
          │                       │
          │ (ignored)             ▼
          │              ┌─────────────────────────┐
          │              │ autoqa_dao.go:Add()      │
          │              │ MomentAnnotations        │
          │              │ ByBehaviorName[name]     │
          │              └───────────┬─────────────┘
          │                         │
          │                         ▼
          │              ┌─────────────────────────┐
          │              │ autoqa_scoring.go        │
          │              │ Behavior trigger lookup  │
          │              │                         │
          │              │ 3 convos → scored       │
          │              │ 238 convos → N/A        │
          │              └─────────────────────────┘
```

## Remaining Questions

1. **Why does the policy engine not evaluate Christian Nelson's conversations?** The behavior policies are active (3,091+ conversations globally). What determines whether a conversation gets behavior evaluation — use case routing, policy deployment, conversation attributes?

2. **Is this a configuration issue or an operational issue?** If the behavior policies are supposed to be deployed for Christian Nelson's use case (Sales batch), then there's a misconfiguration. If they're only deployed for certain batches or teams, then this is expected behavior and the SD: criteria shouldn't be in Christian Nelson's scorecard template.

3. **Should behavior triggers produce NOT_DETECTED instead of NOT_APPLICABLE?** This is the difference between "no data exists" (N/A) and "data exists, behavior not performed" (0). The product team should decide which semantics are correct for the customer's use case.

4. **How should the "Explain ROR" behavior collision be resolved?** The annotation pipeline preferentially assigns to `bbe24255` instead of `b5382146`. The behaviors need to be deduplicated or the moment template needs to be split.
