# Scorecard Template System Knowledge

**Created:** 2026-03-26
**Updated:** 2026-05-07

## Overview

This folder is the project home for understanding scorecard templates as a system.

It covers more than the template JSON schema. The topic includes:

- template authoring in Director
- option, score, and AutoQA wiring
- score computation semantics
- Postgres storage and revisioning
- ClickHouse projection and analytics consumption
- operational edge cases such as process scorecards, N/A behavior, and template duplication

## Start Here

If you only read one document, read:

- `deliverables/scorecard-template-system-reference.md`

That document is the distilled foundation for teammates and AI tools.

## Distilled Deliverables

- `deliverables/scorecard-template-system-reference.md` - Canonical system reference and mental model

## Working Notes and Deep Dives

- `template-structure.md` - Template schema, hierarchy, scoring flow, and storage model
- `criterion-options.md` - Option, score, and criterion-setting semantics
- `fe-template-builder.md` - Builder UI and save-transform behavior
- `na-score-design.md` - N/A modeling and system implications

## Cross-Project Supporting Artifacts

The complete understanding of scorecard templates is spread across multiple project folders. The most relevant supporting docs are:

- `nascore/README.md`
- `nascore/options-scores-lifecycle.md`
- `convi-6709-reversed-scorecard/README.md`
- `convi-6709-reversed-scorecard/exact-flow.md`
- `backfill-scorecards/README.md`
- `pg-ch-scorecard-sync-investigation/scorecard-specific-constraints.md`
- `duplicate-template-across-usecase/investigation.md`

## Key Source Locations

| Area | Path |
|------|------|
| Template Go types & parsing | `go-servers/shared/scoring/scorecard_templates.go` |
| Score calculation algorithm | `go-servers/shared/scoring/scorecard_calculator.go` |
| Criterion percentage scoring | `go-servers/shared/scoring/scorecard_scores_dao.go` |
| Auto-QA scoring | `go-servers/shared/scoring/autoqa_scoring.go` |
| Template DB operations | `go-servers/shared/qa/scorecard_template.go` |
| Analytics QA score stats | `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` |
| ClickHouse score indexing | `go-servers/shared/clickhouse/conversations/conversation.go` |
| Proto definition | `cresta-proto/cresta/v1/coaching/scorecard_template.proto` |

## Project Structure

- `project.yaml` - machine-readable project state
- `deliverables/` - polished canonical documents
- `log/` - daily progress log
- `sessions/` - richer session-level notes

## Log History

| Date | Summary |
|------|---------|
| 2026-05-07 | Created canonical scorecard-template system reference and reorganized this folder around distilled deliverables |
| 2026-03-26 | Initial exploration: template structure, scoring algorithm, analytics pipeline |
