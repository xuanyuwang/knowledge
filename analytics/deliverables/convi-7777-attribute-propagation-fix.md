# CONVI-7777: feature attribute propagation

## Local implementation

The evidence-query constructor uses the existing parseClickhouseFilter function with the request time range and a cloned attribute object, then passes the resulting table-specific conditions and bind arguments to all seven shared feature builders. This fixes the missing source-level attribute predicates while retaining the requested start-inclusive/end-exclusive bounds. The supported assistance list and feature-use predicates are unchanged.

User/team/ACL resolution and conversation metadata membership remain in the shared eligible population. The original user list is removed from the cloned source attribute: each concurrent evidence query already receives query-owned external eligible-agent data, so duplicating an inline user list would reintroduce the large-list problem. Metadata moment groups are also removed from source parsing because population membership owns their latest/stale/value-or-missing semantics. The existing Notes/Workflow source switch still inspects the original metadata request. Voicemail exclusions represented as moment groups are preserved as an explicit conversation criterion before the moment groups are removed. The request object is not mutated.

Notes/Workflow in Events use their shared conversation subquery to enforce language and voicemail filters: the Events event table lacks those columns. The event predicate still receives supported time, use-case and agent conditions. Typed Conversations projection retains language and voicemail predicates on event rows too. This avoids introducing SQL errors by forwarding predicates for absent columns; it does not change the legacy RPCs or shared filter parser.

Implementation remains local in /Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage on xw/convi-7777-population-api. The earlier time-bound/source-selection changes remain part of the local diff. No commit, push, external message, PR update or deployment was performed.

## Regression evidence

Forty new cases failed before attribute forwarding with no setup/query errors: 29 per-feature attribute cases, four Hint selectors and seven full-RPC use-case cases. The full-RPC cases keep two eligible conversations in the denominator and require only the conversation with matching feature evidence in the numerator; the caller's request must remain unchanged. All forty cases passed after forwarding (20.837s focused Go run).

Six additional Notes/Workflow Events language/voicemail/combined cases reproduced code 47 missing-column failures with naive forwarding. The adapter routes those fields to the existing conversation subquery instead. The final attribute matrix has 35 per-feature cases across language, use case, duration, voicemail and combined filters, plus four Hint-selector cases and seven full-RPC cases: 46 new scenarios.

Final full API Go verification passed in 42.545s: 193 passing test nodes, zero failures and zero skips. The full API Bazel target passed in 41.1s (42.815s command). Its first attempt picked up an unhealthy shared container and failed with a ZooKeeper schema-bootstrap error; explicitly overriding the Bazel test environment to use fresh containers resolved that environment failure. The shared container was not stopped or repaired. gofmt and git diff --check passed; Gazelle generated the added behavior/duration test dependencies via a temporary scratch package because the existing package excludes test files from normal discovery. The scratch package was removed.

## Compatibility boundary

This is attribute propagation through existing shared feature query logic, not a new definition of every generic Attribute field. The four Hint-selector tests execute the real shared Hint SQL/connection with supplied eligible pairs. They do not establish full-endpoint support for those selectors: the unchanged ConversationStats population does not support behavior, policy, direct moment-ID or hint-type predicates on its message source. Other generic attributes not consumed by the existing parser remain outside a support claim. No new semantics for unrelated feature/filter combinations were introduced.

Metadata membership is tested at the shared population boundary, not duplicated through feature metadata joins. Existing legacy metadata exclusion and event enum defects are not copied as additional filtering into the new API. The user accepted differences caused by inconsistent stored conversation-start copies; no special matching or bucket workaround was added.

These tests use synthetic local ClickHouse 23.8.16, not production data. No AWS, Okta, Azure or SSH credential/profile was read or selected. Production performance and deployment remain unverified.
