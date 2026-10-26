# CONVI-7208: Behavior summary for PM review

**Ticket**: [Group Calibration — Add column for scorecard criterion-level comments](https://linear.app/cresta/issue/CONVI-7208/group-calibration-add-column-for-scorecard-criterion-level-comments)  
**Surface**: QA Group Calibrations Report → three-dots menu → **Download session as CSV**  
**Decision record**: [../decisions/2026-07-03-convi-7208-director-only-export.md](../decisions/2026-07-03-convi-7208-director-only-export.md)

## Problem

The session CSV export includes criterion **grades** and a single scorecard-level **Comment** column, but not the **per-criterion comments** left by the answer key creator or participating reviewers.

## Expected outcome

After each criterion grade column, add a matching comment column — consistent with Coaching Hub and QM Report scorecard exports.

## Accepted decisions

| Topic | Decision |
|-------|----------|
| **Implementation** | Director-only; no new backend/gRPC export API |
| **Comment source** | Criterion-level score comments only (`scores.comment` from scorecard submission) |
| **Not included** | Collaboration/conversation comments are not merged into criterion comment cells |
| **Column layout** | For each criterion: grade column, then `{Criterion Display Name} comment` |
| **Column naming** | Matches QM Report / Coaching Hub export in go-servers (`{displayName}` + `{displayName} comment`) |
| **Criterion grades** | Unchanged — numeric value or `N/A` only |
| **Scorecard-level Comment** | Keep existing column alongside new per-criterion comment columns |
| **Empty comments** | Blank cell (not a placeholder like `None`) |
| **Comment access roles** | Not applied in export (QA-admin-facing download; consistent with QM export) |
| **Applies to** | Answer key row and every participating reviewer row |

## Example row shape (after change)

**Answer key row** (`Answer Key = Yes`):

```text
Session Name | Due Date | ... | Greeting | Greeting comment | Empathy | Empathy comment | ... | Comment | Total Score | Consistency Score
```

**Reviewer row** (`Answer Key = No`):

Same column structure; per-criterion grades and comments come from that reviewer's scorecard. Consistency Score populated as today.

## Out of scope

- New server-side export endpoint in go-servers
- Merging collaboration/conversation comments into criterion comment cells
- Changes to criterion grade formatting (text/sentence/per-message criteria)
- Comment visibility rules based on `comment_access_roles` in the export builder
- Changes to Group Calibration stats/charts APIs (`RetrieveDirectorTaskStats`)

## Notes for PM confirmation

1. **Dual Comment columns** — Per-criterion comment columns plus the existing scorecard-level **Comment** column is intentional (same pattern as QM export).
2. **Scorecard-level empty comment** — Group Calibration CSV uses a **blank** cell when no scorecard-level comment exists. QM Report uses `"None"` for null scorecard-level comments. Per-criterion empty comments use blank in both places. We are keeping Group Calibration scorecard-level behavior unchanged.

## Manual QA checklist

1. Session with answer key + 2+ reviewers, each with distinct per-criterion comments
2. Download CSV → verify `{criterion}` + `{criterion} comment` pairs for all criteria
3. Answer key row includes criterion comments
4. Reviewer rows include their own criterion comments
5. Empty criterion comment → blank cell
6. Scorecard-level Comment column still present
7. Existing columns (session name, consistency score, total score) unchanged
