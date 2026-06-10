# Codex Session: Blog and Resume Pickup

## Context

User asked to pick up blogs and resume items for `train-for-staff`.

## Inputs reviewed

- `train-for-staff/senior-to-staff.md`
- `train-for-staff/staff-project.md`
- `train-for-staff/resume-snippets.md`
- existing blog posts under `blog/`
- `scorecard-template/README.md`
- `template-schema-version-updater/README.md`
- `convi-6968-schwab-leaderboard-launch/README.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/api-decision-table.md`
- `convi-6968-schwab-leaderboard-launch/deliverables/BE plan.md`
- `convi-6862-disable-editing-on-submitted-scorecard/deliverables/submitted-scorecard-edit-permission-fail-freeze-plan.md`

## Findings

- The existing staff tracker already has polished project narratives and resume bullets for user-filter consolidation, ClickHouse external tables, and scorecard PG-to-ClickHouse consistency.
- The strongest newer blog candidates are scorecard/template domain stewardship, leaderboard agent-vs-submitter metric semantics, and submitted-scorecard permission drift.
- The scorecard/template project is best framed as staff-level domain stewardship rather than a single implementation project.
- The Schwab leaderboard work is best framed as semantic API strategy: current APIs look reusable, but differ by attribution axis, time basis, filter support, and grouping grain.

## Updates made

- Added `train-for-staff/project.yaml` because the legacy folder was reopened for meaningful work.
- Added `train-for-staff/deliverables/blog-and-resume-candidates.md`.
- Drafted `train-for-staff/deliverables/turning-ticket-work-into-a-domain-reference.md`.
- Added candidate resume bullets to `train-for-staff/resume-snippets.md`.
- Added a Project 5 staff narrative to `train-for-staff/staff-project.md`.
- Added daily log entry for this pickup.

## Next steps

- Turn the strongest candidate into a full blog draft.
- Revisit resume bullets after PRs, adoption, or measurable outcomes are available.
