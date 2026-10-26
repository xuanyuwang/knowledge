# Decision Record - Pivot to Permitted Users and Audience-Style Modeling

**Date:** 2026-05-22  
**Status:** Superseded

## Superseded Note

This decision captured an intermediate design direction.

The merged backend and current frontend implementation now use `submitted_scorecard_editors` / `submittedScorecardEditors` as a `UserTeamGroup`-style field with `users + teams + groups`. Keep this record for history, but do not treat it as the active contract.

## Context

The 2026-05-19 draft framed the exception path as a role-based `submitted_scorecard_editors` permission and kept `ResetScorecard` outside the active v1 lock scope.

The 2026-05-22 Linear thread refined the requirement further:

- scope is only normal Closed Conversations scorecards and normal process scorecards
- appeal and calibration scorecard types are out of scope for this round
- reset must be included in the submitted-lock scope
- submit remains a first-submit action
- the exception model is permitted users, not permitted roles

## Decision

The 2026-05-22 thread supersedes the 2026-05-19 role-based product framing.

Specifically:

- `ResetScorecard` is included in the submitted-lock scope
- submit remains a first-submit action and is not itself a disabled action
- permitted-user configuration should align with audience/resolved-audience modeling
- `submitted_scorecard_editors` is deprecated design baggage, not the preferred product-facing model

## Reasoning

- The product clarification now speaks in terms of permitted users, so the schema direction should follow that requirement directly.
- `Audience` and `ResolvedTemplateAudience` already provide the closest existing scorecard-template pattern for user-based configuration plus resolved output.
- Treating `ResetScorecard` as part of the lock scope removes an obvious loophole in the submitted-lock model for the in-scope scorecards.
- Keeping first submit allowed preserves the intended lifecycle transition rather than conflating submission with post-submit editing.

## Consequences

- Knowledge docs and ticket language must stop presenting role-based `submitted_scorecard_editors` as the preferred design.
- Proto planning should pivot to audience-style permitted-user fields and corresponding resolved output.
- Existing downstream work that introduced `submitted_scorecard_editors` may need compatibility handling, but that field should be documented as deprecated.
- The current backend implementation branch can be discarded and restarted from `origin/main` because it explored the superseded role-based framing.
