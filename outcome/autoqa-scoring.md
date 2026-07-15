# AutoQA scoring

AutoQA runs after conversation close (email: when case status is closed). It maps filtered behavior annotations to scorecard criterion outcomes.

## Code paths

| Component | Path | Role |
|-----------|------|------|
| Evidence calculation | `shared/scoring/autoqa_scoring.go` | Map DDX/DNX/SDX → outcome |
| Per-agent filtering | `shared/scoring/autoqa_dao.go` | `FilterContextForMessages` |
| Trigger | `apiserver/internal/autoqa/` | Load annotations, invoke scoring |

## Annotation → outcome mapping

`calculateEvidencesForSdxMomentTrigger`:

| Annotation | Outcome |
|------------|---------|
| `DID_DO_X` (DDX) | **DETECTED** |
| `DID_NOT_DO_X` (DNX) | **NOT_DETECTED** |
| `SHOULD_DO_X` (SDX / synthetic SDX) | **NOT_APPLICABLE** (only if no DDX/DNX in filtered set) |
| `SHOULD_NOT_DO_X` (SNX trigger) | Ignored (not in switch) |
| `NO_OPPORTUNITY_TO_DO_X` | Not handled in this switch (separate paths) |

Logic:

```go
if len(evidences) == 0 && len(sdxEvidences) > 0 {
    return sdxEvidences, nil  // SDX only → N/A
}
return evidences
```

## Scorecard subjects (email)

| Type | Subject | Filtering |
|------|---------|-----------|
| Per-message | `SCORECARD_SCORING_SUBJECT_MESSAGE` | One scorecard per agent message; filter with single `AgentMessageID` |
| Per-agent | `SCORECARD_SCORING_SUBJECT_CONVERSATION` | One scorecard per agent; filter with all message IDs for that agent |

## Manual QA interaction

If a scorecard has been manually graded, autoQM is skipped for that scorecard so manual scores are not overwritten.

## Common failure mode: N/A

Criteria show **N/A** when:

1. Filtered annotation context is **empty** for that agent/message (filtering bug or wrong window semantics).
2. Only SDX (or synthetic SDX) is present with no DDX/DNX yet — window not resolved.
3. SDX + NOX — treated as N/A in performance insights filtering contexts.

COA-2566 was case (1): SNX behaviors used positive-behavior DDX point-only rules, so middle agents got empty context → N/A.

## Ural’s email relevance rule (Jun 2026)

From #convo-intelligence Slack:

> Done behaviors are relevant if it's done on the agent message.
> Not done behaviors are relevant if the agent's message sits inside the window.
> We ignore other behaviors that are tagged to messages.

This is the product intent behind `FilterContextForMessages`; implementation details in [email-window-filtering.md](./email-window-filtering.md).
