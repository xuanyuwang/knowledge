# PR #24020 comment validation

Source repo: /Users/xuanyu.wang/repos/director
Worktree: /Users/xuanyu.wang/repos/director-convi-7777-assistance-usage
Branch: xw/convi-7777-assistance-usage
Head: a9b7d6f3fee198ffcb3af9578c94b7faff585319
Base: 4f6c55458223551e98e62e737add96f55cd4522d

Reviewed all three inline threads, two substantive comments inside a review summary, and all top-level bot comments at the pinned revision. Recommend fixing width, keyboard access and missing team metadata before approval. Percentage suffix is minor polish; the rollout comment needs no code.

## Sticky header width after hiding the metric

- Verdict: Valid; layout defect.
- Comment: https://github.com/cresta/director/pull/24020#discussion_r4341151889

With both the metric rollout and configurable-columns flags enabled, a user hides the new 80px column. TanStack removes its visible leaf, but the hooks still store its width in the sticky parent group. DirectorTableHeader applies that stale width to the grouped header; body offsets use visible cells.

Proposed fix: Recalculate the sticky parent group’s fixedWidth from visible child definitions before passing columns to each DirectorTable. Keep the original definitions for the column menu so the metric can be shown again. Put this in a local Assistance Used column helper, rather than changing all Director tables. The configurable-menu helper alone cannot fix it because it does not receive hidden-column state.

Validation: Hide/show the metric in both tables with configurable columns enabled. Assert group width equals the sum of visible sticky leaf widths and restores on show. Check horizontal scrolling in a browser. The 80px metadata mismatch is executable-model evidence; actual browser pixel layout has not been checked.

## Keyboard access to the definition tooltip

- Verdict: Valid; accessibility defect.
- Comment: https://github.com/cresta/director/pull/24020#discussion_r4351875307

SharedTooltip attaches its focus handlers to the child span. The span has no tabIndex, so Tab cannot focus it. The surrounding sortable header button is focusable, but focusing an ancestor does not dispatch focus to this child. Thus sorting remains keyboard accessible while this explanation is not.

Proposed fix: Add tabIndex={0} to both tooltip trigger spans as the minimal fix. Keep the shared tooltip’s focus and Escape handling. Avoid inserting another button inside the existing sortable header button. A future table-level API could attach the tooltip to the actual sort control, but is unnecessary for this PR.

Validation: Verify with Tab that both explanations open, Escape dismisses them, and sorting still works. This conclusion is based on the rendered element hierarchy and SharedTooltip implementation; no browser keyboard run was performed.

## Team metadata missing from usage-only agent rows

- Verdict: Valid when metadata is available; data presentation defect.
- Comment: https://github.com/cresta/director/pull/24020#pullrequestreview-5615657744

The new result can create an agent absent from legacy assistance results, including an eligible agent with zero usage. Row creation initializes team to an empty string. The usage merge fills name, username and percentage but never team. A supplied user.teamDisplayName="Support" is consequently lost. The API transformer retains this metadata, so it can reach this path.

Proposed fix: In the usage-result merge, fill only a missing team: row.team ||= user?.teamDisplayName || group?.displayName || ''. This preserves an existing legacy team and keeps the fix local to the new merge. The reviewer’s broader change to the shared row factory would work but also changes other legacy row-creation paths.

Validation: Cover a usage-only agent with user team metadata, grouped-team fallback, neither field present, and a legacy row whose non-empty team must remain unchanged. Do not invent a team when the backend supplies none.

## Percentage cells omit the percent sign

- Verdict: Accurate observation; minor display consistency improvement.
- Comment: https://github.com/cresta/director/pull/24020#pullrequestreview-5615657744

For a rate of 37.5 the current renderer returns "37.5"; for zero it returns "0.0". Most neighboring engagement/usage percentage cells include %. The header already contains %, so the number itself is correct and this is not a calculation error. Another adjacent AA percentage also omits the suffix, so the convention is not universal.

Proposed fix: Use the existing locale-aware formatter for this new metric in both tables: rate === undefined ? 'N/A' : fmtPercentage(rate / 100, { numDecimals: 1 }). The adapter already stores a 0–100 percentage, whereas fmtPercentage expects 0–1. Keep CSV values numeric (37.5), with (%) in the CSV heading.

Validation: Check 37.5 → 37.5%, 0 → 0.0%, and undefined → N/A in the active locale. Ensure the CSV remains 37.5 rather than a decorated string. Do not change unrelated legacy percentage columns.

## Rollout flag missing

- Verdict: Already addressed; old comment is outdated.
- Comment: https://github.com/cresta/director/pull/24020#discussion_r4340992142

The original review correctly described the earlier revision. At current HEAD, the metric is absent from alwaysVisible. useConversationAssistanceUsageMetric uses enableConversationAssistanceUsageMetric with default false and returns isResolved && value. The same condition gates both queries, result propagation and the column; CSV derives from the gated column set. Cached results are withheld while off/unresolved.

Proposed fix: No new code or second flag is needed. After review, resolve the old thread with a reference to the existing gate. The suggested director_insights_assistance_usage_percent_column flag would duplicate the implemented flag.

Validation: The existing three container tests cover off/unresolved, enabled, and disabling after data was cached. The five existing column tests cover omitted menu/export columns and zero usage. GitHub marks this thread outdated, but it remains unresolved.

## Other comments

- CodeRabbit Conventional Commit title warning: Do not apply the proposed feat(leaderboards): prefix. Workspace instructions require the PR title to start with its Linear ticket ID; current CONVI-7777 title complies. No code fix.
- CodeRabbit ESLint sandbox warning: The bot could not load a dependency in its own environment. Current GitHub Build / Lint passes, and local changed-file ESLint passed in the preceding implementation turn. Do not add dependencies or disable lint based on this warning.
- Linear linkback: Informational ticket/review links; no requested fix.
- QA metadata check: Reports OK. Confirms metadata, not browser behavior; no fix requested.
- Coverage guardrail: Reports touched areas already have E2E coverage. This does not demonstrate these specific edge cases are tested; no code issue asserted.
- CDN preview deployment comment: Confirms a build artifact and preview override URLs for this commit. Does not establish customer rollout or manual QA. No fix requested.
- Review summaries, risk summary, Autofix/docstring/test checkboxes: Summaries repeat the findings above; opt-in generated-work controls and review quota text are not additional findings. No independent fix. No comments or replies were posted by this validation.

## Executed validation

- 3 existing container tests passed.
- Temporary probe suite: 5 copied existing column tests plus 3 targeted probes passed (8/8). Probes demonstrate an available Support team name being dropped from a usage-only agent, both sticky parent widths remaining 80px larger than visible children after hiding, and both metric renderers returning 37.5 without a suffix. Temporary probe file removed after running. Initial probe harness omissions (TanStack pinning state and filtering parent headers from leaf widths) were corrected; those were harness errors rather than product failures.
- Tooltip finding validated from SharedTooltip focus handler placement and DirectorTableHeader element hierarchy; no browser keyboard validation.
- Current GitHub Build/Typecheck/Lint/unit/E2E checks pass; codeowner approval pending. Review threads remain unresolved.
- No source changes, PR edits, replies or resolutions. gh authenticated itself; no explicit restricted credential selection.
- Saved report: ../../deliverables/convi-7777-pr-24020-comment-validation.html. Structural HTML validation only; native details controls have no custom JavaScript.
