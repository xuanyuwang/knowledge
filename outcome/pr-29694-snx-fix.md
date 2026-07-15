# PR #29694 — SNX support in email AutoQA window filtering

- **PR**: https://github.com/cresta/go-servers/pull/29694
- **Ticket**: [COA-2566](https://linear.app/cresta/issue/COA-2566/marriott-cec-email-rules-stopped-triggering-after-0330)
- **Author**: Ural Bayhan
- **Status**: Open (as of 2026-07-08)
- **Files**: `shared/scoring/autoqa_dao.go`, `shared/scoring/autoqa_dao_test.go`

## Problem

Marriott CEC email Opera rules returned **N/A** on scorecards after ~2026-03-30. SDX-window filtering (#27494) only implemented **positive** behavior semantics:

- DDX = point-only
- DNX = spans

SNX behaviors need the **opposite**. When DDX (pass) spanned but code treated it as point-only, agents between synthetic SDX and DDX were filtered out → empty context → N/A.

## Fix summary

1. `isSNXTrigger()` — detect `SHOULD_NOT_DO_X`.
2. Per behavior, `isSNX` if any SNX trigger in annotation set.
3. Merge DDX/DNX loops; `dxxAppliesToAgent()` with `spansWindow := isPositive == isSNX`.
4. SNX trigger excluded from filtered context (not a scoring outcome).

## Review verdict

**Recommend approve** — correct root cause, minimal scope, good tests.

### Strengths

- Root cause matches incident and code path
- `spansWindow := isPositive == isSNX` is clean
- Tests use real COA-2566 shape (SNX + synthetic SDX + DDX)
- Existing positive-behavior tests unchanged
- Refactor reduces duplication without changing SDX code path (verified by logic equivalence)

### Non-blocking nits

1. **Stale doc comment** on `FilterContextForMessages` still describes only SDX semantics; should mention SNX inversion.

2. **SNX trigger without `MessageID`** can leak via `index == -1 { included[annotation] = true }`. Low production risk; one-line guard with `isSNXTrigger` would harden.

3. **Missing tests** (optional):
   - SNX mirror of `SDXOnlyMessageWithOtherAgentsBetweenSDXAndDNX` for spanning DDX
   - SNX DNX on filtered agent’s message (positive case for violation)

4. **Codeowner**: `@cresta/core` approval missing per reviewbot.

### Logic equivalence check (SDX behaviors preserved)

| Old DDX | New: `!spansWindow` → `filteredAgentIndexSet.Has(dxx.index)` + outer window bounds | ✓ |
| Old DNX at SDX index | New spanning branch `dxx.index == sdxIndex` | ✓ |
| Old DNX spanning | New `hasAgentInRange` + SDX-at-anchor fallback | ✓ |

### Test coverage added

| Subtest | Validates |
|---------|-----------|
| `SNXBehaviorTriggerAnchorAndDDXOnSameAgentMessage` | Trigger excluded; SDX + DDX included |
| `SNXBehaviorDDXSpansToAgentBetweenAnchorAndDDX` | Main COA-2566 regression |
| `SNXBehaviorDNXPointOnlyNotOnAgentMessage` | DNX does not span to wrong agent |

## Related PRs

| PR | Role |
|----|------|
| #27040 | Include behavior annotations when agent has any match (legacy email fix) |
| #27494 | Introduce SDX-window filtering for email |
| #29694 | SNX spanning semantics |

## Deployment note

No new flag — behavior change under existing `ENABLE_SDX_WINDOW_FILTERING` (default true). Affects all email AutoQA using behavior triggers on SNX rules once merged.
