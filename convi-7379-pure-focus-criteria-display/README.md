# CONVI-7379 - Pure Focus Criteria Display

## Status

**Backend and frontend PRs open.** The implementation keeps focus-criterion persistence revision-independent and recovers labels from historical template revisions on reads. Director PR [#21757](https://github.com/cresta/director/pull/21757) contains validated commit `cff1d057b3` directly against `main`.

## Summary

Pure coaching plan 1:1 sessions display raw criterion IDs and scorecard template resource paths because the frontend resolves criterion display names only from **current** scorecard templates. Jeff Sykes' active coaching plan still references criterion IDs that were removed from the Security/MS Security template in December 2025.

## Root Cause

Focus criteria are stored as `{template_id}/{criterion_id}` in DB. The BE read path never populated `criterionDisplayName` (even though the proto has the field). The FE resolved display names only from current scorecard templates. When criteria were removed in later template revisions, the FE couldn't find them → raw IDs displayed.

## Solution

Three-part fix:

### BE Write Path
Keep the existing revision-independent `{template_id}/{criterion_id}` representation.

### BE Read Path
Batch-resolve `criterionDisplayName` from historical template revisions. `resolveCriterionDisplayNames` queries relevant revisions newest-first and uses the newest revision containing each `(template_id, criterion_id)`. Responses retain the wildcard template revision because the label source is not historical provenance.

### FE Simplification
Since BE now always populates `criterionDisplayName`, removed redundant FE-side resolution logic (option-matching, label-splitting). `getCriteriaInfoForTooltip` uses `criterionDisplayName` + `templateDisplayNameMap` directly. Deactivated criteria (present in DB but removed from current templates) are shown with `[Deactivated]` tag and muted styling.

## Implementation Artifacts

- **BE PR:** https://github.com/cresta/go-servers/pull/31048 (single replacement commit `6ebc7a750f` pushed)
- **BE worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7379`, branch `convi-7379-focus-criteria-display-names-be`
- **FE worktree:** `/Users/xuanyu.wang/repos/director-convi-7379`, branch `xw/convi-7379-focus-criteria-display-names-fe`
- **FE commits:** `cff1d057b3` plus review fix `6abb06d6b6` (pushed directly on `origin/main`)
- **FE PR:** https://github.com/cresta/director/pull/21757

## Affected Pure Data

- Agent: Jeff Sykes (`a53848957ec81e6b`)
- Coaching plan: `019952c4-501b-7485-8fdc-becca2fadda0`
- Example session (2026-07-24): `019f9542-63d0-708f-827f-06cb00e926b2`
- Scorecard template: `0196dac5-6c3b-717e-bc9a-d95bb54c51b2` (Security / MS Security)
- 3 active Pure coaching plans reference stale criterion IDs

## Key Design Decisions

1. **BE-side resolution**: BE has DB access to historical revisions; FE shouldn't know about revision semantics
2. **Batch queries**: Single DB call collects all needed template revisions, not N+1 per criterion
3. **Stable logical identity**: `(template_id, criterion_id)` remains revision-independent; the latest revision containing it supplies only its label
4. **FE simplification**: Once BE always populates `criterionDisplayName`, FE removes all redundant lookup logic

## Decision Change

The first implementation changed writes from `T/C` to `T@R/C`. That approach was dropped after review exposed backward-compatibility problems:

- existing and new rows would use different formats;
- wildcard ListCoachingPlans filters would not naturally match revision-qualified rows;
- targets store no template revision and are queried as revision-independent criteria;
- target-based FE callers inconsistently sent `T@*` and the current `T@R`;
- inferring a revision for a legacy row did not provide reliable historical provenance or round-trip filtering.

The ticket therefore fixes the read path only. Existing `T/C` persistence, target joins, and filter behavior remain unchanged; historical revisions are consulted solely to recover `criterionDisplayName`. See the [accepted decision](decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md).

## Latest Review State (2026-08-14)

The backend's minimal read-path design remains validated. Director PR review found and the branch now fixes two frontend issues: selection changes preserve the complete criterion value including `criterionDisplayName`, and missing-current-template criteria are no longer duplicated in outcome options. This follows current default-form classification; fully recovering whether a deactivated criterion was historically an outcome still requires a future contract signal. Current Linear linkage passes. QA metadata still requires Before/After videos. The automated testkit POM warning is a path-mapping false positive. See `sessions/2026-08-14/codex-director-pr-comment-validation.md`.
