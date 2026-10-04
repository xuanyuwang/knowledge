# CONVI-7785 template picker investigation and implementation

Date: 2026-10-02 (America/Toronto)
Source repo: `/Users/xuanyu.wang/repos/director`
Branch/worktree: `xw/convi-7785-exclude-simulator-rules` in `/Users/xuanyu.wang/repos/director-convi-7785`

## Inputs and source trace

- Linear ticket CONVI-7785 asks to check Performance Configuration → Convo template creation → Add criterion and exclude Training Simulator-only rules if present.
- `ScorecardTemplateBuilder` → `TemplateBuilderFormConfigurationStep` → `TemplateBuilderAutoQA` → `getTriggerOptions` is the relevant authoring path.
- `useAutoQATriggers` loads `RetrieveBehaviors` and `ListPolicies`. Behavior has no source type in the API/model; Policy has `sourceType` and `behaviors`.
- Before this change, `getTriggerOptions` used policy ownership for role visibility and links but displayed all returned behaviors, including those owned by `SOURCE_TYPE_TRAINING_SIMULATOR` policies.

## Change

- `getTriggerOptions` builds a set of behavior resource names owned by Training Simulator-only policies and omits them from conversation template trigger choices.
- The template picker requests archived policies as well, covering the existing admin path that can expose archived behaviors. It waits for both API results before displaying choices; the shared hook's default behavior stays as before for other consumers.
- No backend or proto change is required for this picker because the current policy result already carries the source type and behavior association.

## Verification

- `yarn install --immutable` completed to restore missing local dependencies without changing tracked package files.
- Focused Vitest passed (1 test). Test data covers normal, Simulator-only, archived Simulator-only, and legacy behaviors.
- `yarn tsc` in `packages/director-app` passed.
- Targeted `yarn biome check`, targeted `yarn eslint`, and `git diff --check` passed.
- No browser, staging, or deployment validation performed.

## Follow-up

- Review/commit/push/PR and verify the picker in a profile with a Simulator-only rule.
- On 2026-10-02, moved the branch and all four uncommitted ticket files to `/Users/xuanyu.wang/repos/director-convi-7785`. The main `/Users/xuanyu.wang/repos/director` checkout is clean on `main`.
