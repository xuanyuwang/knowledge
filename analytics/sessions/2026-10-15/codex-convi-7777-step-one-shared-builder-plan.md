# CONVI-7777 — Step 1 shared-builder plan

**Primary domain:** analytics; **subdomain:** assistance-insights.
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`.
**Branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, inspected HEAD `8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498`.

## User request and scope

User requested removing Step 0 from the report because branch cleaning is preparation, and plans to try updating the branch to latest origin/main tomorrow (October 16, 2026). Supplied another session's source-only shared-builder plan. This turn updates documentation/report only; no branch cleanup, fetch, merge, rebase, reset, dependency update or implementation is authorized by the deferred statement or was performed. No reminder or automation was requested or created.

## Plan incorporated

Shared builder owns complete population eligibility, including both metadata and ordinary paths, before aggregation. One row per conversation/agent carries selected conversation time and aht_sec. Keep MAX over matching messages so the legacy consumer avoids an extra scan. Return SQL, args and external tables together; use RPC-independent options and retain user/ACL resolution plus empty-selection guards before the builder. ConversationStats wraps the relation for unchanged counts/handle times/buckets; AssistanceUsage projects start-time keys. Keep selected time generic for legacy end-time support. Only these two consumers are in scope; public RPC and feature evidence are later checkpoints.

Source checks confirmed metadata joins use distinct conversation IDs, selected-or-missing parsing precedes ordinary filter parsing, and external tables have an existing attachment helper. Preserve filter placement before latest matching version, time from conversation/label row, branch-specific source/dev checks, and filtered message MAX. Concrete cases and stop conditions are saved in the [Step 1 contract](../../deliverables/convi-7777-backend-step-one-contract.md).

## Artifact changes and limits

Replaced current report's baseline/preparation narrative with Step 1 plan sections and explicit proposed/unimplemented labels. Removed preparation from the active construction record and updated roadmap context, work item and domain summary. Preserved older baseline/session evidence rather than deleting history. No backend tests ran for this documentation update. No credential/profile was explicitly selected or read. No product code, branch state or dependency changed.

## Verification

Verified the active HTML contains no Step 0 or baseline-contract link; JavaScript passed node --check, IDs are unique and local links resolve. Opened the report in the in-app browser, exercised all five plan sections in both snapshot/change views plus Previous/Next controls, and visually inspected the current viewport. A separate mobile viewport was not tested. Scoped git diff --check passed. Backend HEAD and tracked-diff/untracked-query hashes match the prior inspection, confirming no branch/product edits. No backend tests ran because this update changes documentation only.
