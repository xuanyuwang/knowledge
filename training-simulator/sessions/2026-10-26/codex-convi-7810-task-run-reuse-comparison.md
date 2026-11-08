# CONVI-7810: deeper task-run reuse evaluation

Date: 2026-10-26. Primary source repo: `/Users/xuanyu.wang/repos/go-servers`, branch `main`, inspected commit `aca4c13774`. Related sources: Director main `f47845f4f7`; proto worktree `/Users/xuanyu.wang/repos/cresta-proto-training-simulator-csv`, branch `xw/convi-7810-training-simulator-export-protos`, head `fd2e479c76`. Read-only evaluation of product code; no PR change or deployment verification.

## Question and result

Compare extending ListTrainingSimulatorTaskRuns to supply all FE CSV inputs through one paginated API versus orchestrating existing APIs without protobuf changes. Both are feasible at the inspected source level. Existing historical module lookup is implemented, so missing revision-fetch capability is not a reason to introduce a new RPC. User now explicitly allows pagination for this comparison.

## Actual table selection

- Sessions: `useTrainingSessions.ts` calls ListDirectorTasks with training task type, ACTIVE status, assignment-window time range, current usecase, resolved audience user names, and a task-level training-lesson filter. `TrainingSessions.tsx` then applies Created by me and search over session/lesson labels. Teams/groups resolve to current members in `useAudienceScope.ts`; no new group/team request fields are required for the current FE pattern.
- Audience selection keeps tasks containing any selected user. It does not trim each retained task's audience. Existing List task-run `agent_names` filters conversation attempts, so it is not an equivalent substitute. If explicitly filtering attempt owners, both agent_names and quiz_submitter_names are needed for the two run types, but that is different product behavior.
- Lessons: ListTrainingLessons is scoped to current usecase, with focus_criteria; FE searches lesson labels. Date is passed to the statistics API and affects eligible sessions, not which lesson definitions exist. Table can include archived lessons, whereas lesson stats currently restrict latest definitions to ACTIVE: table identity and metrics scope are distinct.
- Modules: ListTrainingModules is scoped to current usecase; FE searches names and applies content state visibility. Focus criteria is disabled on Modules. Date is passed only to statistics.
- Resource names from all filtered table rows encode search, creator, focus criteria, content state, and definition-usecase selection. Use the whole filtered table model, not its current visual pagination page. An empty selection must short-circuit to header-only CSV, never become an unfiltered list request.

## Option 1: extend List

For the existing table-driven workflow, keep director_task_names / training_lesson_names / training_module_names and pagination. Suggested additions (names illustrative, no tags assigned):

- time_range: task assignment overlap, create <= range end and due absent or due >= range start. Never filter run timestamps for this requirement.
- task_status: select ACTIVE for reporting while omitted retains existing List behavior.
- assigned_agents_only: validate each run's normalized trainee against that run's task audience, not a global union of agents from all selected tasks. Historical runs remain; no latest reduction or current module-membership restriction.
- include_reporting_context: opt into page-local label and historical-definition enrichment.

If List must independently reproduce session table filtering rather than receive the already filtered task names, additionally introduce task-level audience_user_names, usecase_names, and session_training_lesson_names (prefer a nested session filter). The last field must be distinguished from existing run-level training_lesson_names: a session containing L1 and L2 selected by the L1 tile can still export all that session's attempts under the session-export requirement. No focus_criteria/search/creator fields are necessary when FE supplies filtered resource identities.

Suggested response context: historical modules keyed by name + revision_id; historical lesson labels keyed by name + revision_id; session names/display labels; trainee names/display labels; exact quiz-template revisions and question definitions with no answer keys. Return metadata referenced by each page and FE merges it across pages. Add normalized attempt_start_time and attempt_complete_time to task-run output, or equivalent conversation timing summaries. Existing create_time/update_time describe the DB task-run record and must not be used as conversation start/end. Existing quiz_start_time/submit_time already cover quiz timings. Existing result fields otherwise cover recorded scores and criterion/question outcomes; quiz_score has no presence, so derive pending from submission state, not numeric zero alone, and verify incomplete-run behavior.

Backend actions: share eligible-task selection and per-task audience matching with reporting; apply new filters before keyset pagination; batch-enrich exact revisions and labels for each page; preserve omitted-field behavior; maintain role/resource access rules. FE iterates next_page_token, merges context by stable/revision identity, unions historical criteria/questions after all pages, and builds CSV. One API still means one HTTP/RPC invocation per page, not one response for the entire download.

## Option 2: no protobuf change

1. Reuse all filtered table resource names. Sessions already have task records. Lesson/module exports call ListDirectorTasks with training type, ACTIVE, assignment-window range; lesson selection may narrow tasks. Module export can list all eligible tasks then narrow actual runs by module names, avoiding reliance on current lesson-module membership.
2. ListTrainingSimulatorTaskRuns with eligible task names plus applicable run-level lesson/module names, consuming every next_page_token (default/max 1000). If no eligible tasks, stop instead of passing an empty task-name list. Keep each attempt once by run name.
3. Match normalized trainee identity (agent_name for conversation, submitter.name for quiz) against that run's task audience. Do not discard historical modules removed from current lesson membership or older retakes.
4. Batch ListTrainingModules with each exact revisioned module name from runs; ListTrainingLessons similarly for historical titles when desired. The module converter only returns a quiz-template name stub, so obtain questions with GetQuizTemplate per distinct revision, include_answer_key=false.
5. Resolve conversation trainee labels through existing user reads. Resolve conversation start/end through GetConversation or compatible ListConversations filtering by names (respect that API's required filters and access). Cache/dedupe by identity, limit concurrency, chunk large name lists for GET URL limits. Reuse loaded session labels.
6. Union criteria/questions across exact historical definitions; blank means absent from that attempt's revision, Pending means configured without result, N/A means recorded N/A. Do not silently substitute latest definitions when an exact revision is missing; report unavailable metadata.

Trade-off: minimal backend/proto work, more FE joins and calls, possible per-template/per-conversation fan-out and cross-call changes. Pagination and metadata reads do not promise an atomic snapshot; neither option should claim one without explicit snapshot design. Verify deployment/client versions before implementation; main proto checkout was older than the PR worktree/backend pagination contract.

## Evidence

- Director: hooks/training-simulator/useTrainingSessions.ts, useListTrainingLessons.ts, useListTrainingModules.ts; features/training-simulator/hooks/useTrainingSessionsFilters.tsx, useLessonConfigurationFilters.tsx; tabs/training-sessions/TrainingSessions.tsx, useAudienceScope.ts; tabs/lesson-configuration/LessonsTab.tsx, ModulesTab.tsx.
- Go: apiserver/internal/trainingsimulator/action_list_training_simulator_task_runs.go (identity filter families, keyset pages, score enrichment); action_list_training_modules.go (exact revision matching); action_get_quiz_template.go (revision-specific question loading); converter/converter.go (DB timestamps and quiz-template stub); action_retrieve_training_simulator_module_stats.go and action_retrieve_training_simulator_lesson_stats.go (active sessions/window and current-membership differences); apiserver/internal/coaching/action_list_director_task.go (audience selects tasks).
- Proto worktree: cresta/v1/trainingsimulator/training_simulator_service.proto lines 846-917 (existing request/response pagination); training_simulator_task_run.proto (results and revision IDs); cresta/v1/conversation/conversation.proto (start_time/end_time).

Next action: choose orchestration versus enriched List contract. No change to PR #10413 until the user chooses. Verification was source inspection only; no tests or live performance/deployment checks were needed or claimed. No credentials were used.
