# Evaluation editing versus template editing roles

**Date:** 2026-08-26  
**Source context:** `/Users/xuanyu.wang/repos/director-current`, `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/cresta-proto`  
**Question:** Can a dedicated QA role edit evaluations without being able to edit scorecard templates?

## Conclusion

- For ordinary, unacknowledged evaluations, yes: `QA_SPECIALIST` (QM Specialist), or a custom role inheriting from it, can grade/update/submit scorecard instances without receiving Performance Templates access.
- This does not solve the reported acknowledged-evaluation case. Director's acknowledgment lock is a frontend role check that only exempts `QA_ADMIN`.
- A custom role inheriting from `QA_ADMIN` is not a safe separation. The inherited proto role is accepted by backend `CreateScorecardTemplate` and `CommitScorecardTemplate` RPC authorization, even if the Performance Templates page is omitted from the custom role's UI features.
- Therefore there is currently no clean role that grants editing after acknowledgment while reliably denying template editing.

## Evidence

- `ScorecardForm.tsx`: `acknowledgedShouldBlock = acknowledgedAt && !has QA_ADMIN`.
- `coaching_service.proto`: `UpdateScorecard` and `SubmitScorecard` accept `QA_SPECIALIST`; template create/commit accept `QA_ADMIN`, `ADMIN`, and `SUPER_ADMIN`.
- `edit_permission.go`: normal scorecard grading defaults include `QA_SPECIALIST`; template-level `scorecard_graders` can override that list.
- Permission feature definitions: Agent Evaluations includes `QA_SPECIALIST`, while Performance Templates defaults to `ADMIN` and `QA_ADMIN`.

## Product direction

Introduce a granular edit-after-acknowledgment capability, or define that an allowed submitted-scorecard editor overrides the acknowledgment lock. Then assign that capability/allowlist to the dedicated QA users without granting `QA_ADMIN`.
