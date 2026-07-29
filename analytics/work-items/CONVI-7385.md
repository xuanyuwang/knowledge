# CONVI-7385: QA scorecard API count mismatch between column and drawer

**Status:** validating — draft PR open
**Primary domain:** `analytics`
**Primary subdomain:** `leaderboard`
**Official ticket:** [CONVI-7385](https://linear.app/cresta/issue/CONVI-7385/qa-scorecard-api-count-mismatch-between-column-and-drawer)
**Last updated:** 2026-07-28

## Objective and Impact

- **Objective:** Explain why Agent Leaderboard scorecard column shows 1 while the scorecard template drawer shows 3 for the same agent/day.
- **Customer/system impact:** Misleading QA scorecard count drill-down on Agent (and likely Manager) Leaderboard.
- **Role:** diagnosed

## Scope

**In scope**

- Agent Leaderboard scorecard count column vs template-breakdown drawer.
- Request-shape and ClickHouse verification for the CNG `us-east-1` repro.

**Non-goals**

- Broader submit-time vs conversation-time semantics (CONVI-7162).
- Changing backend API contracts unless FE parity alone is insufficient.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/go-servers`
- **Worktree:** `/Users/xuanyu.wang/repos/director-convi-7385`
- **Branch:** `convi-7385-qa-scorecard-api-count-mismatch-between-column-and-drawer`
- **Commit:** `506de9db6f`
- **PR:** [cresta/director#21214](https://github.com/cresta/director/pull/21214) (draft)

## Current Understanding

Root cause is a frontend filter mismatch, not a ClickHouse/API count bug.

The Agent column uses `RetrieveQAScoreStats` with the page's `qaScoreFiltersState`, which includes the voicemail exclusion (`excludedMoments` of type `VOICE_MAIL`). The drawer uses `RetrieveQAConversations` via `useAgentScorecardTemplateBreakdown`, which intentionally clears `voicemailMoment: undefined`, so the drawer does not exclude voicemail conversations.

For Agnes Long (`customers/cng/users/2bdb21d567b2118f`) on 2026-07-15 America/Toronto:

- Drawer/`RetrieveQAConversations`: 3 scorecards
- Column/`RetrieveQAScoreStats`: 1 scorecard
- ClickHouse: 3 scorecards exist; 2 conversations are `is_voice_mail = true`, 1 is not

Backend correctly translates a VOICE_MAIL excluded moment into `conversation_d.is_voice_mail <> 1` for QA conversation filtering. The stats path therefore drops the two voicemail scorecards; the drawer path does not.

The Manager drawer helper has the same `voicemailMoment: undefined` pattern and likely has the same parity bug when voicemail exclusion is active.

Implementation is staged in the dedicated Director worktree. Agent drawer now inherits the complete QA filter state, and Manager column plus drawer both pass through `voicemailMoment`. The shared Leaderboard QA filter-state type now includes performance filters explicitly.

The implementation audit found that latest `origin/main` also omitted voicemail from the Manager column request. Fixing only the Manager drawer as originally planned would have created reverse mismatch, so both Manager requests were updated while retaining submitted-only and submitter-audience semantics.

## Findings and Decisions

- FE Agent drawer clears voicemail filter: `useAgentScorecardTemplateBreakdown.ts` sets `voicemailMoment: undefined`.
- FE Manager drawer also clears voicemail filter: `useManagerScorecardTemplateBreakdown.ts`.
- BE voicemail exclusion for QA APIs is implemented in `parseConversationConditionsForQAAttribute` via `excludeVoiceMail(...)` → `is_voice_mail <> 1`.
- CH validation on `cng_us_east_1` confirms 2/3 conversations are voicemail.

## Blockers and Dependencies

- None for diagnosis.
- Fix requires a `director` change to preserve `voicemailMoment` (and any other shared page filters the drawer should inherit).

## Validation and Rollout

- Repro request/response pairs captured in the session note.
- ClickHouse verified on `us-east-1-prod` / `cng_us_east_1`.
- No new tests were added, per implementation direction.
- `git diff --check` and Biome checks pass; CodeRabbit returned zero findings.
- `yarn lint:precommit`, `yarn tsc`, and all commit hooks pass after package authentication was refreshed.
- Manual CNG browser/network validation remains pending.

## Next Actions

1. Validate the CNG Agnes Long repro and Manager network payload.
2. Add preview/proof details to the draft PR and select the appropriate QA testing request.
3. Monitor PR review and CI, then move the ticket to complete after validation.

## Timeline

- 2026-07-28 — Diagnosed column vs drawer mismatch as voicemail filter dropped by drawer FE. Evidence: `sessions/2026-07-28/codex-convi-7385-column-drawer-mismatch.md`, CH queries on `cng_us_east_1`.
- 2026-07-28 — Wrote implementation plan: `deliverables/convi-7385-column-drawer-count-parity-plan.md`.
- 2026-07-28 — Implemented and staged Agent/Manager filter-parity changes without adding tests; validation/commit blocked on private-package authentication. Evidence: `sessions/2026-07-28/codex-convi-7385-fix.md`.
- 2026-07-28 — Refreshed package authentication, passed lint/type/commit hooks, pushed commit `506de9db6f`, opened draft PR [#21214](https://github.com/cresta/director/pull/21214), and linked it from Linear.
