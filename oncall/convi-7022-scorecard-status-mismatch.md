# CONVI-7022: Scorecard Status Mismatch in CEL Customer Experience

Linear: https://linear.app/cresta/issue/CONVI-7022/scorecard-status-mismatch-in-cel-customer-experience

## Symptom

For RCG CEL Customer Experience, two conversations have submitted scorecards for `Customer Xperience Center DTQ`, but:

- Closed Conversation does not show the template as having scorecards.
- QM Report shows `0` evaluated scorecards for the agent/time range, expected `2`.

## Impact

- Customer/profile: `rcg/us-east-1`
- Usecase: `cel-customer-xperience`
- Agent user ID: `a0975ec480306c2`
- Debug time range: `2026-05-01T04:00:00Z` to `2026-05-10T03:59:59.999Z`
- Affected template title: `Customer Xperience Center DTQ`

## Important IDs

- Old archived template ID with scorecards: `019d4e57-ccdd-744f-b465-26e1d3a13527`
- Replacement active template ID with same title: `019df996-3393-7613-b12c-65ed728f4366`
- Conversations:
  - `019dfaff-d827-725b-91c8-efe610bce147`
  - `019dfa14-cfa2-7244-bf4c-720ac163f830`
- Submitted scorecards:
  - `019dfe4f-6e68-731a-8379-1c4057f7bbd1`
  - `019dfb37-6a4b-7157-9722-fcdf42672324`

## Data Checks

Direct count for old archived template:

```text
submitted_count = 106
total_count = 111
first_submitted_at = 2026-04-03 17:48:48.241073+00
last_submitted_at = 2026-05-06 17:47:17.871114+00
```

By revision:

```text
45442765  submitted=103  total=107
d84529df  submitted=3    total=4
```

For the specific QM Report debug filter:

```text
submitted_count = 2
first_submitted_at = 2026-05-06 03:03:50.563531+00
last_submitted_at = 2026-05-06 17:26:52.406076+00
```

ClickHouse `scorecard_d` also has the two submitted rows, so the main Problem B symptom is not caused by missing ClickHouse scorecard rows.

## Root Cause

Historical submitted scorecards are attached to old template ID `019d4e57-ccdd-744f-b465-26e1d3a13527`, whose latest revision is archived, while the QM Report and Performance Template filter auto-discover active current templates and surface replacement template ID `019df996-3393-7613-b12c-65ed728f4366` with the same title.

## Details

`RetrieveDirectorTaskStats` delegates QM requests to `RetrieveQMTaskStats`.

`RetrieveQMTaskStats` calls `qa.ListCurrentScorecardTemplateIDs(..., filterActive=true)` before counting scorecards. That helper selects the latest revision per template ID by `created_at`, then drops templates whose latest revision is not active.

For old template `019d4e57-ccdd-744f-b465-26e1d3a13527`, the latest revision is:

```text
revision = be5b48c1
status = 3 (archived)
created_at = 2026-06-01 20:30:32.990084+00
```

Therefore QM Report does not count historical scorecards on that template unless the request explicitly includes logic that still considers archived historical templates.

## Common Query Mistake

This query overcounts:

```sql
select count(*)
from director.scorecard_templates st
join director.scorecards card on st.resource_id = card.template_id
where card.customer = 'rcg'
  and card.profile = 'us-east-1'
  and st.resource_id = '019d4e57-ccdd-744f-b465-26e1d3a13527'
  and card.submitted_at is not null;
```

It returns `848` because there are 8 template revisions, so each of 106 submitted scorecards is counted 8 times.

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

## Reusable Learning

When a scorecard count is missing from Closed Conversation or QM Report:

1. Verify the scorecards directly from `director.scorecards`.
2. Compare template IDs, not template titles.
3. Check for same-title replacement templates.
4. Check latest template revision status.
5. Check whether the API path uses active-only template discovery.
6. Only then investigate ClickHouse sync.

Search terms: `scorecard template archived`, `same title template`, `ListCurrentScorecardTemplateIDs`, `RetrieveQMTaskStats`, `template revision fanout`, `closed conversation scorecard hidden`.

## Potential Product / Engineering Follow-Up

Historical QM stats may need a mode that includes templates referenced by matching submitted scorecards in the selected time range, not only currently active latest templates. Scope this carefully so inactive/archived templates with no relevant historical data do not clutter filters.
