# CONVI-7974: Cresta-security scorecard listing fails on oversized user login status

**Status:** investigation complete; remediation pending
**Primary domain:** scorecard-workflows
**Primary subdomain:** scorecard-lifecycle
**Official ticket:** [CONVI-7974](https://linear.app/cresta/issue/CONVI-7974)
**Last updated:** 2026-11-05

## Objective and Impact

Diagnose the reported oversized scorecard-list response for cresta-security/ps, recover the exact request, and inspect data safely. Role: diagnosed and coordinated parallel code/log investigation. Impact: identifies the actual failing auth response and avoids an ineffective scorecard-pagination fix.

## Current Understanding

The captured request was `GET /v1/customers/cresta-security/profiles/ps/scorecards?pageSize=1` at 2026-11-04 21:45:19 UTC, trace `7475789543106183524`. Its downstream `UserService.ListUsers` response was 18,087,178 bytes, exceeding the apiserver client's 4,194,304-byte receive limit.

Current auth user `87fd42472e35f08a`, referenced by scorecard creator/updater fields, has 18,102,156 bytes of login-status JSON. Its 214 application-status entries contain 18,080,008 bytes of navigation paths; the largest individual path is 10,485,772 bytes. Source copies all paths into returned users. Writer validation checks slash-separated format, with no byte cap, and status normalization marks stale entries offline without deleting paths.

The ps app DB contains only two scorecards (55 comment bytes total), two criterion scores with no comment/text values, 23 small template rows and 12 collaboration comments (329 bytes total). No large comment or high scorecard population was found.

## Source and Validation

- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`, read-only fetched origin/main `a6e7a34e6b7b0da54a47968581518b926689cf46`; no source edits/worktree creation.
- Proto ref: `ee3696186fabdcab35f73cbbe848b502c3e68525`.
- Config: `12fd13d414fa72997ede666df038aa75fdd90e34`.
- App DB: us-west-2-prod / us-west-2-prod-1 / cresta-security-ps.
- Auth DB: chat-prod / auth-prod / cresta-security.
- Every SQL call read-only with statement timeout 60 seconds and lock timeout 3 seconds; no timeouts.
- Original request recovered from traces, not replayed. No code/data changes or Slack message. CONVI-7974 was created after the investigation.
- ListScorecards has count pagination (default 100, max 5000), no byte-aware response construction. Calibration append can exceed requested count. Smaller pages cannot solve this case because one user's field already exceeds the client cap.
- Five other sampled ListScorecards failures report empty template names and must be handled separately.
- Earlier same-customer incident not located. Prior RCG template and voice-sandbox-2 comment incidents were verified as different cases.

## Next Actions

Content follow-up on 2026-11-05: the largest path is `ps/cc-voice/` plus exactly 10,485,760 uppercase `A` characters. All ten largest page segments are exclusively `A`; 30 such entries total 17,662,336 page bytes. These are filler strings rather than real page identifiers. Graduated lengths and other validation/XSS-probe names strongly suggest input/security testing; actor and write mechanism remain unverified. See detailed session follow-up.

1. Agree and implement navigation-path length/retention limits, then safely repair existing oversized status entries.
2. Provide a lightweight user projection for scorecard enrichment that excludes login status before transport serialization.
3. Add regression coverage for one scorecard referencing an oversized user. General byte-aware paging is useful but insufficient for a single oversized record.

## Timeline and Evidence

- 2026-11-05 — Completed code, trace and read-only DB investigation. [Detailed session](../sessions/2026-11-05/codex-cresta-security-oversized-scorecards.md), [SQL](../sessions/2026-11-05/cresta-security-size-queries.sql), [daily log](../log/2026-11-05.md).

- 2026-11-05 — Created and read back CONVI-7974, linked the Slack report, and recorded confirmed downstream ListUsers failure, probable test origin, input/response protections and logging follow-up. Slack reply remains a draft.
