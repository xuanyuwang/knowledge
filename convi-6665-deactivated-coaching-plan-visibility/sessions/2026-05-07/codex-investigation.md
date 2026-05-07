# Session Note - 2026-05-07 - Codex - CONVI-6665 initial investigation

**Started:** 2026-05-07 12:02 EDT  
**Tool:** Codex  
**Project:** `convi-6665-deactivated-coaching-plan-visibility`  
**Goal:** Determine where deactivated-user coaching visibility is filtered and whether the fix belongs in FE, BE, or both

## Source Context

- **Primary repo:** `director`
- **Repo path:** `/Users/xuanyu.wang/repos/director`
- **Worktree path:** `/Users/xuanyu.wang/repos/director`
- **Branch:** `xuanyu/enable-na-score-v2`
- **Related repo:** `/Users/xuanyu.wang/repos/go-servers` on branch `codex/move-sleep-out-of-api`
- **Ticket / PR:** `CONVI-6665`

## Inputs Reviewed

- `/Users/xuanyu.wang/repos/knowledge/README.md`
- `/Users/xuanyu.wang/repos/knowledge/workspace/repos.yaml`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/filters/user-team-group/types.ts`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/filters/user-team-group/useUserTeamGroup.ts`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useCoachingHubFilters.tsx`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useRequestForRetrieveRecentActivitiesData.ts`
- `/Users/xuanyu.wang/repos/director/packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useRequestForTargetsProgress.ts`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_retrieve_coaching_overviews.go`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_retrieve_coaching_progresses.go`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/coaching/action_list_coaching_plans.go`

## Actions Summary

- Inspected the `knowledge` repo structure and project conventions.
- Searched both codebases for deactivated/inactive user filtering and coaching-plan request paths.
- Traced the frontend request builders used by Coaching Hub.
- Traced backend user-filter parsing for `ListCoachingPlans`, `RetrieveCoachingOverviews`, and `RetrieveCoachingProgresses`.
- Captured line-level evidence for the current behavior.

## Findings

1. The shared FE selector already implements the requested search behavior.
   - `types.ts` documents that deactivated users are hidden by default unless searching.
   - `useUserTeamGroup.ts` sets `state` to `ACTIVE` by default, but switches to `undefined` during search unless `excludeDeactivatedUsers` is forced.

2. Coaching Hub wires the default `UserTeamGroupFilter` without overriding that behavior.
   - The filter bar passes `userRolesFilter={AGENTS_ONLY_FILTER}` but no explicit `excludeDeactivatedUsers`.

3. The data requests do not preserve the selected inactive user.
   - `useRequestForRetrieveRecentActivitiesData.ts` does not send `includeInactiveUsers`.
   - `useRequestForTargetsProgress.ts` explicitly sends `includeInactiveUsers: false`.

4. The backend overview/progress handlers enforce active-only by default.
   - Both handlers initialize `userState := userpb.User_ACTIVE`.
   - Only `req.IncludeInactiveUsers` switches the parser to `ACTIVE_STATE_UNSPECIFIED`.
   - The selected user names and selected groups all flow through the same `State` gate.

5. `ListCoachingPlans` appears less restrictive.
   - `parseListCoachingPlanReqUserFilter()` does not populate `State`.
   - That means the main mismatch is not on this endpoint.

## Decisions Made

- Treat the primary issue as a Coaching Hub request-construction mismatch first.
- Do not start code changes yet; record the scope decision and implementation caveat in the project README.

## Follow-ups

- Confirm whether the ticket scope is only explicit searched-user selection or also mixed audience filters.
- If explicit searched-user selection is enough, implement FE changes for Coaching Hub request builders first.
- If mixed filters must stay precise, design a BE refinement so selected users can include inactive accounts without broadening group expansion.

## Links

- Linear ticket referenced by the user: `CONVI-6665`
