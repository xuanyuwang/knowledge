# CONVI-7777 frontend rollout flag integration

- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`
- Branch context: preparation `xw/convi-7777-assistance-usage-preparation` / #24019, stacked table branch `xw/convi-7777-assistance-usage` / #24020.
- Scope: integrate existing `enableConversationAssistanceUsageMetric`, default false. No flag definition, targeting, customer-config schema or generated frontend flag types.
- Verified definition: flux-deployments #349836 merged 2026-10-21 at `1f842dc1e7234de28af432ba32c0a043bcfebefc`, touching only common flag definitions. No deployment writes.
- Skill: `.claude/skills/director-rollout-flag/SKILL.md`. Its boolean-only example is stale; actual current `useRolloutFlag` returns `{ value: boolean, isResolved: boolean }`. User's exact destructuring is authoritative and matches source.

## Plan and invariants

Add shared feature gate in preparation, using `useRolloutFlag('enableConversationAssistanceUsageMetric', false)` and requiring both isResolved and value. Make query options disabled unless explicitly enabled. Then merge preparation forward into the stacked table branch without rewriting remote history and wire the same gate into unconditional query calls, data forwarding and column eligibility. Column definitions/configuration and CSV already consume the eligible-column set; remove the new field from the unconditional set. Disabled/unresolved must also withhold cached data so previously enabled metric-only rows disappear. Keep valid measured zero visible when enabled.

Verify disabled/unresolved/true requests, flag transitions with cached results, both Agent/Team column eligibility and exports, existing metrics, and underlying rollout-hook tests. Local overrides use URL parameter true/false, persist in localStorage, bypass targeting; clear overrides to test server rules and reload after server targeting changes because evaluations are cached.

## Execution and validation

Published preparation commit `505c55f288eb5e43a500cc06b5be5769beb4dc79` to existing [#24019](https://github.com/cresta/director/pull/24019). Shared helper uses the verified `{value, isResolved}` signature and returns their conjunction; query options default to disabled. No definition/schema/targeting changes.

Merged preparation into the column branch using `6cde1c4299`, then published integration commit `be6456af12591d1f86a89b4839848f159c4cb8de` to existing [#24020](https://github.com/cresta/director/pull/24020), still targeting preparation. Hooks remain unconditional, both Agent/Team query enabled options require resolved true, and disabled cached data is withheld. Column eligibility gates table and CSV. The generic selector excludes sticky columns, so a narrow leaderboard adapter exposes only this metric's existing sticky column as configurable when eligible, preserving all other sticky behavior.

Updated both PR descriptions and feature flag fields, preserving current QA checkbox selections. Column PR includes local override instructions and expected on/off behavior, localStorage clearing and server-cache reload caveats. No new PR or force push. Both PRs are currently ready for review (not draft); their existing state was preserved.

Validation under Node 24.11.1: 38 tests across query/gate (6), existing rollout hook (24), Agent/Team row/configuration/CSV (5) and real-query container glue (3); all pass. Covers disabled/unresolved suppression, enabling exactly two requests, valid zero usage and disabled cached-data withholding with unchanged legacy stats. Director-app tsc, production knip, diff checks and commit ESLint/i18n/Biome hooks pass. Worktree clean after push. No browser QA, deployed API or actual targeting proof is claimed.

CI snapshot: preparation Typecheck and feature-flags lint passed; lint/test jobs ongoing. [Production build job](https://github.com/cresta/director/actions/runs/39661181350/job/119435455750) failed its Receptionist bundle-size guard: +611.3 KB entry bundle against 200 KB threshold, total assets down 31.1 KB. Root cause is not established; no bundle override or unrelated product fix added. Column CI was starting. No green-CI claim.

Credentials: git and gh used their own authentication; no AWS, Okta, Azure or SSH credential/profile was manually selected or read.
