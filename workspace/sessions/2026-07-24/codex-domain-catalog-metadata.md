# Domain Catalog Metadata Update

## Objective

Make the repository's four product domains, their scopes, and current subdomains discoverable from the main metadata surfaces, and record that legacy migration is gradual rather than a bulk cleanup.

## Source Context

- Primary project: `workspace`
- Source repo: `/Users/xuanyu.wang/repos/knowledge`
- Branch/worktree: `main` in the primary checkout
- Date/time zone: 2026-07-24, America/Toronto

## Outcome

- Added a linked domain and subdomain catalog to the root `README.md`.
- Expanded the canonical domain model with all eight Analytics and seven Scorecard Workflows subdomains.
- Explicitly recorded that Scorecard Data Sync and Notifications currently remain cohesive domains without subdomains.
- Added a compact catalog to `workspace/README.md` and discovery guidance to the Codex/Claude adapters.
- Clarified that legacy projects migrate gradually when reopened or needed, using synthesis and pointers before separately reviewed deletion.

## Impact

Humans and AI tools can choose the correct durable home without reconstructing the taxonomy from folder names. The gradual-migration rule prevents both permanent ticket-folder growth and risky bulk deletion.

## Safety

- Preserved existing uncommitted work in product domains and `train-for-staff`.
- Updated `workspace/repos.yaml` with `go-servers-convi-7206` and `go-servers-convi-7350` worktree entries from the CONVI-6862 follow-up.
- Did not stage, commit, push, or modify source repositories.

## Follow-up

- Apply the catalog to new work items immediately.
- Migrate legacy folders opportunistically and review removal only after their durable knowledge and evidence have canonical homes.
