# Omni use case scorecard: criterion applicability model

- Date: 2026-10-04
- Primary subdomain: template-authoring-and-versioning
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only); related contract: `/Users/xuanyu.wang/repos/cresta-proto` (`main`)
- Product input: [Omni Use Case QM / Coaching PRD](https://docs.google.com/document/d/1iEYDUg7NrYo5NjeTcr9nHyNqEbv_sWHrDHJeAkFJS9c), retrieved through Glean on 2026-10-04

## Question

How can criteria in one template apply to different use cases without requiring immediate extraction of criteria into independent resources? The PRD requests template-to-many-use-case scope and criterion-level subset scope, but leaves the containment rule implicit.

## Current source evidence

- `cresta-proto/cresta/v1/coaching/scorecard_template.proto`: the API carries the template structure as a `google.protobuf.Struct`; `usecase_names` exists at template level only. This is the transport shape, not the only schema for the structure.
- `go-servers/apiserver/sql-schema/gen/model/scorecard_templates.go` and `scorecard_template_revisions.go`: both current and revision rows have `usecase_ids` and `template` JSONB. Criterion definitions are inside the JSON, not independent rows.
- `go-servers/shared/scoring/scorecard_templates.go` and `scorecard_template_parser.go`: Go structs already define the template structure schema. The parser decodes JSON into `templateStructure` and `scorecardTemplateItem`, then constructs typed V1/V2 structure and criterion structs such as `BaseCriterion`. V2 has nested `items`, including branch children. These are existing typed model and validation seams; a criterion use-case field must be added to the parser item and the relevant typed criterion model, then carried through parsing and serialization. Otherwise `json.Unmarshal` into the current parser item ignores the new field and parsed scoring logic cannot see it.
- `go-servers/apiserver/internal/coaching/transformers.go:newDBScorecardTemplateFromProto`: Create and Commit share this conversion. It parses the template and separately validates template use-case resource names, then stores the raw JSON and IDs. There is no criterion-to-template scope validation today.
- `go-servers/shared/scoring/scorecard_template_dao.go:IsApplicable` and `action_list_current_scorecard_templates.go:buildCurrentTemplatesQueryConditions`: empty/null template use-case scope means all use cases. This sentinel must retain its meaning.
- `go-servers/insights-server/internal/analyticsimpl/retrieve_group_calibration_stats.go`: analytics SQL directly joins `director.scorecard_templates` for template JSON and score calculations. A service query layer cannot cover this reader by itself. `cron/sync-users/internal/qm-task/backfill_qm_task_config.go` indexes `UsecaseIds[0]`, another single-use-case assumption.
- `go-servers/apiserver/sql-schema/gen/model/scorecards.go` and `scores.go`: evaluated scorecards and scores carry a concrete `usecase_id`; scorecards also carry template ID and revision. This supports evaluating criterion applicability against the revision used for a conversation.

## Proposed data contract to assess

Let `T` be a template's effective use-case set and `C_i` a criterion's effective set at the same template revision. If template `usecase_ids` is empty/null, `T` is all use cases under the profile. An omitted criterion scope means `C_i = T` for legacy compatibility. An explicitly selected criterion scope must have valid use-case names in the template's profile and satisfy `C_i ⊆ T`. Xuanyu clarified on 2026-10-05 that adding a use case to a template adds it to every criterion, including criteria with explicit subsets; removing a use case from a template removes it from every criterion. The semantics of an explicitly empty set and universal template scope still need decisions.

At runtime, a criterion is eligible for a scorecard only if the scorecard's use case belongs to `C_i` in the scorecard's template revision. Omitted criteria must not appear as N/A scores: they are outside the denominator and should not be auto-scored. Chapter/branch applicability and scorecard snapshots need explicit treatment.

The smallest compatible storage direction appears to be extending the existing Go template schema with criterion applicability, serialized inside the versioned template JSON. Add scope validation after the template and template use cases are both parsed in the shared Create/Commit conversion, and use the same effective-scope semantics in scoring. If direct SQL readers need criterion applicability, assess a version-keyed relational projection (or shared database view/function) generated from the authoritative revision JSON in the same transaction. Do not assume an application query layer can fence direct SQL, and avoid making JSON plus an independently edited table competing sources of truth.

## Open decisions and validation gaps

1. Define whether an empty template scope remains universal for newly authored templates, and whether an explicitly empty criterion scope is forbidden or has a real meaning.
2. Criterion scope follows template additions and removals in new revisions. Define behavior for existing scorecards, tasks, and AutoQM rules; historical revision semantics still need confirmation.
3. Define per-use-case Opera rule mapping, global-rule precedence, overlap/duplicate detection, and what happens when only some use cases support AutoQM.
4. Inventory every direct SQL/JSON reader and writer, including analytics and exports, before claiming a query-layer migration fence.
5. Test single-use-case regression, multi-use-case criterion inclusion, historical revisions, branch conditions, score denominator, and SQL analytics parity.
6. The older criterion-extraction initiative mentioned by Xuanyu was not located in the searched Glean results or current knowledge notes. Its design/outcome should be verified before using it as a premise.

This is an investigation and proposed contract, not an implementation decision.
