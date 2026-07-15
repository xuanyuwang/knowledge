# Analytics Domain

## Purpose

The canonical knowledge home for Performance Insights and Leaderboard across frontend and backend. It should make every displayed chart, table, filter, and metric traceable from UI interpretation through request construction, analytics-service behavior, query/aggregation semantics, and source data.

## Scope and Boundaries

**In scope**

- Performance Insights and Leaderboard pages in Director
- exact chart, table, score, count, ranking, and grouping semantics
- shared and page-specific filter interpretation
- analytics API request/response contracts
- insights/analytics-service handlers and aggregation/query behavior
- frontend transformations, fallbacks, and display rules
- user/team/group resolution used by these surfaces
- ClickHouse sources and data lineage needed to explain displayed values
- operational diagnosis of mismatches, missing values, and inconsistent counts

**Out of scope**

- scorecard lifecycle and review workflows: `scorecard-workflows`
- PG-to-ClickHouse scorecard projection consistency: `scorecard-data-sync`
- notification delivery: `notifications`

## Knowledge Standard

For each UI element, the domain should eventually record:

1. user-visible meaning and edge cases;
2. applicable filters and their exact interpretation;
3. frontend component and transformation path;
4. API and request fields;
5. backend handler, grouping, calculation, and query path;
6. source tables/events and time/identity attribution;
7. known discrepancies, tests, and operational checks.

## Initial Knowledge Map

- Performance Insights surface catalog
- Leaderboard surface catalog
- shared analytics API catalog
- filter semantics matrix
- metric and chart semantics catalog
- FE-to-BE request/data-flow maps
- identity, team, and user-filter behavior
- operational playbook and known failure modes
- work-item and historical ticket index

## Migration State

The domain is scaffolded. Existing analytics and ticket folders remain canonical until individual artifacts are synthesized and their legacy folders receive pointers.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `cresta-proto`, `clickhouse-schema`, `config`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
