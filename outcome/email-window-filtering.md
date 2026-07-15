# Email window filtering (`FilterContextForMessages`)

Enabled by `ENABLE_SDX_WINDOW_FILTERING` (default `true`) in `shared/scoring/autoqa_dao.go`. Introduced in go-servers PR #27494.

## Purpose

Email conversations have multiple agents and visitor messages. Annotations (SDX/DDX/DNX) may land on visitor messages while agents sit **between** them. Legacy filtering only kept annotations on the agent’s own message → middle agents got **N/A** instead of **NOT_DETECTED**.

Window filtering uses **message order** (`OrderedMessageIDs`, all messages by `created_at`) to decide which agents are in scope for each behavior window.

## Algorithm (positive / SDX behaviors)

For each behavior:

1. Bucket annotations: SDX, DDX, DNX (SNX trigger is not bucketed).
2. Sort SDX by message index.
3. For each SDX window `[sdx_i, sdx_{i+1})`:
   - For each DDX/DNX in window, decide if it applies to filtered agent(s).
4. If DDX or DNX applies → include it and its corresponding SDX.

### Spanning vs point-only (SDX behaviors)

| Outcome | Rule |
|---------|------|
| **DDX** | **Point-only** — only if DDX is on a filtered agent’s message |
| **DNX** | **Spans** — filtered agent strictly between SDX and DNX, or at SDX with no other agents between SDX and DNX |

### Examples (positive behavior)

**Both agents fail** — SDX and DNX on visitor messages:

```
visitor-1: SDX
agent-1:   (no annotation)
agent-2:   (no annotation)
visitor-2: DNX
```

Both agents → NOT_DETECTED.

**Only one agent passes** — DDX on agent-2 only:

```
visitor-1: SDX
agent-1:   (no annotation)
agent-2:   DDX
```

Only agent-2 → DETECTED. Agent-1 gets nothing from this window.

**Agent at SDX should not inherit spanning DNX** when another agent sits between:

```
msg1: SDX
msg2: (other agent message)
msg3: DNX
```

Filtering for msg1 only → empty (no DNX inherited).

## Algorithm (SNX behaviors) — PR #29694

Detect SNX: `isSNX = any(annotation, isSNXTrigger)` where `SHOULD_NOT_DO_X`.

**Invert** spanning rules:

| Outcome | SNX rule |
|---------|----------|
| **DDX** (pass) | **Spans** window |
| **DNX** (fail) | **Point-only** |

Unified helper:

```go
spansWindow := isDDX(dxx) == isSNX  // XOR: flip for SNX
```

SNX trigger (`SHOULD_NOT_DO_X`) is **not** added to filtered output.

Synthetic SDX (`SHOULD_DO_X`) is the window anchor (bucketed as SDX).

### COA-2566 regression (fixed by #29694)

```
visitor-1: [SNX] [synthetic SDX]
agent-1:   (should be scored)
agent-2:   [DDX]
```

Old code: DDX point-only → agent-1 excluded → **N/A**.

New code: SNX + DDX spans → agent-1 included → **DETECTED**.

## Legacy path

When `ENABLE_SDX_WINDOW_FILTERING` is false, `filterContextLegacy` uses direct message matching (older behavior).

## Feature flags

| Flag | Default | Effect |
|------|---------|--------|
| `ENABLE_SDX_WINDOW_FILTERING` | `true` | SDX-window algorithm vs legacy |
| `EnableAgentMessageIDsInAutoQA` | — | Email per-agent scoring infrastructure |

## Tests

`shared/scoring/autoqa_dao_test.go` — `TestFilterContextForMessages` (positive + SNX subtests).

Integration: `apiserver/internal/autoqa/action_calculate_conversation_autoscoring_evidence_test.go` (e.g. `EmailBehaviorSDXAndDNXBothOnVisitor`).
