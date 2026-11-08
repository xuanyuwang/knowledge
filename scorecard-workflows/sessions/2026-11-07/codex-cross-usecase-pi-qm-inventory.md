# Cross-use-case PI and QM Report investigation and local ticket drafts

Date: 2026-11-07 (America/Toronto). Primary domain: scorecard-workflows. Related domain: analytics. Role: diagnosed and designed the implementation breakdown.

## Request and source context

Inventory every affected Performance Insights and QM Report path, beginning with selected/authorized use-case propagation from frontend through API to PostgreSQL/ClickHouse. Inspect the Walter staging page and prepare concrete ticket descriptions after duplicate checks. The user subsequently requested reversing ticket creation and keeping the proposals only in a local document. Preserve current-versus-historical semantics. No product implementation requested.

Primary source repo: `/Users/xuanyu.wang/repos/go-servers`, branch `main`, main checkout. Reviewed fetched origin/main `fc3fa4fa48de29914c8357892f30f23a6c005533` without switching the source checkout. Related Director `/Users/xuanyu.wang/repos/director` at fetched `a506cbe0684e93bbd9e892a92ad1fb7cf7b5bcae`; proto `/Users/xuanyu.wang/repos/cresta-proto` at `52b428ad53b9ffe6cb5329c2739e807eb5fa00df`. Checkout-vs-fetched refs and drift checks are in the inventory. Source repos were not edited.

Knowledge work happened directly in `/Users/xuanyu.wang/repos/knowledge`; preexisting unrelated changes were preserved. Repository operating-model, domain-model and daily-capture instructions were applied. Existing full plan, P0 sketch, November 4 decisions and task-scope record supplied the design context.

## Outcome and impact

The [change inventory](../../deliverables/cross-usecase-pi-qm-change-inventory.md) records 17 PI surfaces, 14 QM paths across Evaluations/Appeals/Group Calibration, eight export families and 45 evidence groups with immutable source links. It separates current applicability, historical fact/pinned-revision reads, future QM task discovery and independent GC task scope.

Key findings:

- Template picker replaces a supplied array with all available use cases; report/export hooks can overwrite it with global singleton selection.
- Current-template queries filter before latest selection and include scope in DISTINCT identity, in both full and metadata paths/shared readers.
- Two QM fact-query branches omit selected-use-case predicates. Manual QA detail queries already have optional predicates; parsing, task/period resolution and callers still need changes.
- Appeals validates a singleton but drops it from conditions and does not filter its scorecard population. Lifting singleton validation alone cannot fix it.
- QA grouping lacks a use-case dimension; ordinary/metadata ClickHouse paths and current-template-derived defaults/config all need review without losing pinned historical facts.
- ConversationCountChart uses RetrieveQAScoreStats for filtered/unfiltered/delta counts. AHT uses RetrieveConversationStats. This corrects a loose grouping in the earlier sketch.
- GC task filtering does not prove its scorecards' use-case invariant for legacy data; snapshot definitions must follow pinned revisions.
- Export sources include loaded rows, lazy requests and full-scorecard queries. Preserve correct pinned joins and existing list-aware predicates while fixing callers/validation.

The source now contains CONVI-7981's nonempty Create/Commit validation, which is not duplicated. Legacy empty/NULL reads and explicit all-usecases still need a documented contract.

## Live observation and limits

Visited https://walter-dev.voice-staging.cresta.ai/director/insights/performance/conversations in existing Chrome authentication. Chat selected, All scorecards/Template All, Last 30 days/Weekly, criteria adherence; charts populated. Reloaded once. Resource inventory showed unscoped/scoped current-template calls, QA/conversation stats endpoints and scoped/unscoped ancillary catalogs. Live assets identified main-5bf21c0; no equivalence with the reviewed source refs is asserted.

Supported resource inventory exposed URLs only. POST bodies, status/results, request multiplicity and ClickHouse query IDs were unavailable. Native DevTools filtering could not be controlled reliably and later returned no available window. Therefore actual live POST propagation, shared-template results and authorization behavior remain unverified. No HAR or customer responses were exported; no DB queried.

Credentials used: existing browser authentication and self-authenticating git fetch. No AWS/Okta/Azure/SSH credential or profile was explicitly selected or read. No customer-data writes, product edits, deployment, commit or push.

## Local ticket drafts and reverted Linear creation

Duplicate checks covered the project (only CONVI-7791 plan and CONVI-7928 kickoff existed), team searches for cross/usecase/defensive/QM Report and related completed export work. Multiword search returned unreliable empty results, so it was not the sole dedupe evidence.

Nine Backlog children (CONVI-7984–7992) were initially created with verified dependencies and source links. The user then requested reverting creation and keeping the tickets only locally. Saved all nine complete descriptions, acceptance criteria, immutable source links and dependency relationships in the [local draft list](../../deliverables/cross-usecase-pi-qm-ticket-drafts.md), using draft keys instead of Linear IDs.

Deleted exactly those nine newly created issues through Linear's UI. The Linear MCP provides create/update/read actions but no issue-delete action, so browser use was limited to deletion. Each deletion showed the issue ID and a Deleted confirmation. Linear's confirmation states they remain recoverable in Recently deleted for 30 days. Existing CONVI-7928/7791/7981/7795/6392/7941 work was not deleted or edited.

Removed the nine newly generated per-ticket work-item files and their entries from project metadata. The inventory, README, continuing plan and task design context now link to local proposals. No implementation issues from this investigation remain active.

## Open decisions and next steps

The pending user question asks whether a partly unauthorized explicit set should fail or return a displayed authorized intersection. No answer was assumed. The local DEF draft records that unresolved choice and the need to identify the existing server authority for use-case entitlement; frontend configuration and user/group/role checks alone do not prove it. No vulnerability was demonstrated by this review.

Implement prerequisite contracts/template reads, then task/PI/report sources and exports. Reuse the A/B/r1/r2 fixture, preserve scoped-criterion reader compatibility, and measure representative query plans/timing. The October EXPLAIN estimate is not a latency measurement. Product tests and deployed validation belong to later implementation work.

## Closure verification

The initial inventory/source-line and ticket-dependency checks passed. After reversing creation, local draft count, links/anchors, project YAML and tracked/untracked whitespace are checked again. No product tests run because this task changes knowledge documents and removes only the newly created Linear issues. No commit or push.
