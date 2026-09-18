# CONVI-7674 root-cause and PR #22772 review

- Date: 2026-09-14
- Source repo: `/Users/xuanyu.wang/repos/director` (`main`, read-only review)
- Related backend repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only review)
- Ticket: [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after)
- Candidate fix: [Director PR #22772](https://github.com/cresta/director/pull/22772), head `e392f523f3857600e38f0df8f08d3c239300163f`
- Evidence: `/Users/xuanyu.wang/Downloads/rcg.cresta.com.har`

## Conclusion

PR #22772 is the correct minimal frontend fix for the root cause demonstrated by the RCG HAR. It clears the conversation-only `dateRangeTarget` when the selected scorecard template is process type, and applies the same normalization to the count chart's internally reconstructed primary, unfiltered, and delta request states. Conversation-template behavior is preserved.

The RCG attribution is stronger than the Oportun evidence currently described in the PR body: RCG's HAR is from a failing session and contains 20 requests that all reproduce the incompatible request field and zero-result response. The PR is still draft and requires deployed RCG validation. A backend guard against the invalid process-template plus conversation-ended combination would be useful defense in depth, but is not required to correct Director's request construction.

## HAR facts independently rechecked

Filtering the HAR to `qaScoreStats:retrieve` yields 20 POST requests:

- all returned HTTP 200;
- all selected template `019dbd42-2ee4-7684-84f1-faf442fdc2c4` and use case `cel-customer-xperience`;
- all sent empty user, group, and scorecard-status arrays;
- all sent `conversationTimeRangeField: TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`;
- the only windows were July and August 2026;
- all responses had `totalScorecardCount: 0` and `totalConversationCount: 0`.

The HAR establishes the failing request and response, but the full root cause also needs two facts outside the HAR: ClickHouse contains 51 August scorecards for this template/window, and those process scorecards have empty conversation IDs. The backend source then closes the causal chain: `CONVERSATION_ENDED_AT` sets `needsConversationEndTime`, which adds `JOIN conversation ON scorecard.conversation_id = conversation.conversation_id`; the empty IDs are eliminated.

## Fix assessment

- Correct layer for the immediate regression: Director introduced and forwards the incompatible default.
- Correct predicate: clear the date target only for `SCORECARD_TEMPLATE_TYPE_PROCESS`.
- Correct request coverage: PR #22772 normalizes both the page filter state and `ConversationCountChart`'s reconstructed states.
- Preserved behavior: conversation templates retain conversation-ended filtering.
- Remaining proof: deploy the PR and replay the RCG state; verify nonzero counts in fresh/incognito and persisted sessions.
- Optional hardening: make the analytics backend reject or safely ignore conversation-ended filtering for process-only template sets so another caller cannot recreate the zero-result join.

## Credential use

No AWS, Okta, Azure, or SSH credentials were used in this review.

## Live production-versus-local comparison

A follow-up visual comparison initially appeared to invert the expected behavior: RCG production showed data while the local PR branch showed zero. The two tabs were not equivalent:

- production selected `Customer Xperience Center - Social Care DTQ - V1`, daily frequency, and displayed 46 scorecards with 94% average performance;
- local selected `QM Coaching Customer Xperience Center - Social Care DTQ - V1`, weekly frequency, and displayed zero scorecards.

The template-name difference is material because RCG contains many similarly named template instances, including zero-data instances. Frequency cannot explain the different total, but the different selected template can. This comparison therefore does not falsify PR #22772 and cannot validate it either. The next comparison must select the exact same template resource ID, date window, use case, audience, and calculation in both tabs; if results still diverge, capture the `qaScoreStats:retrieve` request and response from each tab.

## Cross-user intermittency resolved

The later customer-success validation and targeted browser-state inspection explain the apparent intermittency:

- Andra's failing-session HAR sends `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT` on every `qaScoreStats:retrieve` request.
- Xuanyu's current RCG production `performance-page-v2` cache for `cel-customer-xperience` selects the data-bearing template `019dbd42-2ee4-7684-84f1-faf442fdc2c4`, but the serialized state omits `dateRangeTarget`.
- The cache key is customer/profile/use-case specific, while the value lives in each browser profile. `useLocalStorageFilters` replaces the default filter state with the cached state rather than merging newly introduced defaults into it. `localStateToFilterState` copies `state.dateRangeTarget` directly, so an old cache without the property continues to produce `undefined`.
- A browser profile with no prior cache starts from the post-CONVI-7586 default of `CONVERSATION_ENDED_AT`. This explains why Andra's incognito session also failed: incognito created the new failing default rather than escaping it.
- For the non-email RCG use case, the date-target selector is not rendered, so two users can see identical filter chips while carrying different hidden request state.

The behavior is therefore deterministic by browser-local persisted state, not random request intermittency. Xuanyu's production request omits the field and returns process data; Andra's production request includes it and loses the process rows in the conversation join. With the PR override suffix, process normalization removes the field regardless of the cached value, and Andra confirmed that data loads. This is deployed-preview validation of the root cause and fix, though the PR still needs the normal review and merge path.
