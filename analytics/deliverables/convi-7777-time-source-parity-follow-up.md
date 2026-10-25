# CONVI-7777: requested-range and source parity follow-up

## Local change

The new API now passes the requested [start, end) range to all seven evidence families. No additional fixed-day scan window is added. Notes and Guided Workflow choose Events without metadata filters and Conversations typed payloads with supported metadata filters, following the existing source selection. The branch changes both SQL projection and connection; Notes uses the legacy copy-entity predicate for each projection. Population membership still applies request metadata and other eligibility filters once, and the final evidence join retains exact conversation/agent membership.

Implementation is local in /Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage, branch xw/convi-7777-population-api. No commit, push, deployment or external reply has been made for this follow-up. The older scan-audit report remains evidence pinned to PR head 0d1b75da684ee0a9237020d6d500e117db6f7b4e, before these local changes.

## Summary records and matching

Summary usage uses two record types: a valid summary-generation record in action_annotation_d and an accepted summary-use event in conversation_event_d. Both carry conversation ID, agent ID and a copy of conversation_start_time. That copied field is not the time the generation or copy occurred.

For conversation C and agent A, generation at 10:00 and copying at 11:00 match normally when both records say the conversation started at 09:00. If the generation row says copied start 09:00 and the usage row says 09:05, their exact three-field join fails even though both records fall inside the requested day. The existing and new APIs already share this join. No additional Summary matching change is required for parity; changing the rule would be a separate correctness decision.

## Concrete bucket example

This is synthetic inconsistent copied data, not a claim about a normal conversation crossing midnight. Request October 1 through October 3, grouped daily. Conversation C / agent A has a conversation/message copy with conversation_start_time October 1 23:50. Its GenAI-use row instead says copied conversation_start_time October 2 00:05. Both source rows are in the request.

The existing daily GenAI query groups messages under October 1 and usage under October 2, then joins by agent and day. If agent A has no qualifying message bucket on October 2, the usage has no matching anchor and the legacy daily result reports zero use. The new query joins by conversation/agent, finds C, and reports one assisted conversation in the population's October 1 bucket. Summary has the same bucket issue if generation and use both agree on the October 2 copied timestamp but the population conversation copy says October 1. Ordinary generation/copy on another day does not itself cause this issue when all copied start fields agree.

The new API counts distinct assisted conversations; legacy endpoints count feature events/sessions or their own aggregates. Their totals need not be identical even after source-scan parity.

## Shared population terminology

Population means the conversations and agents eligible to appear in the answer after the requested time range, authorization and other request filters. It provides the total-conversation denominator and the allowed conversation/agent pairs for evidence lookup.

Latest-version selection means selecting among multiple stored snapshots of a conversation using update_time. For example, an old stored row for C says start October 1; a newer corrected row says start September 30. For an October 1 request, the current source WHERE removes the September 30 row before ORDER BY update_time DESC LIMIT 1 BY pair chooses a row. It can therefore retain the old October 1 snapshot. This describes the existing bounded query's behavior, not a new version-history feature.

Matching messages means the population requires a filtered message row for the same conversation ID and agent ID as the conversation row. A conversation with no matching message is excluded from the denominator, even if an assistance record exists. The message row's copied conversation_start_time and other filters must pass; this is not a requirement that a message was sent at the exact instant of assistance use. This may be an intentional eligibility rule and should not automatically be called a bug.

Fractional request boundaries are subsecond request timestamps. For [00:00:00.500, 00:00:01.500), the shared formatter emits whole-second SQL bounds [00:00:00, 00:00:01). An event at 00:00:00.250 is admitted too early; one at 00:00:01.250 is excluded too early. Whole-day requests do not trigger this particular discrepancy. This shared precision behavior remains unchanged by the parity follow-up.

## Verification

The final combined Go run passed the new API plus existing Notes and Guided Workflow suites in 221.577s, with 223 passing test nodes, zero failures and zero skips. The expanded new API Bazel target passed (39.6s test, 54.398s command). gofmt, Gazelle and git diff --check passed. The initial new boundary/source tests failed against the old implementation; captured evidence is saved in the session evidence directory. Tests use local ClickHouse 23.8.16; production performance and deployment remain unverified. Tests isolate Notes/Workflow family results because typed event values used to reproduce legacy predicates can overlap other feature types in the combined union. They supply an eligible pair directly for the per-feature source checks; time-boundary checks call the full RPC. This does not claim to fix the separate existing Notes projection/event-enum concern.
