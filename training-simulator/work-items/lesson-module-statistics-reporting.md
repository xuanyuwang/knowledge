# lesson-module-statistics-reporting: Lesson & module level stats

**Status:** active — module and lesson dedicated-API contract branches published; PR metadata and prepared backend/Director implementations still require follow-up
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official tickets:** CONVI-7600 (lesson contract), CONVI-7601 (module contract)
**Last updated:** 2026-09-01

## Objective and Impact

- **Objective:** Define and prepare implementation of lesson- and module-level statistics reporting (UI drawers and aggregates; CSV remains an uncommitted design exploration) on top of existing session-level `RetrieveTrainingSimulatorTaskStats`.
- **Customer/system impact:** Lets coaches diagnose content quality (weak modules/criteria) across assignments; fulfills “module/lesson level coming soon” GTM messaging.
- **Role:** investigated requirements, authored the engineering plan, and prepared isolated source implementations for Milestones 2 and 3

## Scope

**In scope**

- Requirements synthesis from Figma, Linear project deferrals, Slack product answers, and current proto/impl
- Project brief in `deliverables/lesson-module-statistics-reporting.md`
- Open questions for product/design before eng spike

**Non-goals**

- Generated protobuf, GORM, and web-client artifacts
- Production KPI uplift linking (GONG-4179)
- CSV implementation until product confirms commitment, row grain, and semantics

## Source Context

