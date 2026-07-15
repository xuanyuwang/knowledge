# Domain Granularity: Top-Level Domains vs Subdomains

**Date:** 2026-07-15
**Tool:** Codex
**Project:** `workspace`
**Goal:** Decide when Analytics and Scorecard Workflows should remain broad domain projects, use subdomains, or split into independent top-level domains.

## Recommendation

Use two levels:

1. **Domain family** at the repository root for shared architecture, ownership, terminology, invariants, and cross-cutting decisions.
2. **Subdomain** for a product surface, bounded capability, or workflow with enough distinct semantics to need its own map.

Start with subdomains. Promote a subdomain to an independent top-level domain only when its operational and architectural independence is demonstrated by real work.

## Why

Splitting every workflow into a top-level domain would recreate the ticket-folder scaling problem at a larger granularity and would duplicate shared concepts. Keeping everything in one flat domain would make navigation and mastery difficult. A two-level model preserves shared truth while giving each workflow an explicit knowledge surface.

## Suggested Analytics Shape

```text
analytics/
  README.md
  subdomains/
    shared-analytics-platform/
      README.md
    performance-insights/
      README.md
    leaderboard/
      README.md
  work-items/
  sessions/
  log/
  decisions/
  deliverables/
```

- `shared-analytics-platform`: API cluster, user/team resolution, filter contracts, time/attribution semantics, common metric calculations, ClickHouse sources, and operational behavior.
- `performance-insights`: exact PI charts/tables, FE transformations, page-specific filter interpretation, drilldowns, and display fallbacks.
- `leaderboard`: ranking/grouping/quintile/tier behavior, page-specific filters, FE presentation, and leaderboard-specific API usage.

The parent domain owns cross-page semantics so Performance Insights and Leaderboard do not duplicate backend/API truth.

## Suggested Scorecard Workflows Shape

Initial subdomains:

- `template-authoring-and-versioning`
- `evaluation-and-scoring`
- `scorecard-lifecycle` (create, edit, submit, publish, reset, reverse)
- `permissions-and-visibility`
- `appeals`
- `group-calibration`
- `process-scorecards-and-generation`

These subdomains share the scorecard/template entity model and many backend/frontend paths, so they should not automatically become independent top-level domains.

## Promotion Criteria

Promote a subdomain into an independent top-level domain when several of these are true:

- it has a distinct owner/on-call path or roadmap;
- it has its own core entities, lifecycle, and invariants;
- its code/data flow is mostly independent of the parent domain;
- it has sustained work volume and multiple active work items;
- it needs its own operational playbook and health model;
- most changes can be understood without loading the parent domain;
- the boundary can be stated without duplicating shared truth.

Duration, document size, or having a distinct UI flow is not sufficient by itself.

## Operating Rules

- Every work item still has one primary top-level domain and one optional primary subdomain.
- Work items, daily logs, sessions, and decisions remain at the top-level domain initially; tag/link the subdomain rather than creating nested operating histories.
- Subdomain folders hold durable maps and reference artifacts, not duplicate logs.
- Cross-cutting facts live at the nearest common parent.
- Avoid more than two taxonomy levels until usage demonstrates a real need.

## Next Decision

If adopted, update the operating model to introduce `subdomains/`, then pilot the structure in Analytics before applying it to Scorecard Workflows.
