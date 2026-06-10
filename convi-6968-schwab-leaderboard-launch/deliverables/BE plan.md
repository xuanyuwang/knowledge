# Migrate QA APIs To Agent And Submitter Filter Axes

## Implementation Status
- Status as of 2026-06-08: backend contract and go-servers implementation are merged.
- Proto PR: `cresta/cresta-proto#8803`.
- Go PR stack:
  - Part 1 of 4: `cresta/go-servers#28525`, migrated `RetrieveQAConversations` user filtering.
  - Part 2 of 4: `cresta/go-servers#28526`, added shared ClickHouse submitter filters.
  - Part 3 of 4: `cresta/go-servers#28530`, added `RetrieveQAConversations` submitter audience support.
  - Part 4 of 4: `cresta/go-servers#28527`, added `RetrieveQAScoreStats` submitter grouping/filter support.
- Worktrees for the proto and go-servers work were cleaned after the PRs merged. The FE worktree was left in place because it still had local/ahead changes.

## Summary
- Do not add a request-level user attribution switch.
- Keep `QAAttribute.users/groups` as agent filters.
- Use existing `QAAttribute.scorecard_reviewer_audience` as submitter filters.
- Add `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` so score stats can group by scorecard submitter explicitly.
- Migrate `RetrieveQAConversations` to `ParseUserFilterForAnalytics`.

## API Changes
- Remove the previously proposed request-level attribution enum.
- Remove the previously proposed request attribution fields from:
  - `RetrieveQAScoreStatsRequest`
  - `RetrieveQAConversationsRequest`
- Keep/add only:
  - `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER = 10`
- Do not change `UserAudience`; `scorecard_reviewer_audience.users/groups` remain string resource names in proto.

## Final Semantics
- `QAAttribute.users/groups` filter agents and apply to `agent_user_id`.
- `QAAttribute.scorecard_reviewer_audience` filters scorecard submitters and applies to `submitter_user_id`.
- `QA_ATTRIBUTE_TYPE_AGENT` groups by `agent_user_id`.
- `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` groups by `submitter_user_id`.
- `QA_ATTRIBUTE_TYPE_GROUP` remains agent-group aggregation. Submitter-group aggregation is out of scope unless a separate explicit group-by type is added later.
- Default requests without `scorecard_reviewer_audience` remain backward compatible and apply no submitter filter.

## Submitter Audience Handling
- In Go, convert `scorecard_reviewer_audience.users/groups` string names to `[]*userpb.User` and `[]*userpb.Group` before calling `ParseUserFilterForAnalytics`.
- Resolve agent and submitter audiences independently:
  - agent audience from `QAAttribute.users/groups`
  - submitter audience from `QAAttribute.scorecard_reviewer_audience`
- Use `listAgentOnly=false` for submitter audience resolution because submitters can be managers/reviewers, not agent-only users.
- Nil vs empty submitter audience:
  - `scorecard_reviewer_audience == nil`: no submitter filter, preserving backward compatibility.
  - `scorecard_reviewer_audience != nil` with empty users/groups: resolve all allowed submitters.
  - Explicitly resolved empty submitter audience returns an empty response.
- When grouping by `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`, resolve submitter users for response metadata and zero-count row construction, but only add a submitter SQL filter when `scorecard_reviewer_audience` is explicitly present or ACL semantics require one.

## Backend Worktree And Commit Milestones
- Proto worktree: complete in `cresta/cresta-proto#8803`.
  - Update the existing proto PR to remove the request-level attribution switch.
  - Keep only `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER`.
  - Do not regenerate proto artifacts.
  - Commit: proto-only cleanup.

- Go milestone 1: complete in `cresta/go-servers#28525`.
  - Migrate `RetrieveQAConversations` agent filter path from `MoveGroupFilterToUserFilterForQA` to `ParseUserFilterForAnalytics`.
  - Commit separately.

