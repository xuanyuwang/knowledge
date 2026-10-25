# Code review skill draft

Date: 2026-10-06 (America/Toronto). Primary source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills`. Worktree: `/Users/xuanyu.wang/repos/coaching-qm-skills-code-review`, branch `xw/code-review-skill-draft`, based on locally cached `origin/main` at `98fc175`.

## Objective and source

Create a reusable draft from the [PR #33223 construction review](../../../training-simulator/deliverables/pr-33223-construction-review.html) and its [review-strategy notes](../../../training-simulator/sessions/2026-10-06/codex-pr-33223-review-strategy.md). The trial established the need for a plain-language purpose summary, explanatory code snapshots that evolve between steps, goal and side-effect assessment beyond the diff, a visual workflow for complex changes, and separate non-blocking maintainability suggestions.

## Result

- Added `review-code-change/SKILL.md` with a purpose contract, construction-step method, dependency and scenario tracing, evidence limits, findings, maintainability nits, and a merge recommendation.
- Added the skill to the source repository README. The PR #33223 details remain case-study evidence; the skill has no Training Simulator-specific rules or hardcoded paths.
- The draft is uncommitted and unpublished in its dedicated worktree. The original checkout's modified README and untracked `diagnose-active-days/` were not changed.

## Validation and next use

- `quick_validate.py` passed with an isolated temporary PyYAML dependency; `git diff --check` passed.
- Apply the draft to another substantial PR and revise only where that review exposes a real gap. Recheck the base branch before preparing a PR because this worktree starts from the locally cached `origin/main` ref.
- No credentials were used for this work.

## Report-output correction

The user clarified that invoking the skill should **produce a report like the PR #33223 artifact**, not merely describe a review method. The skill now makes a saved, standalone interactive HTML page its default deliverable for substantial reviews. A new `references/interactive-report.md` specifies the opening summary, purpose contract, workflow map, selectable construction steps with snapshot/change views, scenario judgment, effects beyond the diff, findings and nits, recommendation, source trail, and interaction checks. The report can still use a different format when the user requests one.

## Publication handoff

The user approved the draft for a commit and PR. Committed the README entry, `SKILL.md`, and report reference as `de429d7` on `xw/code-review-skill-draft`, rebased onto current `main` (`28165dc`) and pushed the branch. The rebase combined the new README entry with the recently merged `diagnose-active-days` entry. `git diff --check` passed. No PR template exists in the repository. GitHub CLI authentication and HTTPS Git push were used; no AWS, Okta, Azure, or SSH credential was used.

The user requested a new CONVI ticket. Created [CONVI-7803](https://linear.app/cresta/issue/CONVI-7803/add-reusable-interactive-code-review-skill) and opened [coaching-qm-skills PR #10](https://github.com/cresta/coaching-qm-skills/pull/10) titled `CONVI-7803 Add interactive code review skill`. The PR body links the ticket, and Linear shows the PR as an attachment. Linear moved the ticket to In Progress after linking the PR.

## Review follow-up

CodeRabbit requested that findings name the failing operation or absent guard when no permissive check exists. Verified the gap in both the skill and report reference, changed three sentences, checked the diff and skill frontmatter/reference, and pushed `b0beb57`. CodeRabbit was rerunning at handoff. Created an hourly thread heartbeat to check PR #10, address actionable blockers, and report merge completion; it stays quiet when nothing changes.

The final CodeRabbit review approved `b0beb57`, all reported checks passed, and PR #10 merged as `59489d2186d34006e80ac7e36710d68bb9c43666` at 2026-10-06 20:46 UTC. Linear shows CONVI-7803 as Done. The heartbeat detected the merge and was disabled.
