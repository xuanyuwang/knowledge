# Codex Session: Milestone 1 Session Reporting Review

## Scope

Reviewed only Milestone 1 session-level reporting. Built the current assignment/attempt/result/API/Director mental model, compared each proposed scope item with current main-branch code and the Figma Reporting frame, and reconciled the assumptions against aggregate staging data. Lesson/module reporting was excluded except for required-module subtype handling needed to compute a session result.

## Source context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`, branch `main`, clean checkout.
- Related repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/cresta-proto`, both branch `main`, clean checkouts.
- Knowledge checkout: `/Users/xuanyu.wang/repos/knowledge`, existing unrelated and project-local changes preserved.
- Design: Figma Reporting node `13108:21741`, inspected through the signed-in browser.
- Data: read-only aggregate queries through `/Users/xuanyu.wang/.cursor/skills/connect-customer-app-db` for staging `cresta/walter-dev`.

## Work performed

- Read the project registry, README, reporting subdomain, current proposal, milestone definition, and prior session notes.
- Traced `RetrieveTrainingSimulatorTaskStats`, `ListTrainingSimulatorTaskRuns`, conversation-score persistence, evaluation timeout persistence, stats protos, and current Director session table/dashboard/drawer adapters.
- Visually inspected the Figma session landing page and agent review drawer.
- Queried only aggregate counts for tasks, audiences, task runs, conversation/quiz outcomes, retry groups, null results, and N/A-like result shapes.
- Produced `deliverables/milestone-1-session-reporting-review.md`.

## Main findings

1. Milestone 1 is justified, but the frontend work is narrower than proposed because the current Director main branch already implements most of Figma.
2. Backend zero-run omission is real, although Director currently masks it by joining a separate assignment list and synthesizing incomplete rows.
3. Attempt count requires a new field because the current response retains only latest-per-module task runs; staging contains 48 retried agent/module groups and one group with 53 attempts.
4. Explicit evaluation status and overall N/A are release blockers. Eighteen staging rows have all criteria N/A but are stored as score zero/passed false; ten are current latest results and look failed.
5. The proposal's “latest settled result” wording must become latest-attempt-first, then classification, to avoid falling back past a newer unfinished retry.
6. Conversation-only session aggregation is incorrect while quiz modules exist: staging contains 40 quiz task runs, and the current handler includes them in session completion.
7. Authorization is not an acceptance-check detail. The RPC allows AGENT and the handler does not enforce self/manageable-user row scope.
8. Figma does not resolve all-N/A filter membership or warning presentation; those are design gates, not implementation details.

## Staging evidence retained

- 68 active tasks; 12 have no runs.
- 97 active-task assigned-agent facts; 39 have no run.
- 317 task runs: 277 conversation and 40 quiz.
- 48 retried agent/module groups; maximum 53 attempts.
- 110 conversation scores have null score/pass; 167 have both values.
- 18 rows have all criteria N/A but score zero/pass false; 10 are latest results.
- 13 latest conversation results have no score or criteria.

No user-level rows, resource IDs, credentials, or connection strings were retained.

## Credential use

- `voice-staging_ro`: SSO refreshed; IAM database authentication failed, so no query succeeded with it.
- `voice-staging_dev`: used only through the read-only customer app DB launcher, which obtained a read-only database connection without printing or persisting the URI.

## Outcome

Milestone 1 should proceed only after the official-attempt, quiz, all-N/A/filter, authorization, and lifecycle gates in the review are closed. No product code or remote design was changed.

## Follow-up: draft design update

Updated `deliverables/superhuman-api-design-update-draft.md` after review:

- changed the shared contract from “latest settled conversation result” to latest-task-run-first classification;
- added conversation/quiz subtype normalization as a session-only requirement while leaving lesson/module quiz aggregates out of scope;
- narrowed Director work to attempt count, all-N/A/filter behavior, visible warnings, latest-activity naming, and aggregate corrections;
- made backend self/manageable-user authorization explicit;
- added the staging evidence, boundary-scale caveat, corrected exit criteria, five Milestone 1 decision gates, and revised ticket descriptions;
- preserved the later lesson/module milestone scope except for references to the shared official-attempt primitive.

The local working draft was updated. The live Superhuman page was not modified in this follow-up.

## Follow-up: persistence ticket and official retry semantics

- Created [CONVI-7582](https://linear.app/cresta/issue/CONVI-7582/persist-evaluation-status-and-overall-na-on-training-simulator) in the Training Simulator project under Convo Intelligence, status Backlog. The issue includes the persistence evidence, rationale, legacy handling, implementation references, and acceptance criteria. No Linear milestone was assigned because the available project milestones do not correspond to design Milestone 1.
- Revalidated the second scope against current code: `findLatestAttempts` groups by task, lesson, module, and subtype-derived agent; selects by task-run create time; and only then calls `taskRunIsScored`. This already implements the intended latest-attempt-first shape, though it lacks the proposed deterministic resource-ID tie break.
- Clarified the product consequence: starting a newer retry temporarily makes that module—and potentially the session—incomplete and removes the older result from current score/pass rollups. If the product needs durable achievement/certification, that is a separate metric; silently falling back to an older completion would mix the two semantics.
- Quiz submission currently creates its result and task run transactionally and is immutable, while conversation results are populated asynchronously. The shared classifier still needs both subtypes because 40 of 317 observed staging runs were quiz runs.
- Created and assigned [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/select-the-latest-training-simulator-task-run-before-classifying-its) to `xuanyu.wang`. It is in the Training Simulator project / Convo Intelligence Backlog and is blocked by CONVI-7582. The ticket records the official-attempt ordering, subtype classification matrix, staging evidence, non-goals, and retry-focused acceptance cases.
- After confirming latest-attempt-first is substantially existing behavior, rewrote [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/complete-the-training-simulator-session-reporting-backend) as the complete session-backend correctness ticket: assignment-rooted reads, zero-run coverage, no 1,000-row truncation, attempt counts, explicit conversation/quiz classification, corrected denominators, authorization, deterministic selection, telemetry, and boundary tests.
- Created [CONVI-7584](https://linear.app/cresta/issue/CONVI-7584/complete-the-focused-director-updates-for-training-simulator-session) for the remaining Director-only work. It preserves the existing Figma implementation and limits work to attempt count, N/A/filter behavior, visible ambiguity warnings, unavailable metrics, latest-activity naming, and focused tests. Assigned to `xuanyu.wang`; blocked by CONVI-7583.
