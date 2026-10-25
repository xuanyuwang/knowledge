# CONVI-7785 construction history review

Date: 2026-10-04 (America/Toronto)
Source repo: `/Users/xuanyu.wang/repos/director`
Branch/worktree: `xw/convi-7785-exclude-simulator-rules` in `/Users/xuanyu.wang/repos/director-convi-7785`

## Scope and evidence

- Linear CONVI-7785 asks whether Performance Configuration conversation template creation offers Training Simulator-only Opera rules and, if so, to exclude them.
- `gh pr list --repo cresta/director --head xw/convi-7785-exclude-simulator-rules --state all` returned no PR on 2026-10-04. `HEAD` is the base commit `f47845f4f7`, with all four ticket files uncommitted; `git diff main...HEAD` is empty. This is a construction history of the working change, not a commit-by-commit PR history.
- Prior merged commit `2c5ba94916` (CONVI-7281, #23164) added `SOURCE_TYPE_TRAINING_SIMULATOR` to the active/inactive Opera policy preset. The existing template picker still iterated the full behavior list without a source check.
- The behavior model lacks source type; the policy model has `sourceType` and `behaviors`. `getTriggerOptions` already walked policies to associate behaviors with roles and policy names.

## Reconstructed decision chain

1. Reuse the existing policy-to-behavior association in `getTriggerOptions` to build a set of behavior names owned by Simulator-only policies; skip those names before constructing conversation template choices. This is directly evidenced by the working diff. The judgment that this is the narrowest FE seam is an interpretation of the structure and ticket scope.
2. Cover the archived/admin path: the shared behavior query includes archived behaviors, but the prior policy query included only active/inactive policies. The template builder opts into an archived-inclusive policy filter. For this opt-in path, `useAutoQATriggers` waits for both policy and behavior results before exposing data, so behavior options cannot appear before source ownership is known. Other consumers retain their default filter/loading behavior.
3. Add a focused `getTriggerOptions` test with normal, Simulator-only, archived Simulator-only, and unowned legacy behaviors. It proves exclusion in the pure picker transform; it does not independently exercise the hook query or deployed UI.

## Boundary and open checks

- The separate compatibility path for an already-selected legacy policy trigger remains in `getTriggerOptions`; the new filter targets behavior choices offered during template authoring.
- The prior session recorded passing focused Vitest, Director app TypeScript, targeted Biome/ESLint, and diff checks. No preview/staging verification, commit, push, or PR exists.
- The exact motivation for every intermediate edit cannot be recovered from Git because this branch has no ticket commits. The current diff and prior session record support the conceptual sequence above.

Later on 2026-10-04, the user requested a PR. The work was committed as `d9de9e1902` and opened as draft [director#23461](https://github.com/cresta/director/pull/23461). The no-PR observation above describes the state at the time of this review.
