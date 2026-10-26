# Vivint orphan criterion in a pinned template revision

**Record state:** Historical June 19 diagnosis; repair and writer follow-up unverified.
**Primary domain:** scorecard-workflows
**Primary subdomain:** template-authoring-and-versioning
**Consolidated:** 2026-10-25

## Findings and Impact

The June investigation found 1,607 Vivint cx-voice scorecards with a criterion score absent from their pinned template revision. The scorecards were created in June 2026, long after that criterion was removed in November 2025. Re-pinning them to an older template would misrepresent their other valid criteria; the problem was not explained by scorecard age alone.

The reindex builder failed while constructing score rows, before its cleanup step, so repeating reindex could not repair the malformed PostgreSQL source. The historical proposal was to remove only the stray criterion rows, preserve valid scores and scorecards, and then reindex. It also suggested investigating a stale template cache and considering builder behavior for orphan criteria. These were recommendations, not recorded completed changes.

## Source and Next Actions

- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`.
- [Full original diagnosis](../sessions/2026-06-19/claude-vivint-orphan-criterion.md), including population counts and the proposed SQL. It is historical evidence, not a current repair instruction.
- The template/criterion consistency question lives here. Projection/reindex mechanics belong to `scorecard-data-sync`.
- If reopened, verify today's affected population and pinned revisions, establish the writer cause, and determine whether any repair or builder change already happened before selecting an action.

## Timeline

- 2026-06-19 — Read-only investigation and targeted repair proposal recorded.
- 2026-10-25 — Orphan root session moved into Scorecard Workflows without changing its content.
