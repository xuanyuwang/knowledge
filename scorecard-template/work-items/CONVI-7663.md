# CONVI-7663: Bound ListCurrentScorecardTemplates responses by serialized size

**Status:** active
**Primary domain:** `scorecard-template`
**Primary subdomain:** none
**Official ticket:** [CONVI-7663](https://linear.app/cresta/issue/CONVI-7663/bound-coaching-list-responses-by-serialized-size)
**Last updated:** 2026-09-14

## Objective and Impact

- **Objective:** Prevent oversized `ListCurrentScorecardTemplates` responses by bounding serialized bytes and returning only the fields each consumer needs.
- **Customer/system impact:** RCG template-permission loading fails at the 50 MiB gRPC server-send ceiling after loading unused full template bodies.
- **Role:** diagnosed and implemented frontend mitigation

## Scope

**In scope**

- Pagination for `ListCurrentScorecardTemplates`.
- Byte-aware pagination contracts.
- Narrow response views/projections.
- Evaluate whether `EvaluateScorecardsPermissions` can replace Director's local template-permission evaluation.

**Non-goals**

- Treating a global gRPC limit increase as the durable fix.
- Deleting or consolidating RCG templates without product/configuration review.
- The scorecard-export / `ListComments` incident, now tracked in CONVI-7671.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/cresta-proto`
- **Worktrees:** `/Users/xuanyu.wang/repos/director-convi-7663` for the Director mitigation; main checkouts for prior read-only investigation
- **Branches:** `xw/convi-7663-usecase-scope` in Director
- **PRs/commits:** Director PR [#22809](https://github.com/cresta/director/pull/22809) / `49e6af05e5`, review fixes `ecb8f70c18` and `1efbdb7431`; go-servers PR #30472 / `a81bcb24ad` is the prior fixed-page-size mitigation

## Current Understanding

RCG's unpaginated full-template response exceeds a 50 MiB send limit. The shared frontend permission hook is mounted across many surfaces; Closed Conversations was the dominant observed trigger (749 failures in the incident window), while Performance Insights and Opera each accounted for 12. Its all-usecase request fetched 3,330 distinct template resources across exactly 98 individual use cases. `EvaluateScorecardsPermissions` is not a direct replacement for the template hook: it requires persisted scorecards and only implements submitted-lock modification; a batched template-permission API is the preferred long-term contract. The separate `ListComments` failure has been split into CONVI-7671.

## Findings and Decisions

- Priority order: narrow the permission contract; paginate current templates; make pagination byte-aware; return less data.
- Record-count pagination is necessary but insufficient for variable-size protobuf messages.
- `EvaluateScorecardsPermissions` cannot currently supply the hook's edit/view/grade/appeal/publish template decisions.
- Prefer server-side batched template-permission evaluation; a permissions-only view is lower cost but preserves duplicated semantics.
- Permission filtering for list screens must happen before pagination.
- The primary observed frontend scenario was opening/viewing conversation scorecard details: `ScorecardFormLoader` and `useConversationCoachingDetails` both mount the shared permission hook. React Query deduplicates their identical permission request.
- The measured 98 `usecase_ids` sets are also exactly 98 individual use cases: all 3,330 selected resources have one use case each, for 3,330 template-to-use-case assignments and 294 distinct titles.
- No use case dominates the payload: `rcg-default` is largest at 126 templates (3.78%) and 2.79 MB of stored JSON (3.87%). The top ten account for only 22.22% of templates. Supplying the conversation use case would have reduced the measured worst-case row set by 96.22%, making it a strong narrow mitigation for conversation pages.
- Use-case scoping is feasible as a quick patch only when applied per consumer: pass the same effective use-case set to the permission hook and the list it filters. Do not change the hook's global default; notification and admin selectors intentionally operate across all use cases.
- The backend use-case filter retains null/empty global templates and uses overlap semantics for multi-usecase templates. RCG has neither current resource IDs spanning multiple use-case sets nor historical use-case reassignment among the 3,330 current resources.
- A naive global change can hide templates in all-usecase screens or create list/permission scope mismatches. Historical reassignment remains a product-wide edge case requiring a behavioral test even though it is absent in RCG.
- The Director quick mitigation is limited to Closed Conversations scoring call sites. It leaves the permission hook's all-use-case default intact, passes the conversation use case through the coaching-details, form-loader, form, and reset-button path, and skips transient permission requests while conversation scope is unknown.
- Filter, Coaching Hub, admin, AI Agent, and Opera consumers are explicitly outside this patch and retain their original behavior.

## Blockers and Dependencies

- Backend serialized-size protection and any narrower permission contract remain separate follow-up work across `cresta-proto` and `go-servers`.

## Validation and Rollout

- The narrowed Director worktree implementation passes TypeScript, changed-file lint, and 2 focused permission-request tests.
- RCG deployed-environment request-shape and serialized-size validation remains pending.

## Next Actions

1. Review Director PR [#22809](https://github.com/cresta/director/pull/22809).
2. Validate an RCG conversation detail request in a deployed environment, including serialized response size.
3. Decide whether backend pagination/byte bounds or a narrower permission contract are still required as defense in depth.

## Timeline

- 2026-09-09 — Created CONVI-7663 and consolidated the two production incidents. Evidence: `../sessions/2026-09-09/codex-list-current-templates-oversize.md`, `../log/2026-09-09.md`, Linear ticket.
- 2026-09-09 — Completed the `EvaluateScorecardsPermissions` audit and appended the conclusion to CONVI-7663.
- 2026-09-10 — Split the `ListComments` incident into CONVI-7671 and narrowed CONVI-7663 to the template-list response.
- 2026-09-10 — Traced the frontend trigger distribution and exact request scope. Closed Conversations produced 749 incident-window failures; the all-usecase hook fetched 3,330 distinct resources across 98 individual use cases. Evidence: `../sessions/2026-09-10/codex-convi-7663-frontend-trigger-scope.md`.
- 2026-09-10 — Measured the per-use-case distribution. The largest use case had 126 templates (3.78%); the top ten had 22.22%, confirming that adding the current use case is a high-leverage conversation-page mitigation rather than merely moving the same concentration into one request.
- 2026-09-10 — Completed the use-case-scoping feasibility review. Recommended consumer-by-consumer scope alignment, rejected a global default change, confirmed backend global/multi-usecase behavior, and identified historical reassignment plus all-usecase selectors as required regression cases.
- 2026-09-13 — Implemented an initial broad consumer-level mitigation in `/Users/xuanyu.wang/repos/director-convi-7663`; this version was superseded by the narrower scope decision on 2026-09-14.
- 2026-09-14 — Narrowed the worktree to Closed Conversations scoring call sites only and restored filter, Coaching Hub, admin, AI Agent, and Opera behavior. TypeScript, lint, and 2 focused tests passed. Evidence: `../sessions/2026-09-14/codex-convi-7663-closed-conversation-scope.md`.
- 2026-09-14 — Rebased onto current `origin/main`, committed as `49e6af05e5`, and opened draft Director PR [#22809](https://github.com/cresta/director/pull/22809) using the repository template with both Closed Conversations call paths in the description.
- 2026-09-14 — Applied review fix `ecb8f70c18`: skipped permission lookups now return no permissions, preventing filtering/grading/reset actions before scope exists, while ordinary loading and error states retain the existing permissive fallback.
- 2026-09-14 — Applied review fix `1efbdb7431`: omitted use-case scope remains an unscoped permission lookup, while Closed Conversations explicitly signals when it is waiting for conversation scope.
