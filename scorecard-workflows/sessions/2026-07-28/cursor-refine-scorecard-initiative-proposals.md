# Refine Scorecard Configuration Safety Proposals

**Date:** 2026-07-28
**Tool:** Cursor
**Primary domain:** Scorecard Workflows
**Primary subdomain:** Template Authoring and Versioning
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`

## Objective

Refine the two scorecard-configuration initiative proposals using the author's original draft text and the customer context from CONVI-7238.

## New Evidence

- Initiative 1 is motivated by the manual, tedious, and error-prone assignment of criterion weights and answer scores. A concrete failure mode is assigning zero weight to every scored criterion, causing every scorecard to have an N/A aggregate.
- The intended preview includes both score calculation and downstream presentation in Performance Insights and Leaderboard.
- AI-assisted authoring may accept typed or spoken requirements, generate weights and answer scores, and automatically simulate the generated template.
- Initiative 2 is motivated by configuration mistakes discovered only after many historical scorecards have been created. Correcting the template protects only future scorecards.
- In confirmed configuration-error cases, carrying historical answers forward and re-evaluating them under corrected business rules can preserve valuable data and save customer and Cresta effort.
- [CONVI-7238](https://linear.app/cresta/issue/CONVI-7238/united-leaderboard-manager-scorecards-evaluated-undercount-vs) provides the concrete example: every scored criterion had weight zero, producing N/A aggregates that were excluded from part of Leaderboard reporting.

## Refinements

- Renamed the first proposal to **AI-Assisted Template Creation and Scorecard Simulation**.
- Made simulated scorecards and downstream analytics previews explicit.
- Added typed, spoken, and document-based input as potential AI authoring modes.
- Renamed the second proposal to **Re-Evaluate Historical Scorecards After Template Corrections**.
- Made preservation of valuable historical answers the primary benefit.
- Distinguished an auditable re-evaluation from silently changing a scorecard's revision pointer.
- Kept detailed policy for submitted scorecards, overrides, appeals, calibration, and analytics as future product discovery.

## Output

- Updated [Scorecard Configuration Safety: Initiative Proposals](../../deliverables/scorecard-configuration-safety-initiative-proposals.md).
