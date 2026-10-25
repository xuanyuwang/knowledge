# CONVI-7777 first construction pass — 2026-10-12

**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Primary branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`
**Related worktrees:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7777-assistance-usage` and `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`, both on `xw/convi-7777-assistance-usage`

## Inputs and settled decisions

- [Feature inventory](../../deliverables/convi-7777-pm-feature-inventory.md) and [API semantics audit](../../deliverables/convi-7777-assistance-api-semantics-audit.md).
- User confirmed the denominator should match the Assistance page's message-bearing total, and Silence/Hold should be excluded pending delivered-hint verification.
- Existing per-feature SQL/code is the source of truth for current use predicates. The Summary branch therefore keeps stored types 1/2, while the proposed copy-only wording remains unresolved.
- The requested construction pass skips tests. Each step is recorded in the [construction record](../../deliverables/convi-7777-construction-steps.md).

## Execution

1. Extended `RetrieveConversationStats` proto source with an opt-in use count at response, group, and frequency levels. Added a response marker to distinguish computed zero from an older server. Generated the two changed Go proto files using a local `protoc-gen-go` and a two-file Buf template. The targeted Bazel proto target also built successfully.
2. Added a conditional V1 use-evidence CTE in go-servers. The CTE deduplicates conversation–agent keys across hints, GenAI answer use, Summary, Notes, KB search, workflow document use, and chat Suggestions. Normal and metadata-filtered ConversationStats queries join that evidence to the existing eligible cohort, scan the assisted count, and carry it through Team aggregation.
3. Wired Director's Agent and Team total-conversation requests to opt in. Added API transformation, a supported-response guard, a period-total percentage column, CSV header, and extracted en-US translation keys. The existing Powered by AA calls and Overall Engagement formula were left untouched.
4. Created the [interactive construction review](../../deliverables/convi-7777-assistance-usage-construction-review.html). Its JavaScript passed `node --check`; static source links were corrected. Browser visual inspection of a local file was blocked by the browser URL policy and was not attempted through another route.

## Verification and limits

- Completed: `gofmt`, Director Biome formatting, en-US locale extraction, `git diff --check` in source worktrees, targeted proto Go generation, targeted Bazel proto build, HTML JavaScript syntax check.
- Deferred by request: unit and integration tests, deployed API and customer-row checks, source/bot and transfer scenarios, historical coverage, and query latency.
- `go-servers` still pins `cresta-proto/v2` 2.31.32, and Director pins `@cresta/web-client` 2.30.77. Neither published version contains the new fields. The Director adapter uses a structural type bridge for this first pass; dependency updates and end-to-end compilation remain.
- Existing Notes/Guided Workflow metadata RPCs use conversations-cluster event types 5/4 where stored types are 17/14. The new query uses 17/14; parity with those existing metadata branches is unresolved.
- The evidence query is bounded by conversation start time. An opt-in request with conversation-ended time is explicitly rejected so the new count cannot silently use a different time basis from its denominator.
- No credential/profile was explicitly selected and no customer data was queried. No Linear mutation, PR, commit, or push occurred.

## Next

Review the implementation and construction walkthrough. Then publish/update generated proto clients, replace the Director type bridge, run focused tests and representative data checks, and benchmark the conditional union query before rollout.
