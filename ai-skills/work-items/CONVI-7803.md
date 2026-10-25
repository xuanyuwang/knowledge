# CONVI-7803: Add reusable interactive code review skill

**Status:** complete
**Primary domain:** `ai-skills`
**Primary subdomain:** none
**Official ticket:** [CONVI-7803](https://linear.app/cresta/issue/CONVI-7803/add-reusable-interactive-code-review-skill)
**Last updated:** 2026-10-06

## Objective and Impact

- **Objective:** Publish a shared skill that produces a standalone interactive HTML report for substantial code reviews.
- **System impact:** Reviewers get a reusable purpose, behavior, scenario, and findings format with pinned source evidence.
- **Role:** designed and implemented

## Scope

Add `review-code-change/SKILL.md`, its interactive report reference, and the repository README entry. No product code changes.

## Source Context

- **Repo:** `/Users/xuanyu.wang/repos/coaching-qm-skills`
- **Worktree:** `/Users/xuanyu.wang/repos/coaching-qm-skills-code-review`
- **Branch:** `xw/code-review-skill-draft`
- **PR/commits:** [#10](https://github.com/cresta/coaching-qm-skills/pull/10), `de429d7`, `b0beb57`

## Current Understanding

The skill merged through PR #10 as `59489d2`, and CONVI-7803 is Done. The PR and Linear ticket link to each other. A CodeRabbit finding about missing guards and failing operations was addressed in `b0beb57`; the final review approved the change.

## Validation and Rollout

- Skill validation passed during drafting; `git diff --check` passed after rebase.
- The follow-up wording passes `git diff --check`; frontmatter and reference existence were checked.
- PR #10 merged on 2026-10-06. All reported checks passed. No deployment is needed for this repository-only skill change.

## Next Actions

None for CONVI-7803.

## Timeline

- 2026-10-06 — Drafted the skill from the PR #33223 report trial, committed `de429d7`, created CONVI-7803, and opened PR #10. Evidence: [session](../sessions/2026-10-06/codex-code-review-skill-draft.md), [PR](https://github.com/cresta/coaching-qm-skills/pull/10).
- 2026-10-06 — Fixed the first review finding in `b0beb57` and started an hourly quiet monitor for blockers and merge completion.
- 2026-10-06 — CodeRabbit approved, checks passed, and PR #10 merged as `59489d2`; CONVI-7803 moved to Done.
