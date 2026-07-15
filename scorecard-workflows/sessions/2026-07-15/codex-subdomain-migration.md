# Scorecard Workflows Subdomain Migration

## Objective

Reorganize broad Scorecard Workflows knowledge into durable workflow references without fragmenting ticket execution history.

## Source Context

- Knowledge repo: `/Users/xuanyu.wang/repos/knowledge`
- Primary source repo named by the domain: `/Users/xuanyu.wang/repos/go-servers`
- Branch context: `main`

## Result

- Created seven subdomains for template authoring/versioning, evaluation/scoring, lifecycle, permissions/visibility, appeals, group calibration, and process scorecard generation.
- Kept operating artifacts at the parent domain and retained one optional primary-subdomain field for future work items.
- Added a legacy source index mapping existing deep references and tickets to their canonical durable homes.
- Preserved the boundary with `scorecard-data-sync`: workflow eligibility and generation live here; PG/CH projection, ordering, drift, and reindex live there.

## Follow-up

- Add migration banners to the retained source folders.
- Expand state-transition, scoring-field, permission, and generation decision matrices as future tickets validate behavior.
