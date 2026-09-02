# Prompt: CONVI-7600 dedicated lesson statistics protobuf

```text
Update the lesson-level Training Simulator reporting protobuf contract in:

- Repo/worktree: /Users/xuanyu.wang/repos/cresta-proto-milestone-2
- Branch: codex/milestone-2-lesson-reporting
- PR: https://github.com/cresta/cresta-proto/pull/9677
- Ticket: CONVI-7600
- Stacked base PR: https://github.com/cresta/cresta-proto/pull/9676
- Required base branch: origin/codex/milestone-3-module-reporting
- Required base SHA at the start of this work: 97873e3108a0cc1366f1bcd3868c0c802103b7c6

Do not push until I have reviewed the result locally.

Read these records first:

- /Users/xuanyu.wang/repos/knowledge/training-simulator/decisions/2026-08-28-dedicated-lesson-module-stats-apis.md
- /Users/xuanyu.wang/repos/knowledge/training-simulator/decisions/2026-08-27-collapse-overall-zero-false-results.md
- /Users/xuanyu.wang/repos/knowledge/training-simulator/sessions/2026-08-28/codex-convi-7601-dedicated-module-stats.md
- /Users/xuanyu.wang/repos/knowledge/training-simulator/sessions/2026-08-26/codex-milestone-2-lesson-reporting.md

Goal

Replace the ListTrainingLessons statistics extension with a dedicated batched
RetrieveTrainingSimulatorLessonStats API. Keep ListTrainingLessons strictly
content-focused. Reuse the final TrainingSimulatorModuleStats contract from
stacked module PR #9676; do not duplicate or modify the module contract in the
lesson-only diff.

PR #9677 already contains reviewer feedback and obsolete draft history. Read
every relevant review thread before editing. Adapt useful feedback to the
dedicated RPC, but do not mechanically retain names or semantics tied to the old
ListTrainingLessons extension.

1. Rebuild the stack cleanly

- Fetch origin and verify that
  origin/codex/milestone-3-module-reporting resolves to
  97873e3108a0cc1366f1bcd3868c0c802103b7c6. If it has moved, inspect the new
  base before continuing and report the actual SHA.
- The current lesson branch is based on obsolete module commits from before
  #9676 was force-rewritten. Rebuild/rebase #9677 onto the current module branch.
- Preserve only the lesson changes above the module base. The final branch
  should have one clean lesson-only commit above #9676 unless repository history
  provides a concrete reason otherwise.
- Do not copy old module changes into the lesson commit.
- Do not push or edit GitHub metadata until local review.

2. Restore ListTrainingLessons

Restore ListTrainingLessonsRequest and ListTrainingLessonsResponse to their
pre-PR content-list contract.

Remove:

- include_stats
- stats_time_range
- stats_user_names
- ListTrainingLessonsResponse.lesson_stats
- the AIP-132 suppression added only for lesson_stats
- field-behavior annotations added to existing ListTrainingLessons fields only
  by this PR

The final lesson-only diff must not change the behavior or shape of
ListTrainingLessons. Because #9677 is stacked, also verify that the inherited
ListTrainingModules contract is the content-only version from #9676; do not
modify it in the lesson commit.

3. Add the dedicated RPC

Add:

  rpc RetrieveTrainingSimulatorLessonStats(
      RetrieveTrainingSimulatorLessonStatsRequest)
      returns (RetrieveTrainingSimulatorLessonStatsResponse);

Use a read-only GET binding parallel to the module/task statistics APIs:

  /v1/{parent=customers/*/profiles/*}/trainingSimulatorLessonStats

Use only the reporting/admin role surface:

- ADMIN
- SUPER_ADMIN
- QA_ADMIN

Do not inherit AGENT, MANAGER, or MANAGER_2ND from ListTrainingLessons without
explicit product and authorization evidence.

Retain only the narrowly necessary AIP-131 synonyms suppression used by the
adjacent custom Retrieve statistics RPCs. Explain any other suppression rather
than copying one from the old list response.

4. Add the request and response

Use this request shape:

  message RetrieveTrainingSimulatorLessonStatsRequest {
    string parent = 1;
    repeated string training_lesson_names = 2;
    cresta.v1.common.time.TimestampRange time_range = 3;
  }

Requirements:

- parent is required and references cresta.v1.customer.Profile.
- training_lesson_names is required and references TrainingLesson resources.
- Document that training_lesson_names contains 1–200 unique, nonempty names and
  that caller order is preserved.
- Do not add validate.rules annotations in this PR. Keep these constraints as
  documented API/service requirements, matching the final #9676 direction.
- time_range is optional and narrows matching assignments by the accepted
  reporting time semantics.
- Do not add agent_user_names, stats_user_names, group/team fields,
  direct_team_only, or TrainingSimulatorStatsFilter. Current lesson/module
  reporting design has no agent-population grouping or selector on this surface.

Use this response shape:

  message RetrieveTrainingSimulatorLessonStatsResponse {
    repeated TrainingSimulatorLessonStats lesson_stats = 1;
  }

Document that:

- lesson_stats has exactly the same length and order as training_lesson_names;
- every valid requested lesson receives one entry;
- a lesson with no matching assignments receives its identity plus zero/default
  metrics and ordered module entries as defined below;
- omitted entries are contract violations, not an empty-result representation.

5. Refine lesson_stats.proto

Keep TrainingSimulatorLessonStats in lesson_stats.proto and reuse
TrainingSimulatorModuleStats from module_stats.proto.

Use this intended clean, unreleased shape:

  message TrainingSimulatorLessonStats {
    string training_lesson_name = 1;
    int64 session_count = 2;
    double average_applicable_score = 3;
    int64 passed_assignment_count = 4;
    int64 total_assignment_count = 5;
    repeated TrainingSimulatorModuleStats modules = 6;
    int64 applicable_score_count = 7;
  }

Semantics:

- training_lesson_name references TrainingLesson.
- session_count is the number of distinct matching Training Simulator
  DirectorTasks that assign the lesson; it is not assigned-agent count.
- total_assignment_count counts matching assignment-rooted agent-lesson facts,
  including never-started, incomplete, and failure-equivalent results.
- passed_assignment_count counts the subset whose selected latest module results
  make the lesson pass. A lesson passes only when every required applicable
  module passes.
- Overall zero/false/N/A remains failure-equivalent under the accepted decision;
  do not add a lesson-level N/A count or persisted evaluation-status field.
- Clients derive lesson passed fraction as
  passed_assignment_count / total_assignment_count.
- applicable_score_count counts lesson facts with a usable lesson score.
- average_applicable_score is the mean of those usable normalized lesson scores
  in the 0.0–1.0 range. First compute each fact's mean across its applicable
  required-module scores, then average those lesson-fact scores so lessons with
  more modules are not overweighted.
- applicable_score_count == 0 means average_applicable_score is unavailable; a
  real average of zero remains distinguishable because the count is positive.
- Use a normal double, not proto3 optional, to avoid an awkward generated oneof
  in Director. Remove the draft average_score and pass_rate scalars.
- Remove has_missing_result_status; persisted evaluation status is no longer a
  reporting prerequisite under the collapsed overall-result decision.
- modules contains one TrainingSimulatorModuleStats for every module in the
  current TrainingLesson.training_modules list, in exactly that order.
- Each modules entry is aggregated only from the matching assignment facts for
  this lesson. Preserve the shared module message semantics from #9676,
  including assignment/result-specific denominators, non_applicable_result_count,
  score availability, and criterion ordering.
- Quiz reporting remains out of scope. Keep the response structurally aligned to
  the lesson definition and document the zero/default/unavailable behavior for a
  module type that has no supported statistics; do not silently omit an ordered
  module entry.

The draft is unreleased, so choose clean field numbering. Before renumbering,
verify that no released tag or generated downstream consumer depends on the old
draft field numbers.

6. Preserve the final module contract

The base #9676 contract currently establishes:

- no agent selector on RetrieveTrainingSimulatorModuleStatsRequest;
- no validate.rules introduced by that PR;
- TrainingSimulatorCriterionStats identified by criterion_id within its parent
  module, with no behavior_name;
- criterion passed_result_count, applicable_result_count, and
  non_applicable_result_count;
- module passed_assignment_count, total_assignment_count,
  average_applicable_score, and applicable_score_count;
- no has_missing_result_status.

Do not reverse or duplicate any of these base decisions in #9677. If the lesson
branch appears to require a module-contract edit, stop and explain why rather
than mixing it into the lesson-only commit.

7. Preserve scope

Do not add:

- module-level RPC/message changes beyond importing/reusing the base contract;
- backend or Director implementation;
- database/schema changes;
- persisted overall evaluation status or overall N/A fields;
- generated clients;
- per-row statistics RPCs;
- agent/group/team filters;
- a shared statistics filter message.

The API must support one batch request for the loaded lesson page, never one
request per lesson row.

8. Adapt PR review feedback

For every relevant naming or contract thread on #9677:

- verify whether it still applies after moving to a dedicated RPC;
- adopt it when it improves semantic precision;
- adapt it when it was tied to ListTrainingLessons or the old module draft;
- record a short rationale for anything not adopted;
- do not resolve/reply on GitHub until the local branch is approved for push.

Explicitly reconcile:

- restoration of ListTrainingLessons;
- dedicated request/response naming and HTTP binding;
- session_count meaning;
- optional-scalar/Director oneof impact;
- passed/total assignment counts instead of a pass_rate scalar;
- score availability through applicable_score_count;
- removal of has_missing_result_status;
- ordered reuse of TrainingSimulatorModuleStats;
- inherited removal of behavior_name;
- inherited criterion non_applicable_result_count;
- absence of agent/group/team filters;
- absence of PR-added validate.rules;
- Gazelle ownership of BUILD.bazel.

Update the PR description draft so it no longer says ListTrainingLessons is
extended and clearly states that #9677 is stacked on #9676. Do not publish that
description before local approval.

9. BUILD and formatting

- Keep lesson_stats.proto in the Training Simulator proto target.
- Import module_stats.proto only through the dependency required for
  TrainingSimulatorModuleStats.
- Run Bazel Gazelle; do not hand-edit generated BUILD structure.
- Confirm whether Gazelle changes BUILD.bazel relative to the updated #9676 base.
- Use repository formatting for changed proto files.
- Do not hand-edit generated clients.

10. Validation

Run:

- bazel run //:gazelle
- buf lint
- buf build
- bazel build //cresta/v1/trainingsimulator:all
- git diff --check

The repository's Bazel convenience workspace symlink can cause Buf to traverse
the checkout twice. Remove only the generated Bazel convenience symlinks before
running Buf if necessary; do not delete source files.

Inspect the three-dot diff against the updated stacked base, not origin/main:

  git diff origin/codex/milestone-3-module-reporting...HEAD

It should contain only:

- lesson_stats.proto;
- the dedicated lesson-statistics RPC and messages in
  training_simulator_service.proto;
- the Gazelle-produced lesson_stats.proto BUILD entry.

Report:

- actual module-base SHA;
- final lesson field-name mapping and rationale;
- exact changed files;
- validation results;
- any remaining API-lint suppression and why it is necessary;
- review comments adopted, adapted, or rejected;
- local commit SHA;
- any generated-client or downstream compatibility issue discovered.

Do not push. Let me review locally first.
```
