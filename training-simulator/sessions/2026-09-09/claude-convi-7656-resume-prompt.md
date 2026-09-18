# Resume prompt — testing RetrieveTrainingSimulatorModuleStats (CONVI-7656)

Copy everything below the line into a fresh Claude Code session (run from `/Users/xuanyu.wang/repos`).

---

Continue the API testing work for Linear ticket [CONVI-7656](https://linear.app/cresta/issue/CONVI-7656/testing-api-retrievetrainingsimulatormodulestats-and). The full plan and progress tracker already live at `/Users/xuanyu.wang/repos/knowledge/training-simulator/sessions/2026-09-09/claude-convi-7656-module-stats-api-testing.md` and the work item at `/Users/xuanyu.wang/repos/knowledge/training-simulator/work-items/CONVI-7656.md` — read those first, then continue from the blocker.

## Goal
Black-box test the deployed `cresta.v1.trainingsimulator.TrainingSimulatorService.RetrieveTrainingSimulatorModuleStats` gRPC API (then `RetrieveTrainingSimulatorLessonStats`). Build request scenarios from real DB data, compute expected responses from the DB, call the API, and cross-check. Track progress in the session doc; when done/blocked/paused, sync plan + results to the Linear ticket.

## Target environment
- Cluster: `voice-staging` (kubectl context `voice-staging`, AWS profile `voice-staging_k8s_admin`).
- Customer `cresta`, profile `walter-dev` → request `parent` = `customers/cresta/profiles/walter-dev`.
- Profile config: `configv3/staging/cresta/walter-dev/config.yaml` (domain `walter-dev.voice-staging.cresta.ai`).
- gRPC endpoint: `grpc-cresta-api.voice-staging.internal.cresta.ai:443`.

## Auth (already solved — read carefully)
- Bearer token: `cresta-cli cresta-token voice-staging cresta --bearer` (cached, ~1h TTL, admin roles, VOICE scope). Use the raw JWT after stripping the `Bearer ` prefix only where noted.
- **Known gateway auth quirk:** direct gRPC through `grpc-cresta-api...:443` does NOT work. A fronting Go JWT gateway parses the raw `authorization` metadata without stripping `Bearer `: `Authorization: Bearer <token>` ⇒ `tokenstring should not contain 'bearer '`; raw token ⇒ gateway forwards it without the `Bearer ` prefix the apiserver's `shared-go/framework/server/auth` interceptor requires ⇒ `No valid credential is provided.`. REST on that host returns 415; the profile domain serves the SPA.
- **Working channel:** `kubectl --context voice-staging port-forward` to an apiserver pod, then `grpcurl -plaintext -H "Authorization: Bearer <token>" -d '{...}' localhost:<port> cresta.v1.trainingsimulator.TrainingSimulatorService/RetrieveTrainingSimulatorModuleStats`. The apiserver's own interceptor handles `Bearer <token>` correctly. (Use `-plaintext` since port-forward is local; reflection is available for message types.)

## First blocker to clear
The `voice-staging` AWS SSO session is expired — needed for both kubectl (port-forward) and the DB. Ask the user to run `aws sso login --profile voice-staging_k8s_admin` (or `cresta-cli startmyday`) before proceeding.

## DB access (for scenarios + expected values)
Use the `connect-customer-app-db` skill: `zsh /Users/xuanyu.wang/repos/ai-skills/connect-customer-app-db/scripts/connect-customer-app-db.zsh cresta --environment staging --profile walter-dev --query "<read-only SQL>"`. Read-only only. Tables (customer=`cresta`, profile=`walter-dev`): `training_modules`, `training_lessons`, `tasks`, `training_simulator_task_runs`, `training_simulator_conversation_scores`, `quiz_scores`, `quiz_questions`, `quiz_question_scores`.

## API contract (field numbers matter)
Request: `parent` (1, required); `training_module_names` (2, repeated, empty=discover ALL modules); `training_lesson_names` (4 — not 3 — repeated, non-empty narrows assignments to those lessons, empty=ignored); `time_range` (3, `cresta.v1.common.time.TimestampRange`, narrows tasks: `created_at <= end` AND (`dueTime` unset OR `dueTime >= start`)). Authz roles: ADMIN/SUPER_ADMIN/QA_ADMIN.
Response: repeated `TrainingSimulatorModuleStats` — `training_module_name`(1), `module_type`(8: CONVERSATION|QUIZ), `active_lesson_count`(2; profile-global, NOT narrowed by lesson scope), `average_applicable_score`(3; 0–1, meaningless when `applicable_score_count`=0), `applicable_score_count`(7), `passed_assignment_count`(4), `total_assignment_count`(5; includes never-started), `criteria`(6; conversation only), `question_stats`(9; quiz only, configured order).

## Handler logic reference
`apiserver/internal/trainingsimulator/action_retrieve_training_simulator_module_stats.go` on `go-servers` `origin/main` (lesson-scope fix in worktree `go-servers-convi-7638`, PR #32039). Key: latest revision per resource_id for modules/lessons; assignment keys = `(task, lesson, module, agent)` from active TS tasks' content `training_lesson_names` ∩ found lessons × audience `user_ids`; latest attempt per key by `created_at` tiebreak `resource_id`; conversation `scored` iff score non-null, quiz `scored` iff `submitted_at`+score valid and `passed = score >= passingScore/100` only if `hasPassingScore` (0 ⇒ everyone passes); `total_assignment_count` includes never-started; criteria/question_stats only from scored latest attempts; N/A criteria excluded from applicable.

## Scenarios to run (see session doc §3 for full list)
- Phase 1 validation: invalid parent, cross-profile module/lesson name, empty-string entries, malformed lesson name, inverted time_range → all `InvalidArgument`.
- Phase 2 happy paths (DB-driven): discover-all; single conversation module (verify total/passed/applicable/average + criteria counts); quiz module (question_stats order + correct/answered); module with no lessons/assignments (zero metrics); multi-attempt latest-only (older pass superseded by newer fail); `training_lesson_names` scope (out-of-scope excluded, `active_lesson_count` stays global — CONVI-7638); `time_range` narrowing; never-started agent; quiz `submitted_at` null; duplicate module names deduped.

## Workflow rules
- Repo workspace protocol: keep durable notes in `/Users/xuanyu.wang/repos/knowledge/training-simulator/` (update `sessions/.../claude-convi-7656-module-stats-api-testing.md`, `work-items/CONVI-7656.md`, `log/2026-09-09.md`, `project.yaml`). Don't commit unless asked.
- Never print/persist DB credentials (the skill keeps the URI in process memory only). Never remove `--read-only`.
- For each scenario, record in the session doc: request, API response, DB-derived expectation, verdict (✅/❌). When finished or paused, post a summary to Linear CONVI-7656.
