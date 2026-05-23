# Scorecard & Template Working Reference

**Created:** 2026-03-26
**Updated:** 2026-05-17

## Overview

This folder is the project home for understanding scorecard and template as a business-rule-heavy system in coaching.

The goal is not to capture every detail at once. The goal is to maintain the best working reference for the domain so future feature work, bug investigation, and design discussion do not restart from zero.

It covers more than the template JSON schema. The topic includes:

- template authoring in Director
- option, score, and AutoQA wiring
- score computation semantics
- Postgres storage and revisioning
- ClickHouse projection and analytics consumption
- operational edge cases such as process scorecards, N/A behavior, and template duplication
- the relationship between template, scorecard, and surrounding coaching concepts
- recurring business rules, historical constraints, and system sharp edges

## Start Here

If you are starting fresh, read these in order:

- `deliverables/scorecard-template-domain-skeleton.md`
- `deliverables/scorecard-template-concept-map.md`
- `deliverables/template-lifecycle.md`
- `deliverables/scorecard-lifecycle.md`
- `deliverables/business-rules-catalog.md`
- `deliverables/ticket-pattern-log.md`
- `deliverables/scorecard-template-working-reference-project.md`
- `deliverables/scorecard-template-system-reference.md`

The domain skeleton is the starting framework. The concept map turns that framework into a first concrete model. The template lifecycle and scorecard lifecycle describe the two main moving artifacts in the domain. The business-rules catalog organizes the repeated rules, and the ticket-pattern log captures recurring patterns from real work. The project brief explains how this reference should grow. The system reference is the deeper distilled foundation.

## Distilled Deliverables

- `deliverables/scorecard-template-domain-skeleton.md` - Minimal framework for organizing the domain
- `deliverables/scorecard-template-concept-map.md` - First populated map of the main concepts and relationships
- `deliverables/template-lifecycle.md` - Lifecycle of the reusable template definition from authoring to historical interpretation
- `deliverables/scorecard-lifecycle.md` - Runtime lifecycle from instantiation through analytics projection and repair
- `deliverables/business-rules-catalog.md` - First organized catalog of repeated rules by lifecycle stage
- `deliverables/ticket-pattern-log.md` - Seeded log of recurring patterns surfaced by ticket work
- `deliverables/scorecard-template-working-reference-project.md` - Project brief, scope, and working method
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

## Working Method

This project should grow from real ticket work.

For each new scorecard/template investigation, try to capture:

- the local issue
- the concepts involved
- the lifecycle stage involved
- the rule discovered or clarified
- the sharp edge or historical constraint
- whether the pattern points to a doc gap, naming gap, model gap, API gap, test gap, observability gap, or ownership gap

The point is to turn repeated ticket pain into a better domain model over time.

## Log History

| Date | Summary |
|------|---------|
| 2026-05-17 | Added template lifecycle, business-rules catalog, and ticket-pattern log to complete the first working-reference stack |
| 2026-05-17 | Added a scorecard lifecycle document covering instantiation, scoring, persistence, projection, and repair |
| 2026-05-17 | Added a first-pass scorecard/template concept map to bridge the skeleton and the system reference |
| 2026-05-17 | Reframed this folder as the Scorecard & Template Working Reference and added the domain skeleton plus project brief |
| 2026-05-07 | Created canonical scorecard-template system reference and reorganized this folder around distilled deliverables |
| 2026-03-26 | Initial exploration: template structure, scoring algorithm, analytics pipeline |
