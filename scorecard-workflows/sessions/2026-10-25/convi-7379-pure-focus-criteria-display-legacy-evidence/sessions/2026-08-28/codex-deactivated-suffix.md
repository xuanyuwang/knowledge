# CONVI-7379 deactivated-criterion presentation follow-up

## Product decision

The reviewed Slack thread selected a textual `(deactivated)` suffix as the common pattern for historical criteria that no longer exist in the current scorecard template. Strikethrough should not be used.

Surface behavior remains intentionally different:

- Historical coaching-session notes retain the saved criterion and show its backend-resolved historical name plus `(deactivated)`.
- Current coaching-plan criteria and trends continue excluding criteria removed from the current template.
- Historical notes are not rewritten when the current plan changes.
- Archived Opera rules remain outside CONVI-7379 and belong to CONVI-7280.

## Director implementation

- Removed `text-decoration: line-through` from the deactivated session pill style while preserving its muted color.
- Added one suffix formatter used by focus-criterion pills, outcome-goal pills, and selector options.
- Applied the suffix to tooltip titles, including criteria rendered only in overflow tooltips.
- Changed the session-note locale value from `[Deactivated]` to `(deactivated)`.
- Preserved the explanatory tooltip note: `This criterion has been removed from the current scorecard template.`

No current-plan filtering, trends filtering, effectiveness calculation, or Opera-rule logic changed.

## Validation

- Biome check/write on changed files: passed.
- Director changed-file precommit ESLint: passed.
- Focused Vitest (`utils.test.ts`, `useSessionCriteriaOptions.test.ts`): 20 tests passed.
- Director app TypeScript build: passed.
- `git diff --check`: passed before knowledge handoff.
