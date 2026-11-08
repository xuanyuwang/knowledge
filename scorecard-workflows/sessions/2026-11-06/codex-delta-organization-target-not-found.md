# Delta organization target creation: evidence collection

- Date: 2026-11-06 (America/Toronto)
- Primary domain: scorecard-workflows; subdomain: template-authoring-and-versioning
- Primary source repo: /Users/xuanyu.wang/repos/go-servers
- Source context: main checkout inspected read-only; implementation read from freshly fetched origin/main. No product edits.
- User request: collect information about the oncall failure, identify target API, explain template-not-found conditions including deletion.
- Oncall source: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793944533660419

## Reported evidence

Glean indexed thread text reports Delta Air Lines (`delta-air-lines`), profile `us-west-2`, use case `synthetic-care`. Creating an organization target fails across criteria, including `agent-apologize-empathy-criterion` and `agent-summarize-actions-criterion`, with `404 NOT_FOUND: Scorecard template '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a' not found`. Demo planned Monday. The connector returned indexed text only, not structured replies; two attachment URLs require Slack sign-in, so their exact payload values have not been independently viewed.

## Confirmed source findings

Snapshots: fetched go-servers origin/main `b013c5ae3b`, Director `7fcbe35c81`, cresta-proto `3e804086cf`. No source checkout edits.

- Generic `CoachingService.CreateTarget`, HTTP `POST /v1/customers/{customer}/profiles/{profile}/targets`, takes Target as its body. Absent coaching_plan_name means organization-wide (`cresta/v1/coaching/coaching_service.proto:832`, `target.proto:54-62`). Director `CriteriaOrgTargetCell.tsx` builds the four fields described by the user.
- `action_create_target.go:24-75` calls ConvertTargetAPIToDB and the template lookup with the target's use case before checking the criterion or inserting a target.
- `action_get_scorecard_template.go:63-98` matches customer/profile/template ID, optional explicit revision, and overlap with the requested use case or `all-usecases`. Empty arrays fail. The wrapper at lines 53-59 maps any lookup error, even a DB error, to the same NotFound response.
- `action_list_current_scorecard_templates.go:390-412` explicitly includes NULL or empty usecase_ids when listing for a use case. This differs from target creation and explains how an empty-scope template can be listed while target creation fails. The affected browser list request was not captured.
- `action_delete_scorecard_template.go:16-36` normally soft-deletes by setting deleted_at. Target lookup has no deleted/deactivated predicate, and the model uses sql.NullTime rather than GORM soft-delete fields. Ordinary soft deletion alone does not explain this 404.

## Read-only production checks

Used connect-customer-app-db skill, customer `delta-air-lines`, app profile `us-west-2`. Config `origin/master:configv3/prod/delta-air-lines/us-west-2/config.yaml`; AWS account `us-west-2-prod`, cluster `us-west-2-prod-2`, database `delta-air-lines`. cresta-cli authenticated itself and obtained read-only access; no AWS credential/profile or SSH key was explicitly selected, inspected, or copied. No secrets printed or persisted. Broad config fetch warned about case-colliding refs; explicit `git fetch origin master` subsequently succeeded.

First query selected metadata from director.scorecard_templates for the exact customer/profile/template ID, ordered by created_at descending:

| Revision | Title | Created at (UTC) | usecase_ids | deleted_at | deactivated_at |
| --- | --- | --- | --- | --- | --- |
| 7508e489 | POC Template | 2026-11-05 20:34:22.611306 | {} | NULL | NULL |
| a95546ec | Agent Adherence | 2026-11-05 04:34:06.536737 | {} | NULL | NULL |
| 2496e40f | Agent Adherence | 2026-11-05 04:34:06.216123 | {} | NULL | NULL |

Second query reproduced the lookup predicate and checked criterion text presence:

```sql
SELECT revision, title, cardinality(usecase_ids) AS usecase_count,
       ARRAY[usecase_ids]::text[] && ARRAY['synthetic-care','all-usecases']::text[]
         AS matches_create_target_usecase_filter,
       position('agent-apologize-empathy-criterion' in template::text) > 0 AS has_empathy_criterion,
       position('agent-summarize-actions-criterion' in template::text) > 0 AS has_summarize_criterion
FROM director.scorecard_templates
WHERE customer = 'delta-air-lines' AND profile = 'us-west-2'
  AND resource_id = '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a'
ORDER BY created_at DESC;
```

Every row returned usecase_count=0, matches_create_target_usecase_filter=false, and both criterion checks true. The current production data matches the reported failure mechanism; this was not an API replay or deployed-build verification.

## Conclusion and follow-up

The template exists and is neither deleted nor deactivated. All three revisions have empty scope, so target creation's lookup excludes them before criterion validation. Listing and creation have inconsistent empty-scope behavior. The error conceals the actual use-case mismatch.

Still unknown: why the template was authored with empty scope, screenshot revision suffix/target number, deployed build, exact browser list request. Next: trace authoring and decide between aligning API semantics or changing template scope through the supported authoring workflow. No customer-data writes, product fixes, target POSTs, Slack posts, or other external messages were performed.

