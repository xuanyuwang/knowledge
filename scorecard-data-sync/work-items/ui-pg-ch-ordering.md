# UI to PostgreSQL to ClickHouse scorecard ordering

**Status:** investigation complete; explicit-version redesign preferred; implementation details pending
**Primary domain:** scorecard-data-sync
**Primary subdomain:** none
**Last updated:** 2026-11-04

## Objective

Explain why the original PG timestamp proposal was unsafe and compare low-cost mitigations through complete eventual-convergence designs, covering FE, source mutations, projection, delivery, deletion and reads.

## Current conclusion

**November 4 update:** the [reviewed race/sync reference](../deliverables/scorecard-races-corrected-recommendations.md) records Xuanyu's preferred shared-mutation/explicit-version redesign. Do not use PG `updated_at` as the scorecard version. Field masks and correctly protected aggregate recomputation remain useful near-term fixes; primary reads need a capacity assessment. The earlier outbox recommendation remains separate technical analysis, since the Google Doc's sync-redesign section is empty.

- `ReindexConversations` plus offline cleanup interprets CH `update_time` as indexing time. Assigning PG `updated_at` makes freshly rebuilt unchanged rows eligible for the `update_time < cutoff` deletion. Verified in the original #23999 commit, refreshed current source, and isolated ClickHouse execution. Exact historical production incident attribution remains unverified.
- The main frontend form already serializes queued mutations. The concrete gap is pending debounce and successful-save dependency, plus callers outside the shared scope. This corrects earlier debounce-only descriptions.
- Recommend an immediate no-schema/no-broad-backfill increment: save-success barrier, narrow lifecycle field writes, transactionally correct final aggregate calculation, and coherent projection reads.
- For full eventual convergence within a migrated cohort, prefer server aggregate revisions, explicit versioned deletes/stable CH identity, and a PG transactional outbox. Existing tooling suffices; CDC and Temporal command ownership remain alternatives.
- PR #32447 was still open/unmerged at inspection. This work did not implement or deploy any candidate.

## Source context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`, main checkout read-only; analysis pinned to verified `origin/main` at `ab941601e66af653b19a7cf91777b793bdedb2c7`.
- Related: `/Users/xuanyu.wang/repos/director` main fetched at `74c41e4585efd3440d32e74f416189cfe700602e`; `/Users/xuanyu.wang/repos/clickhouse-schema` main fetched at `36b195838e9329751332af6488c0c3d4574b6ab3`.
- No source branch checkout, code modification, commit, push, ticket edit or production operation.

## Validation

Nine actual clickhouse-local checks and three installed TanStack Query schedule checks passed. They validate isolated engine/library semantics; they do not establish production deployment, distributed behavior or a full browser reproduction. Detailed constraints and rollout proof obligations are in the deliverable.

## Next actions

1. Turn the corrected field-mask, aggregate-consistency and queue recommendations into concrete implementation work.
2. Define the preferred explicit integer-version contract across all aggregate writers and caller conflict behavior; retain timestamp-token analysis as a historical alternative.
3. Design revision-aware projection, migration, cleanup and deletes; evaluate the earlier outbox proposal separately for recoverable delivery. Define baseline, stable routing and visibility explicitly.
4. Obtain the historical postmortem/workflow evidence if exact November incident attribution is required.

## Evidence

- [Timestamp-contract explanation with concrete examples](../deliverables/why-pg-updated-at-breaks-ch-cleanup.md)
- [Design comparison](../deliverables/ui-pg-ch-ordering-options.md)
- [Session](../sessions/2026-09-19/codex-ui-pg-ch-ordering-options.md)
- [Daily log](../log/2026-09-19.md)

## Timeline

- 2026-09-19 — Recovered cleanup incompatibility, corrected frontend serialization assumption, compared seven solution families, and validated key counterexamples locally.
- 2026-09-21 — Added a standalone explanation that starts with the conflicting contracts, then demonstrates out-of-order replacement, valid-row cleanup deletion, and mixed-version migration. Clarified enqueue-before-write versus independently scheduled cleanup. Documentation only; historical production attribution remains unverified.
- 2026-09-21 — Added three Mermaid diagrams to the standalone explainer while retaining plain-Markdown explanations for viewers without diagram support.

- 2026-11-04 — Integrated the reviewed Google Doc and eight indexed comments. Explicit versions are the preferred redesign; timestamp-token and outbox options are no longer presented as equally undecided or jointly endorsed.
