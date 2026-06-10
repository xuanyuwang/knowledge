# CONVI-7022 Linear Update Draft

## Suggested Description Addition

```md
## Oncall Investigation Summary

### Symptom

For RCG CEL Customer Experience, two conversations have submitted scorecards for `Customer Xperience Center DTQ`, but:

- Closed Conversation does not show the template as having scorecards.
- QM Report shows `0` evaluated scorecards for the agent/time range, expected `2`.

### Impact

- Customer/profile: `rcg/us-east-1`
- Usecase: `cel-customer-xperience`
- Agent user ID: `a0975ec480306c2`
- Time range: `2026-05-01T04:00:00Z` to `2026-05-10T03:59:59.999Z`
- Old template ID with scorecards: `019d4e57-ccdd-744f-b465-26e1d3a13527`
- Active replacement template ID with same title: `019df996-3393-7613-b12c-65ed728f4366`

### Root Cause

Historical submitted scorecards are attached to old template ID `019d4e57-ccdd-744f-b465-26e1d3a13527`, whose latest revision is archived, while QM Report and Performance Template filters auto-discover active current templates and surface replacement template ID `019df996-3393-7613-b12c-65ed728f4366` with the same title.

### Evidence

Direct count for old archived template:

```text
submitted_count = 106
total_count = 111
first_submitted_at = 2026-04-03 17:48:48.241073+00
last_submitted_at = 2026-05-06 17:47:17.871114+00
```

For the specific QM Report debug filter:

```text
submitted_count = 2
first_submitted_at = 2026-05-06 03:03:50.563531+00
last_submitted_at = 2026-05-06 17:26:52.406076+00
```

ClickHouse `scorecard_d` also has the two submitted rows, so the primary QM Report issue is not missing ClickHouse scorecard data.

### Code Paths

- `go-servers/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go`
- `go-servers/shared/qa/scorecard_template.go`
- `director/packages/director-app/src/hooks/coaching/useCurrentScorecardTemplates.ts`
- `director/packages/director-app/src/hooks/conversation/useConversationCoachingDetails.ts`

### Query Trap

Do not join templates to scorecards only by `resource_id`; that fanouts across revisions. This query returned `848` because 106 submitted scorecards joined to 8 template revisions.

Correct join:

```sql
select count(*)
from director.scorecard_templates st
join director.scorecards card
  on st.customer = card.customer
 and st.profile = card.profile
 and st.resource_id = card.template_id
 and st.revision = card.template_revision
where card.customer = 'rcg'
  and card.profile = 'us-east-1'
  and st.resource_id = '019d4e57-ccdd-744f-b465-26e1d3a13527'
  and card.submitted_at is not null;
```

This returns `106`.

### Reusable Learning

When a scorecard count is missing from Closed Conversation or QM Report:

1. Verify scorecards directly from `director.scorecards`.
2. Compare template IDs, not template titles.
3. Check for same-title replacement templates.
4. Check latest template revision status.
5. Check whether the API path uses active-only template discovery.
6. Only then investigate ClickHouse sync.

Search terms: `scorecard template archived`, `same title template`, `ListCurrentScorecardTemplateIDs`, `RetrieveQMTaskStats`, `template revision fanout`, `closed conversation scorecard hidden`.

### Follow-Up

Historical QM stats may need a mode that includes templates referenced by matching submitted scorecards in the selected time range, not only currently active latest templates. Scope this carefully so inactive/archived templates with no relevant historical data do not clutter filters.
```

## Suggested Comment

```md
## Oncall Learning

This issue is a template identity/status mismatch, not a missing scorecard row.

The submitted scorecards exist in PG and ClickHouse under old template ID `019d4e57-ccdd-744f-b465-26e1d3a13527`. The latest revision for that template is archived, and the UI/report path is showing the active replacement template ID `019df996-3393-7613-b12c-65ed728f4366` with the same title, `Customer Xperience Center DTQ`.

Fast diagnosis next time:

1. Count directly from `director.scorecards`.
2. Join templates by `(customer, profile, resource_id, revision)`.
3. Compare template IDs, not titles.
4. Check latest template revision status.
5. Check whether the API uses active-only template discovery.

Saved team note: `knowledge/oncall/convi-7022-scorecard-status-mismatch.md`.
```
