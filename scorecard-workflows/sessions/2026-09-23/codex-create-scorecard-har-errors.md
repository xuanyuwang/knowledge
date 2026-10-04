# Create-scorecard HAR error investigation

## Context

- Date: 2026-09-23 (America/Toronto)
- Primary subdomain: scorecard lifecycle
- Source artifact: `/Users/xuanyu.wang/repos/create-scorecard-error.har`
- Primary source repo: `/Users/xuanyu.wang/repos/director`
- Branch/worktree: `main` at `1410a0b75dc8ec06ce086d6d4b81b161509258b4`
- Compared local `origin/main` ref: `9e9126d44e261295befa4b481db5c164b3394605`; the two relevant Director files have no diff.
- Related backend repo: `/Users/xuanyu.wang/repos/go-servers`, `main` at `6e9999f0ad45c0e900c3860cbdb48ae2862fe40c`
- Linear ticket: [CONVI-7744](https://linear.app/cresta/issue/CONVI-7744/creating-a-new-scorecard-encounter-errors-say-the-scorecard-for-the)
- Scope: investigation only; no product code or application data changed.

## Outcome

The HAR contains two related failures while manually scoring conversation `01a0cf1c-a42d-72de-b2bb-1f6c0151bd57` with template `0199ba42-d3af-76b9-95db-5611ef8d15e3@98cfcf4c` (`06.10.25`):

1. `CreateScorecard` returned HTTP 409 `ALREADY_EXISTS` because a scorecard for the same conversation, template ID, and agent already existed.
2. The next queued autosave returned HTTP 404 `NOT_FOUND` because Director retained the client-generated scorecard name from the rejected create and attempted to patch it.

The primary cause is incomplete pagination in the conversation-detail scorecard loader. The affected conversation had 197 scorecards in the available 1,000-row control response. The detail request omitted `pageSize` and returned exactly 100 rows plus a non-empty `nextPageToken`. The pre-existing `06.10.25` scorecard was outside that first page, so Director treated the template as unscored and rendered a new form.

## HAR evidence

- A Closed Conversations list request at `16:40:23.802Z` used `pageSize=1000` and returned 197 scorecards for the affected conversation. The existing scorecard was `01a0cf1c-ff77-744d-9ad0-a1e9c4cd0262`, created at `16:32:59.256167Z`, with the exact selected template revision. It was item 175 (zero-based index 174) within that conversation's returned scorecards.
- The conversation-detail `ListScorecards` request at `16:40:51.102Z` requested `scorecardView=FULL` without `pageSize` or `pageToken`. It returned 100 scorecards, spanning create times `16:33:04.328904Z` through `16:33:01.087470Z`, and a non-empty `nextPageToken`. The existing selected-template scorecard was absent.
- At `16:40:54.183Z`, Director attempted to create client-generated scorecard `01a0cf24-3ea6-7649-a6fd-36c842f30b0f`. The backend rejected it with 409 because the stable conversation/template/agent tuple already existed.
- At `16:40:55.766Z`, the next autosave patched that same client-generated name and received 404 because the failed create never persisted it.
- The incident is data-shape-specific rather than a general outage: a previous conversation created and updated a scorecard successfully, and the next conversation also created and updated one successfully in the same HAR.

## Source trace

### Missing existing scorecard

`packages/director-app/src/hooks/conversation/useConversationCoachingDetails.ts` calls `CrestaAPI.coaching.listScorecards` once for the selected conversation. It forwards `scorecardView`, calibration, and use-case filters, but no page size or page token, and it ignores `nextPageToken` returned by `packages/director-api/src/services/cresta-api/coaching/coachingApi.ts`.

`ScorecardFormLoader.tsx` passes that truncated `scorecards` array to `getSelectedScorecard`. The selector correctly compares stable template resource IDs, but it cannot find a record missing from the response, so it returns `undefined` and the form follows the create path.

The backend behavior is expected. `apiserver/internal/coaching/action_list_scorecards.go` applies request pagination and returns `nextPageToken`. `shared/scoring/scorecard_scores_dao.go` checks default scorecard duplicates by customer, profile, conversation, template ID, agent, message overlap, type, and calibration status; template revision is not part of this uniqueness check.

### Secondary 404

`useSaveScorecardMutation.ts` generates a resource name and assigns it to `currentScorecardNameRef.current` before awaiting `CreateScorecard`. The mutation scope serializes autosaves, but a rejected create leaves the ref populated. The next queued mutation therefore skips create and calls `UpdateScorecard` for the unpersisted name, producing the observed 404.

## Recommended correction

1. Make `useConversationCoachingDetails` retrieve all pages for its single-conversation scorecard request. Existing Director code in `useListAllScorecards` already demonstrates looping on `nextPageToken`. Raising only `pageSize` would move the correctness boundary without removing it.
2. Do not retain a generated scorecard name until create succeeds, or clear the ref when create fails. This prevents a primary create error from cascading into misleading update failures.
3. Add a regression case with more than 100 scorecards for one conversation and an existing selected-template scorecard on a later page. Also cover a rejected create followed by another autosave.

The backend duplicate guard should remain. Returning the pre-existing scorecard from `CreateScorecard` would change the API's deliberate non-idempotent contract and would conceal incomplete client reads.

## Regression assessment

This is not supported as a recent code regression. Git history shows the conversation-detail `listScorecards` call has omitted pagination parameters since commit `6ce8917fad0d3b33f94bf7e4cbe998de8d42a6f5` on 2023-11-23. The shared backend query default is still 100 rows. The generated-name ref that causes the secondary 404 was added in commit `df4a13c66b35a2e03b104a77e01450a8ea098c3e` on 2025-03-18.

The evidence therefore classifies the incident as a long-standing pagination correctness gap newly exposed when one conversation accumulated more than 100 scorecards. A separate recent change may have increased the likelihood of crossing that threshold, but no such trigger was established from this HAR or source history.

## Linear update

Posted the verified root cause, HAR evidence, source links, and recommended correction to CONVI-7744 as comment `d35a7fa9-1d23-4011-9344-be3769ea2f94`. Re-read the ticket comments after posting and confirmed the full comment is present.

## Security note

The HAR was inspected locally. Authorization headers, cookies, and session values were not copied into this note or terminal summaries.
