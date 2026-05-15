# Codex Session: Project Bootstrap

## Date

2026-05-14

## Inputs Reviewed

- Linear issue `CONVI-6842`
- `knowledge/CLAUDE.md`
- `knowledge/workflow/ai-operating-model.md`
- `knowledge/workspace/repos.yaml`
- Existing knowledge projects for `CONVI-6808` and `CONVI-6494`
- PI request construction in `director`
- Closed Conversations request path in `director`

## Actions Taken

- Created knowledge project folder `convi-6842-holiday-inn-pi-vs-closed-conversations`
- Created dedicated backend worktree at `/Users/xuanyu.wang/repos/go-servers-convi-6842`
- Initialized `project.yaml`, `README.md`, and today’s log

## Early Findings

- The issue is already `In Progress` in Linear and assigned to Xuanyu Wang.
- Linear provides the branch name `convi-6842-holiday-inn-voice-and-transfer-performance-insights-shows`.
- Performance Insights and Closed Conversations are not backed by the same API path:
  - PI uses analytics score queries and score-filter builders.
  - Closed Conversations uses conversation listing.
- That difference makes a backend conversation-set mismatch the most likely first target.

## Additional Findings

- The user provided a concrete no-template request for Apr 15 that returned `8748` conversations. That request shape does **not** match the default performance-score widgets; it looks like the intended no-template conversation-volume path.
- The user also reproduced a `No template -> template -> No template` UI issue where the page reused a template-like count around `7239/7240` without issuing the original no-template request again.
- After a full refresh, the page still issued a different `RetrieveQAScoreStats` request that returned `7240` conversations:
  - no `scorecardTemplates`
  - `criterionIdentifiers: []`
  - `includeNaScored: false`
  - `scoreResource: QA_SCORE_RESOURCE_SCORECARD_SCORE`
- That refreshed request matches the shape produced by the default performance-score widgets (`ScoreInsightsMetric` / `ScoreLineChartGraph`) when no template is selected, not the shape of the correct no-template conversation-volume request.
- Current frontend hypothesis is therefore split:
  - one bug around stale template-scoped data in the conversation-volume widget after toggling back to no template
  - another, possibly intentional but confusing, all-scorecards QA request from the default performance-score tab when no template is selected

## Open Questions

- Is PI counting duplicated conversations after joining through scorecards or criteria?
- Does template selection change the score resource or filter semantics?
- Is the mismatch in analytics query generation, result aggregation, or in director request construction?
- Which visible UI element is actually surfacing the `7240` number after refresh: the conversation-volume card, or a different widget whose request happens to share the same total conversation count?
- Should the performance-score widgets issue all-scorecards QA requests when the user explicitly selects `No template`, or should they be hidden / switched to a different no-template mode?

## Next Session Entry Point

Start from the dedicated worktree and trace the template-filtered `RetrieveQAScoreStats` path in `go-servers`, then compare it against the no-template path and Closed Conversations filtering semantics.
