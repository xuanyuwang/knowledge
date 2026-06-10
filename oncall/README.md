# QM and Coaching Oncall Knowledge

This folder is the team memory for QM and Coaching oncall work. Each oncall issue should leave behind enough context for the next person to diagnose the same class of problem faster.

## Principle

Treat each oncall Linear issue as an incident notebook, not just a task. The useful artifact is not only the fix; it is the pattern, the checks that ruled things out, and the shortest path to diagnosis next time.

Every oncall ticket should produce at least one of:

- a fix
- a monitor or alert
- a runbook entry
- a known limitation
- a customer/support explanation

If it produces none of these, the team likely lost knowledge.

## Linear Ticket Shape

Use this structure in the Linear description or in a final `Oncall Learning` comment:

```md
## Symptom
What the user/customer saw. Include exact page/API/metric and expected vs actual.

## Impact
Customer, profile/usecase, affected users, time range, severity, workaround.

## Repro / Evidence
Links, request payloads, response snippets, IDs, screenshots, DB queries.

## Data Checks
Postgres:
ClickHouse:
Elastic / other stores:
Config / feature flags:

## Root Cause
One sentence first. Then details.

## Fix / Mitigation
Immediate mitigation:
Long-term fix:
Owner / PR / deploy:

## Detection
How could we notice this next time? Logs, monitors, query, dashboard, alert.

## Reusable Learnings
Search terms:
Related code paths:
Debug checklist:
Similar past issues:
```

## Labels

Keep the `oncall` label, then add labels that describe the debugging domain. Examples:

- `qm-report`
- `closed-conversations`
- `scorecards`
- `template-status`
- `pg-ch-sync`
- `frontend-filter`
- `backend-query`
- `customer-data`

Labels should answer: "Have we seen this class of issue before?"

## Close Criteria

Do not close an oncall ticket until:

- root cause is stated in one sentence
- at least one reusable debug query or code path is documented
- the "next time check X first" lesson is written
- if unresolved, a clear known unknown and owner are recorded

## Weekly Review

Run a short weekly review:

1. Pick 2-3 oncall tickets.
2. Ask what would have made each one 2x faster.
3. Extract one checklist item, query, dashboard link, or code pointer.
4. Update this folder or a linked team playbook.

## Current Playbooks

- [Scorecard template status mismatch](./scorecard-template-status-mismatch.md)
- [CONVI-7022 example](./convi-7022-scorecard-status-mismatch.md)
- [percentage_value bug](./percentage-value-bug.md)
