# Session Note - 2026-05-07 - Codex - Scorecard Template Distillation

**Started:** 2026-05-07 00:00 America/Toronto  
**Tool:** Codex  
**Project:** `scorecard-template`  
**Goal:** Distill scattered scorecard-template knowledge into a canonical reference document.

## Source Context

- **Primary repo:** `go-servers`
- **Repo path:** `/Users/xuanyu.wang/repos/go-servers`
- **Worktree path:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch:** `codex/move-sleep-out-of-api`
- **Ticket / PR:** None

## Inputs Reviewed

- `scorecard-template/README.md`
- `scorecard-template/template-structure.md`
- `scorecard-template/criterion-options.md`
- `scorecard-template/fe-template-builder.md`
- `scorecard-template/na-score-design.md`
- `nascore/README.md`
- `nascore/options-scores-lifecycle.md`
- `convi-6709-reversed-scorecard/README.md`
- `convi-6709-reversed-scorecard/exact-flow.md`
- `backfill-scorecards/README.md`
- `pg-ch-scorecard-sync-investigation/scorecard-specific-constraints.md`
- `duplicate-template-across-usecase/investigation.md`

## Actions Summary

- Searched the repository for scorecard-template-related knowledge outside the core project folder.
- Identified the major cross-project threads that materially shape the scorecard-template mental model.
- Wrote a canonical distilled reference under `deliverables/`.
- Added `project.yaml` and updated the project README to separate distilled knowledge from working notes.

## Findings

- The scorecard-template system is wider than the template JSON shape; it includes builder transforms, scoring math, AutoQA wiring, projection behavior, and read-only rendering.
- The most important invariant is the logical wiring between options, scores, and AutoQA mappings.
- Several past bugs came from representation mismatches between builder form state, persisted template semantics, and read-only rendering.
- Process scorecards and projection repair behavior are part of the template system’s real operational boundary.

## Decisions Made

- Keep the canonical distilled knowledge inside `scorecard-template/deliverables/`, not in a new top-level folder.
- Use one comprehensive reference doc first, then add narrower operational deliverables later if needed.

## Follow-ups

- Add a precise source map deliverable with code paths and table names.
- Add a debugging/failure-modes deliverable for scorecard-template incidents.
- Backfill additional `project.yaml` files for nearby scorecard-related projects if they are reopened often.

## Links

- `scorecard-template/deliverables/scorecard-template-system-reference.md`
