# CONVI-7385 Implementation Plan — Scorecard Column/Drawer Count Parity

**Status:** ready for implementation
**Ticket:** [CONVI-7385](https://linear.app/cresta/issue/CONVI-7385/qa-scorecard-api-count-mismatch-between-column-and-drawer)
**Primary domain:** `analytics` / `leaderboard`
**Primary repo for fix:** `director`
**Created:** 2026-07-28
**Diagnosis evidence:** `work-items/CONVI-7385.md`, `sessions/2026-07-28/codex-convi-7385-column-drawer-mismatch.md`

Use this document as the handoff for a separate coding session. Prefer implementing only the frontend filter-parity fix described below; do not reopen broader QA time-semantics work unless the repro changes.

---

## 1. Background

### Symptom

On Agent Leaderboard, the scorecard count in the main table/column can disagree with the scorecard template side drawer for the same agent and date range.

Confirmed repro:

- Customer/profile: `customers/cng/profiles/us-east-1`
- Agent: Agnes Long / `customers/cng/users/2bdb21d567b2118f`
- Day: 2026-07-15 (`America/Toronto`)
- Column (`RetrieveQAScoreStats`): `totalScorecardCount = 1`
- Drawer (`RetrieveQAConversations`): 3 `conversationInfos`

### Product expectation

The drawer is a drill-down of the column count. For a selected agent (or manager), the drawer should show the same set of scorecards that contributed to the column value under the same page filters.

### APIs involved

Both surfaces already use QA APIs:

| Surface | API | Frontend entry |
|---|---|---|
| Agent column count | `RetrieveQAScoreStats` | `AgentLeaderboardPage` → `useGetQAStats` → `useQAScoreStats` |
| Agent drawer rows | `RetrieveQAConversations` | `useAgentScorecardTemplateBreakdown` → `useRetrieveQAConversationsRequestParams` |
| Manager column count | `RetrieveQAScoreStats` | `ManagerLeaderboardPage` → `useQAScoreStats` |
| Manager drawer rows | `RetrieveQAConversations` | `useManagerScorecardTemplateBreakdown` → `useRetrieveQAConversationsRequestParams` |

No backend API change is required for the diagnosed root cause.

---

## 2. Root Cause

### Frontend mismatch

Agent drawer rebuilds filter state and clears voicemail:

`packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useAgentScorecardTemplateBreakdown.ts`

```ts
voicemailMoment: undefined,
```

Manager drawer does the same:

`packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useManagerScorecardTemplateBreakdown.ts`

```ts
voicemailMoment: undefined,
```

The column path keeps page `qaScoreFiltersState.voicemailMoment`. When set, `useMomentGroupFilterFromFilterState` converts it into:

```json
"momentGroups":[{
  "moments":[],
  "excludedMoments":[{"type":"VOICE_MAIL", ...}]
}]
```

### Backend behavior (correct for each request)

In `go-servers` QA ClickHouse helpers, an excluded `VOICE_MAIL` moment becomes:

```sql
is_voice_mail <> 1
```

on `conversation_d`, via `excludeVoiceMail(...)` in `parseConversationConditionsForQAAttribute`.

So:

- column request with voicemail exclusion → fewer scorecards
- drawer request without that exclusion → more scorecards

### ClickHouse confirmation (`cng_us_east_1`)

Of the three drawer conversations:

| conversation_id | is_voice_mail |
|---|---|
| `019f6687-16cd-781d-bc2f-5d82694bfb51` | true |
| `019f6735-3c15-7add-ae07-7352132f686a` | false |
| `019f67ad-7483-77e0-889d-94634ece9d3a` | true |

With voicemail excluded: 1 remains. Without: 3 remain. That matches the UI.

---

## 3. Goal / Success Criteria

1. When voicemail exclusion is enabled on Leaderboard filters, Agent drawer scorecard rows match the Agent column count for the selected agent/day.
2. When voicemail exclusion is disabled, both surfaces continue to include voicemail conversations.
3. Manager drawer preserves the same voicemail filter parity.
4. No intentional change to Manager submitted-only / submitter attribution semantics.
5. Knowledge docs updated so future sessions know drawer filters must inherit page filters.

---

## 4. Implementation Plan

### 4.1 Create worktree / branch

- Repo: `/Users/xuanyu.wang/repos/director`
- Suggested worktree: `/Users/xuanyu.wang/repos/director-convi-7385`
- Suggested branch: `convi-7385-qa-scorecard-api-count-mismatch-between-column-and-drawer`
- Base: latest `origin/main`

Register the worktree in `knowledge/workspace/repos.yaml` if that registry is maintained in the implementing session.

### 4.2 Fix Agent drawer filter inheritance

File:

`packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useAgentScorecardTemplateBreakdown.ts`

Change:

- Stop forcing `voicemailMoment: undefined`.
- Preserve `filtersState.voicemailMoment` from the incoming page filter state.

Current pattern spreads `filtersState` then overwrites voicemail. Preferred result:

```ts
const agentFiltersState = useMemo(
  (): PerformanceFiltersState => ({
    ...filtersState,
    submitDateRangeInternal: filtersState.submitDateRange,
    // Keep page voicemail exclusion so drawer matches column filters.
    usersTeamsGroups: {
      userNames: selectedAgent ? [selectedAgent.resourceName] : [],
      teamNames: [],
      groupNames: [],
    },
  }),
  [filtersState, selectedAgent]
);
```

Notes:

- Narrowing `usersTeamsGroups` to the selected agent remains correct.
- Do not clear other page filters that affect scorecard membership unless there is an explicit product reason.
- Confirm `filtersState` type already includes `voicemailMoment`; Agent call site passes `qaScoreFiltersState`.

### 4.3 Fix Manager drawer filter inheritance

File:

`packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useManagerScorecardTemplateBreakdown.ts`

Change:

- Stop forcing `voicemailMoment: undefined`.
- Preserve `filtersState.voicemailMoment`.

Manager drawer currently reconstructs a narrower `PerformanceFiltersState` rather than spreading page state. Keep Manager-specific fields (`scorecardStatus = MANUALLY_SUBMITTED`, `scoreResource = SCORECARD`, submitter audience), but pass through `voicemailMoment: filtersState.voicemailMoment`.

Also verify whether Manager page actually exposes/enables voicemail filtering in the shared Leaderboard filters. Even if currently uncommon, preserve the field for parity and future-proofing.

### 4.4 Audit other drawer filter deltas before shipping

While editing, compare column vs drawer request construction for accidental additional mismatches:

- `conversationDurationBuckets`
- `includeNaScored`
- `usecaseNames` / selected usecase
- `scorecardStatuses`
- `scoreResource`
- moment groups beyond voicemail
- agent-only / deactivated-user filters

For Agent, drawer should inherit page QA filters except for the intentional selected-agent narrowing.

For Manager, keep intentional submitter + submitted-only semantics from CONVI-6968; only restore shared conversation filters such as voicemail.

If additional mismatches are found, either:

- include them in this PR if they are clearly the same class of bug, or
- document them as follow-ups in the work item / Linear comment without blocking the voicemail fix.

### 4.5 Tests to add

There are currently no dedicated tests under `scorecard-template-breakdown-drawer/`. Add focused unit tests.

Suggested files:

- `useAgentScorecardTemplateBreakdown.test.ts`
- `useManagerScorecardTemplateBreakdown.test.ts`

Or, if hooks are awkward to render, test via request-param construction by mocking:

- `useRetrieveQAConversationsRequestParams`
- `useRetrieveAllQAConversations`
- `useGetScorecardTemplatesFilteredByPermissions`

Minimum assertions:

1. When input filters include a `voicemailMoment`, the drawer request params / filter state passed into `useRetrieveQAConversationsRequestParams` retain that moment (not `undefined`).
2. When input filters have `voicemailMoment = undefined`, drawer still works and does not invent an exclusion.
3. Agent drawer still scopes `usersTeamsGroups.userNames` to the selected agent.
4. Manager drawer still sets submitted-only + scorecard resource + submitter audience.

If existing test utilities already cover QA filter attribute conversion, prefer asserting the final `filterByAttribute.momentGroups` contains the VOICE_MAIL exclusion.

### 4.6 Manual validation

1. CNG `us-east-1`, Agent Leaderboard, Agnes Long, 2026-07-15 America/Toronto, voicemail exclusion on:
   - column count and drawer row count should both be 1.
2. Same view with voicemail exclusion off:
   - both should be 3 (or whatever current non-excluded total is).
3. Spot-check Manager Leaderboard drawer with voicemail exclusion on, if the filter is available on that tab.
4. Confirm network payloads:
   - drawer `RetrieveQAConversations` includes the same VOICE_MAIL `excludedMoments` as column `RetrieveQAScoreStats` when exclusion is enabled.

Optional CH sanity check after FE fix is not required if network payloads match, but can re-query `cng_us_east_1` if needed.

---

## 5. Out of Scope

- Do not change `RetrieveQAScoreStats` / `RetrieveQAConversations` backend contracts for this ticket.
- Do not add a new proto time-range field here; that is CONVI-7162 follow-up work.
- Do not change Manager submitter attribution or `MANUALLY_SUBMITTED` status filtering.
- Do not “fix” counts by making the column include voicemail while exclusion is on; the column semantics are correct.

---

## 6. Documentation To Update

Update these knowledge artifacts in the same implementing session (or immediately after the code PR):

### Required

1. `knowledge/analytics/work-items/CONVI-7385.md`
   - Status → `validating` / `complete`
   - Add implementation branch, PR link, and validation results
   - Update Next Actions

2. `knowledge/analytics/log/YYYY-MM-DD.md`
   - Record implementation movement and PR

3. `knowledge/analytics/sessions/YYYY-MM-DD/<tool>-convi-7385-fix.md`
   - Capture exact code changes, tests run, and any extra mismatches found

4. `knowledge/analytics/subdomains/leaderboard/README.md`
   - Convert the CONVI-7385 open question into a documented invariant:

   > Scorecard template drawers must inherit page conversation filters that affect scorecard membership, including voicemail exclusion. Drawer requests may narrow to the selected agent/manager, but must not silently drop shared filters such as `voicemailMoment`.

5. `knowledge/analytics/project.yaml`
   - Move CONVI-7385 out of active work items when complete
   - Refresh `last_validated` / notes

### Recommended

6. `knowledge/analytics/subdomains/qa-score/README.md` (if it discusses Leaderboard scorecard counts)
   - Add a short cross-link that Agent/Manager Leaderboard scorecard count and drawer share QA APIs and must share filter membership semantics.

7. Linear CONVI-7385
   - Comment with PR link and before/after validation
   - Mention Manager drawer included for the same parity bug

### Optional / only if product docs exist

8. Any internal Leaderboard QA/scorecard runbook or FE engineering notes under `convi-6968-schwab-leaderboard-launch` that describe drawer APIs should note filter inheritance.

---

## 7. Suggested Implementation Prompt For Next Session

Copy/paste starter:

```text
Implement CONVI-7385 using the plan in
/Users/xuanyu.wang/repos/knowledge/analytics/deliverables/convi-7385-column-drawer-count-parity-plan.md

Create a director worktree/branch from origin/main, preserve voicemailMoment in
useAgentScorecardTemplateBreakdown and useManagerScorecardTemplateBreakdown,
add regression tests, validate request parity, and update the knowledge docs listed
in section 6 of the plan. Do not change backend QA APIs.
```

---

## 8. Risks / Edge Cases

- Clearing `voicemailMoment` may have been intentional historically for drawer UX; product expectation now is count parity with the column. If anyone objects, escalate before shipping, but diagnosis evidence strongly supports inheritance.
- Manager drawer reconstructs filters more narrowly than Agent; only restore shared filters needed for membership parity.
- If page filters include additional moment groups beyond voicemail, Agent drawer currently spreads `filtersState` and should already inherit them once voicemail is no longer overwritten. Confirm after the change.
- Drawer pagination (`pageSize = 1000` + `useRetrieveAllQAConversations`) is unrelated to this 1-vs-3 mismatch, but keep an eye on large-result pages during manual QA.

---

## 9. Definition of Done

- [ ] Director branch/PR opened
- [ ] Agent drawer preserves `voicemailMoment`
- [ ] Manager drawer preserves `voicemailMoment`
- [ ] Unit/regression tests added and passing
- [ ] Manual CNG repro no longer shows 1 vs 3 with voicemail exclusion on
- [ ] Knowledge work item, log, session note, and leaderboard subdomain docs updated
- [ ] Linear ticket commented with PR + validation
