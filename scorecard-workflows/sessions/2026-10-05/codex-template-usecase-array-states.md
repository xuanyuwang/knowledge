# Template `usecase_ids` storage states

- Date: 2026-10-05
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/cresta-proto` (`main`, read-only)
- Question: Can SQL `NULL`, `{}`, and a non-empty array encode all, none, and a finite subset?

## Verified in source

- Both `director.scorecard_templates.usecase_ids` and `director.scorecard_template_revisions.usecase_ids` are nullable `VARCHAR[]` columns with no `DEFAULT` clause (`apiserver/sql-schema/director/director-schema.sql`). An INSERT that omits the column gets SQL `NULL`; that is not what the normal write path does.
- `newDBScorecardTemplateFromProto` allocates `make([]string, len(pbTemplate.UsecaseNames))` and assigns it to `ScorecardTemplates.UsecaseIds`. Create and Commit call this converter and use GORM `Create`; `templateRevisionFromTemplate` copies the same array to the revision row. With no names, this is a non-nil empty slice. `lib/pq.StringArray.Value` encodes non-nil empty as `{}` and nil as SQL `NULL`.
- The Director builder initializes a new template with `[usecase.name]`, retains `template.usecaseNames` on edit, and sends it on save. This establishes the normal UI path, not the population of deployed rows. The backend converter has no non-empty use-case validation, so another API client can create an empty array.
- Current list and QA template-selection paths include both `usecase_ids IS NULL` and `usecase_ids = '{}'` when filtering by use case. `shared/scoring/scorecard_template_dao.go:IsApplicable` treats any zero-length array as universally applicable. Thus `{}` cannot be reassigned to none without changing readers. Other paths are inconsistent: `scorecards.getAndValidateUsercaseID` requires an explicit membership, so neither empty nor NULL passes that check for a concrete use case.
- `ScorecardTemplateProtoFromDB` allocates a zero-length `UsecaseNames` list for either DB NULL or `{}`. The API field is proto `repeated string usecase_names`; the existing read/write contract does not preserve the DB distinction.

## Read-only deployed data check

Queried the user-specified `voice-sandbox-2` application DB through `cresta-cli connstring -i --read-only voice-prod voice-prod voice-sandbox-2`, using the connection skill's in-memory launcher. Latest config resolved to `origin/master:configv3/prod/cresta-sandbox-2/voice-sandbox-2/config.yaml`. `information_schema.columns` confirms both deployed columns are nullable and have no column default.

| Table | Total | NULL | Empty | Non-empty | Cardinality > 1 | Contains `all-usecases` |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| `director.scorecard_templates` | 1,513 | 0 | 0 | 1,513 | 0 | 0 |
| `director.scorecard_template_revisions` | 1,513 | 0 | 0 | 1,513 | 0 | 0 |

Every row in this database has exactly one use-case ID. This is not an all-customer or all-environment population check; other databases may contain legacy NULL or empty rows.

The count query was run once per table using this projection:

```sql
SELECT count(*) AS total,
       count(*) FILTER (WHERE usecase_ids IS NULL) AS null_rows,
       count(*) FILTER (WHERE usecase_ids IS NOT NULL AND cardinality(usecase_ids) = 0) AS empty_rows,
       count(*) FILTER (WHERE cardinality(usecase_ids) > 0) AS nonempty_rows,
       count(*) FILTER (WHERE cardinality(usecase_ids) > 1) AS multi_usecase_rows,
       count(*) FILTER (WHERE 'all-usecases' = ANY(usecase_ids)) AS all_sentinel_rows
FROM director.scorecard_templates;
```

The second query replaced the table name with `director.scorecard_template_revisions`. Column metadata came from `information_schema.columns` filtered to these two tables and `usecase_ids`.

## Decision consequence

The proposed three-state representation is possible in PostgreSQL, but requires an explicit API representation of all versus none, conversion changes, review of every array reader, and a migration/compatibility rule for any existing NULL and `{}` rows across other databases. The implicit SQL default is insufficient because the application writes the column explicitly and current API conversion collapses the two zero-length states.
