# CONVI-7747 Active Days skill PR

- Date: 2026-09-25 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills`
- Source branch/worktree: `xw/convi-7747-diagnose-active-days` at `/Users/xuanyu.wang/repos/coaching-qm-skills-convi-7747`
- Base: `origin/main` at `98fc175`
- Commit: `0d20e3b` (`CONVI-7747 Add Active Days diagnostic skill`)
- PR: https://github.com/cresta/coaching-qm-skills/pull/9

## Review and execution

Compared the draft skill against `investigate-analytics-data-issue/SKILL.md`, the CONVI-7747 identity-cutover evidence, and the INSI-4262 Bill West record. The new skill extends the general metric-to-API and PG/ClickHouse workflow with exact Active Days label, online-session, agent-identity, and repair decisions. It distinguishes Comcast's true labels under old IDs from Bill West's missing labels after the session candidate-window regression.

The original checkout was on an unrelated branch with a modified README and untracked draft skill. Created a fresh worktree from current `main`, copied only the draft skill, and added its single README entry. The PR diff contains exactly those two files. Left the original checkout changes untouched.

The skill validator passed using the existing `cresta-assistant` Python environment with PyYAML. `git diff --check` and staged `git diff --cached --check` passed before commit. No database query, backfill, or product-code change occurred in this PR task. The PR body uses the repository's observed Summary and Validation structure; no PR template file exists in current `main`.

GitHub authentication was used for the PR and HTTPS push. A normal GitHub SSH identity was used by the initial read-only fetch because the user's Git URL rewrite maps HTTPS to SSH; subsequent push disabled that rewrite and used HTTPS. No AWS, Okta, or Azure credential was used in this task.

## PR monitoring

At 2026-09-25 14:25 UTC, [PR #9](https://github.com/cresta/coaching-qm-skills/pull/9) remains open. Codeowner, `donotmerge`, and Linear checks pass; CodeRabbit is still pending, there are no inline review comments, and GitHub reports `REVIEW_REQUIRED` / `BLOCKED`. The only current discussion comments are the Linear linkback and CodeRabbit's in-progress notice. Created a quiet 15-minute thread heartbeat named `Monitor CONVI-7747 Active Days PR` to inspect new feedback, make scoped fixes, and merge once checks and approvals permit. No PR content changed in this monitoring pass.

CodeRabbit later requested narrowing the CONVI-7747 precedent. Checked the current line, live Linear ticket, and the September 25 Comcast PG/ClickHouse investigation. The reported `0/N/A` through August 19 and `1` on August 20 for `rgardn843` and `wlanda466` is directly in the ticket. The investigation independently supports old-ID true labels and qualifying online intervals for three sampled agents, but the full old-to-new mapping and historical repair cohort remain unverified. Commit `74507fc` scopes the wording to those samples, adds the reported team pattern, removes the specific profile, UPN-to-NTID wording, and timing relative to the Bill West fix, and states the remaining evidence gap. Retained the supported old-ID finding and explained why in the inline review reply. The skill validator, working diff check, and full PR diff check passed. The PR remains blocked by CodeRabbit's earlier changes-requested review while its new review is pending; the monitor remains active.

CodeRabbit's rereview completed with a passing check and no new actionable comments, but GitHub still records its original `CHANGES_REQUESTED` review. The PR remains blocked; no review was dismissed or merge attempted. Continue monitoring for the review gate to clear.

The user identified one remaining wording issue: the precedent still said `Comcast Acquisition voice`. Commit `4fb034a` removes that qualifier, leaving only Comcast and the two ticket-named leader teams. The skill validator and diff checks passed, and a follow-up reply was posted to the original review thread. The identity evidence remains explicitly scoped to three sampled agents.

At 2026-09-25 14:38 UTC, CodeRabbit approved the latest commit, all four reported checks passed, and GitHub showed `mergeStateStatus=CLEAN`. Squash-merged [PR #9](https://github.com/cresta/coaching-qm-skills/pull/9) as `28165dc9df424f4e5a2c77281e48341ea8631834`. Deleted the monitoring heartbeat after verifying `state=MERGED`. No backfill or product-code change occurred.