- Go milestone 2: split across `cresta/go-servers#28526` and `cresta/go-servers#28530`.
  - Add submitter audience resolution for `RetrieveQAScoreStats` and `RetrieveQAConversations`.
  - Convert `UserAudience` string names to user/group proto messages before calling `ParseUserFilterForAnalytics`.
  - Add ClickHouse submitter filter support using `submitter_user_id`.
  - Add separate external tables for agent and submitter filters to avoid collisions.
  - Commit separately.

- Go milestone 3: complete in `cresta/go-servers#28527`.
  - Add `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` group-by support in `RetrieveQAScoreStats`.
  - Construct submitter grouped rows using resolved submitter users.
  - Reject unsupported or ambiguous combinations, especially `QA_ATTRIBUTE_TYPE_GROUP` plus `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` if behavior is not explicitly supported.
  - Commit separately.

- Go milestone 4: complete in the relevant API PRs.
  - Add/update tests for dual filtering, submitter group-by, empty resolved audiences, nil submitter audience, and default backward-compatible agent behavior.
  - Commit separately.

## Review Fixes Captured In The Merged Stack
- `RetrieveQAConversations` migration now uses `ParseUserFilterForAnalytics` for the agent filter path.
- Submitter audience resolution is shared and named around scorecard reviewer audience semantics instead of a request-level attribution switch.
- Common ClickHouse parsing can add submitter filters on `submitter_user_id` without colliding with agent user filter external tables.
- Conversation query output does not expose an unnecessary submitter column.
- External-table construction checks resolution errors before using parsed user-filter results.
- Common ClickHouse group-by handling emits `NULL AS <groupByKey>` when a query shape does not need a physical column, instead of selecting a non-existent column.
- Invalid score stats group-by combinations are rejected when both group keys would map to `QAScoreGroupBy.user`, including `AGENT + SCORECARD_SUBMITTER`.
- `submitterUserID` is part of the shared `clickhouseKeyTimeRow` path so grouping keys are handled consistently.

## Frontend Cost Estimate
- Medium cost, about 1-2 focused FE days after generated client types land.
- Manager leaderboard aggregate switches to `RetrieveQAScoreStats` with:
  - manager/submitter selections in `scorecard_reviewer_audience`
  - `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER` when grouping by manager/submitter
- Manager drawer switches to `RetrieveQAConversations` with manager/submitter selections in `scorecard_reviewer_audience`.
- Keep agent selections, if any, in normal `users/groups`.

## Test Plan
- Proto source diff contains only `QA_ATTRIBUTE_TYPE_SCORECARD_SUBMITTER = 10`.
- Go tests:
  - `RetrieveQAConversations` with agent filter only.
  - `RetrieveQAConversations` with submitter filter only.
  - `RetrieveQAConversations` with both agent and submitter filters.
  - `RetrieveQAConversations` with nil `scorecard_reviewer_audience` preserves existing behavior.
  - `RetrieveQAConversations` with explicit empty `scorecard_reviewer_audience` resolves all allowed submitters.
  - `RetrieveQAScoreStats` grouped by agent.
  - `RetrieveQAScoreStats` grouped by scorecard submitter.
  - Empty resolved agent audience returns empty.
  - Empty resolved submitter audience returns empty.
  - ClickHouse SQL/golden tests verify `agent_user_id` and `submitter_user_id` filters can appear in the same query.
- Existing default calls without `scorecard_reviewer_audience` remain backward compatible.

Representative validation used during the stack:

- `bazel test //insights-server/internal/analyticsimpl:retrieve_qa_score_stats_test`
- `go test ./insights-server/internal/analyticsimpl -run 'TestRetrieveQAConversations|TestParseUserFilter'`

## Assumptions
- Manager FE will put manager/submitter selections into `scorecard_reviewer_audience`.
- Manager leaderboard no longer needs old submit-time filtering semantics.
- Submitter audience resolution must include non-agent users.
- Submitter-group aggregation is out of scope.
- Each milestone should leave its repo buildable/testable and should be committed before starting the next milestone.
