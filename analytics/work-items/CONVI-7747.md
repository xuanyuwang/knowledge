# CONVI-7747: Comcast Residential Active Days before August 20

**Status:** Production impact quantified; backfill decision deferred pending Comcast feedback and authoritative mapping
**Primary domain:** `analytics`; **subdomain:** `active-days`
**Ticket:** https://linear.app/cresta/issue/CONVI-7747/comcast-residential-agent-leaderboard-active-days-shows-0n-a-for-all

## Objective

Determine why Comcast Acquisition (voice) Agent Leaderboard shows Active Days 0/N/A before 2026-08-20 and 1 starting on that date across the Rebecca Gardner and Walter Landaverde teams. Establish the exact source-data failure before selecting any replay or backfill.

## Confirmed scope

- Latest `config` `origin/master` at `aa9f2cafce47ebfbf6cbf54a109b8bfdb556dbf5` has one `comcast` production profile: `profileId: prod`, `customerId: comcast`, cluster `comcast-prod`, app DB `comcast`, use case `acquisition` with channel `voice`.
- The ticket names two August 19 examples: Nick DiVeglia conversation `32d004246a85ad8a_23010101133_1787145698863529` and Matthew Nadolski conversation `271414366a861578_23010100692_1787172936281664`.
- The checked-in Comcast profile and acquisition insights YAML do not set `excludeIngestionPipelineFromAgentStats`; code defaults that flag to false. This is a config-file observation, not a live ConfigService read-back.
- The June 10 session-start containment regression fixed by go-servers PR #32443 is ruled out for the checked ticket examples: their true labels predate that fix and exact qualifying sessions cover the messages.
- Comcast config changes on August 20 added Acquisition skill `9993199` at 06:12 UTC, demo chat routing skills at 16:33 UTC, and a frontend conversation-access flag change at 20:17 UTC. None by itself explains Active Days.

## Access and remaining evidence

Read-only Comcast app PostgreSQL, conversations ClickHouse, and events ClickHouse are available through the customer DB skills. The auth DB skill resolves `auth-comcast-prod` but currently fails cluster validation, so its authoritative `auth.users.migrated_to_user_id` mapping remains unread. The original CE-7255 merge input is an alternative mapping source. A full pre-August-20 Acquisition label audit and target-key collision check are complete; live effective flag, all agents' PG ownership, and current team visibility remain unchecked.

## Current decision

The sampled Comcast symptom is **not** the Bill West online-session overlap bug. A Comcast Residential UPN-to-NTID user merge on August 19-20 moved historical conversations and messages to short-username user IDs, while the pre-cutover qualifying online events and `conversation_with_labels_d` rows retained email-style user IDs. `RetrieveAgentStats` groups and filters by `agent_user_id`, so the current short-username agents have conversations but zero matching active labels before August 20. On August 20, online events and labels begin using the short IDs. See the September 25 session for exact IDs, counts, timestamps, and the independent merge-job ticket CE-7255.

Do not reuse the provisional Bill West backfill job. A normal label-cron replay from current PG rows is also insufficient: current messages use short IDs, whereas the historical online events use the old email-style IDs. The [identity-aware backfill job specification](../deliverables/convi-7747-identity-backfill-job.md) defines mapping, inventory, transformation, cleanup, idempotency, rollback, and verification gates. A bounded Aug 18–22 audit found 1,668/1,668 and 5,988/5,988 true labeled Acquisition conversations mismatched on Aug 18 and 19, only 12/4,407 on Aug 20, and none on Aug 21–22. This is not yet the complete historical repair cohort. [coaching-qm-skills#9](https://github.com/cresta/coaching-qm-skills/pull/9) merged the dedicated diagnostic skill. No mutation has been run.

The full read-only Acquisition audit found 220,476 stale label rows across 220,472 conversations and 1,006 current agent IDs before August 20. Of these, 219,824 conversations had true labels under old IDs; they span 16,171 current-agent/day pairs. The reported Aug 8–19 window alone has 50,177 such conversations across 846 agents and 4,343 agent-days. Source-0 conversations on the same local day make 16,062 full-range and 4,325 reported-window agent-days strong candidates for incorrect 0 values. The observed exact-conversation mapping is one-to-one for 1,006 IDs with zero target-label-key collisions, but it still needs authoritative validation. See [impact audit](../sessions/2026-09-25/codex-convi-7747-identity-cutover.md#impact-and-customer-feedback-assessment).

The customer report says multiple supervisor teams are affected and the impact is reporting/coaching visibility, with no live-call impact. The Linear ticket has no later customer answer about whether to correct all history or only a recent window; Zendesk is unavailable. Defer implementation and production mutation until the customer-facing owner confirms the historical scope and value. If correction is wanted, use a one-time mapped batch repair; do not edit agents or conversations manually.

Joined true-label rows show no further current-ID mismatches in weekly cohorts from Aug 23 through Sep 25, supporting a historical-only repair scope in the observed Acquisition population.

## Timeline

- 2026-09-24: Resolved the exact Comcast profile from latest config and read CONVI-7747. Inspected source/config changes and attempted approved read-only DB skills; production SQL remains blocked by missing dedicated-cluster access. See `sessions/2026-09-24/codex-convi-7747-comcast-active-days.md`.
- 2026-09-25: New ordinary Comcast AWS profiles enabled read-only PG and CH access. Confirmed pre-August-20 old-ID online events/labels versus post-merge short-ID conversations for Steven Ebert, Nick DiVeglia, and Matthew Nadolski; August 20 new-ID events/labels reproduce the reported cutover. See `sessions/2026-09-25/codex-convi-7747-identity-cutover.md`.
- 2026-09-25: Expanded the read-only CH audit across Aug 18–22, confirmed the Agent Stats grouping and CH replacement-key boundary, and prepared the identity-aware backfill specification and diagnostic skill. Auth DB cluster validation still blocks authoritative mapping retrieval.
- 2026-09-25: Measured the full pre-August-20 Acquisition label mismatch, observed 220,472 stale-label conversations across 1,006 current IDs and 16,062 likely wrong daily cells with qualifying source-0 conversations. Assessed the reported Aug 8–19 window separately and deferred repair implementation pending Comcast's desired historical scope.
