# CONVI-6862 — FE/BE feature flag mismatch bug

**Date:** 2026-07-02  
**Source:** [Linear comment 3433a8bf](https://linear.app/cresta/issue/CONVI-6862/disable-editing-on-submitted-scorecard#comment-3433a8bf) by Sergei Radchenko

## Summary

QA smoke test on walter-dev found inconsistent submitted-scorecard edit behavior when the frontend feature flag `disableEditingOnSubmittedScorecards` is disabled for a user.

- **Denied user, FF off:** UI looks editable; save fails with generic permission error (reactive 403), not the designed proactive lock message.
- **Permitted user, FF off:** Scorecard remains editable (consistent only if backend enforcement is also flag-gated).

## Root cause

Frontend gates proactive lock UX and permission queries behind `disableEditingOnSubmittedScorecards`. Backend enforces submitted-editor permissions without an equivalent flag gate.

## Ticket created

- **CONVI-7206:** [Bug] FE/BE mismatch: submitted scorecard lock gated by feature flag on FE only
- Parent: CONVI-6862
- https://linear.app/cresta/issue/CONVI-7206/bug-febe-mismatch-submitted-scorecard-lock-gated-by-feature-flag-on-fe

## Fix options captured in ticket

1. Add matching feature-flag gate on backend (gradual rollout).
2. Remove FE flag gating if backend is always-on (always show proactive lock UX).