- **Repos:** `cresta-proto`, `go-servers`, `director`
- **Milestone 2 worktrees:** `/Users/xuanyu.wang/repos/cresta-proto-milestone-2`, `/Users/xuanyu.wang/repos/go-servers-milestone-2`, `/Users/xuanyu.wang/repos/director-milestone-2`
- **Milestone 2 branch:** `codex/milestone-2-lesson-reporting`
- **Milestone 2 proto PR:** [cresta-proto#9677](https://github.com/cresta/cresta-proto/pull/9677)
- **Reporting contract tickets:** [CONVI-7600](https://linear.app/cresta/issue/CONVI-7600/add-training-simulator-lesson-reporting-api-contract) for lesson reporting; [CONVI-7601](https://linear.app/cresta/issue/CONVI-7601/add-training-simulator-module-reporting-api-contract) for module reporting
- **PRs/commits:** n/a

## Current Understanding

Session-level reporting is shipped. August Figma defines Session / Module / Lesson reporting sidecards; product has publicly said module/lesson reporting is coming soon. No dedicated Linear implementation ticket was found. Figma explores CSV exports, but no authoritative commitment, spec, or release date was found.

Milestone 2's proto contract has now been rebuilt locally on final module-contract head `97873e3108` around the dedicated ordered batch `RetrieveTrainingSimulatorLessonStats` API. `ListTrainingLessons` is content-only, lesson results use explicit assignment and score-availability denominators, and ordered entries reuse the final `TrainingSimulatorModuleStats` contract. The prepared backend and Director implementations still use the superseded list transport and require rework. See `sessions/2026-08-28/codex-convi-7600-dedicated-lesson-stats.md`.

Milestone 3's proto contract has been reworked locally on `codex/milestone-3-module-reporting` around `RetrieveTrainingSimulatorModuleStats`; `ListTrainingModules` is restored to its pre-PR content-only shape. The local source commit is pending human review and has not been pushed. The prepared backend and Director worktrees still use the superseded list-extension transport and require follow-up rework. See `sessions/2026-08-28/codex-convi-7601-dedicated-module-stats.md`.

On 2026-09-01, two module-contract review comments were addressed: `applicable_score_count` now directly follows `average_applicable_score`, and `time_range` explicitly selects assignments by overlap with the DirectorTask window from create time through due time. The exact predicate is `create_time <= end_timestamp && (due_time unset || due_time >= start_timestamp)`; task-run/result timestamps do not drive the filter. Scoped Buf and Bazel validation passes. Commit `dd6f7086ab` is pushed to `origin/codex/milestone-3-module-reporting`; the review threads remain open. See `sessions/2026-09-01/codex-convi-7601-review-followup.md`.

The shipped frontend/backend path confirms this interpretation. Director's Session tab converts its date-only page filter to one `TimestampRange` and sends it to both the task list and task-stats RPC. The task-stats backend forwards it to `ListDirectorTasks`, selects ACTIVE Training Simulator assignments by create/due-window overlap, and loads all runs for matched tasks without filtering run timestamps. Shipped Lesson/Module screens display the page filter but do not consume it. Two pre-existing inconsistencies remain: `TimestampRange` documents a half-open end while task SQL is end-inclusive, and the existing task-stats request comment says “filtering task runs” despite task-window behavior.

A 2026-09-01 performance review removed nested module/criterion results from the local batched lesson response. Instead, the lesson drawer will lazily call `RetrieveTrainingSimulatorModuleStats` with ordered module names, the current time range, and optional repeated `training_lesson_names`. A nonempty list narrows assignment aggregation to those lessons; when empty, the field is ignored. This avoids building every module of every lesson when no drawer is opened while preserving the standalone Module page's unscoped aggregation. Whether metadata such as `active_lesson_count` is global or lesson-scoped is deliberately deferred until frontend consumption and implementation are finalized. Both local proto worktrees validate; changes remain uncommitted and unpushed. See `sessions/2026-09-01/codex-lazy-lesson-module-stats.md`.

Compatibility follow-up split CodeRabbit's warning into shipped and unshipped contracts. Module PR #9676 merged and its generated artifact was tagged `v2.22.6`, so module-request `time_range = 3` remains stable and new `training_lesson_names` uses field 4 despite no application consumer found on organization default branches. Lesson PR #9677 remains open; `TrainingSimulatorLessonStats` is absent from default branches and retains its contiguous renumbering without reserving removed `modules = 6`. The correction was pushed in commit `85240b0b08`.

The backend and frontend plans are defined in separate canonical deliverables and have been reconciled against the rewritten domain README. The backend reads DirectorTask assignments, current lesson/module definitions, task runs, and conversation-score rows directly and aggregates the latest conversation attempts. On 2026-08-28 the API review selected dedicated `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats` batch RPCs. The former Option 2 list-extension implementations and proto PRs must be adapted without changing the aggregation semantics. The 2026-08-27 product decision accepts failure-equivalent reporting for the listed zero/false timeout, all-criteria-N/A, and failed results, so persisted overall evaluator status/N/A is no longer a prerequisite for that distinction.

## Findings and Decisions

- Treat merged `RetrieveTrainingSimulatorTaskStats` as the session-grain baseline; design-doc `GetTrainingSimulatorStats` is historical.
- Prefer new lesson/module grains rooted in `TrainingSimulatorTaskRun` rather than ClickHouse for v1 unless scale requires it.
- Preserve current semantics: latest module attempt is official, lesson score is the simple average of required module scores, all modules must pass, and criterion-level N/A is counted separately but excluded from the applicable/pass-rate denominator. Overall all-criteria-N/A zero/false is failure-equivalent under the 2026-08-27 decision.
- Track CSV as an open product decision rather than an implementation requirement.
- Use dedicated `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats` batch APIs. Keep `ListTrainingLessons` and `ListTrainingModules` content-only. The accepted extra frontend request is mitigated through batching, caching, and inactive-tab prefetch and must be measured during implementation.
- Use current lesson/module definitions for aggregation. Revision-exact historical reporting and assignment snapshots are important future work but are not part of this project.
- `go-servers/main` moved conversation evaluation results to `training_simulator_conversation_scores`; the tracked task-run schema no longer contains the obsolete pre-migration result/agent columns, so the new reader joins the current conversation result store directly without inventing a task-run fallback.
- The existing task-run composite index is likely sufficient for a task-scoped query; do not add lesson/module indexes until an `EXPLAIN ANALYZE` spike proves the need.
- Content-level aggregate RPCs should not copy the existing `AGENT` role. Start with the admin/QA-admin Lesson Configuration surface; manager access requires a separate reporting route and explicit manageable-user authorization.
- Director can reuse the current filter state, table placeholders, `FullDrawer`, and reporting patterns, but must batch by content name rather than issue per-row requests.
- Per reviewer direction, do not add a shared `TrainingSimulatorStatsFilter`. The dedicated module request owns only `time_range`; it has no agent, group, or team selector because current Figma shows one population-wide aggregate per module and no assignee filter on that surface. No `include_stats` flag is needed. Lesson reporting reuses `TrainingSimulatorModuleStats`, so its proto PR remains stacked on the module-contract PR.
- Preserve current DirectorTask time semantics: `time_range` selects assignment windows that overlap the requested range, using task create time as the window start and due time as the end. An absent due time is open-ended; attempt and result timestamps are not filters.
- Treat the current Session frontend/backend path as the compatibility baseline for Lesson/Module statistics. Do not infer task-run filtering from the stale `RetrieveTrainingSimulatorTaskStatsRequest` comment.
- The module report reuses the Date Range UI but not the Assignee selection. Add agent/group/team filtering later only if Product explicitly adds that filter to the module-reporting surface.
- Reporting must include `ACTIVE` and `ARCHIVED` assignments and exclude `DRAFT`/`DELETED`, pending product confirmation, so archival does not erase history.
- Quiz statistics are excluded because quiz is not yet mature enough to define reliable reporting semantics.
- Do not add overall `evaluation_status` or `not_applicable` solely to distinguish the accepted zero/false timeout, all-criteria-N/A, and failed cases. Preserve criterion-level N/A exclusion.
- Rebaseline missing-result warning behavior under the collapsed overall semantics; do not retain warnings whose only purpose was distinguishing the three accepted zero/false cases.
- No result-state DB schema change is currently required for lesson/module reporting under the accepted collapsed overall semantics. The contracts use direct per-request reporting fields; lesson results reuse the module/criterion statistics messages. Add an index only if representative query measurements justify it.
- Published review artifact: [Training Simulator Lesson and Module Statistics — Engineering Design](https://docs.superhuman.com/d/_dE0dCcz8Bub/_subF07cy).

### API boundary review

Closed on 2026-08-28 with Option 3 selected. [The Slack-backed decision](../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md) records why cleaner responsibility, convention, and discoverability outweigh avoiding one batched frontend request. The dedicated RPCs must define independent authorization, failure, caching, and measurable latency behavior.

## Blockers and Dependencies

- Product answers needed for Insights vs Lesson Configuration placement, release scope, and the “Active lessons” metric on the lesson drawer.
- Full document bodies were not exposed through the plugin search path; indexed Glean results were vetted and linked, with reduced confidence where metadata/content was partial.
- Figma MCP hit View-seat rate limit mid-session after primary screenshots were captured.
- Historical calculations can change after lesson/module edits because this project intentionally reads current content. Revision pinning and assignment snapshots remain known future work, not blockers.
- Result classification must be revalidated against the 2026-08-27 collapsed semantics, including the still-unspecified case of a timed-out snapshot with partial applicable results or a nonzero score.

## Validation and Rollout

- The canonical draft now organizes implementation into three independently reviewable product milestones: complete session reporting, add lesson reporting, then add module reporting.
- All milestones need one internal official-attempt primitive, but the session review corrected its contract: select the latest attempt first, then classify that attempt as settled/applicable/N/A/incomplete. Do not fall back past a newer unfinished retry. Session completion must also retain quiz outcomes while quiz modules remain assignable.
- The earlier 18-ticket breakdown is superseded. The simplified plan suggests seven coarse backend/frontend tickets and keeps result-state correctness, authorization, bounded reads, telemetry, performance measurement, and staging reconciliation as acceptance criteria inside each milestone.
- The focused Milestone 1 review found that Director already implements most of the Figma session dashboard/table/drawer, so frontend scope should be limited to attempt count, complete-all-N/A behavior, warning presentation, and corrected aggregates. Read-only staging aggregates validated the zero-run, retry, and N/A gaps and showed that quiz runs are material session inputs. Evidence: `deliverables/milestone-1-session-reporting-review.md`.

## Next Actions

1. Update #9677's PR description and review threads to match the published dedicated lesson-statistics contract.
2. Complete any remaining #9676 PR-description/review-thread follow-up.
3. Publish the locally applied lazy lesson-drawer contract: remove nested lesson module stats and add repeated lesson scope to the module stats request.
4. Rework the prepared backend and Director implementations to call the dedicated batch APIs; preserve content loading on reporting failure and add caching/prefetch.
5. Validate cold/cached tab loading, query bounds, authorization, and representative result reconciliation before release.

## Timeline

- 2026-08-11 — Collected Figma/Linear/Slack/proto context; wrote project brief. Evidence: `deliverables/lesson-module-statistics-reporting.md`, `sessions/2026-08-11/cursor-lesson-module-stats-docs.md`, `log/2026-08-11.md`.
- 2026-08-11 — Corrected the brief after three Glean plugin enterprise-search passes: added vetted PRD/design/customer/implementation sources, clarified current metric semantics, and downgraded CSV from requirement to uncommitted exploration.
- 2026-08-11 — Defined the backend read model, aggregation grains, assignment-snapshot prerequisite, query/index strategy, and additive proto/RPC proposal after inspecting authoritative August 10 `origin/main` refs.
- 2026-08-13 — Revalidated the plan against current `go-servers`, `cresta-proto`, and `director` main checkouts; corrected the backend for normalized conversation/quiz score storage and produced a concrete FE integration design. Evidence: `deliverables/lesson-module-statistics-eng-design.md`, `deliverables/lesson-module-statistics-fe-design.md`, `sessions/2026-08-13/codex-lesson-module-stats-design.md`.
- 2026-08-16 — Reconciled both plans against the rewritten domain rules; added immutable snapshot/exact-revision evaluation, explicit result state, legacy-warning, archived-history, and quiz-unavailable contracts; published the combined design to Superhuman Docs. Evidence: `deliverables/lesson-module-statistics-eng-design.md`, `deliverables/lesson-module-statistics-fe-design.md`, `sessions/2026-08-16/codex-lesson-module-stats-plan-publication.md`.
- 2026-08-17 — Read all four Superhuman review comments, traced reuse/complexity patterns in analytics proto/backend/Director call sites, and proposed a three-option API comparison with a preliminary recommendation to retain separate lesson/module stats RPCs plus prefetch/cache latency mitigation. Evidence: `sessions/2026-08-17/codex-stats-api-design-review.md`.
- 2026-08-18 — Reorganized the local API design into one canonical chapter, finalized option 3, split the proto examples by responsibility, and linked the frontend loading contract instead of duplicating the decision across artifacts.
- 2026-08-20 — Reopened the API boundary after reviewer discussion. Refactored the local API design into three equal option sections; option 2 is the current preference and option 3 retains the cleanest boundaries.
- 2026-08-23 — Reviewed the PDF-exported comments, then used Superhuman Docs URI support to update the live design and reply to all 12 active review threads. Added complete shared-message protos, clarified filter reuse and active-lesson semantics, renamed the legacy warning, removed the redundant passing-score field, and left threads open for reviewer confirmation.
- 2026-08-24 — Converted the approved local design into a dependency-aware 18-ticket implementation plan. Option 2 is the working transport assumption; the API/product contract remains the first gate. Evidence: `deliverables/lesson-module-statistics-ticket-plan.md`, `sessions/2026-08-24/codex-statistics-ticket-plan.md`.
- 2026-08-24 — Superseded the 18-ticket breakdown with a simpler three-milestone model: prove the shared assignment/latest-settled-result foundation through session reporting, then deliver lesson and module reporting separately. The draft now suggests seven coarse tickets and keeps cross-cutting release checks inside each milestone.
- 2026-08-26 — Reviewed post-session proto and storage needs. Milestones 2–3 need additive lesson/module statistics contracts but no planned schema change after Milestone 1's result-state persistence; index changes remain measurement-driven. Evidence: `sessions/2026-08-26/codex-post-session-proto-db-needs.md`.
- 2026-08-26 — Implemented independent lesson/module reporting proto sources and direct per-request reporting fields in the local `cresta-proto` checkout, following offline reviewer direction to omit `TrainingSimulatorStatsFilter`. Bazel and Buf validation passed; no generated code was committed.
- 2026-08-26 — Implemented Milestone 2 lesson reporting in dedicated proto/backend/Director worktrees. The assignment-rooted reader preserves never-started agents, uses latest-attempt-first classification, returns aligned lesson/module rates, and keeps content usable on reporting failure. Handwritten proto checks pass; generated-contract tests plus representative query/page-load and environment reconciliation remain blocked release gates. Evidence: `sessions/2026-08-26/codex-milestone-2-lesson-reporting.md`.
- 2026-08-24 — Reviewed Milestone 1 only against current code, Figma, and aggregate staging data. Corrected the shared primitive to latest-attempt-first classification, identified quiz outcomes as a hard session dependency, narrowed net-new Director scope, and made authorization plus all-N/A/warning behavior explicit gates. Evidence: `deliverables/milestone-1-session-reporting-review.md`, `sessions/2026-08-24/codex-milestone-1-session-reporting-review.md`.
- 2026-08-24 — Applied the Milestone 1 review to the local Superhuman design draft: corrected definitions, session API changes, milestone scope/exit criteria, staging evidence, decision gates, ticket boundaries, and open gates without expanding the later reporting milestones.
- 2026-08-24 — Created [CONVI-7582](https://linear.app/cresta/issue/CONVI-7582/persist-evaluation-status-and-overall-na-on-training-simulator) for the result-persistence foundation and revalidated the current latest-attempt-first behavior. A newer unfinished retry remains official and temporarily makes current session state incomplete; historical achievement is a separate possible metric, not an older-result fallback.
- 2026-08-24 — Created [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/select-the-latest-training-simulator-task-run-before-classifying-its) for deterministic latest-task-run-first conversation/quiz classification, assigned to `xuanyu.wang`, blocked by CONVI-7582.
- 2026-08-25 — Corrected the ticket boundary: expanded [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/complete-the-training-simulator-session-reporting-backend) into the full session-backend correctness work and created [CONVI-7584](https://linear.app/cresta/issue/CONVI-7584/complete-the-focused-director-updates-for-training-simulator-session) for only the remaining Director deltas. Dependency order: CONVI-7582 → CONVI-7583 → CONVI-7584.
- 2026-08-26 — Implemented Milestone 3 module reporting in dedicated proto/backend/Director worktrees without generated code. Handwritten proto validation passed; end-to-end compilation, focused tests, and live query/page-load measurements remain blocked on normal generation and worktree dependencies. Evidence: `sessions/2026-08-26/codex-milestone-3-module-reporting.md`.
- 2026-08-26 — Rebased lesson-reporting [PR #9677](https://github.com/cresta/cresta-proto/pull/9677) and module-reporting [PR #9676](https://github.com/cresta/cresta-proto/pull/9676) onto current `main`. Together with evaluation-result persistence [PR #9656](https://github.com/cresta/cresta-proto/pull/9656), all three target `main`, contain independent diffs, and cross-link as one review series.
- 2026-08-26 — Created [CONVI-7600](https://linear.app/cresta/issue/CONVI-7600/add-training-simulator-lesson-reporting-api-contract) and [CONVI-7601](https://linear.app/cresta/issue/CONVI-7601/add-training-simulator-module-reporting-api-contract) as focused proto-contract tickets, assigned to Xuanyu in the Training Simulator project and related to CONVI-7582. Updated PRs #9677 and #9676 to carry the new identifiers.
- 2026-08-27 — Superseded CONVI-7582 as a reporting prerequisite after Product accepted the listed zero/false timeout, all-criteria-N/A, and failed outcomes as failure-equivalent. Criterion-level N/A remains excluded from score calculation. The three implementation PRs were closed with decision comments; Linear cleanup remains pending. Evidence: `decisions/2026-08-27-collapse-overall-zero-false-results.md`.
- 2026-08-27 — Addressed both review threads on module-contract [PR #9676](https://github.com/cresta/cresta-proto/pull/9676): verified Gazelle output, documented criterion ordering, resolved both threads, applied repository formatting, and aligned the request/response with API lint. Retained passed and failed criterion counts as the explicit applicable denominator; defer an additive N/A count until Product needs N/A prevalence. Evidence: `sessions/2026-08-27/codex-milestone-3-proto-review.md`.
- 2026-08-28 — Superseded the prior passed/failed contract after reviewer discussion. Module reporting now exposes `passed_count` and assignment-rooted `total_count`; criteria expose only `passed_count` and share the module total. The residual is deliberately broad non-pass state, and redundant pass-rate fields were removed. Group/team reporting filters were removed from both #9676 and #9677. Evidence: `sessions/2026-08-26/codex-milestone-3-module-reporting.md`.
- 2026-08-28 — Reused `TrainingSimulatorModuleStats` inside lesson results per reviewer direction, removed `TrainingSimulatorModulePassRate`, and stacked [PR #9677](https://github.com/cresta/cresta-proto/pull/9677) on [PR #9676](https://github.com/cresta/cresta-proto/pull/9676). The ordered lesson chips now derive from the shared `passed_count / total_count` contract. Replied to and resolved the review thread.
- 2026-08-28 — Reproduced #9676's go-servers generation check against its synthetic merge ref. The module contract produced no Training Simulator generated diff; the failures were unrelated converter incompatibilities already covered by [go-servers #31683](https://github.com/cresta/go-servers/pull/31683). Cross-linked both PRs and acknowledged the proto readiness checkbox rather than creating a duplicate companion. Evidence: `sessions/2026-08-28/codex-module-proto-go-servers-companion.md`.
- 2026-08-28 — Rebased [#9676](https://github.com/cresta/cresta-proto/pull/9676) onto current cresta-proto `origin/main` and rebuilt stacked [#9677](https://github.com/cresta/cresta-proto/pull/9677) directly on the updated module branch. Both retain their exact reviewed file-level patches; targeted Buf and Bazel validation passed. Evidence: `sessions/2026-08-28/codex-module-proto-go-servers-companion.md`.
- 2026-08-28 — Closed the API boundary review in favor of dedicated lesson/module statistics RPCs after the Slack review concluded that cleaner responsibility, convention, and discoverability outweigh avoiding one batched frontend request. Evidence: `decisions/2026-08-28-dedicated-lesson-module-stats-apis.md`, `deliverables/milestone-2-3-dedicated-stats-proto-prompts.md`.
- 2026-08-28 — Added `non_applicable_result_count` to each criterion statistic after product follow-up. It counts explicit completed criterion N/A results but remains outside `applicable_result_count` and the criterion pass-rate denominator. The change is local pending review.
- 2026-08-28 — Rebuilt #9677 locally as one CONVI-7600 lesson-only commit on final #9676 head `97873e3108`. Restored `ListTrainingLessons`, added the ordered admin-only `RetrieveTrainingSimulatorLessonStats` batch API, aligned lesson count/score availability fields with the final module contract, and preserved ordered shared module entries including explicit quiz zero/default behavior. Required local validation passed; push and GitHub metadata remain gated on review. Evidence: `sessions/2026-08-28/codex-convi-7600-dedicated-lesson-stats.md`.
- 2026-08-28 — Force-pushed the approved rewritten #9677 branch from old remote head `37608b3499` to `6fa5404349` with an explicit lease. PR metadata remains unchanged pending follow-up.
- 2026-09-01 — Addressed [#9676 review comment r3899077991](https://github.com/cresta/cresta-proto/pull/9676#discussion_r3899077991) by grouping the score availability count with its average. Field numbers and semantics are unchanged; scoped Buf/Bazel checks pass.
- 2026-09-01 — Addressed [#9676 review comment r3900412776](https://github.com/cresta/cresta-proto/pull/9676#discussion_r3900412776) by defining `time_range` as overlap with the assignment's DirectorTask create-time-through-due-time window. Pushed both review follow-ups in commit `dd6f7086ab`; the threads remain open.
- 2026-09-01 — Traced the shipped Director date filter through `ListDirectorTasks` and `RetrieveTrainingSimulatorTaskStats`: both use the same frontend range, and the backend filters ACTIVE assignment windows before loading unbounded-by-time task runs. Confirmed the #9676 wording matches current behavior; recorded the shared half-open/inclusive and stale-comment inconsistencies for follow-up.
- 2026-09-01 — Replaced eager nested module statistics in the local lesson response with a lazy module statistics request carrying optional repeated lesson scope. Deferred metadata scoping details; both proto worktrees validate and remain uncommitted. Evidence: `sessions/2026-09-01/codex-lazy-lesson-module-stats.md`.
