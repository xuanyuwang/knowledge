# UI to PostgreSQL to ClickHouse ordering investigation

Date: 2026-09-19 (America/Toronto)
Primary domain: scorecard-data-sync
Primary source repo: /Users/xuanyu.wang/repos/go-servers
Branch/worktree context: /Users/xuanyu.wang/repos/go-servers main checkout read-only; relevant source verified against origin/main ab941601e66af653b19a7cf91777b793bdedb2c7.
Related sources: director, clickhouse-schema, cresta-proto.

## Request

Recover why the original PostgreSQL timestamp version proposal was unsafe, especially Temporal stale-row cleanup; compare lightweight through comprehensive end-to-end ordering solutions on cost, coverage, tools, and complexity. Session budget requested: $50; no dollar-usage meter is exposed, so use bounded source investigation and checkpoint durable artifacts.

## Initial evidence

- Legacy pg-ch-scorecard-sync-investigation is migrated; canonical home is this domain.
- Historical CONVI-5565 investigation identifies PR #23999 and revert #24095, but its explanation that an earlier source timestamp overwrites a later source timestamp is inconsistent with normal ReplacingMergeTree max-version behavior. Verify original review/revert and actual cleanup source before relying on that explanation.
- Existing main checkouts are behind current work (backend September 16, frontend August 26); refresh source refs for design claims.

## Findings and completed artifact

See [the canonical comparison](../../deliverables/ui-pg-ch-ordering-options.md) for the full argument, seven candidate families, matrix, source links, migration requirements and acceptance schedules.

1. Recovered the exact ReindexConversations → BatchIndexConversations → delete queue → offline cutoff mutation chain. The contract existed in original timestamp PR commit d04ef2d3faeaadc0bb36760c1a75017675169b37, not just current code. Using PG updated_at for update_time makes rebuilt unchanged data eligible for deletion.
2. Verified original PR #23999 and revert #24095 metadata/diffs via GitHub; reviewed CONVI-5565 and its returned comments via Linear MCP. No original incident workflow payload/postmortem was recovered; do not claim exact production attribution.
3. Corrected the FE premise: saveScorecard already has a shared mutation scope. Pending debounce and save-success dependency remain concrete gaps. Standalone update/submit hooks do not share that scope. The wrapper returned for submit implements PromiseLike, so this is not a missing-await claim about its successful response.
4. Confirmed backend preparation happens before the write transaction, omitted criterion values can be retained, and parent/child projection reads are separate. Ordering, atomic aggregate consistency, durable delivery, cleanup and visibility need separate contracts.
5. PR #32447 was open/unmerged at inspection (head 741632169913060d46504444d6aa85841026b9d5).
6. Recommended no-schema/no-broad-backfill mitigation first, then aggregate revisions + PG outbox + versioned/stably keyed CH projection for complete eventual convergence in a migrated scope. Temporal commands and CDC are alternatives, not prerequisites.

## Source freshness

- Backend checked-out main: b2bc3849854d124ed659058f8a8de6813262ddc6 (September 16). GitHub main and local origin/main both ab941601e66af653b19a7cf91777b793bdedb2c7. Relevant files diffed/read at that ref; concurrency functions unchanged (an unrelated scoring helper changed).
- Director checked-out main: 0a9afcb32cbd9eaf7a49a2c7e064e0291b4fe637 (August 26). Fetched/inspected main 74c41e4585efd3440d32e74f416189cfe700602e, matching GitHub. Compared form, save hook, standalone hooks and wrapper; current save scope remains present.
- Schema checked-out commit: 69d6a05d05023fea18db9c70bee50352c12d2507. Fetched/inspected main 36b195838e9329751332af6488c0c3d4574b6ab3, matching GitHub.
- No checkout was switched. Main source does not prove deployment.

## Validation

- Ran `mutation-scope-check.mjs` with Node v24.11.1 against the installed TanStack Query implementation: three deterministic schedules passed. It validates queue semantics, not a browser-level UI reproduction.
- Ran `ordering-counterexamples.sql` in an isolated temporary clickhouse-local data directory, ClickHouse 26.8.2.7. Nine outputs were 1 and process exit was 0, including actual DELETE mutation and OPTIMIZE FINAL. Temporary database directory was removed. Synthetic clocks/data only; no PG or remote CH connection.
- Saved ClickHouse output in `ordering-check-results.txt`; TanStack results appended there.
- No production service tests, distributed topology tests, PG concurrency implementation tests or browser E2E tests were run. They are future acceptance work, not implied completed validation.
- Final scoped Markdown/YAML whitespace and relative-link checks recorded below.

## Access and credential accounting

- Used existing GitHub CLI authentication for read-only PR/commit metadata and the Linear connector for read-only issue/comments. No messages or ticket updates were sent.
- Two explicitly HTTPS Git fetch commands followed a configured global HTTPS-to-SSH URL rewrite. The GitHub host selects `~/.ssh/id_ed25519`; its host block, included client configuration and public-key comment were inspected after the fetch when the rewrite was discovered. This was later than the required pre-use clearance, a procedural error. The specific key accepted by the SSH server was not traced. No further SSH operations were run.
- No private-key content was read, and no AWS/Okta/Azure or production database credentials were used. No restricted key was selected or inspected.

## Knowledge closure

- Added comparison, canonical work item, daily log, source/session evidence and local reproduction files under scorecard-data-sync.
- Updated domain README/project metadata and the original pg-ch-scorecard-sync-investigation README pointer.
- Corrected the misleading historical timestamp explanation in the CONVI-5565 README and the queue claim in the RCG work item. Historical session records and memory files were not rewritten.
- Preserved unrelated pre-existing knowledge changes. No source code, commit, push, production write or external communication.

## State

Requested investigation and option comparison complete. Design selection and implementation remain future work. No actual dollar usage meter was available; no claim about exact session spend.

Final verification: scoped `git diff --check` passed; seven new artifacts passed whitespace checks and 15 relative links resolved.
