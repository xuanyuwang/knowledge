# Director PR Before/After video skill

**Date:** 2026-10-07
**Source repo:** `/Users/xuanyu.wang/repos/coaching-qm-skills`
**Branch/worktree:** `xw/director-pr-proof-videos` at `/Users/xuanyu.wang/repos/coaching-qm-skills-director-pr-videos`

## Inputs and decisions

- Used the CONVI-7812 Director PR recording and upload workflow as a concrete example, and read Director `docs/pull_request_template.md` plus `docs/qa-metadata-checks.md` for current evidence requirements.
- Named the skill `record-director-pr-videos`; kept it tool-agnostic to match `coaching-qm-skills` conventions. It supports file-only delivery or PR attachment according to the request.
- Captured the non-obvious parts: same test data and page for Before/After; full-screen URL and navigation; checking beginning/middle/end for overlays or loading; bare GitHub attachment URLs for inline playback; live-body patching to preserve the PR template; and verifying `qa-metadata-check`.
- Created a dedicated worktree from `origin/main` because the main checkout has unrelated changes.

## Validation

- Skill-creator `quick_validate.py`: passed with the existing temporary PyYAML dependency path.
- `git diff --check`: passed in the skill worktree.
- No recording or GitHub upload was repeated to validate a documentation-only skill; the workflow was exercised on CONVI-7812 immediately before this draft.

## State

- Skill and README entry are uncommitted in the dedicated worktree. No PR was requested for this skill.
