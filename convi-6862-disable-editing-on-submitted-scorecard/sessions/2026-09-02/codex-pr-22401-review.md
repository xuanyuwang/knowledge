# Review: director PR #22401 against #22149

Date: 2026-09-02  
Source repo: `/Users/xuanyu.wang/repos/director`  
Review refs: `origin/pr-22401` (`d7e951a2467`), `origin/pr-22149` (`157d826e890`)

## Inputs

- [director#22401](https://github.com/cresta/director/pull/22401)
- Completed draft [director#22149](https://github.com/cresta/director/pull/22149)
- Matching backend [go-servers#31889](https://github.com/cresta/go-servers/pull/31889)
- Original acknowledged-scorecard policy [director#13338](https://github.com/cresta/director/pull/13338)
- Existing CONVI-7598 work item and RCG root-cause notes

## Comparison

#22149 keeps authorization unchanged and makes a shared lock reason/message drive the closed-scorecard banner and submit tooltip, while also aligning submitted-permission messaging in `ProcessScorecardScoring`.

#22401 independently centralizes the closed-scorecard lock reason and additionally:

- bypasses submitted-editor evaluation for Cresta admins;
- bypasses acknowledged and appeal-requested UI locks for Cresta admins;
- requires backend PR #31889;
- omits the process-scorecard messaging changes from #22149.

## Findings

### Major: visual lock and write guard diverge

At #22401 head, `disableScorecardForm` is derived from `lockReason`, but `useSaveScorecardMutation({ readOnly })` and `renderSubmitButton` still use the old `baseReadOnly || submittedEditPermissionReadOnly` calculation.

Consequences:

- `acknowledged`, `answer-key`, and `appeal-requested` disable the controls without entering the save guard;
- a queued autosave or other submission path can still reach the mutation after one of those locks becomes active;
- submit-button rendering and the banner no longer derive from the same state.

The correction should derive a write-level read-only value from `lockReason`, excluding `in-appeal` because appeal edits intentionally save through the appeal path. Integration tests should cover the form wiring, not only the pure helper.

CodeRabbit independently posted the same major finding on [the PR thread](https://github.com/cresta/director/pull/22401#discussion_r3917429160).

### Follow-up: process-scorecard parity is absent

#22149 updates `ProcessScorecardScoring` to use the shared reason-specific message for both its warning and submit tooltip, including checking/denied/revoked distinctions. #22401 changes the shared submitted-permission hook, which is consumed by process scorecards, but leaves that surface's generic message and tooltip behavior unchanged.

This is a scope/consistency follow-up rather than the primary blocker if CONVI-7612 intentionally targets only the closed-scorecard form.

## Validation

- `git diff --check` passed for both PR heads.
- #22401 build, tests, typecheck, lint, coverage, dependency review, and secret scan passed.
- #22401 codeowner check remained pending at review time.
- No GitHub review or comment was submitted during this session.
