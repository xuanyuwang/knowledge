# SNX behaviors and synthetic SDX anchors

## Problem

Adherence infrastructure is built around **SDX → DDX/DNX** windows. Negative behaviors are conceptually **SNX** (“should **not** do X”), but windowing, dedup, hints, and email AutoQA all need an **SDX-shaped window start**.

## Solution: two “opportunity” annotations

For a negative-adherence behavior, Opera typically emits **three** annotations per episode:

| Annotation | Adherence type | Label | Role |
|------------|----------------|-------|------|
| SNX trigger | `SHOULD_NOT_DO_X` | — | Real trigger: “watch for bad behavior” |
| Synthetic SDX anchor | `SHOULD_DO_X` | `negative_adherence: true` | Mechanical window anchor at same position as SNX |
| Outcome | `DID_DO_X` or `DID_NOT_DO_X` | — | Pass or fail |

“Synthetic” = same **type** as positive SDX (`SHOULD_DO_X`), but it does **not** mean the agent should do something good. It exists only to anchor the window.

## Outcome semantics (inverted meaning vs type names)

For SNX, the **scoring** meaning of DDX/DNX is unchanged at the AutoQA layer:

| Outcome | Type | Meaning for SNX | AutoQA |
|---------|------|-----------------|--------|
| Positive | DDX | Agent **adhered** — did **not** do the forbidden thing | DETECTED |
| Negative | DNX | Agent **violated** — **did** the forbidden thing | NOT_DETECTED |

Spanning rules for email are **inverted** relative to positive behaviors (see [email-window-filtering.md](./email-window-filtering.md)).

## Generation rules (`behavior.proto`)

```text
negative_adherence_config set → negative adherence behavior

Behave well (never did forbidden thing in window):
  → single SDX + DDX pair at SNX position

Violate (did forbidden thing in window):
  → SDX + DNX pair(s) when violation happens
  → record_negative_adherence_once: at most one DNX if true
```

Source: `apiserver/sql-schema/protos/behavior/behavior.proto` (`BehaviorConfig.NegativeAdherenceConfig`).

## Downstream usage

### Dedup

`post_process_coach_builder_node.go`: `operaNegativeAdherenceLabel = "negative_adherence"`. Synthetic SDX and SNX use per-message dedup.

### AutoQA SNX detection (PR #29694)

`shared/scoring/autoqa_dao.go`:

- **`isSNX`** = any annotation in the behavior bucket has `SHOULD_NOT_DO_X` (SNX trigger present).
- **`isSDX`** = synthetic SDX anchor (`SHOULD_DO_X`) — buckets into `sdxAnnotations`, defines window start.
- **SNX trigger** is **not** added to filtered scoring context (no scoring outcome).
- **Synthetic SDX** **is** included when DDX/DNX applies to an agent (as the window anchor).

AutoQA does **not** use the `negative_adherence` label for SNX detection; it uses the SNX trigger’s adherence type.

### Evidence scoring

`calculateEvidencesForSdxMomentTrigger` only handles `SHOULD_DO_X`, `DID_DO_X`, `DID_NOT_DO_X`. SNX trigger (`SHOULD_NOT_DO_X`) is ignored for evidence.

## Example: Marriott COA-2566

Same agent message:

```
SNX trigger          (SHOULD_NOT_DO_X)
synthetic SDX anchor (SHOULD_DO_X)
DDX                  (DID_DO_X)   ← pass
```

Email multi-agent:

```
visitor-1: [SNX] [synthetic SDX]
agent-1:   (middle agent, behaved well)
agent-2:   [DDX]
```

Agent-1 gets pass credit because for SNX, DDX **spans** the window between synthetic SDX and DDX.
