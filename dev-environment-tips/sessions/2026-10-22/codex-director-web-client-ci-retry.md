# Director web-client PR #23970 CI investigation and retry

Date: 2026-10-22 (America/Toronto)
Primary source repo: /Users/xuanyu.wang/repos/director
Branch/worktree: remote renovate/cresta-web-client-2.x at e9e40e566196053a79b70ee860eb82f6b6e0c4c0; local main checkout read-only.

## Evidence
- PR: https://github.com/cresta/director/pull/23970; @cresta/web-client 2.32.41 → 2.32.59.
- Failed job: https://github.com/cresta/director/actions/runs/39769661666/job/119793884683.
- Only failed job in that workflow: Build / Typecheck, TSC step. Lint, production build, unit test shards, Storybook, Director and Receptionist E2E passed.
- TypeScript TS2741 at `packages/director-app/src/features/ai-agent/testing-evaluation/shared/ui/LlmSettings/modelOptions.ts:20`: `MODEL_GPT_6_LUNA` missing from exhaustive `Record<TestCaseLLMSettingsModel, string>`.
- Dependency installation succeeded; peer dependency warnings are not the failing step.
- Live GitHub source comparison: failed head lacks `[TestCaseLLMSettingsModel.MODEL_GPT_6_LUNA]: 'gpt-6-luna'`, current main contains it.
- The mapping was added by merged PR https://github.com/cresta/director/pull/24000, merge commit 8b10f3e94e5f63a2188f1ccda5822c7c7ea7b83c, merged at 2026-10-22T11:29:56Z after failed typecheck at 11:21:04Z.
- PR is CONFLICTING/DIRTY and has its Renovate rebase/retry checkbox already checked. A plain rerun of the old revision would reproduce the deterministic type error; branch refresh is necessary.

## Action
Triggered the existing single-repository Renovate workflow with repos=cresta/director (default configuration), to process the requested rebase and pick up main's mapping fix:
https://github.com/cresta/action-renovatebot-execute/actions/runs/39787151648

No source code edited, issue/comment posted, or merge requested. Authentication: gh's existing self-managed authentication; no AWS/Okta/Azure/SSH credential selected or read.

## Validation
Renovate refreshed PR head to 4670c2787c8ac936b91f7e4fdffb8ba3f5b4c71e. GitHub now reports MERGEABLE rather than CONFLICTING. Verified the new head contains the missing MODEL_GPT_6_LUNA mapping at modelOptions.ts:31. Fresh Build and Release Frontend CI started: https://github.com/cresta/director/actions/runs/39787511627. Typecheck queued; passing result not yet established.

## Latest observed state
Renovate advanced the same PR again to @cresta/web-client 2.32.60 at head 525d36787577878f5a51104bd75b07ee0a0be316. Verified the required MODEL_GPT_6_LUNA mapping remains present. PR remains MERGEABLE. Latest CI is https://github.com/cresta/director/actions/runs/39787987966; typecheck pending. Earlier refreshed-head CI run 39787511627 is superseded for final validation. The manual Renovate workflow itself is still in progress. No passing CI or merge-readiness claim is made.
