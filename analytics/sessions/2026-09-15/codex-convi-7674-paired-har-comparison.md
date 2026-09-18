# CONVI-7674 paired RCG HAR comparison

- Date: 2026-09-15
- Source repo: `/Users/xuanyu.wang/repos/director` (`main`, read-only source tracing)
- Ticket: [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after)
- Fix: [Director PR #22772](https://github.com/cresta/director/pull/22772), head `e392f523f3857600e38f0df8f08d3c239300163f`
- Andra HAR: `/Users/xuanyu.wang/repos/andra-rcg.cresta.com.har`, SHA-256 `cd2d3393aa4b92772918c6d358c7f541d459e90e3c8dbf490562b149f611e47f`
- Xuanyu HAR: `/Users/xuanyu.wang/repos/xuanyu-rcg.cresta.com.har`, SHA-256 `2ea9956b30b05e4cef4508bd2e148bb7db6ea68147c933768578c85b99ae03ed`

## Conclusion

The paired HARs confirm that the account difference is not template selection, audience/ACL filters, use case, time window, or random backend behavior. The clients send different hidden time-target state from the same production code:

- Andra: all 20 `qaScoreStats:retrieve` requests send `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`; all 20 return HTTP 200 with zero score groups, zero conversations, and zero scorecards.
- Xuanyu: 19 of 21 requests omit `conversationTimeRangeField`; 18 complete successfully and return populated results. The remaining omitted-field request has HAR status 0. The only two requests that send `CONVERSATION_ENDED_AT` both return HTTP 200 with zero groups, conversations, and scorecards.

Both captures use the exact same data-bearing template `019dbd42-2ee4-7684-84f1-faf442fdc2c4`, use case `cel-customer-xperience`, July/August 2026 windows, empty user/group/status filters, and `filterToAgentsOnly: false`. After removing only the incompatible time target and inconsequential omitted/default serialization differences, 15 request shapes are shared between the captures.

This pins the user-visible intermittency to browser-local filter state. It is deterministic across state populations, not intermittent inside the analytics backend.

## Strongest within-session control

Xuanyu's own HAR contains the clearest control pair for the August time-range count request:

| Hidden field | Result |
|---|---:|
| `conversationTimeRangeField` omitted | 46 scorecards, 16 grouped conversations |
| `CONVERSATION_ENDED_AT` | 0 scorecards, 0 conversations |

The requests have the same parent, template, use case, window, daily frequency, grouping, empty audience/status filters, `includeNaScored: true`, and `filterToAgentsOnly: false`. The only other serialization difference is an explicit empty `criterionIdentifiers` array versus omission, which is semantically empty in both cases.

The two zero-result Xuanyu calls are the count chart's internally reconstructed unfiltered/delta states. Production's primary state inherited Xuanyu's old cached omission and returned data, while those reconstructed states reintroduced the new conversation-ended default. PR #22772's second commit normalizes these auxiliary states too.

## Why the same code diverges

`useLocalStorageFilters` loads cached state by replacing the initial filter state. `localStateToFilterState` copies `state.dateRangeTarget` without merging a newly introduced default. Therefore:

- browser state created before CONVI-7586 can keep `dateRangeTarget` absent and send no field;
- fresh/reset/incognito state starts from the new conversation-ended default and sends it;
- the RCG non-email UI does not expose the date-target control, so the visible filter chips can look identical;
- PR #22772 removes the field after state loading whenever the selected template is process type, converging both state populations.

## Timing observation

Andra's 20 zero-result joined requests took 9.3-14.0 seconds (11.8-second mean). Xuanyu's 20 successful requests took 2.2-3.2 seconds (2.5-second mean). This is consistent with the conversation-join path being more expensive, but the timing difference is secondary: request/response contents already establish the correctness failure. It does not replace the separate RCG latency investigation.

## Evidence boundary

The HAR comparison proves the clients send different hidden fields and that the field perfectly predicts zero versus populated results in these captures. Backend source and the earlier ClickHouse reproduction complete the mechanism: conversation-ended time adds an inner conversation join, while RCG process scorecards have empty conversation IDs. Andra's successful PR-preview test provides affected-user end-to-end validation.

No HAR headers, cookies, or credentials were inspected. No AWS, Okta, Azure, or SSH credentials were used.

## Linear publication

Posted the paired-HAR evidence to [CONVI-7674](https://linear.app/cresta/issue/CONVI-7674/rcg-process-scorecards-show-no-data-in-performance-insights-after#comment-a395041b-24bd-4153-a5c3-31b5a76f34da) on 2026-09-15 and verified it through the Linear API.

- Comment ID: `a395041b-24bd-4153-a5c3-31b5a76f34da`
- Included: cross-user request/result counts, the within-session 46-versus-0 control, persisted-state explanation, backend join mechanism, and PR-preview validation.
- Excluded: HAR files, headers, cookies, and credentials.
