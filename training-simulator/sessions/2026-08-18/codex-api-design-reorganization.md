# API design reorganization

Date: 2026-08-18
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree context: design-only work in `/Users/xuanyu.wang/repos/knowledge`; no source worktree changed

## Objective

Clean the local lesson/module statistics API proposal before editing the published Superhuman document again.

## Changes

- Consolidated the three API options, selected boundary, analytics-service lesson, loading trade-off, proto examples, request/response behavior, and authorization under one backend `API design` chapter.
- Finalized option 3 locally: separate lesson- and module-grain stats RPCs with shared value objects and backend machinery.
- Split the proto examples into shared outcome/result messages, shared filter and request/response messages, and service/HTTP declarations.
- Linked the frontend decision summary to the canonical backend rationale and kept implementation-specific batching/cache/prefetch details in the frontend plan.
- Replaced the work item's duplicated preliminary analysis with a short link to the canonical decision.
- Recorded the decision in `decisions/2026-08-18-lesson-module-stats-api-boundary.md`.
- Expanded all three options with comparable implementation steps, pros, and cons across proto shape, backend reads, authorization, caching, failure behavior, and frontend integration.
- Drafted an exact Superhuman page edit in `deliverables/superhuman-api-design-update-draft.md`, including replacement boundaries, organized API-option text, proto examples, frontend loading changes, review-note changes, and removal of the malformed trailing addendum.
- Rewrote the publication draft in plainer English, replacing terms such as `cohort`, `grain`, `projection`, `value objects`, and `transport consolidation` with selected users, result level, requested results, shared messages, and combining API requests.

## Validation

- Reviewed the reorganized Markdown in source order.
- Ran `git diff --check` on the touched Training Simulator artifacts.

## External state

The [Superhuman engineering design](https://docs.superhuman.com/d/_dE0dCcz8Bub/_subF07cy) was updated through the write-capable Coda MCP:

- revised the Overview API decision;
- replaced the old API section with all three options, balanced implementation/pros/cons, the selected boundary, and protobuf examples;
- added frontend loading/prefetch guidance and closed review decisions;
- removed the malformed trailing addendum and `.order;` fragment;
- updated the review date to 2026-08-18.

The final page was read back through Coda to verify the option headings, protobuf blocks, loading section, review notes, and absence of the deleted fragments.

Terminology follow-up: `outcome` in this design meant the training-attempt result, not Conversation-Level Outcome (CLO). The canonical plans were updated to prefer `training result`, `attempt result`, or `evaluation result`; proposed identifiers were aligned to `TrainingSimulatorResultSummary`, `result_status_missing`, and `attemptResult`. A page-wide Coda replacement was started, but the connector remained locked afterward and did not return a completion result; the live terminology change therefore still requires read-back verification.

The Superhuman page was subsequently copied into `deliverables/superhuman-api-design-update-draft.md` as a full local working copy. Its Markdown heading hierarchy and protobuf fences were restored, and its ambiguous `outcome`/`cohort` wording was replaced contextually with training result, attempt result, evaluation status, selected users, or user group language.

Scope review then simplified the correctness plan:

- verified that `EvaluateTrainingConversation` already returns `EvaluationStatus` and overall `NotApplicable`, while `UpdateTrainingSimulatorTaskRun` persists only score, passed, and criterion results to `director.training_simulator_conversation_scores`;
- selected that existing Training Simulator table for new `evaluation_status` and `not_applicable` fields—no `app.chats` change and no new result table;
- moved assignment snapshots and exact-revision evaluation out of this project's scope, while preserving current-content drift as a known limitation;
- excluded quiz statistics because the quiz feature and reporting semantics are not mature;
- removed the task-run list's 1,000-row limit as a statistics correctness concern because the new RPCs query filtered database data directly rather than aggregating a `ListTrainingSimulatorTaskRuns` response.
- split `TrainingSimulatorStatsFilter.group_names` into `virtual_group_names` and `team_group_names`. Director already stores dynamic groups and teams separately, so the backend can populate the shared user-filter parser directly and avoid a preliminary `GroupsByGroupType` User Service call; membership resolution remains on the backend.

## Next step

Review the remaining product, data, security, and latency-target gates; do not resolve review comments without an explicit request.
