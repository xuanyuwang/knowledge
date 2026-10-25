# CONVI-7777 rollout flag versus deployment config

## Request and outcome

Investigated why `enableConversationAssistanceUsageMetric` is absent from Walter-dev Director's `/director/system-config/deployment-config/config` page. Confirmed this is expected: the new rollout system is separate from static customer config.

## Evidence

- Glean retrieved [Feature Flags & Configuration](https://docs.superhuman.com/d/_dpaeiSbktTF/_suf10KmR), updated October 22, 2026. Its Director section explicitly distinguishes `useFeatureFlag` (static `featuresConfig.featureFlags`) from `useRolloutFlag` (async server evaluation via flagd). Definitions and stage overrides live in flux-deployments, with Flux/flagd propagation and no application redeploy required for flag changes.
- [Engineering announcement](https://crestalabs.slack.com/archives/C08M88ES630/p1783697010254599) explicitly describes rollout definitions as separate from customer config.
- Glean's [evaluation reference](https://github.com/cresta/claude-code-marketplace/blob/main/plugins/flux-deployments/skills/rollout-flag/references/evaluate.md) says GetRolloutFlag is a standalone RPC and does not overlay its values onto the Features response.
- Local Director `packages/director-app/src/features/config/deployment-config/config/DeploymentConfig.tsx` maps only `currentConfig?.featuresConfig?.featureFlags` into the flag display.
- Local Director `packages/director-app/src/hooks/useRolloutFlag.ts` calls `CrestaAPI.config.getRolloutFlag`, returns a local override first, and configures infinite stale time with no refetch on window focus. Reload is appropriate after changing targeting. A successful boolean result is read from `boolValue` when `usedDefault` is not true.
- In the CONVI-7777 Director worktree, `useConversationAssistanceUsageMetric.ts` calls `useRolloutFlag('enableConversationAssistanceUsageMetric', false)` and requires `isResolved && value`.
- Earlier in this chat the Swiftly page selected `00-head`, displayed no targeting rules, and showed environment default `on` with no pending editor changes. This is displayed configuration, not a deployed runtime evaluation check.

## Limits and next verification

Did not inspect Walter-dev's live network traffic or independently establish which stage its backend uses. Missing from deployment config does not establish whether the rollout flag evaluates true or false. For runtime proof, reload Director without a local override, inspect the `getRolloutFlag` network call for this exact flag, and check `boolValue: true` with `usedDefault` false or absent. A missing request may indicate the consuming code is not loaded or a local override is in use; it is not by itself proof the rollout is off.

No product code or external configuration changed. No AWS, Okta, Azure, or SSH credentials were selected or read; Glean used its existing authenticated connection.