## Follow-up: can template authoring APIs write empty scope?

User requested this as the next investigation step. Inspected the same fetched source snapshots above; no new DB queries or mutations.

- **Yes, CreateScorecardTemplate accepts omitted usecase_names or an empty list.** The proto declares the repeated field OPTIONAL without a minimum-item rule (`scorecard_template.proto:77-79`). CreateScorecardTemplate calls newDBScorecardTemplateFromProto and inserts its result (`action_create_scorecard_template.go:57-73`).
- The shared converter allocates `make([]string, len(pbTemplate.UsecaseNames))`, validates each supplied name's syntax/customer/profile, then assigns it directly to `UsecaseIds` (`transformers.go:434-445,499-511`). Zero names skip validation and become an empty PostgreSQL array. It neither requires a use case nor defaults to the selected use case or `all-usecases`. An empty list differs from `[""]`: an empty string entry is parsed and rejected.
- **CommitScorecardTemplate also accepts an empty/omitted list.** Its previous-template lookup receives that list; an empty list bypasses the use-case filter. The same converter then writes empty UsecaseIds to a new revision, with no fallback to the previous revision's scope (`action_commit_scorecard_template.go:18-26,61-65`; `transformers.go:434-445,519-523`). Thus omitting the field on Commit can clear previously nonempty scope. This is source-derived behavior, not a performed API replay.
- UI trace: new templates normally default to `[usecase.name]` (`TemplateBuilderForm.tsx:290-299`). Existing templates initialize from `template.usecaseNames` (lines 161 and 211), and save forwards `data.usecaseNames` unchanged (`useSaveScorecardTemplate.ts:350-379`). Thus editing an already-empty template preserves its empty scope. The access form displays use case read-only only when the list is nonempty (`TemplateBuilderFormAccessStep.tsx:214-225`). Copying only overrides use cases when an explicit targetUsecase exists (`ScorecardTemplateBuilder.tsx:104-110`).

Conclusion: empty scope is accepted by backend authoring, not proof of manual DB modification. Normal new-template UI defaults do not explain Delta's original empty revision; API callers/imports, copying existing empty-scope templates, and omitted scope during Commit remain possible paths. The exact original caller is not established.

## Ticket creation follow-up

Created [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-enforce-template-use-case-validation-in-backend-apis) at the user’s explicit request. Priority High; status Backlog at creation; no assignee selected. Checked existing issues for Delta targets, empty use cases, customer slug, and exact template ID; no matching duplicate was identified. The ticket emphasizes backend-owned scope invariants across scripts/imports/AI agents/other API clients, explicit omitted-field behavior, consistent lookup errors, direct API tests, and remediation for both Delta templates. Descriptive IDs do not prove copying or violate an established ID contract; the original client remains unknown. No product or customer-data changes. Linear connector authentication was used; no AWS/Okta/Azure/SSH credential selected or accessed for ticket creation.

## Operator-run data repair prepared

User requested a writable connection command (remove --read-only) and SQL to assign synthetic-care to both affected templates. Prepared [repair SQL](delta-template-synthetic-care-repair.sql), restricted to Delta/us-west-2, the two identified resource IDs, empty scope, and undeleted/non-deactivated rows. This repairs all five known revisions because lookups may specify a revision. SQL returns affected rows inside a transaction for the operator to inspect before COMMIT (ROLLBACK if unexpected). Connection instructions reuse the existing URI-to-PG-environment launcher, without printing a connection URI. The user’s explicit writable-command request supersedes the database skill’s read-only default; the original skill/launcher were not modified. No connection opened, credentials accessed, or mutation executed by Codex. Operator execution and target-creation verification remain pending.

## Post-repair empty-scope inventory

At 2026-11-06 16:02:58 UTC, scanned all rows in director.scorecard_templates in the Delta application database through the read-only database skill. Result: 4 templates, 17 revisions, zero empty/NULL scopes across all revisions, zero latest templates with empty scope, and zero NULL/blank elements inside usecase_ids arrays. This confirms the previously observed empty-scope rows are no longer present; it does not by itself verify the exact assigned scope or successful target creation. No writes executed by Codex. Config resolved from origin/master:configv3/prod/delta-air-lines/us-west-2/config.yaml, AWS account us-west-2-prod, cluster us-west-2-prod-2, database delta-air-lines. cresta-cli self-authentication only; no AWS/Okta/Azure/SSH credential explicitly selected or inspected. User confirmed this inventory should cover Delta only; no cross-customer scan requested.

## Incident closure

At the user’s request, updated [CONVI-7981](https://linear.app/cresta/issue/CONVI-7981/delta-air-lines-resolve-organization-target-failure-caused-by-empty) with the read-only post-repair inventory and changed its status to Done. Linear returned status Done and completedAt 2026-11-06T17:04:42.767Z. Retitled the issue to describe the Delta target incident; preserved backend/API/UI prevention recommendations as not implemented. Explicitly recorded that the inventory confirms no empty scope but did not check exact assigned scope or replay target creation. No new ticket, customer-data write, or Slack message.
