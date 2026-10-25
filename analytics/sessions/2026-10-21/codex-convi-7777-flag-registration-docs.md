---
summary: Merged CONVI-7777 rollout flag definition and frontend integration guidance
read_when:
  - Integrating or rolling out the CONVI-7777 assistance usage metric
---

# CONVI-7777 flag registration and documentation

- Primary domain: analytics; subdomain: assistance-insights.
- Source repo: `/Users/xuanyu.wang/repos/flux-deployments`.
- Branch context: `xw/convi-7777-assistance-usage-flag`, definition commit `ef31f4f948d`.
- Canonical handoff: [CONVI-7777](../../work-items/CONVI-7777.md#rollout-flag-and-frontend-integration-october-21).

## Outcome and evidence

Created `enableConversationAssistanceUsageMetric` in the existing common FeatureFlag definition, with a false default and no stage patches. Metadata records team `insights`, owner `xuanyu.wang@cresta.ai`, type `rollout`, created October 21, 2026, and target GA December 1, 2026. Current metadata requirements superseded the earlier simplified example with owner `convi`.

[PR #349836](https://github.com/cresta/flux-deployments/pull/349836) is confirmed merged through `gh pr view`: October 21, 2026 at 19:13:19 UTC (15:13:19 America/Toronto), merge commit `1f842dc1e7234de28af432ba32c0a043bcfebefc`. Before publication, all 11 flag checks and `mage sdpValidate` passed; all six stage overlays built with the new flag defaulting off. The validator emitted warnings for three existing empty ASR router pools. No explicit credential/profile was selected; git and gh authenticated themselves.

## Documentation and integration boundary

The user clarified that the frontend PRs already exist and requested flag integration guidance only, then asked to persist it in knowledge/analytics. Updated the canonical work item with the current hook's `{ value, isResolved }` signature, query enablement, matching column/menu/CSV visibility, true-zero handling, persistent local overrides, and cache refresh behavior. The hook signature was rechecked in `/Users/xuanyu.wang/repos/director/packages/director-app/src/hooks/useRolloutFlag.ts` during documentation. Updated the domain README and project PR reference, preserving existing changes.

An existing [frontend integration session](codex-convi-7777-rollout-flag.md) already records that work; it was preserved. This documentation update changes no frontend code, targeting, or backend behavior. Cluster reconciliation/live evaluation and the frontend integration result were not verified. Next: apply the guidance to the existing frontend PR stack, validate both states, and decide stage targeting separately.
