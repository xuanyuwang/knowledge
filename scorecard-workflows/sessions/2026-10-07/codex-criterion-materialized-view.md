# Criterion materialized-view option

- Date: 2026-10-07
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Input: Xuanyu proposed making criteria queryable as rows derived from template JSONB using a PostgreSQL materialized view.

## Existing source evidence

- `apiserver/sql-schema/director/director-schema.sql` stores versioned template JSONB and `usecase_ids` in both `director.scorecard_templates` and `director.scorecard_template_revisions`; criteria are not independent rows.
- `apiserver/sql-schema/app/app-schema.sql` defines `app.moment_template_references_materialized`, uses `jsonb_path_query` to extract JSON references, creates a unique index, and refreshes the view through write triggers.
- `shared/db/testing/test_suite.go` disables those refresh triggers in many tests because a synchronous `REFRESH MATERIALIZED VIEW CONCURRENTLY` costs about 500 ms even in an empty test database. This is a precedent, not a measured cost for a criterion view.
- `shared/scoring/scorecard_template_parser.go:parseTemplateStructureItems` checks duplicate identifiers within each sibling list. `shared/scoring/scorecard_templates.go:GetChaptersAndCriteria` collects criteria into a template-wide map keyed by identifier. A proposed unique row key on criterion identifier within a revision needs separate validation or a structural path.

## Design implications

A materialized view can expose one read-only row per criterion per template revision while keeping JSONB and the Go model authoritative. It should include a revision key and enough scope information to answer queries; it does not make criteria independent writeable resources. Expansion must traverse V1/V2 shapes and branch children, preserve `null`/`[]`/list distinctions, and inherit missing historical scope from that row's template `usecase_ids`. Choose one template source table to avoid duplicating revision rows, then compare the view's output with the normalized Go parser.

The standard materialized view refreshes the whole result, rather than incrementally updating only the edited template. Synchronous refresh adds write cost; deferred refresh creates a stale-read window. A normal view avoids refresh but does JSON work on each query. A transactionally maintained projection table can update only rows for the committed template revision, at the cost of another write path and derived-state maintenance.

The next decision is whether criterion lookup must reflect a new template revision immediately or can tolerate a short delay. This determines whether deferred materialized-view refresh is viable. No materialized view or projection is selected yet.
