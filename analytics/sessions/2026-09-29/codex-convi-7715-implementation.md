# CONVI-7715 dynamic-group filter implementation — 2026-09-29

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Branch/worktree:** `xw/convi-7715-dynamic-group-filter` at `/Users/xuanyu.wang/repos/go-servers-convi-7715`  
**Base:** refreshed `origin/main` at `19cd65cbb5`; implementation committed as `899da6f56e`.

## Change

- `insights-server/internal/analyticsimpl/common_user_filter.go`: after `listAllUsers`, resolve requested group names through profile-scoped `shared.ListGroups`. Use returned group type, not the name-only request object's type. Retain TEAM groups; flatten DYNAMIC_GROUP members in `groundTruthUsers` into requested users. Apply the existing ACL and Step 2 logic afterward.
- Preserve selected audience intent before normalization. If no requested users or TEAM groups remain, return an empty result; do not enter the existing no-filter fallback.
- `insights-server/internal/analyticsimpl/common_user_filter_test.go`: add cases for manager-scoped dynamic members, empty and missing groups, direct membership, wrong request type, dynamic plus TEAM under disabled and limited ACL, and dynamic plus explicit user. Ordinary request fixtures remain name-only; mock UserService groups provide authoritative types.

## Validation

- `bazel test //insights-server/internal/analyticsimpl:common_user_filter_test --test_output=errors`: passed locally after the functional and test edits. The initial run failed only because an existing invalid-group test expected config loading before validation; its mock was updated to the new order.
- `gofmt` on both edited source files and `git diff --check`: passed.
- Committed as `899da6f56e`; no PR, handler integration test, customer staging request, or deployment in this session.

## Review note

Selected-group requests now make one additional `ListGroups` call for metadata. Unknown or profile-absent group names contribute no members, and an entirely unresolved selection returns empty. The historical Oportun request/build remains unverified.
