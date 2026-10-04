# Timestamp contract explanation

Date: 2026-09-21
Primary domain: scorecard-data-sync
Primary source repo: /Users/xuanyu.wang/repos/go-servers
Source context: read-only pinned commit ab941601e66af653b19a7cf91777b793bdedb2c7, the verified backend main snapshot from the September 19 investigation. No source checkout or deployment change.

## Request

Write a standalone detailed explanation, beginning with the conflicting source-time and indexing-time assumptions, then concrete examples explaining why assigning PG updated_at to CH update_time breaks reindex cleanup.

## Scope

Synthesize the existing investigation and user discussion. Verify source references and clarify that cleanup is queued before insertion in the batch path but executed independently by the offline job. Preserve the distinction between source-derived failure mechanism and unverified historical production attribution.

## Result

Created [the standalone explainer](../../deliverables/why-pg-updated-at-breaks-ch-cleanup.md). It begins with the source-version versus projection-freshness contract, defines all timestamps, then uses four examples: ordinary propagation hiding the distinction; delayed old-state projection motivating source versions; historical unchanged data reindexed and deleted by the cutoff; and old/new timestamp meanings defeating a repair during migration.

Verified at the pinned backend commit that `indexClickhouseBatch` invokes cleanup enqueue at lines 394–400, while writes occur later; the offline job executes independently. Preserved this distinction instead of implying synchronous delete-after-reindex orchestration.

Linked the explainer from the domain README, project artifact catalog, original investigation README, broader option comparison and existing canonical work item. Added the daily log without creating a duplicate work item. Used the repository daily-capture workflow for closure.

## Validation and limits

- Reused the recorded September 19 ClickHouse local reproduction with explicit attribution; did not rerun tests for a documentation-only change.
- Pinned-source references and the mathematical example `S < C <= I` support the failure mechanism. Historical incident run, deployment flags and tenant IDs remain unverified.
- No external access, credentials, product code changes, commits or pushes. Unrelated existing knowledge edits preserved.

## State

Requested document complete. Scoped `git diff --check` passed; four documents passed whitespace checks and all 11 checked relative links resolved. The explainer contains 2,266 words.

## Visual explanation follow-up

The user proposed Mermaid or HTML to improve comprehension while keeping the artifact easy to view. Kept Markdown as the canonical format and added three diagrams: the shared column's conflicting consumers, the asynchronous enqueue/reindex/cleanup sequence, and the preserve-versus-delete comparison for the same valid row. Retained explanatory prose and tables so the argument remains readable without Mermaid support. Documented that Mermaid needs a compatible renderer; HTML is not required for these static explanations.

The earlier word count describes the initial prose version. Diagram syntax/format validation for this follow-up is recorded below. No product source, production system or credential access.

Validation: all three diagram blocks passed Mermaid 11.17.2 parsing (two flowcharts and one sequence diagram). Removed semicolons from sequence message labels after the parser identified them as statement separators. Four edited Markdown files passed whitespace/local-link checks, and scoped `git diff --check` passed. Browser layout was not separately rendered. Validation dependencies were installed with lifecycle scripts disabled into a temporary directory, then removed; no repo dependencies changed.
