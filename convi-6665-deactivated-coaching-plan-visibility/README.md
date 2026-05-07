# CONVI-6665: Deactivated Coaching Plan Visibility

**Created:** 2026-05-07
**Updated:** 2026-05-07
**Status:** Initial investigation complete

## Overview

Ticket asks for the coaching experience to keep its default behavior of hiding deactivated users, while still allowing a specifically searched deactivated user to surface that user's coaching plan.

This investigation checked:

- `go-servers` backend filtering for coaching-plan-related requests
- `director` frontend request construction and shared user/team/group search behavior
- whether the change should be made in backend, frontend, or both

## Current Objective

Identify where deactivated-user filtering is applied today and define the lowest-risk implementation plan for the requested search-only behavior.

## Key Findings

1. The shared Director user/team/group picker already matches the desired search UX.
   It defaults to `UserState.ACTIVE`, but when a search term is present it drops the user state filter unless `excludeDeactivatedUsers` is explicitly set. The component contract also documents: "By default, deactivated users won't be shown unless by searching."

2. Coaching Hub request builders do not carry that search-specific behavior into the backend calls.
   - `useRequestForRetrieveRecentActivitiesData()` does not set `includeInactiveUsers`.
   - `useRequestForTargetsProgress()` explicitly sets `includeInactiveUsers: false`.

3. In `go-servers`, both `RetrieveCoachingOverviews` and `RetrieveCoachingProgresses` translate `includeInactiveUsers=false` into `State: userpb.User_ACTIVE`, so any selected deactivated user is filtered back out before the coaching query runs.

4. `ListCoachingPlans` is different.
   Its request-to-user-filter conversion does not set `State`, so it does not appear to force active-only filtering for specifically selected users. That means the strongest mismatch is on the overview/progress endpoints used by Coaching Hub, not on the list-coaching-plans path.

## Recommendation

### Likely minimum fix

Frontend change in `director` for Coaching Hub requests:

- Set `includeInactiveUsers=true` when the filter contains explicitly selected user names that may include a searched deactivated user.

This should allow the selected deactivated user to survive backend filtering for:

- Recent Coaching Activities
- Target Progress

### Important caveat

The backend flag is coarse-grained. If `includeInactiveUsers=true`, `go-servers` will allow inactive users for the whole user filter parse, including group expansion, not only for explicitly selected users.

So:

- If the expected behavior is strictly "specific searched users only," and mixed user-plus-group filters must still exclude other deactivated users, then a backend refinement is needed.
- If the practical use case is "search for a deactivated user and select that user directly," a frontend-only change is probably sufficient.

## Evidence

### Director

- Shared filter behavior:
  - `packages/director-app/src/components/filters/user-team-group/types.ts`
  - `packages/director-app/src/components/filters/user-team-group/useUserTeamGroup.ts`
- Coaching Hub filter wiring:
  - `packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useCoachingHubFilters.tsx`
- Coaching Hub request builders:
  - `packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useRequestForRetrieveRecentActivitiesData.ts`
  - `packages/director-app/src/features/coaching-workflow/coaching-hub/hooks/useRequestForTargetsProgress.ts`

### go-servers

- Active-only filtering in overview/progress endpoints:
  - `apiserver/internal/coaching/action_retrieve_coaching_overviews.go`
  - `apiserver/internal/coaching/action_retrieve_coaching_progresses.go`
- ListCoachingPlans user-filter construction:
  - `apiserver/internal/coaching/action_list_coaching_plans.go`

## Proposed Next Step

Validate the intended scope with product/QA:

- If only explicitly searched users need support, implement the FE change first and test the Coaching Hub tabs.
- If mixed filters must remain exact, add a BE-safe design before coding so group expansion does not accidentally include extra deactivated users.

## Log History

| Date | Summary |
|------|---------|
| 2026-05-07 | Initial cross-repo investigation completed and implementation plan narrowed |

## Related Artifacts

- `project.yaml`
- `log/2026-05-07.md`
- `sessions/2026-05-07/codex-investigation.md`
