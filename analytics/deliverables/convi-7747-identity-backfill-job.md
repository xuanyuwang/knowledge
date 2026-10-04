# CONVI-7747 identity-aware Active Days backfill job (prepared, not submitted)

**Target:** `comcast/prod/acquisition` (voice), conversations ClickHouse `comcast_prod.conversation_with_labels`, following the Comcast Residential UPN-to-NTID merge in CE-7255. No product code change is required. This is a reviewable production job specification; the authoritative mapping manifest and customer-approved historical scope are required before submission. No mutation has run.

## Why this job

`RetrieveAgentStats` counts distinct agents from `conversation_d` and true labels from `conversation_with_labels_d`, grouped by `agent_user_id`. Existing historical labels use source IDs, while migrated PG conversations/messages and later ClickHouse conversation rows use target IDs. Three ticket agents have true labels before August 20 but 0 target-ID active labels on August 19. The normal label cron reads current PG IDs and historical online events with old IDs, so a replay cannot be assumed to reconstruct the matches.

The bounded read-only cohort audit on September 25 used `FINAL`, current conversation ID as `argMax(agent_user_id, update_time)`, and exact conversation-ID joins:

| America/New_York message day | True labeled conversations | Label/current-ID mismatches | Distinct old IDs |
| --- | ---: | ---: | ---: |
| Aug 18 | 1,668 | 1,668 | 413 |
| Aug 19 | 5,988 | 5,988 | 474 |
| Aug 20 | 4,407 | 12 | 9 |
| Aug 21 | 4,862 | 0 | 0 |
| Aug 22 | 2,652 | 0 | 0 |

Across Aug 18–19, 560 old IDs mapped to exactly one current ID in observed conversations; 51 had only one observed conversation. This is strong cohort evidence, **not** a complete migration manifest or proof that every latest ClickHouse row matches PG. The initial August 19 query restricted both conversation start and message time to that day and matched 5,984; the wider audit allowed cross-midnight conversations and found 5,988.

The later full pre-August-20 Acquisition read-only audit found 220,476 stale label rows across 220,472 conversations and 1,006 current agent IDs, including 648 false rows and 16,427 legacy blank-message rows. The observed old-to-current pairs are one-to-one for all 1,006 IDs; 141 pairs rest on one observed conversation, and no existing target label key collided with a stale row at the same conversation/message/target ID. The Aug 8–19 customer-reported slice contains 50,177 stale true-labeled conversations across 846 agents. These are data-level counts, not authorization to run the job. [Detailed audit](../sessions/2026-09-25/codex-convi-7747-identity-cutover.md#impact-and-customer-feedback-assessment).

## Required immutable input

1. Export CE-7255 merge job `01a01de1-3fc8-70eb-9f54-70a5d3dc456d` source→target usernames or read `auth.users.migrated_to_user_id` for `customer_id='comcast'`. Resolve both sides to user IDs. The auth DB skill currently fails cluster validation for `auth-comcast-prod`; this access gap must be resolved before a full-cohort run. Do not substitute a username-prefix guess.
2. Produce a versioned mapping manifest with `source_user_id,target_user_id,source_username,target_username,merge_job_id`. Require nonempty IDs, unique source, no self-maps or cycles, and target existence. Compare the manifest with exact PG chat/message IDs and latest CH conversation IDs for representative agents and all mismatching pairs. Investigate any source with multiple observed target IDs.
3. Freeze a read-only inventory of `conversation_with_labels_d FINAL` rows keyed by `(toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id, message_id)` where `customer_id='comcast'`, `profile_id='prod'`, `usecase_id='acquisition'`, and `agent_user_id` is in the manifest. Include both true and false rows, blank-message legacy rows, min/max end and message times, existing target-key collisions, and exact per-source counts. Compare current PG agent/message ownership by conversation/message ID. Exclude any mismatch not explained by this merge.

## Execution contract

- **Scope:** only manifest source IDs and inventory-approved label keys. Use the observed actual end-time bounds per batch, not a guessed August 20 cutoff; the merge moved historical data. Preserve original conversation/message times, `has_agent_assistance`, `create_time`, use case, and profile.
- **Transformation:** for every approved old-key label row, write one target-key row with `agent_user_id=target_user_id`; map `message_agent_user_id` through the same manifest when it names a migrated source. Set a monotonic `update_time` above existing target-key versions. If a target-key row already exists, compare the evidence fields and apply a documented winner rule; abort on conflicting true/false or attribution rather than silently overwrite.
- **Order:** snapshot/export original rows and counts; stage and validate transformed rows; insert target-key rows in bounded end-time batches; verify target-key reads with `FINAL`; then delete the exact old-key rows from the shard-local `conversation_with_labels` table across the cluster and wait for mutations to finish. The distributed `_d` table is a read/write router. `FINAL` cannot remove an old ID because `agent_user_id` is part of the replacement key. Do not delete old rows before target verification.
- **Idempotency:** identify each batch by manifest version and exact source-key set. Re-running an insert must replace the same target key with identical label evidence; a cleanup retry must target the same old keys. Refuse a run when manifest hash, source counts, or target collision inventory differ from the dry-run snapshot.
- **Rollback:** keep the exported old rows and target-key preimages until verification. If a batch fails, restore prior target-key versions/remove only this job's inserted keys using the recorded key inventory, then restore old rows if cleanup had started. Confirm the rollback procedure against a nonproduction table before production submission.

## Verification and release gate

For each batch and at completion: old-key approved rows = 0; target-key count equals the pre-run union after collision resolution; no two agent IDs remain for an approved conversation/message; true/false distribution and message timestamps are conserved; CH target IDs match PG current IDs. Re-run the exact August 19 and August 20 Agent Stats daily requests with Acquisition, All minutes, the same agent/team filters, and America/New_York bounds. Require the sampled Steven Ebert, Nick DiVeglia, and Matthew Nadolski August 19 values to change from 0 to 1 while August 20 remains 1. Audit all affected daily buckets before calling the backfill complete.

**Submission blockers:** Comcast's desired historical correction scope, authoritative mapping validation, exact batch manifest, and a mutation-capable execution/rollback runbook. The existing DB skills deliberately provide read-only access. The customer has not yet answered whether all history since June, only Aug 8–19, or an explanation without backfill is needed. This document does not authorize or launch a production write.
