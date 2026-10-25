# CONVI-7777 — Refresh main and verify protobuf pin

**Primary domain:** analytics; **subdomain:** assistance-insights.
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`.
**Branch/worktree:** clean main checkout `/Users/xuanyu.wang/repos/go-servers`, fast-forwarded from `435cb9047a6bb42775f10cd8680517a5704ea234` to fetched origin/main `40faeac75f2d55b06a4e3e29ac64fe782b8abd99`. Ticket worktree `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, branch `xw/convi-7777-assistance-usage`, remains at `8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498` with its pre-existing draft unchanged.

## Request and result

User asked to refresh the codebase and check the new protobuf pin, and stated intent to use a stack of small PRs. Fetched origin/main, confirmed main was clean and an ancestor, then fast-forwarded it. Go and Bazel both pin `cresta-proto/v2 v2.32.18`, checksum `h1:LTFTQzNKgilm2ouUY/Lkml8ypU9lcWX58zmg4Qz7llM=`. The dependency update is commit `bf667921c4`; the module release resolves to cresta-proto commit `b382d67a94869811d57a653ae2f361f3d7145877`. Downloaded that exact module and inspected generated RPC/request/response/enum symbols. The dedicated RPC, opt-in details request flag, response details and final feature enum including Chat Suggestions are present. This confirms dependency availability, not a backend RPC implementation or runtime validation.

## Source and preservation checks

The Step 1 handler, cohort SQL file, shared parser, user-filter helper, and versioned-conversation integration test are byte-identical between ticket HEAD and refreshed main. Main remains clean after download. Ticket tracked diff SHA256 remains `6d33c90d77c8886c22a0815a895d0b5824e7270c44e87155a084448280115c98`, and untracked query SHA256 remains `f4162182f74f1c6ca566b7b6f365fba8090fce36a3ec1fa722535b2115336e99`. No draft cleanup, ticket branch rebase/merge, implementation, commit, push or PR creation occurred. Git/Go used configured authentication; no AWS, Okta, Azure or SSH credential/profile was explicitly selected or read.

## PR stack direction

Step 1 is one PR for the shared eligible-population builder plus regression coverage. Step 2 may be split into feature-family PRs to keep shared queries reviewable; this is a recommendation pending inspection of code/diff size, not a fixed split. Steps 3/4 may combine if the concrete combined diff is small enough. Preserve conceptual step review checkpoints even when PR packaging differs. Updated contract, report, implementation plan and work item with these current facts; no implementation has started.

## Validation and next action

Pin and generated-symbol checks passed; main is clean and ticket hashes match. No backend tests ran for this dependency-inspection task. Report text changes preserve its existing controls. Refresh/prepare the first PR's branch from current main when starting Step 1, preserving the superseded draft as needed. Do not treat the still-old ticket worktree as refreshed.
