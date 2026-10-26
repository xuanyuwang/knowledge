# Oversized coaching gRPC response investigation

## Context

- Date: 2026-09-09 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: current `go-servers` checkout, read-only investigation
- Reported symptoms:
  - `CrestaAPI.coaching.listCurrentScorecardTemplates` is slow and ends in a gRPC maximum-send-message-size error.
  - `ExportScorecards` fails while receiving a page from `ListComments` because that page exceeds the client receive limit.

## Investigation scope

- Trace the endpoint implementation and response construction.
- Review production observability for occurrence, affected surface, and timing.
- Distinguish database/query latency from response-size failure.
- Identify bounded mitigation and durable fix options; no production or product-code mutation is authorized.

## Relationship between the incidents

These are the same failure class but not the same concrete incident:

| Dimension | Current-template permissions | Scorecard export comments |
| --- | --- | --- |
| Customer/profile | `rcg / us-east-1` | `cresta-sandbox-2 / voice-sandbox-2` |
| Path | Director permission hook -> `ListCurrentScorecardTemplates` | `ExportScorecards` -> collaboration `ListComments` |
| Oversized message | Full template-list response | One paginated comments response |
| Transport boundary | Server send limit | Apiserver client receive limit |
| Observed size/limit | About 79.86 MB vs 50 MiB | 6,303,757 bytes vs 4 MiB |
| Immediate design gap | Unpaginated full-body list for a permissions-only consumer | Fixed 250-record page size does not bound variable-sized comment content |

The shared root cause is a record-count or unbounded-list contract that does not constrain serialized response bytes. Both paths do expensive work and then fail only at the gRPC transport boundary. They belong in the same investigation as two subcases, while their fixes and rollout ownership remain separate.

## Code findings

- `ListCurrentScorecardTemplates` has three views: the default full response, full with recent scorecard stats, and `VIEW_METADATA_ONLY`.
- The full paths load complete template rows and serialize template structure, QA configuration, permissions, resolved audiences, trigger display names, creator data, and applicable-agent usernames.
- The metadata-only path intentionally omits the large JSONB-backed fields and exists specifically to avoid oversized responses for lightweight consumers.
- The response remains a single unpaginated repeated message, so aggregate payload size grows with both template count and per-template body/config size.

## Incident A: current-template permission loading

### Production database findings

The current RCG `us-east-1` production dataset was inspected through the read-only customer app-DB workflow. Only aggregate sizes and non-secret template metadata were queried; template bodies and credentials were not returned.

- The permission-hook request shape includes active and inactive templates, excludes archived templates, and does not provide a usecase filter by default.
- That shape currently selects 3,330 rows, all with distinct template resource IDs, across 294 distinct titles and 98 distinct usecase sets.
- Stored template/config/permission JSON totals 71,977,880 bytes (about 69 MiB) before protobuf representation overhead.
- Template bodies account for 67,783,614 bytes; permissions account for 4,102,513 bytes. QA task and score configs are comparatively small.
- The largest single row is only 37,874 bytes, so this is aggregate fan-out rather than one corrupt or abnormally large row.
- The served set contains 3,103 active conversation templates, 171 active process templates, 49 inactive conversation templates, and 7 inactive process templates.

### Caller and history

- `useGetScorecardTemplatePermissions` calls `useCurrentScorecardTemplates` without a `view`, which maps to `VIEW_BASIC_WITHOUT_STATS` and fully materializes every selected template.
- The hook only reads template name and permissions to build a synchronous checker; it does not consume template bodies, QA configs, resolved triggers, creators, or audience mappings.
- Its default `usecaseNames` is an explicit empty list, so the shared hook does not fall back to the currently selected usecases.
- The July 2026 backend change adding `VIEW_METADATA_ONLY` documented this exact large-tenant failure mode and cited RCG at roughly 58 MiB at that time.
- Existing Director migrations use metadata-only listing plus targeted `GetScorecardTemplate` fetches for body-dependent paths. The permission hook was not migrated because metadata-only deliberately omits permissions.

### Root-cause assessment

The API spends substantial time querying, resolving, parsing, and serializing 3,330 complete templates, constructs a response larger than the 50 MiB gRPC send ceiling, and fails at the send boundary. HTTP 429 is the gateway mapping of gRPC `RESOURCE_EXHAUSTED`; it is not evidence of request-rate throttling.

## Incident B: scorecard export comment loading

