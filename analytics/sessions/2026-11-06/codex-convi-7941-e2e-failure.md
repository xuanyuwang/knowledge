# CONVI-7941: unrelated E2E trace-button failure

Primary source repo: Director. Worktree `/Users/xuanyu.wang/repos/director-convi-7941-export-errors`, branch `xw/convi-7941-export-errors`, HEAD `0d6a4637368fbd112a001844d569140613a8d272`. User asked whether the failing E2E job is related to our changes. Investigation read-only; no code, PR metadata or external messages changed.

## Evidence

- PR #24681 head remains `0d6a463736`. GitHub diff has exactly the progression handler and new Vitest export test; no transcript, tracing, testkit or dependency changes. Typecheck, lint, all three unit-test shards and production build passed.
- Failed job: https://github.com/cresta/director/actions/runs/42152734757/job/127681052688 . Only failed test is `closed-conversation-transcript-elements.spec.ts`, both original and retry. Messages and action wrapper load; trace button is absent at lines 36/37/43, click times out at 44.
- Downloaded `e2e-test-results-director-testkit-1_2` artifact to `/tmp/convi-7941-e2e-artifacts`; inspected error-context and network records inside real Playwright trace ZIP (no live environment changes).
- Mock moment-annotation response HTTP 200 contains one AI_AGENT_TURN with `tracingId=mock-trace-id-12345`, no `crestaTraceId`.
- Recorded rollout RPC body has flag `enableSharedLangfuseProject`; response `boolValue=true, usedDefault=false`. Recorded customer config has nonempty `deploymentConfig.langfuseProjectIds.shared`. Only relevant values/keys were reported, no credentials printed.
- `useCustomerProfileDeploymentConfig.ts` selects shared mode when both rollout and project are present. `useSelectTraceId.ts:38-39` in this mode selects only `crestaTraceId`, returning undefined for the legacy-only mock. `ConversationTranscriptMessage.tsx:572-583` therefore has no traceId/showTraceIcon, explaining the missing button exactly.
- The failed test never navigates to Performance progression or clicks export; the changed handler is never invoked. New Vitest mocks do not run in separate Playwright test execution.
- Baseline: same transcript-elements test passed on main in run 42136014457, job 127626180251, and also in main run 42129777896, job 127605154028 (that run failed on unrelated org change-log test). This is not evidence of an existing identical main failure; source/recorded config proves the mock mismatch.

## Conclusion

Failure unrelated to await-both ZIP repair. The test relies on live tracing configuration while mocking legacy trace data. A separate test repair should pin the shared rollout off when exercising legacy FetchAIAgentTurnTrace, or supply unified IDs and mock the unified tracing endpoint for shared mode. Do not change production trace selection just to satisfy this fixture. No rerun initiated because recorded config explains the deterministic failure; no claim that simply rerunning will fix it. No explicit AWS/Okta/Azure/SSH credentials used; gh used existing authentication. Worktree remains clean.
