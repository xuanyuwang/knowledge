# Omni-use-case full engineering plan

- **Date:** 2026-11-04 (America/Toronto).
- **Primary domain / subdomain:** Scorecard Workflows / Template Authoring and Versioning.
- **Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`.
- **Branch/worktree context:** existing checkout unchanged; reviewed fetched `origin/main` at `01c0d818b7793656005500f84efb2d1b69aa54bd` using git object reads. No new source or knowledge worktree.
- **Related source snapshots:** Director `a03e8e6597da5619fdbeb4e2b29120147ca1c2a1`; cresta-proto `1938bb241393f0f54e5dd8242f03ba1a6617728e`, both freshly fetched `origin/main` without switching branches.
- **Canonical work item:** [omni-usecase-scorecard-plan](../../work-items/omni-usecase-scorecard-plan.md).

## Request and inputs

Expand the existing draft into a full company-template engineering plan, with focused questions on core choices and no product changes. Read `/Users/xuanyu.wang/.codex/attachments/4faa326e-c6bd-43b6-8f7b-28bc6c14dcb5/Pasted text.txt`, existing long draft, P0 sketch, domain README/project, operating model/domain model, daily-capture skill, and relevant October 4–8 design sessions/decision. Quick memory lookup located the omni-use-case design registry; repository records and refreshed source supplied current evidence.

Relevant prior records: October 5 canonical model, write contract, three-state JSON, scope transitions and validity; October 7 full-Commit precedence/branch containment; October 8 AutoQM mapping and October 9 correction, QM scope decision, PI and QM report query studies. The template-level NULL/NONE experiments are superseded; template columns keep ALL/some semantics. The materialized-view proposal remains deferred.

## Questions and confirmed answers

1. Uncovered AutoQM use case: allow manual scoring and display coverage gap; skip ineligible rule before absent-evidence/N/A interpretation.
2. B-only report of A+B task-wide quota: filtered evaluated count, completion N/A, whole-task quota labeled separately.
3. Finite→ALL template: allow explicit reset confirmation; reset all criteria, including NONE, to ALL.
4. New overlapping AutoQM mappings: reject overlap, including global-plus-specific.
5. New AutoQM evaluation rule scope: use current Opera scope; preserve historical stored scores.

Recorded as [November 4 decisions](../../decisions/2026-11-04-omni-usecase-autoqm-and-reporting.md). V3, derived task scope, grouping response, and lock mechanism remain proposed engineering choices.

## Source ledger — verified November 4

All links below are pinned to the fetched source commits; no deployment claim follows.

| Seam | Current evidence |
| --- | --- |
| Existing storage | [director schema](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/sql-schema/director/director-schema.sql): both template tables have nullable arrays/JSON; scorecards have revision/use case and task IDs. |
| Typed structure and version gate | [parser](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/scoring/scorecard_template_parser.go) accepts V1/V2; [typed model](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/scoring/scorecard_templates.go) has no criterion scope; [DAO](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/scoring/scorecard_template_dao.go) has strict version-2 checks near 179 and 529. |
| Commit and persistence | [Commit](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/coaching/action_commit_scorecard_template.go) loads old by incoming scope and compares expected revision before insert transaction; [transformers](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/coaching/transformers.go) validates parsed JSON then assigns original templateJSON to DB record. |
| Current discovery | [shared/qa template helper](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/qa/scorecard_template.go) filters scope before DISTINCT ON including usecase_ids. |
| Score mapping | [calculator](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/scoring/scorecard_calculator.go): no-row criteria skipped; mapScoresByCriterion tests membership in full template, no criterion-use-case field available. |
| Historical structure / current permissions | [GetScorecard](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/coaching/action_get_scorecard.go) uses stored TemplateRevision; [permission inputs](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/coaching/action_evaluate_scorecards_permissions.go) loads LatestRevisionName. |
| Trigger authoring | [TemplateBuilderAutoQA](https://github.com/cresta/director/blob/a03e8e6597da5619fdbeb4e2b29120147ca1c2a1/packages/director-app/src/features/admin/coaching/template-builder/configuration/TemplateBuilderAutoQA/TemplateBuilderAutoQA.tsx): first trigger read, zero/one written; [scoring model](https://github.com/cresta/director/blob/a03e8e6597da5619fdbeb4e2b29120147ca1c2a1/packages/director-api/src/types/models/scoring.ts) validates first trigger; [picker hook](https://github.com/cresta/director/blob/a03e8e6597da5619fdbeb4e2b29120147ca1c2a1/packages/director-app/src/hooks/coaching/useAutoQATriggers.ts) uses default page use-case filter. |
| Opera authoring / scope | [ReviewPolicySection](https://github.com/cresta/director/blob/a03e8e6597da5619fdbeb4e2b29120147ca1c2a1/packages/director-app/src/features/opera/policy-editor/ReviewPolicySection.tsx) gates picker on enableGlobalOperaRule; [policy DB builder](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/policy/policy_db_model_builder.go) propagates scope to behavior models; [filters](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/policy/policy_filters.go) includes all-usecases sentinel. Flag deployment/population not checked. |
| AutoQM runtime | [evidence action](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/apiserver/internal/autoqa/action_calculate_conversation_autoscoring_evidence.go) passes GetAutoScorableItems; [scorer](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/shared/scoring/autoqa_scoring.go) combines trigger evidence without criterion/rule use-case selection; empty evidence can produce NOT_APPLICABLE. |
| Jobs | [task config](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/cron/sync-users/internal/qm-task/backfill_qm_task_config.go): skips len==0 / copies first ID; [task ID](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/cron/sync-users/internal/scorecard-task-id/backfill_scorecard_task_id.go): singular task.UsecaseID; [scorecard backfill](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/temporal/ingestion/backfillscorecards/template_processor.go): entire GetAutoScorableItems. |
| QM report | [stats](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go): task singleton discovery and selected-use-case predicate missing in fetchScorecardsForPeriod / fetchScorecardsCountGroupedByFrequency; no-task time-range path already accepts list. [manual hook](https://github.com/cresta/director/blob/a03e8e6597da5619fdbeb4e2b29120147ca1c2a1/packages/director-app/src/components/qa/hooks/useRetrieveManualQAStats.ts) overrides list with current singleton. |
| Suggestions / appeals | [suggestion](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/analyticsimpl/suggest_manual_qa_conversations.go) sends singleton to template/period/progress and fills template use cases only on empty task filter; [appeals](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/analyticsimpl/retrieve_appeal_stats.go) requires exactly one use case near 851. |
| Other direct consumers | [calibration stats](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go) uses singular task scope and pinned template revision; [export converter](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/dataplatform/postgres/converter/converter.go) emits singleton use case only if len==1. |
| PI query | [current ClickHouse builder](https://github.com/cresta/go-servers/blob/01c0d818b7793656005500f84efb2d1b69aa54bd/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go) includes newer conversation-version logic, both query paths, parent source-version join; preserve these while adding use-case grouping. |
| API | [template](https://github.com/cresta/cresta-proto/blob/1938bb241393f0f54e5dd8242f03ba1a6617728e/cresta/v1/coaching/scorecard_template.proto): Struct + repeated names; [Commit](https://github.com/cresta/cresta-proto/blob/1938bb241393f0f54e5dd8242f03ba1a6617728e/cresta/v1/coaching/coaching_service.proto): full message/no FieldMask; [analytics service](https://github.com/cresta/cresta-proto/blob/1938bb241393f0f54e5dd8242f03ba1a6617728e/cresta/v1/analytics/analytics_service.proto): suggestion singleton field 8, request currently ends at field 10; [QA stats](https://github.com/cresta/cresta-proto/blob/1938bb241393f0f54e5dd8242f03ba1a6617728e/cresta/v1/analytics/qa_stats.proto): repeated filter but grouping ends at enum value 10; [task](https://github.com/cresta/cresta-proto/blob/1938bb241393f0f54e5dd8242f03ba1a6617728e/cresta/v1/coaching/task.proto): singular usecase_name field 13. Proposed additions are not allocated. |

## Result and limits

Replaced the existing draft in place with a full company-format plan, preserving its canonical URL. It now contains model examples, semantics/transitions, all five P0 designs, API/storage/security/monitoring/SLO/tests/debt/cost/release/review sections, explicit recommendations, and pinned source evidence. Rough 19–29 SWE-week effort is unstaffed and unmeasured. No production benchmark, database census, deployment or flag state was established. Existing October sample results remain dated feasibility evidence only.

Used brainstorming for design refinement and daily-capture for closure. No subagents, product code, commits, pushes, external posts, database access, or production mutations. Git authenticated itself; no credential/profile was explicitly selected, and no AWS/Okta/Azure/SSH credential entries were read or passed to another tool.

## Validation

Verification completed on November 4:

- All 16 company-template headings are present in the requested order.
- Relative Markdown links resolve across all eight edited/new Markdown artifacts.
- Fenced JSON examples parse; main-plan fences are balanced; source-reference labels resolve.
- The illustrative Go snippet parses with `gofmt -e` in a temporary `/tmp` file. This verifies snippet syntax, not an implemented normalizer.
- Project YAML syntax is checked with Ruby Psych; canonical work-item/artifact paths are checked separately.
- `git diff --check -- scorecard-workflows` and an explicit whitespace scan of new/untracked artifacts pass after removing trailing header spaces.

SQL/protobuf examples are explicitly illustrative, not production migrations or allocated contracts. They were reviewed as excerpts and were not executed/compiled. No product runtime tests, performance benchmark, or deployment checks were run.

## Next actions

Engineering review of V3/derived-task/grouping/concurrency proposals; reader/writer/job/data inventory; owner assignment and security/API/DB review; performance and cost baselines. No implementation requested.
