# Outcome: Opera adherence annotations and email AutoQA

> Migrated navigation: [Scorecard Workflows / Evaluation and Scoring](../scorecard-workflows/subdomains/evaluation-and-scoring/README.md). This folder remains the retained annotation reference.

**Created:** 2026-07-08
**Status:** Active reference

Knowledge base for how Cresta Opera behavior annotations (SDX, DDX, DNX, SNX, synthetic SDX, NOX) are generated at conversation time and consumed by email AutoQM scoring.

## Documents

| Doc | Contents |
|-----|----------|
| [adherence-types.md](./adherence-types.md) | Canonical adherence type definitions and vocabulary |
| [annotation-generation.md](./annotation-generation.md) | How Opera / policy engine emits annotations (positive and negative behaviors) |
| [synthetic-sdx.md](./synthetic-sdx.md) | SNX behaviors, synthetic SDX anchors, and why both exist |
| [autoqa-scoring.md](./autoqa-scoring.md) | How AutoQA maps annotations to scorecard outcomes |
| [email-window-filtering.md](./email-window-filtering.md) | `FilterContextForMessages`, SDX windows, per-agent email semantics |
| [pr-29694-snx-fix.md](./pr-29694-snx-fix.md) | COA-2566 / PR #29694 review notes |
| [references.md](./references.md) | Design doc, protos, PRs, tickets |

## Two-phase mental model

```
Phase 1 (runtime)     Opera / coach builder  →  writes moment_annotations (SDX, DDX, DNX, SNX, …)
Phase 2 (post-close)  AutoQA / scoring       →  reads annotations, filters per agent/message, scores criteria
```

AutoQA does **not** infer outcomes by scanning messages. It only reads annotations Opera already wrote.

## Quick reference: positive vs SNX spanning rules

For **email** per-agent filtering (`ENABLE_SDX_WINDOW_FILTERING`, default `true`):

| Outcome | Positive (SDX) behavior | SNX (should NOT do) behavior |
|---------|-------------------------|------------------------------|
| DDX (pass) | Point-only | **Spans** window |
| DNX (fail) | **Spans** window | Point-only |

DDX is always the positive outcome; DNX is always the negative outcome. Which one spans vs is point-only depends on behavior direction.