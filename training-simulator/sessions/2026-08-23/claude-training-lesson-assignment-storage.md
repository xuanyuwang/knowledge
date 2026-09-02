# How a Training Lesson Assignment Is Stored in the DB

Date: 2026-08-23
Scope: narrow factual answer tying `knowledge/training-simulator` README §6 to the `go-servers` SQL schema.

## TL;DR

A training-lesson **assignment is not stored in a training-specific table**. It is a generic `director.tasks` row (a "DirectorTask") whose `task_type` marks it as a training-simulator task and whose JSONB config columns carry the audience, content (lesson), and schedule. Per-agent, per-module **attempts** are the separate `director.training_simulator_task_runs` rows.

## The assignment row: `director.tasks`

DDL: `go-servers/apiserver/sql-schema/director/director-schema.sql:286`.

| Column | Role for a training assignment |
|---|---|
| `task_id` (PK with `customer_id`,`profile_id`) | The assignment's stable resource id |
| `task_type` | `DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR` |
| `task_status` | Administrative lifecycle; created as `ACTIVE` |
| `task_display_name` | Supervisor-defined session title |
| `usecase_id` | Use-case scope for content/listing |
| `task_audience_config` (JSONB) | `training_simulator_audience_config.user_names` — explicit target-agent resource names (Alice, Bob, Carol). Must not be mixed with generic task `user_names`/`group_names`. |
| `task_content_config` (JSONB) | `training_simulator_content_config.training_lesson_names` — repeated in proto, but current product/runtime use the first/one lesson. |
| `task_schedule_config` (JSONB) | `training_simulator_schedule_config.due_time` — deadline, saved end-of-day in the current Director flow. |
| `task_complete_config` (JSONB) | Completion semantics config |
| `created_at` | Assignment creation/start time used by display and time-window logic |

Key invariants from README §6:
- **One DirectorTask per assignment, shared by the whole audience.** There is no per-agent task row. The agent-facing "assigned session" is a view of the same task filtered to one assignee + joined to that agent's attempts.
- **One lesson per task** is the current product invariant even though the proto allows repeated lesson names.
- Audience mutation is a full replacement (no field mask). Removing an agent rewrites the shared audience list; removing the last member archives the task. Existing task runs survive as historical evidence.

## The attempt rows: `director.training_simulator_task_runs`

DDL: `director-schema.sql:315`. PK = `(customer_id, profile_id, resource_id)`. Index `ix_training_sim_task_runs_task_lesson_module` on `(customer_id, profile_id, director_task_id, training_lesson_id, training_module_id)`.

| Column | Meaning |
|---|---|
| `director_task_id` | FK back to the assignment (`director.tasks`) |
| `training_lesson_id` / `training_lesson_revision_id` | Pinned lesson + **revision** (history-preserving) |
| `training_module_id` / `training_module_revision_id` | Pinned module + revision |
| `conversation_score_id` | Outcome link for a scenario/conversation attempt (nullable) |
| `quiz_score_id` | Outcome link for a quiz module attempt (nullable) |

Each module attempt creates one task run linking `task + agent + lesson revision + module revision + scenario/quiz outcome`. Audience members never share task runs; retries by one agent create multiple task runs under the same `(task, module, agent)` identity. Official reporting picks the latest attempt per `(DirectorTask, lesson, module, agent)`.

## Content tables (what the assignment references, not the assignment itself)

- `director.training_lessons` (`schema.sql:5`) — revisioned; `training_module_ids VARCHAR[]`, `focus_criteria JSONB`.
- `director.training_modules` (`schema.sql:29`) — revisioned; `training_scenario_ids VARCHAR[]`, `evaluation_config JSONB`, optional `quiz_template_id/revision_id`.
- `director.training_scenarios` (`schema.sql:54`).
- `director.training_simulator_conversation_scores` (`schema.sql:418`) — per-conversation evaluation result (score, passed, criterion_results).
- `director.quiz_templates` / `quiz_questions` / `quiz_scores` / `quiz_question_scores` (`schema.sql:336+`) — quiz content and per-trainee scoring.

## Two grains hidden behind "session"

- **Training assignment / task** = the cohort-level DirectorTask.
- **Agent session** = `(DirectorTask, agent)` and its progress (a projected view, not a stored row).
- **Module attempt** = one `training_simulator_task_runs` row.
- **Conversation** = the media/transcript artifact for one scenario attempt.

## Sources

- `knowledge/training-simulator/README.md` §6 "Assignment and Session Model" (reviewed).
- `go-servers/apiserver/sql-schema/director/director-schema.sql` lines 5, 29, 54, 286, 315, 418.
- `go-servers/apiserver/sql-schema/gen/model/tasks.go`, `training_simulator_task_runs.go`.
