# CONVI-7810 criterion-column precedent

Source repo: `/Users/xuanyu.wang/repos/director`; branch/worktree: `main` at `/Users/xuanyu.wang/repos/director`, inspected commit `f47845f4f7`. Related backend source: `/Users/xuanyu.wang/repos/go-servers`, main at `aca4c13774`.

The user asked whether existing group-calibration exports already flatten all criteria into columns and whether Training Simulator can use that pattern across modules.

Verified in `packages/director-app/src/components/qa/report/group-calibration/recent-calibrations-table/components/RecentCalibrationsThreeDotsMenu.tsx`: `buildCriterionCsvColumns` (lines 73–104) traverses all criteria of the session template, keys values by criterion identifier, and adds grade/comment columns. `downloadCSV` uses that shared template for the answer key (196–199) and response scorecards (219–220). Missing scores return undefined (28–34); recorded N/A is explicit (36–38). Thus the wide criterion-column pattern is established, but this path is one shared template across reviewers, not a union of different module templates.

The direct multi-template precedent is `apiserver/internal/coaching/action_export_scorecards.go`: `buildCriteriaHeaders` (823–848) loops templates and their revisions, unions criteria by identifier, and adds grade/comment columns. Per-row score arrays start empty (676–679) and only matching criterion cells are populated (686–705), leaving other cells blank. This supports one combined CSV across different modules, using a union across configurations actually referenced by exported attempts and mapping each attempt's recorded result into its applicable columns. Module one A/B/C plus module two A/B/D gives A/B/C/D; absent rules are blank. Same display labels must not merge distinct rule identities. Mapping module criterion identifiers to stable Opera rule identities still needs implementation validation.

Read-only source verification; no runtime/deployment claim and no source changes. Group calibration also exports non-submitters, unlike the agreed attempt-only Training Simulator contract; reuse the column pattern without copying all row-selection behavior.
