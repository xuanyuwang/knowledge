# Inactive legacy project consolidation

**Primary project:** workspace
**Source repo:** /Users/xuanyu.wang/repos/knowledge
**Branch/worktree:** main, existing checkout
**Updated:** 2026-10-25 (America/Toronto)

## Objective

Consolidate legacy projects into their existing domain homes when they have no meaningful content on or after September 25, 2026. Navigation-only edits and bulk commit dates do not reset the content clock. Preserve original evidence and current uncommitted changes, synthesize durable knowledge, repair references, and remove the old folders after verification. No product code, external service, commit, or push is requested.

## Current State

Completed October 25, 2026 in the existing checkout. Removed 36 inactive top-level project folders after consolidating their findings and unfinished work into Analytics (16), Scorecard Workflows (15), Scorecard Data Sync (4), and Shared AI Skills (1). The empty root sessions folder was also removed after its one orphan session moved to Scorecard Workflows.

All 624 tracked originals are accounted for within 648 preserved files: the originals, 17 added project metadata files, six ignored nascore draft notes, and one orphan session. File contents and modes are recorded in the preservation manifest. Historical scripts remain evidence and were not run. Three ignored environment/cache directories were moved to the local Codex cache rather than discarded.

Four credential-URI warnings in two source documents were cleaned from the archived copies. Verification allows only recorded link rewrites and those credential-value substitutions; no credentials were used. Recent user-filter/configuration projects, credential-bearing or unresolved projects, and coaching prototypes without a suitable domain remain in place. The first five-project migration and all other pre-existing uncommitted work remain intact. Nothing was staged, committed or pushed.

## Canonical Domain Synthesis

- [Analytics](../../analytics/deliverables/inactive-legacy-project-consolidation-2026-10-25.md).
- [Scorecard Workflows](../../scorecard-workflows/deliverables/inactive-legacy-project-consolidation-2026-10-25.md), including [the orphan Vivint session](../../scorecard-workflows/work-items/vivint-orphan-criterion-2026-06.md).
- [Scorecard Data Sync](../../scorecard-data-sync/deliverables/inactive-legacy-project-consolidation-2026-10-25.md).
- [Shared AI Skills](../../ai-skills/work-items/team-feature-flag-cleanup-plugin.md).

## Evidence and Follow-up

- [Selection and retained exceptions](../sessions/2026-10-25/one-month-legacy-consolidation/selection.json).
- [Original file baseline](../sessions/2026-10-25/one-month-legacy-consolidation/baseline.json) and [preservation manifest](../sessions/2026-10-25/one-month-legacy-consolidation/manifest.json).
- [Final verification](../sessions/2026-10-25/one-month-legacy-consolidation/verification.json): file preservation, local links, YAML, whitespace, secret scan, directory removal, and empty Git index.
- No migration work remains. Historical ticket, PR, deployment and customer claims require a live check if those investigations resume. The retained exception list identifies separate future cleanup or domain decisions.

## Timeline

- 2026-10-25 — User set inactivity to one month without meaningful content and authorized further consolidation/removal.
- 2026-10-25 — Consolidated 36 projects, preserved originals and supplemental notes, repaired navigation, removed verified-empty roots, and recorded final checks.
