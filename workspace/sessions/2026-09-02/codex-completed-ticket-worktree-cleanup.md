# Completed-ticket worktree cleanup

## Context

- Source registry: `/Users/xuanyu.wang/repos/knowledge/workspace/repos.yaml`
- Source repositories: `go-servers`, `director`, `cresta-proto`, `config`, and `flux-deployments`
- Linear was treated as authoritative for ticket status.

## Actions

- Confirmed completed status for CONVI-7049, CONVI-7162, CONVI-7254, CONVI-7350, CONVI-7379, CONVI-7383, CONVI-7386, CONVI-7387, CONVI-7543, CONVI-7582, and CONVI-7601.
- Removed 17 clean worktrees associated with those tickets:
  - 9 from `go-servers`
  - 2 from `director`
  - 1 from `cresta-proto`
  - 5 from `config`
- Pruned stale Git worktree registrations in the affected repositories.
- Removed deleted named worktrees from `workspace/repos.yaml`.

## Preserved work

- Kept `/Users/xuanyu.wang/repos/cresta-proto-convi-7162` because it has modified generated Go files.
- Kept ticket worktrees whose Linear issues are not complete: CONVI-7384, CONVI-7402, CONVI-7583, and CONVI-7598.
- Kept worktrees without an unambiguous Linear ticket association.

## Validation

- Re-listed worktrees after cleanup.
- Verified that the only remaining CONVI-7162 worktree is the dirty `cresta-proto` worktree.
