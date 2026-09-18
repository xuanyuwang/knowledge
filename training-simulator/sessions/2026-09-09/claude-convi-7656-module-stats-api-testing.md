# CONVI-7656 — Testing RetrieveTrainingSimulatorModuleStats API

Session: 2026-09-09 · Claude/Codex · Source repo: `go-servers` (module handler on `origin/main`; lesson handler on `origin/claude/convi-7600-lesson-stats`), `cresta-proto` (contract on `main`).
Target: voice-staging, customer `cresta`, profile `walter-dev`.
Ticket: [CONVI-7656](https://linear.app/cresta/issue/CONVI-7656/testing-api-retrievetrainingsimulatormodulestats-and). Work item: `work-items/CONVI-7656.md`.

## 1. API under test

`cresta.v1.trainingsimulator.TrainingSimulatorService.RetrieveTrainingSimulatorModuleStats`

Request (`RetrieveTrainingSimulatorModuleStatsRequest`):
- `parent` (1, required): `customers/{customer_id}/profiles/{profile_id}`
- `training_module_names` (2, optional, repeated): `.../trainingModules/{id}`. Empty = discover ALL modules in the profile.
- `training_lesson_names` (4, optional, repeated): `.../trainingLessons/{id}`. Non-empty narrows assignments to those lessons; empty = ignored. (Note: field number 4, not 3.)
- `time_range` (3, optional): `cresta.v1.common.time.TimestampRange`. Narrows tasks by `created_at <= end` AND (`dueTime` unset OR `dueTime >= start`).

Response (`RetrieveTrainingSimulatorModuleStatsResponse`): repeated `TrainingSimulatorModuleStats`:
- `training_module_name` (1), `module_type` (8: CONVERSATION|QUIZ), `active_lesson_count` (2; profile-global, NOT narrowed by lesson scope), `average_applicable_score` (3; 0–1, meaningless when `applicable_score_count`=0), `applicable_score_count` (7), `passed_assignment_count` (4), `total_assignment_count` (5; includes never-started), `criteria` (6; conversation only), `question_stats` (9; quiz only, configured order).

Authz: roles ADMIN, SUPER_ADMIN, QA_ADMIN.

## 2. Handler logic (for building expected values)

File: `apiserver/internal/trainingsimulator/action_retrieve_training_simulator_module_stats.go` (on `origin/main`; lesson-scope fix in worktree `go-servers-convi-7638` PR #32039).

1. `parseModuleStatsRequest`: validate parent, module names (non-empty, parse, match parent profile), lesson names (same), time range (start<end). Empty module list ⇒ discover all. Empty lesson list ⇒ unscoped.
2. `findModuleStatsModuleMetadata`: latest revision of each module (self-join `MAX(created_at) per resource_id`). Type = QUIZ if `quiz_template_id` set, else CONVERSATION. Capture `evaluation_config.passing_score` (hasPassingScore) and quiz questions (ordered by ordinal, question_id).
3. `findActiveLessonCounts`: latest revision of `training_lessons` with `state=ACTIVE`; count per module via `training_module_ids` overlap. NOT narrowed by lesson scope.
4. `findModuleAssignmentKeys`:
   - `findLessonsContainingModules`: latest revision lessons whose `training_module_ids && requestedModuleIDs`; if lesson scope set, also `resource_id IN lessonIDs`.
   - `findActiveModuleStatsDirectorTasks`: `tasks` where `task_type=TRAINING_SIMULATOR`, `task_status=ACTIVE`, time-range filtered.
   - `expandModuleAssignmentKeys`: for each task, intersect content-config `training_lesson_names` with found lessons; for each requested module in that lesson × each audience `user_ids` → key `(task, lesson, module, agent)`. Tasks with no audience or no matching lesson skipped; malformed lesson names silently skipped.
5. `findLatestModuleAttemptResults`: load `training_simulator_task_runs` for the task/module sets; join `training_simulator_conversation_scores` (agent from `agent_user_id`) and `quiz_scores` (agent from `submitter_user_id`, only if `submitted_at` valid). Group by key, pick latest run by `created_at` tie-broken by `resource_id`.
   - Conversation result: `scored` iff `score` non-null; `passed` from `passed`; criteria from `criterion_results` JSONB.
   - Quiz result: `scored` iff `score` non-null; `passed = score >= passingScore/100` only if `hasPassingScore` (passing score 0 ⇒ everyone passes). `submitted_at` null ⇒ not scored.
   - No run for a key ⇒ `hasAttempt=false` (counts toward `total_assignment_count` only).
6. `aggregateModuleStats`: per module: `total_assignment_count = len(facts)`; `passed_assignment_count` and `applicable_score_count` counted only over scored latest attempts; `average = scoreTotal/applicableScoreCount` when >0. Criteria: only scored attempts contribute; `not_applicable` ⇒ `non_applicable_result_count` and excluded from applicable. Quiz `question_stats`: only scored latest attempts; per-question results for question_ids not in the current template dropped; unanswered configured questions appear with zero counts.

### DB tables (app DB, customer=`cresta`, profile=`walter-dev`)
`training_modules`, `training_lessons`, `tasks`, `training_simulator_task_runs`, `training_simulator_conversation_scores`, `quiz_scores`, `quiz_questions`, `quiz_question_scores`.

## 3. Test plan / scenarios

Each scenario: build inputs + expected response from DB, call API, compare. Mark ☐ todo / 🟡 in progress / ✅ pass / ❌ fail / ⛔ blocked.

### Phase 0 — Access & deployment
- ✅ P0.1 Confirm API deployed: gRPC reflection lists `RetrieveTrainingSimulatorModuleStats` (and `...LessonStats`).
- ✅ P0.2 Obtain bearer token via `cresta-cli cresta-token voice-staging cresta --bearer`.
- ✅ P0.3 Establish a working call channel. On retry, direct `grpcurl` to `grpc-cresta-api...:443` with `Authorization: Bearer <token>` succeeded; no port-forward is needed. The earlier gateway-auth diagnosis is stale for the currently deployed route. `voice-staging_k8s_ro` and `voice-staging_k8s_contributor` both deny `pods/portforward`, but that no longer blocks testing.
- ✅ P0.4 Connect read-only to `walter-dev` app DB via `connect-customer-app-db` skill. Resolved `origin/master:configv3/staging/cresta/walter-dev/config.yaml`, AWS account/cluster `voice-staging`, database `walter-dev`, schema `director`.

### Phase 1 — Validation / error cases (no DB needed beyond a valid parent)
- ✅ P1.1 Invalid parent format → `InvalidArgument`.
- ✅ P1.2 Module name that parses but mismatches parent profile → `InvalidArgument`.
- ✅ P1.2b Lesson name that parses but mismatches parent profile → `InvalidArgument`.
- ✅ P1.3 Empty string in `training_module_names` → `InvalidArgument`.
- ✅ P1.4 Empty string in `training_lesson_names` → `InvalidArgument`.
- ✅ P1.5 Malformed lesson name → `InvalidArgument`.
- ✅ P1.6 `time_range` start after end → `InvalidArgument`.

### Phase 2 — Happy paths (DB-driven)
- ✅ P2.1 Empty `training_module_names` → discovers ALL modules in profile; each returned with identity + `active_lesson_count` (DB: count ACTIVE latest-revision lessons containing it).
- ✅ P2.2 Single conversation module with assignments → verify `total_assignment_count` = distinct `(task,lesson,module,agent)` keys from active TS tasks; `passed_assignment_count`, `applicable_score_count`, `average_applicable_score` from latest attempts only; `criteria` counts (passed/applicable/non_applicable) match latest-attempt criterion results.
- ✅ P2.3 Quiz module → `module_type=QUIZ`, `question_stats` in configured template order; `correct/answered` from latest scored quiz attempts; unanswered configured questions appear with zeros.
- ✅ P2.4 Module with NO lessons / NO active assignments → returned with identity, `active_lesson_count` per DB, all metrics zero, empty criteria/question_stats.
- ✅ P2.5 Multi-attempt for one `(task,lesson,module,agent)`: only the latest run (by `created_at`, tiebreak `resource_id`) contributes; verified an older pass superseded by a newer fail.
- ✅ P2.6 `training_lesson_names` scope (CONVI-7638): narrow to one lesson; assignments from out-of-scope lessons under the same task excluded; `active_lesson_count` stays profile-global (NOT narrowed).
- ✅ P2.7 `time_range`: only tasks with `created_at <= end` and (`dueTime` null or `dueTime >= start`) contribute.
- ✅ P2.8 Never-started agent: a key with no run counts in `total_assignment_count` but not in passed/applicable.
- ✅ P2.10 Duplicate module names in request ⇒ deduped to one stats entry.

### Phase 3 — RetrieveTrainingSimulatorLessonStats

- ✅ L1.1 Invalid parent format → `InvalidArgument` (gateway/request validation).
- ✅ L1.2 Empty and malformed lesson names → `InvalidArgument` (gateway/request validation).
- ✅ L1.3 Cross-profile lesson, duplicate lesson, and inverted time range → `InvalidArgument` from the deployed handler.
- ✅ L2.1 Empty `training_lesson_names` discovers all 30 latest ACTIVE lessons; API and DB match on the full aggregate digest.
- ✅ L2.2 Single-module lesson aggregate: sessions, total/applicable/passed assignments, and average score match independently expanded task audiences and latest attempts.
- ✅ L2.3 Multi-module completeness: only assignments whose latest result for every current module is scored contribute to applicable/average/pass.
- ✅ L2.4 Active lesson with no matching active assignments returns one zero/default stats entry.
- ✅ L2.5 `time_range` task-window narrowing matches `created_at <= end` and (`dueTime` unset or `dueTime >= start`).
- ✅ L2.6 Latest retake changes completeness/outcome using only the newest module attempt, including a newest unscored conversation retake that makes the assignment incomplete.
- ✅ L2.7 Quiz-containing lesson derives pass from current module passing score.

## 4. Progress log

- 2026-09-09 (start) — Read proto + handler; confirmed deployment via reflection; got token; diagnosed gateway auth quirk; resolved `walter-dev` DB profile. Blocked on voice-staging AWS SSO expiry for port-forward + DB.
- 2026-09-09 (resume) — AWS SSO refreshed. Read-only Kubernetes access works but both cleared non-admin roles deny port-forward. Retried direct `grpcurl`; it now works with `Authorization: Bearer <token>`, so the prior gateway blocker is stale. Connected read-only to the app DB and completed all feasible module scenarios. Lesson valid requests return `Unimplemented`, so lesson happy paths are deployment-blocked.
- 2026-09-09 (lesson deployment retry) — A valid `RetrieveTrainingSimulatorLessonStats` request now succeeds, confirming the handler is deployed. DB-backed validation, full discover-all aggregation, single/multi-module completeness, zero assignments, time-range narrowing, quiz pass derivation, and latest-retake handling all passed.

## 5. Results

Bearer and DB credentials were held only in process memory and were not recorded.

| Scenario | Request / candidate | DB-derived expectation | API response | Verdict |
|---|---|---|---|---|
| P1.1–P1.6 + P1.2b | Invalid parent; cross-profile module/lesson; empty module/lesson; malformed lesson; inverted range | All rejected before DB aggregation | All returned `InvalidArgument`; wording differs for empty names because deployed validation reports the resource-name format directly | ✅ |
| P2.1 discover all | `{parent}` only | 41 latest module IDs; type from `quiz_template_id`; global ACTIVE latest-lesson count per module. Canonical DB digest `aec1efb5eb1f88a6515bf47bee83d644` | 41 entries; same canonical digest over `(module_id, active_lesson_count, module_type)` | ✅ |
| P2.2 conversation | Module `019f3897-eb3d-7a0f-8b21-18e6867c14c6` | Active lessons 1; assignments 25; scored 7; passed 5; average `5/7 = 0.7142857142857143`; criterion `019f3896...`: passed 5, applicable 7 | Exact match | ✅ |
| P2.3 quiz | Module `019ff4a3-23d5-7713-beeb-c21d86ecd6ec` | Active lessons 2; assignments 9; scored/passed 6; average `0.8888888888888888`; ordered questions: `(6/6), (5/6), (5/6)` correct/answered | Exact match, including configured order | ✅ |
| P2.4 + P2.10 zero/dedupe | Module `019ff118-5a2a-749c-8eef-506810d56701` supplied twice | Latest module exists; zero active lessons and assignments; one deduped response | One conversation entry with zero/default metrics | ✅ |
| P2.5 latest only | Conversation key `(task 01a01f54..., lesson 019f3898..., module 019f3897..., agent 95e8bc...)` | Older run `01a01f56...` score 1/pass; newer run `01a01f59...` score 0/fail; aggregate must use newer only | Conversation aggregate matches latest-only DB calculation | ✅ |
| P2.6 lesson scope | Module `019eb458...`, lesson `019edc41...` | Scope: 3 assignments, all scored, 0 passed, average 0; criterion `019edbfb...` applicable 3; global active-lesson count remains 5 | Exact match; out-of-scope assignments removed and active count remains 5 | ✅ |
| P2.7 time range | Module `019f3897...`; start `2026-08-15T00:00:00Z`, end `2026-08-19T00:00:00Z` | 22 assignments, 4 scored, 3 passed, average 0.75; criterion applicable 4/passed 3 | Exact match | ✅ |
| P2.8 never started | Same conversation module | 25 assignments include 18 keys with no scored latest result (including keys with no run); only 7 applicable | Exact total/applicable separation | ✅ |
| L1 validation/deployment | Cross-profile lesson; duplicate lesson; inverted time range; valid explicit lesson | Invalid requests rejected; valid request reaches aggregation | Invalid requests return `InvalidArgument`; valid request returns stats | ✅ |
| L2.1 discover all | `{parent}` only | 30 latest ACTIVE lessons; canonical digest over name + session/average/applicable/passed/total (average rounded to 12 decimals) = `6fe7b93e6ccb3eaa2c116c31ce9ad7c5` | 30 entries; same full aggregate digest | ✅ |
| L2.2 single module | Lesson `019eb459-7160-7a24-9111-59a9675b3b1d` | 15 sessions; 17 assignments; 9 applicable; 2 passed; average `2/9 = 0.2222222222222222` | Exact match | ✅ |
| L2.3 multi-module completeness | Lessons `019ee161-deca-710a-9073-e45f80ab5678` (3 modules) and `019ebce1-f635-7431-bcb5-d8b531235c79` (2 modules) | First: 3 sessions/assignments, 0 complete/applicable. Second: 5 sessions, 6 assignments, 1 applicable, 0 passed, average `0.5` | Exact match | ✅ |
| L2.4 zero assignments | Active 3-module lesson `019f18ac-8c44-7726-85cb-1df372271cf5` | Zero sessions/assignments/applicable/passed; zero/default average | Exact match | ✅ |
| L2.5 time range | Lesson `019eb459...`; start `2026-08-15T00:00:00Z`, end `2026-08-19T00:00:00Z` | 4 sessions; 5 assignments; 3 applicable; 0 passed; average 0 | Exact match | ✅ |
| L2.6 latest retake | Lesson `019eb6c7...`, task `019eb801...`; time range isolates the task | Audience has 7 assignments. Two agents have attempts, but both newest attempts are unscored; 9 historical scored attempts include score 1. Expected: 1 session, 7 total, 0 applicable/passed | Exact match; older scores do not keep the lesson complete | ✅ |
| L2.7 quiz lessons | Lessons `019ffe3d...` (2 quiz modules) and `01a0355e...` (1 quiz module) | First: 6 sessions/assignments, 4 applicable, 3 passed, average `0.8020833333333333`. Second: 2 sessions/assignments/applicable, 1 passed, average `0.75` | Exact match | ✅ |

## 6. Next actions / blockers

- Lesson stats conclusion: all planned black-box scenarios passed against independent read-only DB expectations.
- Module stats conclusion: all feasible black-box scenarios passed against independent read-only DB expectations.
