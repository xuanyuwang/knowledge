# Training Simulator: Lesson & Module Level Statistics Reporting

**Created:** 2026-08-11
**Updated:** 2026-08-13
**Domain:** `training-simulator` / subdomain `reporting`
**Status:** Discovery / requirements synthesis (no implementation ticket yet)
**Primary design:** [Figma — Training Simulator / Coaching Simulator, node 13108:21741](https://www.figma.com/design/B5tJUlNnKbbjVfxH44nqNl/Training-Simulator--Coaching-Simulator-?node-id=13108-21741)

> Implementation note (2026-08-13): the requirements and proposed product contract remain useful, but the detailed storage/query proposal below predates the normalized conversation/quiz score tables now on `go-servers/main`. Use the [backend design](./lesson-module-statistics-eng-design.md) and [frontend design](./lesson-module-statistics-fe-design.md) as the authoritative implementation plan.

## Executive summary

Training Simulator launched with **session-level** reporting only. Product and customers now need **lesson-level** and **module-level** statistics so coaches can diagnose *content* performance (which lessons/modules/criteria fail) rather than only *assignment* performance (which sessions agents completed).

The current Figma defines three reporting layers — Session, Module, Lesson — with consistent statistics sidecards. Backend today exposes `RetrieveTrainingSimulatorTaskStats` (task/session aggregation); lesson/module rollups are not yet first-class APIs and no dedicated implementation ticket was found.

Figma also explores CSV exports, but the research found **no authoritative Training Simulator CSV commitment, implementation spec, or release date**. Treat export as a product decision still requiring scope confirmation.

## Background

### Product hierarchy (canonical)

| Concept | Meaning | Reporting grain today |
|---------|---------|------------------------|
| **Lesson** | Ordered collection of modules; content object | Referenced by task stats; no lesson-centric rollup API |
| **Module** | Atomic unit (scenario pool or quiz + evaluation) | Present inside task/agent task runs; no module-centric rollup API |
| **Session / DirectorTask** | Assignment of a lesson to agents | **Shipped** via `RetrieveTrainingSimulatorTaskStats` |
| **Task run** | One module attempt (= one conversation or quiz) | Source rows for all higher rollups |

### Why this matters

- Session reporting answers: “Did agents finish *this assignment* and pass?”
- Lesson/module reporting answers: “Is *this content* effective? Which criteria/scenarios are weak across assignments?”
- Customers (e.g. Woolworths) explicitly asked for out-of-the-box adoption + diagnostics beyond Dashboard Builder. Product replied that session-level exists and **module/lesson level is coming soon**.

### Roadmap context

Linear project **Training Simulator** (lead: Krystal Truong) lists Q2 P0 as session-level analytics, and defers:

> Additional reporting and analytics (e.g., Coaching Hub view, aggregated agent- or lesson-level analytics for performance tracking and diagnostic insights over time)

The original PRD prioritized session analytics at P0, agent analytics at P1, and lesson adoption / aggregated module diagnostics at P2. August 2026 Figma work now actively designs all three grains, but this does not prove committed release scope. **Post-Launch Enhancements** (target ~2026-09-01) is the likely planning home on the path to GA (late Q3 / ~2026-10-29), pending product confirmation.

## Current state (shipped)

### Backend

- Proto: `cresta/v1/trainingsimulator/stats.proto` + `RetrieveTrainingSimulatorTaskStats` in `training_simulator_service.proto`
- Response: `TaskStats[]` with lesson refs, focus criteria, score (0–1), pass rate, `agent_performance[]`, embedded `training_modules`
- Filters: `time_range`, `training_lesson_names`, `direct_team_only`, `user_names`, `group_names`
- Tickets: [CONVI-7020](https://linear.app/cresta/issue/CONVI-7020/build-reportingstats-api) (Done), related proto PRs `#8836`, `#9020`, `#9022`, go-servers `#28636`

### Frontend

- Training Sessions tab summary cards + session table + session review side panel
- Tickets: [CONVI-7044](https://linear.app/cresta/issue/CONVI-7044/training-simulator-fe-reporting-ui), [CONVI-7105](https://linear.app/cresta/issue/CONVI-7105/training-simulator-fe-integrate-be-reporting-apis), [CONVI-7157](https://linear.app/cresta/issue/CONVI-7157/training-simulator-fe-update-active-sessions-count-to-total-sessions)
- Current Lesson Configuration placeholders confirm missing backend fields: `LessonsTab.tsx` hard-codes `sessionsAssigned: 0`; `ModulesTab.tsx` hard-codes `activeLessons: 0`.

### Known semantics / pitfalls (reuse for new levels)

- Score only lesson-required modules; exclude stale/nil-score runs
- Agent pass = all required modules completed **and** passed
- Lesson score = simple average of module scores
- N/A (`not_applicable`) values are excluded from score aggregation; the all-N/A denominator needs explicit behavior
- Task-run score basis: latest attempt per module (PRD + 2026-06-09 product/engineering clarification + domain verification)
- Incomplete lessons must remain incomplete when any required module has no attempt ([CONVI-7227](https://linear.app/cresta/issue/CONVI-7227/retrievetrainingsimulatortaskstats-marks-incomplete-lesson-as-complete))
- Training conversations use `ConversationSource.TRAINING_SIMULATOR` and must stay out of live analytics

## Requirements (from Figma + product signals)

Design section labels three rows: **Session-Level Report**, **Module-Level Report**, **Lesson-Level Report**.

### 1) Session-level (mostly shipped)

**UI (Training Sessions)**

- Filters: date range, teams/groups/users, direct team only, lessons
- Summary: total sessions, avg score, pass rate; assigned agents with incomplete/overdue; score distribution
- Table: session name, completion, agents assigned, avg score, pass rate, duration/due
- Side panel: session avg score / pass rate / due; module list; agent list with All / Passed / Failed / Incomplete; attempts; View link

**CSV exploration (not committed)**

The design explores reporting on a **single session’s** results using Coaching Hub scorecards export as a reference pattern. No Training Simulator export ticket/spec was found.

Proposed columns (design; `?` = unresolved in Figma):

| Column | Notes |
|--------|--------|
| Agent name | |
| Username | |
| Team name | |
| Submitted on | |
| Conversation URL? | Optional / TBD |
| Module name | |
| Scenario name | Empty for quizzes |
| Criteria A/B/… Score | Yes / No / N/A per criterion |
| Total score | |
| Number of attempts | |
| Duration (AHT)? | Optional / TBD |

### 2) Module-level (new)

**UI**

- Entry from Lesson Configuration → Modules (and/or Insights-style surfaces in the design file)
- Module detail drawer example (“Identification Fundamentals”):
  - Pass rate with fraction (e.g. `87% (23/31)`)
  - Average score vs threshold (e.g. `22% | Threshold: 70%`)
  - Active lessons count
  - Created / last edited metadata
  - **Evaluation Criteria** breakdown with per-criterion pass fractions and traffic-light coloring

**CSV exploration (not committed)**

The design explores a **single module’s** results filtered by date range, with parity to Performance Insights progression / scorecards export patterns. This is design intent, not confirmed scope.

### 3) Lesson-level (new)

**UI**

- Lesson detail drawer example (“Onboarding Fundamentals”):
  - Aggregate pass rate, average score vs threshold, metadata
  - **Modules** list with description + per-module pass-rate chips (color by threshold)
  - Drill-through affordance into module detail

**CSV exploration (explicit Figma annotation; not committed)**

> This reports on a single lesson’s results (filtered by date range). CSV columns: Agent name, Username, Team name, Submitted on, Conversation URL?, Module name, Scenario name (empty for quizzes), Criteria A Score (Yes/No/N/A), Criteria B Score (Yes/No/N/A), …, Total score, Number of attempts, Duration (AHT)

### Shared product requirements (cross-cutting)

1. **Three aggregation grains:** session (assignment), lesson (content across assignments), module (content across lessons/sessions).
2. **Diagnostic depth:** criterion-level pass fractions at module (and via export at all levels).
3. **Attempts:** expose attempt/retry counts and preserve enough history to analyze first-pass rate and improvement, while current official score remains the latest attempt.
4. **Filterability:** at least date range; align with existing team/user/group filters where applicable.
5. **Threshold-aware presentation:** show configured pass threshold next to average score; color pass rates.
6. **Quiz compatibility:** scenario name empty for quiz modules; criteria scoring still required where applicable.
7. **Adoption:** assigned, started, completed, incomplete, and completion rate at the appropriate grain.
8. **Content diagnostics:** frequently missed behaviors/criteria, cohort and agent drilldowns, and weak lesson/module identification.
9. **Longitudinal analysis (roadmap):** recurring criterion gaps and scenario/content quality over time.
10. **Exportability (candidate):** customers need portable data and native LMS connectivity is absent, but CSV scope remains uncommitted.

## Customer & GTM signals

| Signal | Source | Freshness | Confidence | Takeaway |
|--------|--------|-----------|------------|----------|
| Module/lesson reporting “coming soon” unlocks insights/diagnostics | Krystal Truong in [#internal--woolworths-group-limited](https://cresta.enterprise.slack.com/archives/C09H52T5FK7/p1785906804472019) (2026-08-05) | ✅ <1 mo | High | Explicit public product commitment |
| Deeper longitudinal analytics planned; session/agent reporting exists today | Krystal Truong in [#partnerships](https://cresta.enterprise.slack.com/archives/C04NVB575S7/p1781825314932599) (2026-06-18) | ⚠️ ~2 mo | High | Lesson/module fits “diagnose criteria/scenario quality” |
| Xanterra, Greenix, and Zenni repeatedly asked which behaviors are missed at lesson/module level | [Three-demo synthesis](https://cresta.enterprise.slack.com/archives/C0BAGU1AV4P/p1784825292249789) (2026-07-23) | ✅ <1 mo | Medium-high | Add criterion miss frequency/ranking, not only average scores |
| Product response says richer diagnostics should resemble scorecard performance reporting | [Reporting follow-up](https://cresta.enterprise.slack.com/archives/C0BG1DB82JW/p1784831189963659) (2026-07-23) | ✅ <1 mo | High | Keep diagnostics evidence-grounded in Opera outcomes; AI advice is not near-term scope |
| Consistent statistics sidecards across session/lesson/module are under design review | [Design status](https://cresta.enterprise.slack.com/archives/C0337QYJAQN/p1785598668896329) (2026-08-01) | ✅ <1 mo | High | Separate average score from pass rate and keep definitions comparable |
| Q2 deferred “aggregated agent- or lesson-level analytics” | Linear project description | ✅ current | High | Official backlog deferral now being pulled forward |
| Engagement / drop-off / success-over-time | [GONG-783](https://linear.app/cresta/issue/GONG-783/lms-reporting-needs-engagement-drop-off-and-ai-query-accuracy) | ⚠️ triage May 2026 | Medium | Adjacent LMS ask; not identical to TS lesson/module stats |
| Tie training completion to later production performance | [GONG-4179](https://linear.app/cresta/issue/GONG-4179/tie-training-completion-to-real-performance-improvements) | ⚠️ triage | Medium | Out of scope for first lesson/module stats slice |

## Source inventory

### Design

| Artifact | URL | Updated | Authority / confidence | Notes |
|----------|-----|---------|------------------------|-------|
| Figma reporting section | [node 13108-21741](https://www.figma.com/design/B5tJUlNnKbbjVfxH44nqNl/Training-Simulator--Coaching-Simulator-?node-id=13108-21741) | 2026-08-07 | Official product design / High | Phoebe Wang (previously Yue Peng), PM partner Krystal Truong; current Session / Module / Lesson sidecards; CSV is exploratory |
| Coaching Training Simulator PRD | [Coda / Superhuman Docs](https://coda.io/d/_ddKtmYQWQVC/Coaching-Training-Simulator-PRD_suAmSoxf) | latest verified 2026-06-01 | Official product / High requirements, Medium metadata | Defines module score/pass/attempts, latest-attempt reporting, lesson averaging, all-modules-pass, and P0/P1/P2 analytics prioritization |
| Training Simulator Design (eng) | [Google Doc](https://docs.google.com/document/d/1GCeE9XCAVcgetOhYWPvqZJ3hp3qeVhK5YMk4rBkWmd4/edit) | latest verified reference 2026-08-04 | Official engineering design / Medium | Jack Jee; historical reporting RPC design. Merged `RetrieveTrainingSimulatorTaskStats` is authoritative |
| Training Simulator Design P1 Update | [Google Doc](https://docs.google.com/document/d/1luOZUCJ3FfzyrijTH1PY5heF2TTIKbCX5-bUNHx5hSw/edit) | 2026-07-23 | Official engineering design / High | Quiz/task-run architecture; scenario and quiz module result semantics |
| 07/07 SE Enablement: Training Simulator | [Slides](https://docs.google.com/presentation/d/1lY6UOohDrUn7zxcW_DgfNpq0rvz9z_zz0ZfNJSVktgg/edit?slide=id.g3f0842e9369_0_277#slide=id.g3f0842e9369_0_277) | 2026-07-07, reconfirmed 2026-08-04 | Official enablement / High | Directional leaderboards/diagnostics; current outputs are configured Yes/No/N/A criteria, not AI-generated feedback |
| Reporting/scoring clarification | [Slack decision record](https://cresta.enterprise.slack.com/archives/C0AEE1T2U1X/p1781029644559089) | 2026-06-09 | Product + engineering / High | Latest attempt, simple module average, all modules pass, incomplete denominator and P0 filter semantics |
| Launch enablement / out-of-scope table | [#training-simulator-launch-jul-9](https://cresta.enterprise.slack.com/archives/C0BAGU1AV4P/p1782224934.214999) | 2026-06-23 | Semi-official / Medium | Thread confirms a P1+ capability table; attachment content was not exposed |

### Engineering (authoritative for current behavior)

| Artifact | Location |
|----------|----------|
| Domain map | `knowledge/training-simulator/README.md` |
| Reporting subdomain | `knowledge/training-simulator/subdomains/reporting/README.md` |
| Stats proto | `cresta-proto/cresta/v1/trainingsimulator/stats.proto` |
| Stats RPC | `RetrieveTrainingSimulatorTaskStats` |
| BE impl | `go-servers/apiserver/internal/trainingsimulator/action_retrieve_training_simulator_stats.go` |
| FE | `director/.../features/training-simulator`, `hooks/training-simulator/useTrainingSimulatorTaskStats.ts` |
| Released FE integration | [CONVI-7105](https://linear.app/cresta/issue/CONVI-7105/training-simulator-fe-integrate-be-reporting-apis) + Director PRs [#20045](https://github.com/cresta/director/pull/20045), [#20090](https://github.com/cresta/director/pull/20090), [#20095](https://github.com/cresta/director/pull/20095) |
| Completion semantics fix | [CONVI-7227](https://linear.app/cresta/issue/CONVI-7227/retrievetrainingsimulatortaskstats-marks-incomplete-lesson-as-complete) + [go-servers #29745](https://github.com/cresta/go-servers/pull/29745) |
| N/A aggregation | [CONVI-7221](https://linear.app/cresta/issue/CONVI-7221/review-trainingsimualtor-conversation-evaluation-for-na) + [go-servers #29693](https://github.com/cresta/go-servers/pull/29693) |

### Glean collection note

Glean is installed as a Cursor plugin and was used for three focused enterprise-search passes: product/design documents, customer/GTM requirements, and implementation evidence. The plugin surfaced indexed Slack/file results and authoritative links, but direct full-document reading was unavailable in these passes. Claims that depend on access-restricted document bodies are therefore labeled with reduced confidence or “latest verified” dates.

The result set was vetted for relevance, freshness, and authority. Tangential LMS, generic analytics/export, unrelated simulator scoring, and stale/superseded Notion results were excluded.

## Backend data contract

### Readiness conclusion

The backend requirements are clear enough to define the **outcome-reporting** contract (scores, pass rate, attempts, and criterion diagnostics). Existing task-run rows contain those facts.

Accurate **adoption/completion** reporting is not fully supportable after content edits until [CONVI-7263](https://linear.app/cresta/issue/CONVI-7263/update-training-simulator-session-to-take-snapshot-of-revisions-from) is completed:

- Task runs snapshot lesson/module/scenario revisions when an attempt starts.
- DirectorTask content stores only stable lesson resource names.
- A never-started agent has no task run, so the assigned lesson/module revisions cannot be reconstructed.
- `RetrieveTrainingSimulatorTaskStats` currently reloads the latest lesson revision, which can rewrite historical completion semantics after modules are added/removed.

Recommended sequence:

1. Add assignment-time lesson/module revision snapshots to DirectorTask content.
2. Make task-run creation use the assignment snapshot rather than “latest revision at attempt start.”
3. Add lesson/module statistics RPCs over DirectorTask assignments + task runs.

Current session-stats implementation should not be generalized mechanically:

- It starts from active tasks but returns no row when a task has zero runs, losing never-started assignments.
- `time_range` is passed to `ListDirectorTasks` despite the request comment saying “filtering task runs.”
- `direct_team_only` is declared in the request but is not consumed by `RetrieveTrainingSimulatorTaskStats`.
- It assumes all runs under a task use the first run’s lesson.
- It fetches task runs through an unpaginated 1,000-row list RPC.
- It loads the latest lesson/modules instead of the task-run or assignment revisions.

### Data to read

| Source | Fields | Purpose |
|--------|--------|---------|
| `director.training_simulator_task_runs` | `director_task_id`, lesson/module/scenario IDs **and revision IDs**, `agent_user_id`, `score`, `passed`, `criterion_results`, `created_at`, `updated_at` | Attempt facts, latest-attempt selection, score/pass/N/A, criterion diagnostics |
| `director.tasks` | task type/status, create time, audience config, schedule/due time, Training Simulator content config | Assignment denominator, never-started/incomplete agents, task/date scoping |
| `director.training_lessons` at assigned revision | title, ordered `training_module_ids`, focus criteria | Required-module set and historical lesson metadata |
| `director.training_modules` at assigned/run revision | display name, description, `evaluation_config`, quiz reference | Threshold, allowed attempts, criterion configuration, historical module metadata |
| User service | users, groups, current team/direct-team membership, display names | Request filtering and agent/team labels |
| Conversation service (deferred) | conversation URL and duration | Only needed if CSV/AHT scope is later committed |

No new outcome table is required. The existing task-run table already stores immutable revision IDs and criterion-result JSON. The reporting query should read the DB model directly; the public `TrainingSimulatorTaskRun` proto currently omits revision IDs.

Do not build this by calling `ListTrainingSimulatorTaskRuns`: it has a hard 1,000-row cap and no page token. Use dedicated aggregate queries (or a repository method) and start from DirectorTask assignments so agents with zero runs are retained.

### Required query indexes

The existing indexes cover conversation, agent, and `(task, lesson, module)` lookup, but not date-ranged content aggregation. Add/validate indexes such as:

```sql
CREATE INDEX ... ON director.training_simulator_task_runs
  (customer_id, profile_id, training_lesson_id, created_at DESC);

CREATE INDEX ... ON director.training_simulator_task_runs
  (customer_id, profile_id, training_module_id, created_at DESC);
```

Use query plans and production cardinality before finalizing wider covering indexes for latest-attempt windowing.

### Aggregation grain and rules

**Assignment facts**

- Lesson assignment grain: `(director_task_id, training_lesson_id, agent_user_id)`.
- Module assignment grain: expand each snapshotted lesson into `(director_task_id, training_lesson_id, training_module_id, agent_user_id)`.
- Stable lesson/module IDs are the product grouping keys; revision IDs preserve the exact historical configuration.
- Include Training Simulator tasks in reporting scope and exclude draft/deleted tasks. Do not require a task run to include the assignment.
- Expand user/group/direct-team filters to an allowed-agent set, then intersect assignment facts with that set. Filtering only the task list would incorrectly retain other agents from the same task.

**Attempt selection**

- Official outcome uses the latest task run by `created_at` for each `(director_task_id, lesson_id, module_id, agent_user_id)`.
- `total_attempt_count` counts all runs before latest-attempt reduction.
- Retry/improvement metrics may inspect all runs, but must not replace the latest attempt as the official score.

**Module metrics**

- `assigned_count`: module assignment facts.
- `started_count`: assignment facts with at least one run.
- `completed_count`: latest runs with completed evaluation results.
- `not_applicable_count`: completed runs where every criterion result is N/A.
- `passed_count` / `failed_count`: applicable completed latest runs.
- `average_score`: mean score over applicable evaluated latest runs.
- `pass_rate`: `passed_count / (passed_count + failed_count)`; N/A and incomplete runs are excluded.
- Criterion rollups group by `behavior_name`; fall back to `(module_revision_id, criterion_id)` when no behavior exists.

**Lesson metrics**

- Complete only when every snapshotted required module has an evaluated latest run.
- Lesson score per agent = simple average of applicable required-module scores.
- Lesson pass = every required module passes.
- Aggregate average score = mean of per-agent lesson scores (prevents lessons with more modules from overweighting agents).
- Pass-rate denominator excludes incomplete and all-N/A lesson outcomes.

**Time range**

- Preserve current task-stats semantics for v1: `time_range` scopes DirectorTasks by overlap with the task window (`create_time` through due time), not by task-run time.
- Do not silently reinterpret it as conversation or evaluation time.
- If product requires “submitted during range,” add an explicit evaluation-completion timestamp; `updated_at` is not a durable semantic substitute.

**Content revisions**

- Outcome grouping may span revisions under a stable content ID because `score`/`passed` were recorded under the attempt’s configuration.
- Return `revision_count` / `mixed_revisions` so consumers know whether a rollup crosses revisions.
- Resolve display metadata from the latest revision, while resolving required-module membership and criterion fallback keys from the assigned/run revision.

## Proposed proto changes

### 1. Snapshot assignment content (`cresta/v1/coaching/task.proto`)

Keep `training_lesson_names` for backward compatibility and add immutable assignment snapshots. This serializes into the existing DirectorTask content-config JSONB, so it does not require a relational schema migration.

```proto
message TrainingSimulatorScenarioSnapshot {
  string training_scenario_name = 1 [
    (google.api.resource_reference).type =
        "cresta.v1.trainingsimulator.TrainingScenario",
    (google.api.field_behavior) = REQUIRED
  ];
  string training_scenario_revision_id = 2
      [(google.api.field_behavior) = REQUIRED];
}

message TrainingSimulatorModuleSnapshot {
  string training_module_name = 1 [
    (google.api.resource_reference).type =
        "cresta.v1.trainingsimulator.TrainingModule",
    (google.api.field_behavior) = REQUIRED
  ];
  string training_module_revision_id = 2
      [(google.api.field_behavior) = REQUIRED];
  repeated TrainingSimulatorScenarioSnapshot training_scenarios = 3;
  string quiz_template_name = 4
      [(google.api.resource_reference).type =
           "cresta.v1.trainingsimulator.QuizTemplate"];
}

message TrainingSimulatorLessonSnapshot {
  string training_lesson_name = 1 [
    (google.api.resource_reference).type =
        "cresta.v1.trainingsimulator.TrainingLesson",
    (google.api.field_behavior) = REQUIRED
  ];
  string training_lesson_revision_id = 2
      [(google.api.field_behavior) = REQUIRED];
  repeated TrainingSimulatorModuleSnapshot training_modules = 3
      [(google.api.field_behavior) = REQUIRED];
}

message TrainingSimulatorContentConfig {
  // Legacy stable names; continue reading for old tasks.
  repeated string training_lesson_names = 1 [...];

  // Exact content assigned to this task.
  repeated TrainingSimulatorLessonSnapshot training_lesson_snapshots = 2
      [(google.api.field_behavior) = IMMUTABLE];
}
```

Compatibility behavior:

- New assignments populate both fields.
- Existing assignments with names only use a legacy fallback and are marked `historical_snapshot_missing` in stats.
- Task-run creation copies revision IDs from the task snapshot.
- Mirror the snapshot messages/fields in `go-servers/apiserver/sql-schema/protos/qa/task.proto` and update the public↔DB task-content converters; that internal proto is what `director.tasks.task_content_config` persists.

### 2. Add shared statistics messages (`cresta/v1/trainingsimulator/stats.proto`)

```proto
message TrainingSimulatorOutcomeSummary {
  int64 assigned_count = 1;
  int64 started_count = 2;
  int64 completed_count = 3;
  int64 incomplete_count = 4;
  int64 not_applicable_count = 5;
  int64 passed_count = 6;
  int64 failed_count = 7;
  optional double average_score = 8;  // 0.0–1.0
  optional double pass_rate = 9;      // 0.0–1.0
  int64 total_attempt_count = 10;
  int64 retried_agent_count = 11;
}

message TrainingSimulatorCriterionStats {
  string behavior_name = 1;
  string criterion_id = 2;
  string display_name = 3;
  int64 applicable_count = 4;
  int64 passed_count = 5;
  int64 failed_count = 6;
  int64 not_applicable_count = 7;
  optional double pass_rate = 8;  // 0.0–1.0
}

message TrainingSimulatorModuleOutcomeSummary {
  string training_module_name = 1;
  string training_module_display_name = 2;
  TrainingSimulatorOutcomeSummary outcomes = 3;
  uint32 revision_count = 4;
  bool mixed_revisions = 5;
}

message TrainingSimulatorLessonStats {
  string training_lesson_name = 1;
  string training_lesson_title = 2;
  TrainingSimulatorOutcomeSummary outcomes = 3;
  repeated TrainingSimulatorModuleOutcomeSummary modules = 4;
  uint32 revision_count = 5;
  bool mixed_revisions = 6;
  bool historical_snapshot_missing = 7;
  int64 session_count = 8;  // Matching DirectorTasks in the cohort.
}

message TrainingSimulatorModuleStats {
  string training_module_name = 1;
  string training_module_display_name = 2;
  TrainingSimulatorOutcomeSummary outcomes = 3;
  repeated TrainingSimulatorCriterionStats criteria = 4;
  int64 active_lesson_count = 5;
  optional double current_passing_score = 6;  // normalized 0.0–1.0
  uint32 revision_count = 7;
  bool mixed_revisions = 8;
  bool historical_snapshot_missing = 9;
}
```

Use `optional` for rates/scores with empty denominators instead of the existing `-1 + not_applicable` sentinel.

Keep aggregate scores/rates on the existing stats scale (`0.0–1.0`). Normalize `EvaluationConfig.passing_score` from its stored `0–100` scale into `current_passing_score` on the response.

Count invariants:

- `incomplete_count = assigned_count - completed_count`
- `completed_count = passed_count + failed_count + not_applicable_count`
- `started_count <= assigned_count`

### 3. Add two explicit RPCs (`training_simulator_service.proto`)

Separate lesson/module RPCs match the two UI tables and avoid returning unused criterion detail.

```proto
rpc RetrieveTrainingSimulatorLessonStats(
    RetrieveTrainingSimulatorLessonStatsRequest)
    returns (RetrieveTrainingSimulatorLessonStatsResponse) {
  option (google.api.http) = {
    get: "/v1/{parent=customers/*/profiles/*}/trainingSimulatorLessonStats"
  };
}

rpc RetrieveTrainingSimulatorModuleStats(
    RetrieveTrainingSimulatorModuleStatsRequest)
    returns (RetrieveTrainingSimulatorModuleStatsResponse) {
  option (google.api.http) = {
    get: "/v1/{parent=customers/*/profiles/*}/trainingSimulatorModuleStats"
  };
}
```

Both requests should carry:

- required `parent`
- optional `time_range` (DirectorTask window overlap, matching current task-stats semantics)
- existing `direct_team_only`, `user_names`, and `group_names`
- required `training_lesson_names` or `training_module_names`

Require 1–100 content names per call. The existing paginated Lesson/Module list drives the current page, then requests stats for only those names; this avoids an unbounded stats response.

Use the same role annotations as `RetrieveTrainingSimulatorTaskStats`; enforce agent-vs-manager row visibility in the backend rather than relying on request filters.

Requests/responses (resource-reference and validation annotations omitted here only for readability; copy the corresponding annotations from `RetrieveTrainingSimulatorTaskStatsRequest` and enforce 1–100 content names):

```proto
message RetrieveTrainingSimulatorLessonStatsRequest {
  string parent = 1;
  cresta.v1.common.time.TimestampRange time_range = 2;
  repeated string training_lesson_names = 3;
  bool direct_team_only = 4;
  repeated string user_names = 5;
  repeated string group_names = 6;
}

message RetrieveTrainingSimulatorLessonStatsResponse {
  repeated TrainingSimulatorLessonStats lesson_stats = 1;
}

message RetrieveTrainingSimulatorModuleStatsRequest {
  string parent = 1;
  cresta.v1.common.time.TimestampRange time_range = 2;
  repeated string training_module_names = 3;
  bool direct_team_only = 4;
  repeated string user_names = 5;
  repeated string group_names = 6;
}

message RetrieveTrainingSimulatorModuleStatsResponse {
  repeated TrainingSimulatorModuleStats module_stats = 1;
}
```

Do **not** extend `TaskStats` for this work: its grain is a single DirectorTask/session, while the new APIs aggregate stable content across tasks.

### 4. No task-run proto change required for v1

The reporting implementation can read revision IDs from the DB model. Exposing lesson/module/scenario revision IDs on public `TrainingSimulatorTaskRun` would improve debugging, but it is not required by the lesson/module stats clients and should be a separate additive change.

## Gap analysis

| Capability | Session | Lesson | Module |
|------------|---------|--------|--------|
| In-product summary metrics | ✅ | ❌ | ❌ |
| Detail drawer / drill-down | ✅ (agents + modules list) | ❌ (designed) | ❌ (designed) |
| Criterion pass fractions | Partial (agent/run evaluation UI) | ❌ aggregate | ❌ aggregate (designed) |
| CSV export | ❌ (explored, uncommitted) | ❌ (explored, uncommitted) | ❌ (explored, uncommitted) |
| Dedicated stats API grain | ✅ task | ❌ | ❌ |

### Likely engineering workstreams

1. **Assignment snapshots:** complete CONVI-7263 across public proto, persisted task-content proto/converters, assignment creation, and task-run creation.
2. **Proto / API:** add shared outcome/criterion messages plus explicit lesson/module retrieve RPCs; leave `TaskStats` unchanged.
3. **Aggregation/indexes:** dedicated DB queries over assignment facts + task runs; latest-attempt and N/A rules; lesson/module date indexes; no list-RPC dependency.
4. **FE:** replace `sessionsAssigned: 0` / `activeLessons: 0`, add Lesson Configuration stats hooks/drawers, drilldowns, and threshold-aware colors.
5. **Export discovery (not committed):** decide whether to build CSV; if yes, define row grain, revision identity, attempt policy, permissions, Conversation URL, and AHT before implementation.
6. **Permissions:** same Coaching Hub roles as session stats; intersect the assignment cohort with permitted/requested agents in the backend.

## Open questions (need product/design)

1. Is lesson/module reporting in **Lesson Configuration**, a new **Insights** tab, or both (Figma shows “Insights” chrome in some frames)?
2. Does “Active lessons: 5” on a **lesson** drawer reflect a design copy-paste from module, or a real metric (e.g. active *sessions*)?
3. Cohort definition for lesson/module stats: all historical assignments in date range, or only active/incomplete sessions?
4. Confirm that the current latest-attempt scoring policy applies to cross-session lesson/module rollups; define separate first-pass and improvement metrics from attempt history.
5. Is CSV export committed at all? If yes, confirm row grain and optional fields: Conversation URL, Duration (AHT).
6. What release phase owns August’s lesson/module design (P1, P2, beta, GA, or uncommitted)?
7. Should module stats span **all lessons** that include the module, or only when viewing from a parent lesson?
8. How should randomized scenarios and unequal sample sizes be normalized in module aggregates?
9. Should historical aggregation use assignment-time snapshots/revisions so later lesson/module edits do not rewrite history?
10. What should score/pass-rate show when every criterion is N/A?

## Recommended scope for v1 slice

**In scope**

- Lesson and module aggregate metrics + detail drawers matching Figma
- Criterion pass-rate breakdown on module drawer
- Most-frequently-missed behavior/criterion ranking
- Attempt count and retry visibility while keeping latest attempt as the official score
- Date-range filter; reuse existing team/user filters where cheap
- Backend APIs grounded in `training_simulator_task_runs`
- Metric definitions documented and shared with enablement

**Non-goals (v1)**

- Linking training outcomes to live production KPIs (GONG-4179)
- Native LMS connectors
- CSV export until product commits scope and semantics
- AI-generated qualitative coaching advice
- Dashboard Builder replacement / org-wide ClickHouse marts (unless load requires it)
- Chat-channel simulation reporting beyond existing voice/quiz runs

## Stakeholders

| Role | People |
|------|--------|
| PM | Krystal Truong |
| Design | Phoebe Wang; previously Yue Peng |
| Eng (BE) | Jack Jee, Tinglin Liu |
| Eng (FE) | Kurt Choi |
| Domain owner (knowledge) | Xuanyu Wang |
| GTM / enablement | Danielle McKenzie, Odhran Reidy (launch thread) |

## Success metrics

- Managers can identify weakest modules/criteria without exporting to sheets
- Managers can compare completion, retries, pass rate, average score, and frequently missed criteria without reconstructing results manually
- Metric definitions match session-level pass/score semantics (no contradictory numbers across grains)
- No regression in session stats performance for large cohorts

## Next actions

1. Product/design walkthrough of Figma node 13108:21741 to close open questions, especially release scope and whether CSV is committed.
2. Read the full PRD and engineering design through authenticated document access to verify indexed snippets and denominator semantics.
3. File Linear epic/tickets under Training Simulator → Post-Launch Enhancements (proto, BE, FE, QA; export only if confirmed).
4. Spike aggregation SQL/plan against `training_simulator_task_runs`, including revision identity, all-N/A cohorts, randomized scenarios, and lesson/module latency.
5. Update `subdomains/reporting/README.md` as implementation lands.

## Related artifacts

- Work item: `work-items/lesson-module-statistics-reporting.md`
- Session: `sessions/2026-08-11/cursor-lesson-module-stats-docs.md`
- Subdomain: `subdomains/reporting/README.md`
- Prior domain seed: `sessions/2026-08-09/claude-training-simulator-domain-setup.md`
