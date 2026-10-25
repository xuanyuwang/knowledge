# Criterion-level use-case model: provisional assessment

- Date: 2026-10-07
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only; local checkout 64 commits behind `origin/main`)
- Design artifact: `deliverables/omni-usecase-scorecard-eng-plan-draft.md`

## User direction

Treat branch-child subset containment as an edge validation rule. Assess whether criterion-level scope in the existing template JSON/Go model is roughly 80% viable, then map Coaching, QM, Performance Insights, and related consumers at a high level. Defer materialized-view design and detailed consumer solutions.

## Assessment

The core model is provisionally ready to pursue. Template all/some and criterion none/all/some can be represented in the versioned JSON, while the typed Go parser provides a normalization point for historical omitted fields. Criteria remain template-owned. Revision-pinned scorecards supply a concrete use case for evaluation. Remaining core proof points are presence-aware JSON parsing, full-write validation, and use-case-filtered scoring with no inapplicable score row.

This is confidence in representational fit, not confidence in rollout size. Consumer work is real: `shared/qa/scorecard_template.go:ListCurrentScorecardTemplateIDs` uses `DISTINCT ON (..., usecase_ids, resource_id)` but returns a map keyed by resource ID; AutoQA traverses `GetAutoScorableItems()` after selecting a template; QM task backfill chooses `UsecaseIds[0]`; Coaching and analytics call the shared latest-template helper; data-platform template/criterion export assigns one `UsecaseId` only for single-use-case templates. Group calibration also parses template JSON after direct SQL. These paths need use-case-aware contracts, with local query changes where necessary.

## Scope of this pass

Reviewed the listed source paths in the local checkout and the existing design notes. No code or database changes were made. The draft now has a provisional confidence assessment and high-level consumer map. Recheck source against the eventual implementation base because this checkout is behind its remote branch.

## Product contract clarified from the PRD

Xuanyu pointed to the Performance Insights requirements, then the full [Omni Use Case QM / Coaching PRD](https://docs.google.com/document/d/1iEYDUg7NrYo5NjeTcr9nHyNqEbv_sWHrDHJeAkFJS9c/edit?tab=t.0#heading=h.i4n1p4ads1o) was read through Glean. Performance Insights supports a multi-select use-case filter and cross-use-case template filter. Its first table displays template and criterion applicability; its second and third tables include per-use-case breakdowns and show N/A when a criterion does not apply to one use case. The page can therefore contain both combined results over selected use cases and separate per-use-case results. N/A is a presentation state, distinct from the agreed evaluation model of no score row. QM Report similarly calls for a selected-use-case roll-up plus task use-case drilldown. This resolves the earlier false choice between combined and separate metrics at a high level; exact formulas remain consumer design work.

## QM task and backfill review gate

Xuanyu confirmed one QM task should span A and B when its template applies to both and asked to review backfill jobs before implementing cross-use-case behavior. The initial source trace found:

- `cron/sync-users/internal/qm-task/backfill_qm_task_config.go` creates one task per template, writes only `template.UsecaseIds[0]` to singular task `UsecaseID`, and skips templates with an empty all-use-cases list.
- `cron/sync-users/internal/scorecard-task-id/backfill_scorecard_task_id.go` matches submitted scorecards using task template ID plus the task's singular `UsecaseID`.
- `temporal/ingestion/backfillscorecards/template_processor.go` checks a template against each conversation's use case but collects all AutoQA criteria through `GetAutoScorableItems()` before processing conversations.

These are concrete migration seams, not an exhaustive backfill inventory. The task scope representation or derivation needs a decision before these jobs can be corrected. Audit remaining task/scorecard/template backfill paths and test A/B plus legacy single-use-case behavior before enabling shared templates.

Xuanyu gave a provisional answer to the next transition question: an existing QM task should automatically follow template use-case additions/removals for future evaluations. Leave the final decision open. Existing scorecards remain historical and revision-pinned. The expected ease of implementation is a hypothesis pending the task schema/backfill review.
