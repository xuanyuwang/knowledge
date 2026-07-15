# Adherence annotation types

Canonical definitions from the unified moments/actions design doc (linked from `moment_annotation.proto`):

- Design doc: https://docs.google.com/document/d/12Y6JsOaVHkgo7lM-h8B10dW9xHDW-0b1CtP1EsfEzrQ/edit#heading=h.75a34mg3eey3
- Proto: `apiserver/sql-schema/protos/moment/moment_annotation.proto`

## Types

| Type | Abbrev | Meaning |
|------|--------|---------|
| `ADHERENCE_TYPE_SHOULD_DO_X` | **SDX** | Opportunity opened — agent should do X; starts an adherence window |
| `ADHERENCE_TYPE_DID_DO_X` | **DDX** | Behavior detected → **adherence** (positive outcome) |
| `ADHERENCE_TYPE_DID_NOT_DO_X` | **DNX** | No detection within the adherence window → **non-adherence** (negative outcome) |
| `ADHERENCE_TYPE_SHOULD_NOT_DO_X` | **SNX** | Opportunity opened — agent should **not** do X (negative-adherence trigger) |
| `ADHERENCE_TYPE_NO_OPPORTUNITY_TO_DO_X` | **NOX** | Like DNX, but conversation ended before the agent had a chance to complete the behavior |
| `ADHERENCE_TYPE_END_OF_ADHERENCE` | — | Window ended via a cutoff moment (not simply “time window expired”) |
| `ADHERENCE_TYPE_UNSPECIFIED` | — | Raw detection; not behavior-tracking |

## SDX windows

Multiple opportunities per behavior are possible when policy config allows it. Annotations group into **windows** from one SDX (or synthetic SDX anchor) to the next SDX for the same behavior:

```
SDX₁ ─── [DDX or DNX] ─── SDX₂ ─── [DDX or DNX] ─── …
```

Each window is evaluated independently.

## Adherence window (Opera side)

Configured per behavior via `BehaviorConfig` (`apiserver/sql-schema/protos/behavior/behavior.proto`):

- `sdx_adherence_window` — duration, agent turns, etc.
- `sdx_adherence_cutoff_moment_id` — must complete before a cutoff moment
- `conversation_close_option` — what happens to open SDX when conversation closes
- `allow_multiple_sdx`, `allow_multiple_positive_adherence`, `allow_per_message_adherence`
- `skip_adherence_without_behavior_moment` — resolve at trigger time from LLM boolean (no window tracking)

Opera watches within the window and emits DDX when the behavior is detected, or DNX when the window closes without detection. NOX when the conversation ends without opportunity.
