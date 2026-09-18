# CONVI-7656: Testing API RetrieveTrainingSimulatorModuleStats and RetrieveTrainingSimulatorLessonStats

**Status:** complete (module and lesson APIs validated on voice-staging)
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7656](https://linear.app/cresta/issue/CONVI-7656/testing-api-retrievetrainingsimulatormodulestats-and)
**Last updated:** 2026-09-09 (module and lesson validation complete)

## Objective and Impact

- **Objective:** Black-box test the deployed `RetrieveTrainingSimulatorModuleStats` (and then `RetrieveTrainingSimulatorLessonStats`) gRPC API on voice-staging, customer `cresta`, profile `walter-dev`. Design request scenarios from real DB data, compute expected responses from the DB, call the API, and cross-check.
- **Customer/system impact:** Validates the merged module/lesson stats reporting contract (CONVI-7601/CONVI-7600) and the lesson-scope fix (CONVI-7638) against live data before Director wiring.
- **Role:** tested

## Scope

**In scope**

- Verify the API is deployed on voice-staging apiserver.
- Inspect the `walter-dev` app DB to build scenarios: modules with no lessons, conversation vs quiz modules, lessons with/without assignments, multi-attempt latest-only selection, time-range narrowing, lesson-scope narrowing, validation errors.
- Call the gRPC API per scenario and cross-check against DB-derived expected values.
- Track plan + progress in the session doc and sync to the Linear ticket on pause/complete.

**Non-goals**

- Director frontend integration testing.
- `RetrieveTrainingSimulatorTaskStats` (already shipped/validated in CONVI-7583).
- Performance/load testing.

## Source Context

- **Repos:** `go-servers` (handler on `origin/main`: `apiserver/internal/trainingsimulator/action_retrieve_training_simulator_module_stats.go`), `cresta-proto` (contract on `main`: `cresta/v1/trainingsimulator/training_simulator_service.proto`, `module_stats.proto`).
- **Worktrees:** none for this testing work; reading existing `go-servers-convi-7638` worktree for handler reference.
- **Target environment:** voice-staging, customer `cresta`, profile `walter-dev` (config `configv3/staging/cresta/walter-dev/config.yaml`, domain `walter-dev.voice-staging.cresta.ai`, k8s cluster `voice-staging`).
- **gRPC endpoint:** `grpc-cresta-api.voice-staging.internal.cresta.ai:443` (fronted by a JWT gateway — see Current Understanding).

## Current Understanding

Direct `grpcurl` on `grpc-cresta-api.voice-staging.internal.cresta.ai:443` now succeeds with `Authorization: Bearer <token>`. The earlier gateway-auth failure could not be reproduced after AWS SSO refresh and is stale for the current deployment. Kubernetes port-forward is unnecessary (and both permitted non-admin Kubernetes roles deny `pods/portforward`).

The module API is black-box validated against independent read-only DB calculations. All planned cases passed: validation errors, all-41-module discovery identity/type/active-lesson counts, conversation and quiz aggregates, configured quiz-question ordering, zero assignments, duplicate-name dedupe, latest-only attempts, lesson scoping with global active count, time-range narrowing, and never-started assignees.

The lesson RPC is now deployed and black-box validated. DB-backed coverage passes for handler validation, full all-lesson aggregate discovery, single- and multi-module completeness, zero assignments, time-range narrowing, newest-attempt selection (including a newest unscored retake making an assignment incomplete), and quiz pass derivation.

## Findings and Decisions

- Module handler deployment and behavior confirmed by successful authenticated calls, not reflection alone.
- DB profile resolved read-only: `walter-dev` on cluster `voice-staging` (config `configv3/staging/cresta/walter-dev/config.yaml`, schema `director`).
- Discover-all returned 41 modules and matched the DB canonical digest over identity, module type, and global active-lesson count.
- Selected conversation module `019f3897...` matched 25 total / 7 applicable / 5 passed / average `5/7`; selected quiz module `019ff4a3...` matched 9 total / 6 applicable+passed / average `0.888888...` and question counts `(6/6, 5/6, 5/6)` in configured order.
- CONVI-7638 scope behavior passed: lesson `019edc41...` narrowed module `019eb458...` to 3 assignments while retaining global active-lesson count 5.
- Lesson deployment was confirmed by a successful valid request, not reflection alone.
- Lesson `019eb459...` matched 15 sessions / 17 assignments / 9 applicable / 2 passed / average `2/9`.
- Multi-module lessons matched completeness-first aggregation: one 3-module lesson had 3 assignments and none complete; one 2-module lesson had 6 assignments with exactly 1 complete at average `0.5`.
- Discover-all returned all 30 latest ACTIVE lessons and matched a canonical full-aggregate digest over identity and all numeric fields (`6fe7b93e6ccb3eaa2c116c31ce9ad7c5`, average rounded to 12 decimals).
- Quiz lessons matched current-threshold pass derivation, including a 2-module lesson at 6 total / 4 complete / 3 passed / average `0.8020833333333333`.
- An isolated single-module task with 7 assignees had 9 historical scored attempts (best score 1), but both agents' latest attempts were unscored; the API correctly returned total 7 with zero applicable/passed.

## Blockers and Dependencies

- None.

## Validation and Rollout

- Module API: all feasible planned scenarios passed; full evidence is in the session doc.
- Lesson API: all planned validation and DB-backed scenarios pass.

## Next Actions

1. Investigate only if later staging data or Director integration exposes a mismatch.

## Timeline

- 2026-09-09 — Confirmed API deployed via reflection; obtained bearer token; diagnosed the gRPC gateway auth-layering quirk; resolved the `walter-dev` DB profile. BLOCKED on voice-staging AWS SSO refresh. Evidence: `sessions/2026-09-09/claude-convi-7656-module-stats-api-testing.md`.
- 2026-09-09 — SSO restored; direct authenticated `grpcurl` works. Completed all feasible module API scenarios against read-only DB expectations with no mismatches. Lesson valid requests return `Unimplemented`; paused pending CONVI-7600 deployment. Evidence: `sessions/2026-09-09/claude-convi-7656-module-stats-api-testing.md`.
- 2026-09-09 — Lesson handler deployment confirmed with a valid request. Initial DB-backed lesson tests passed: validation, 30-lesson discovery identity, single/multi-module completeness, zero assignments, and time-range narrowing. Evidence: `sessions/2026-09-09/claude-convi-7656-module-stats-api-testing.md`.
- 2026-09-09 — Completed lesson testing. Full 30-lesson aggregate digest, quiz pass derivation, and latest-retake/incomplete-retake scenarios also matched the DB. No remaining blocker. Evidence: `sessions/2026-09-09/claude-convi-7656-module-stats-api-testing.md`.
- 2026-09-09 — Replaced the Linear ticket's main description with the completed module- and lesson-validation summary.
