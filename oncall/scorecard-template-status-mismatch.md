# Scorecard Template Status Mismatch Playbook

## Pattern

Closed Conversation or QM Report can show zero or hide scorecards even when submitted scorecards exist in the database, if historical scorecards are attached to an archived or replaced scorecard template ID while the UI/report filter only discovers active current templates.

This is especially easy to miss when old and new templates share the same title.

## Fast Checks

1. Count directly from `director.scorecards` before joining templates.
2. If joining `director.scorecard_templates`, join by `(customer, profile, resource_id, revision)`, not only `resource_id`.
3. Compare scorecard `template_id` and `template_revision` with current/latest template rows.
4. Check whether the latest template revision is active, inactive, or archived.
5. Check whether the API path auto-discovers only active templates.
6. Check ClickHouse only after Postgres semantics are clear.

## Correct Template Join

```sql
select count(*)
from director.scorecard_templates st
join director.scorecards card
  on st.customer = card.customer
 and st.profile = card.profile
 and st.resource_id = card.template_id
 and st.revision = card.template_revision
where card.customer = '<customer>'
  and card.profile = '<profile>'
  and st.resource_id = '<template_id>'
  and card.submitted_at is not null;
```

Joining only on `st.resource_id = card.template_id` fanouts one scorecard per template revision.

## Direct Scorecard Count

```sql
select
  template_id,
  count(*) filter (where submitted_at is not null) as submitted_count,
  count(*) as total_count,
  min(submitted_at) filter (where submitted_at is not null) as first_submitted_at,
  max(submitted_at) filter (where submitted_at is not null) as last_submitted_at
from director.scorecards
where customer = '<customer>'
  and profile = '<profile>'
  and template_id = '<template_id>'
group by template_id;
```

## Current Template Status Check

```sql
select resource_id, revision, title, status, usecase_ids, created_at
from director.scorecard_templates
where customer = '<customer>'
  and profile = '<profile>'
  and resource_id = '<template_id>'
order by created_at desc;
```

## Relevant Code Paths

- `go-servers/insights-server/internal/analyticsimpl/retrieve_director_task_stats.go`
- `go-servers/insights-server/internal/analyticsimpl/retrieve_qm_task_stats.go`
- `go-servers/shared/qa/scorecard_template.go`
- `director/packages/director-app/src/hooks/coaching/useCurrentScorecardTemplates.ts`
- `director/packages/director-app/src/hooks/conversation/useConversationCoachingDetails.ts`

## Common Trap

The QM Report response may include a template with the expected title but a different template ID. Always compare `template_id`, not title.
