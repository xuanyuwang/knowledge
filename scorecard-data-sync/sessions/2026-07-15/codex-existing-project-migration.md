# Scorecard Data Sync Existing-Project Migration

**Date:** 2026-07-15
**Tool:** Codex
**Project:** `scorecard-data-sync`
**Goal:** Synthesize existing ticket projects into the canonical domain without deleting historical evidence.

## Source Context

- **Primary repo:** `go-servers`
- **Repo path:** `/Users/xuanyu.wang/repos/go-servers`
- **Worktree path:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch:** `main`

## Inputs Reviewed

- `pg-ch-scorecard-sync-investigation/`
- `convi-5565-scorecard-ch-pg-sync/`
- `convi-6298-reindex-process-scorecards/`
- `backfill-scorecards/`
- `historic-scorecard-missing/`
- `convi-6841-process-scorecard-update-race/`
- `hilton-coaching-discrepancy/`

## Findings

- The strongest reusable structure is source/projection contract → failure taxonomy → diagnosis → repair.
- The existing monitor detects missing IDs but not stale fields on existing CH rows.
- Process scorecards require scorecard-centric recovery because conversation reindex cannot select them.
- Reindex insertion does not remove rows that cease to be eligible; cleanup semantics must be explicit.
- Historical count comparisons must account for non-emitted chapter aggregate rows.
- CONVI-6841 and the Hilton coaching discrepancy were incorrectly classified in the initial migration matrix and were reclassified based on their actual root causes.
- A tracked historic investigation contained an embedded database credential. The current tree was redacted; rotation and history remediation remain separate authorized work.

## Decisions

- Keep raw ticket folders as evidence archives with migration pointers.
- Make the new domain deliverables canonical rather than moving every script/log into the domain.
- Preserve the pre-existing CONVI-7186 uncommitted session/log and promote its current state into a completed work item.
- Do not promote historical scripts as current safe executables.

## Validation

- Cross-checked the canonical case index against source investigations.
- Preserved source links and explicit adjacent-domain boundaries.
- Parsed changed YAML successfully.
- Verified relative Markdown links and whitespace checks.
- Verified the final migration scope contains no embedded ClickHouse credential URL.
