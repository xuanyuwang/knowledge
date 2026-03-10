# Known Edge Case: GROUP Branch FinalUsers Mismatch

**Created:** 2026-03-10
**Status:** Noted, needs verification — may be intended behavior

## Summary

When both user filter AND group filter are set, `FinalUsers` (used for the ClickHouse WHERE clause) can be wider than `UserNameToGroupNamesMap` (used for bucketing results into groups). This means top-level totals may include data from users that don't appear in any group bucket.

## Affected Files

- `retrieve_conversation_stats.go`
- `retrieve_suggestion_stats.go`
- `retrieve_summarization_stats.go`

(Any file that has a GROUP branch assigning `users = result.FinalUsers`)

## How It Happens

1. `ParseUserFilterForAnalytics` resolves filters → `FinalUsers` = explicit user filter ∪ group-expanded users
2. In the GROUP branch, `UserNameToGroupNamesMap` only contains users that belong to selected groups
3. `users = result.FinalUsers` is used for the ClickHouse query (wider set)
4. Response only returns group buckets for users in `UserNameToGroupNamesMap` (narrower set)

## Example

- User filter: `[alice, bob]`
- Group filter: `[team-A]` (contains only `alice`)
- `FinalUsers` = `[alice, bob]` (union)
- `UserNameToGroupNamesMap` = `{alice: [team-A]}`
- ClickHouse query filters by both alice and bob
- Only alice appears in the team-A bucket
- **Top-level totals include bob's data, which isn't in any bucket**

## Questions to Verify

1. Is the union of user filter + group expansion the intended semantic? (See `user-filter-consolidation/user-filter-behavioral-standard.md`)
2. Does the frontend ever send both user filter and group filter simultaneously?
3. If it's a real bug, should the GROUP branch use `UserNameToGroupNamesMap` keys instead of `FinalUsers`?

## Source

Flagged by CodeRabbit on [PR #26178](https://github.com/cresta/go-servers/pull/26178). Pre-existing behavior, not introduced by ext table changes.
