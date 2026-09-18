# CONVI-7706 `RetrieveConversationStats` value-or-missing fix

Date: 2026-09-17  
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`  
Branch/worktree context: `/Users/xuanyu.wang/repos/go-servers-convi-7706`, `xw/convi-7706-conversation-stats-value-or-missing`, rebased onto `origin/main` at `6e2648ebed`

## Scope

Implement Elasticsearch-compatible `selected metadata value OR annotation missing` semantics only for `RetrieveConversationStats`. Leave the shared-parser rejection active for all other unsupported ClickHouse endpoints.

## Implementation

- Generalized the existing QA value-or-missing parser so the caller chooses raw `moment_annotation_d` or the metadata materialized view; QA behavior is unchanged.
- Added conversation-stats preprocessing that extracts only the supported same-template metadata overlap group and passes every unrelated filter to `parseClickhouseFilter` unchanged.
- Added selected-value and existence CTEs using left joins and a final OR predicate.
- Preserved latest-value selection by default and any-value selection when `allowMatchingStaleMetadataValues=true`.
- Preserved missing-only, value-only, other moment groups, and voicemail exclusion behavior.

## Validation

- Passed:
  - `TestRetrieveConversationStatsValueOrMissingMetadataFilter`
  - full `TestRetrieveConversationStats` suite
  - `TestParseClickhouseFilter_RejectsValueOrMissingMetadataFilter`
  - all `TestParseMomentConditionsForQAAttribute*` cases
  - `TestRetrieveQAScoreStatsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue`
- `bazel run //:gazelle` passed and produced no generated changes.
- `git diff --check` passed.
- ClickHouse local semantic fixtures:
  - latest-value mode returned `['missing','selected']` and excluded both a different current value and a stale-only match;
  - stale-enabled mode returned `['missing','selected','stale']`.

## State

- Product fix is committed as `e7704f6637` (`[CONVI-7706] Support value-or-missing in conversation stats`) after a clean rebase onto current `origin/main`.
- Opened ready-for-review [go-servers PR #32440](https://github.com/cresta/go-servers/pull/32440). CI is pending; Linear attached the PR and moved CONVI-7706 to In Progress.
- Repeated the focused Go test selection after the rebase; it passed. Gazelle again completed without generated changes, and `git diff --check` passed.
- No production change was made. No AWS, Okta, Azure, or SSH credentials were read or used. GitHub authentication used the existing keyring token over HTTPS; dependency downloads used the Go module proxy.
