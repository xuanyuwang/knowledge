# Scorecard Data Sync and Notifications Migration

**Date:** 2026-07-15
**Tool:** Codex
**Project:** `workspace`
**Goal:** Commit the approved domain model and migrate existing Scorecard Data Sync and Notifications projects into canonical domain knowledge.

## Source Context

- **Primary repo:** `knowledge`
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch:** `main`

## Actions

- Created baseline commit `9db5afa` containing only the approved domain model, scaffolding, templates, and capture skills.
- Inventoried tracked/untracked artifacts and inbound links for planned migration sources.
- Synthesized Scorecard Data Sync into architecture/invariants, failure modes/cases, monitoring/diagnosis, repair/backfill, source index, and CONVI-7186 work item.
- Synthesized Notifications into routing architecture, notification catalog, recipient semantics, operational playbook, and source index.
- Added migration pointers to relevant legacy sources while retaining raw evidence.
- Corrected two migration classifications based on source evidence.
- Redacted an embedded database credential from the current historic investigation file without printing it during the edit.

## Findings and Decisions

- Scorecard-sync correctness is multidimensional: existence, field, version, projection, and freshness must be evaluated separately.
- Existing missing-ID monitoring does not detect stale fields on rows that already exist.
- Notification ownership, recipient eligibility, destination routing, and delivery are separate decisions and need separate artifacts.
- Cross-domain projects should link rather than move wholesale: `oncall` retains incident operations; scorecard permission retains policy semantics.
- CONVI-6841 is primarily Scorecard Workflows/PG consistency, and Hilton coaching discrepancy is coaching analytics; neither belongs in Scorecard Data Sync.

## Safety

- Pre-existing unrelated changes outside the requested migration remain unstaged.
- The `pg-ch-scorecard-sync-investigation` July 8 changes were preserved and brought into scope because they are the latest migration evidence.
- The redacted credential can still exist in Git history; rotation/history cleanup requires separate authorization.
- Historical Go/scripts remain evidence, not endorsed current production tooling.

## Validation

- Changed project YAML parsed successfully.
- Whitespace and relative Markdown link checks passed after repairing one stale legacy link.
- The final migration scope contains no embedded ClickHouse credential URL.
- Staged names were reviewed and unrelated existing work remains unstaged.
- Migration is prepared as a separate commit from baseline `9db5afa`.