Source thread: [Director Backend error alert](https://crestalabs.slack.com/archives/C02DHEB9V9P/p1788969019284059).

### Confirmed occurrence

- Groundcover identifies the failing request as `customers/cresta-sandbox-2/profiles/voice-sandbox-2/scorecards:export` in `voice-prod`.
- Four request traces failed between 15:45:31 and 15:46:19 UTC on 2026-09-09.
- `ExportScorecards` called collaboration `ListComments`; the client rejected a 6,303,757-byte response against the default 4,194,304-byte receive ceiling.
- This is therefore not the RCG template request and not a server-send failure. It is an internal client-receive failure on a separately paginated dependency.

### Code and history

- `getLinkedCommentsForExport` collects all conversation IDs in the export, then walks `ListComments` pages with a fixed page size of 250.
- PR #30472 (`a81bcb24ad`, 2026-07-27) reduced the comments page size from 1,000 to 250 specifically to stay under the default gRPC limit.
- The renewed failure proves that record-count pagination is not a sufficient byte bound: 250 comments can still serialize above 4 MiB when comment bodies are large.
- The current client call has neither a larger explicit receive limit nor retry logic that shrinks the page after `RESOURCE_EXHAUSTED`.

### Root-cause assessment

The export path is logically paginated but operationally byte-unbounded. A page-size constant assumes a typical comment size; sufficiently large comment content breaks that assumption and fails the whole export before CSV construction completes.

## Permission-evaluation follow-up

`EvaluateScorecardsPermissions` cannot replace Director's `useGetScorecardTemplatePermissions` as currently implemented.

- The RPC requires persisted scorecard resource names and returns decisions keyed by scorecard. Many current consumers evaluate template permissions before a scorecard exists.
- Although its enum declares view, edit, grade, appeal, publish, and submitted-lock modification, the backend accepts only submitted-lock modification and `VIEW`; `VIEW` has no grant branch and currently always denies. The other four operations return `InvalidArgument`.
- Director already uses the API for its implemented purpose: deciding whether a persisted submitted scorecard can be modified.
- The local template hook supplies five decisions: edit template, view scorecard, grade, appeal, and publish. These drive list/filter visibility, template editing, scorecard form mutability/reset, appeals, publishing, and policy/AI-agent attachment flows.
- The decisions are necessarily template-specific when overrides exist because each template stores its own editors, viewers, graders, appealers, publishers, and submitted-editor allowlists. Submitted editing additionally depends on scorecard state/type and user/team/group membership.

The preferred long-term contract is a batched template-permission evaluation API that accepts template names plus required operation context and uses shared server-side helpers. List APIs should apply view permission before pagination so post-page filtering does not create sparse or incorrect pages. A paginated `PERMISSIONS_ONLY` view is a smaller change but retains duplicated client/server semantics; that duplication already disagrees for an empty publisher-role list (Director allows while the backend denies explicit publishing).

## Recommendations

### Current templates

1. Add a semantically explicit permissions-only list view that projects only identity/status/type/permissions fields, and migrate `useGetScorecardTemplatePermissions` to it. The stored permissions portion is roughly 4 MiB for the same RCG row set, leaving ample headroom.
2. Add pagination or another bounded contract for any remaining full-body list consumers. Pagination alone is not the preferred permission-hook fix because the client does not need the bodies at all.
3. Do not treat the 3,330 rows as database duplicates without product/configuration review: the resource IDs are distinct even though many titles are repeated across usecases.

### Scorecard export

1. Make comment pagination resilient to variable payload size: on `RESOURCE_EXHAUSTED`, retry the same page with a smaller page size until it fits, with a lower bound and clear failure if one comment alone is too large.
2. An explicit, bounded receive-limit increase can provide headroom, but it should accompany adaptive paging or a byte-aware service contract rather than replace it.
3. Add a test whose 250-comment response exceeds 4 MiB; the current mock tests validate requested page counts but not serialized response size or retry behavior.

### Shared guardrails

1. Instrument selected-row count, serialized response size, and dependency page size around these paths.
2. Alert before transport ceilings are reached and distinguish server-send from client-receive exhaustion.
3. Treat limit increases alone as mitigations: they preserve unnecessary work and move rather than remove the failure threshold.

## Observability handling

Groundcover was used read-only to confirm the active production failure and its latency profile. Raw observability records are intentionally not copied into this workspace artifact under the connector data-safety policy.
