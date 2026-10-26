# Consolidate five inactive Analytics ticket projects

- Date: 2026-10-25 (America/Toronto).
- Primary domain: analytics; work spans Active Days, Insights User Filter, Leaderboard, and QA Score.
- Source repo: /Users/xuanyu.wang/repos/knowledge.
- Branch/worktree context: main, existing main checkout. Product repo paths are resolved through workspace/repos.yaml; no product checkout is edited.
- Request: select five older ticket-named projects and consolidate them into the existing domain structure.
- User approved removing all five top-level folders after verifying links.

## Selection

| Ticket | Last substantive Git update | Last navigation-only update | Canonical subdomain |
|---|---|---|---|
| CONVI-6192 | 2026-02-06 | 2026-07-15 | Active Days |
| CONVI-6242 | 2026-02-19 | 2026-07-15 | Active Days |
| CONVI-6260 | 2026-02-17 | 2026-07-15 | Insights User Filter |
| CONVI-6494 | 2026-05-07 | 2026-07-15 | Leaderboard |
| CONVI-6753 | 2026-05-14 | 2026-07-15 | QA Score |

These are small, inactive projects with an established domain home, rather than a claim that they are the five oldest folders in the repository. CONVI-6247 is older than some selected records but overlaps continuing user-filter consolidation work.

## Plan and completion checks

- [x] Add missing legacy metadata from templates/project.yaml before moving the source records.
- [x] Preserve all 21 original files and file modes under this dated session, with original paths/hashes in an evidence manifest. Preserve the existing CONVI-6494 project.yaml and four migration-added metadata files.
- [x] Create one canonical work item per ticket and synthesize durable lessons into existing subdomains.
- [x] Update domain navigation, metadata, the legacy source index, and inbound references.
- [x] Verify preservation, relative Markdown links, deleted-path references, whitespace, YAML, and secret scans before removing the empty legacy directories.
- [x] Record concise daily movement. Do not commit or push unless requested for this migration.

## Evidence interpretation

The records are historical. No live Linear, PR, deployment, or customer-database state is inferred. CONVI-6242's completed February backfill supersedes the stale In Progress README for that operation, but does not establish the ticket's current status. CONVI-6260's investigation contains contradictory flag-activation sentences; its later summary and logs identify the new path as enabled at the time, while current rollout remains unverified. CONVI-6494 was partial implementation in May, and CONVI-6753's proposed/implemented fix was still pending publication in its last record.

## Preservation and verification

All 21 original files were preserved, along with four migration-added metadata files. Sixteen original files are unchanged byte-for-byte; five original README files have only their relative links rewritten. Executable permissions were retained. Each rewritten file was checked against Git HEAD after reversing only the recorded link substitutions.

The five old directories were removed only after preservation, local-link, YAML, and secret checks passed. Inbound paths elsewhere in the repository now lead to the canonical work items. All work remains unstaged and uncommitted.

Evidence: [path and hash manifest](five-legacy-project-consolidation/manifest.json), [verification results](five-legacy-project-consolidation/verification.json), and [daily log](../../log/2026-10-25.md).

Credentials used: none. Historical scripts are preserved without execution.
