# August 20 Active Days backfill preparation

> **Superseded for CONVI-7747:** The user clarified the August 20 ticket is Comcast Residential Acquisition (voice), profile `comcast/prod`. This note's Bill West job shape was based on a provisional wrong-customer assumption and must not be used for Comcast. See `codex-convi-7747-comcast-active-days.md`; no Comcast root cause or backfill scope has been confirmed.

- Date: 2026-09-24
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: main checkout at merge commit `772c0f2590b45803acd07092a9460dc15cdb61f9`; read-only source, Linear, and deployment-config inspection
- Candidate customer: Bill West, profile `us-west-2`; **the user has not yet identified the August 20 example, so this association is provisional**
- No product-code change, production query, data mutation, or job submission occurred.

## What is established

Last week's INSI-4262 investigation confirmed false-negative Active Days for Bill West on July 23 and August 11/13. Long qualifying Agent Assist online sessions began before the labeler's three-hour start-time lookback, then overlapped the agent messages. The former session-start containment query excluded them. PR [go-servers#32443](https://github.com/cresta/go-servers/pull/32443), attached to INSI-4262 and INSI-4872, changed that query to interval overlap and merged September 17. Linear marks both issues Done. Merge and issue status do not establish the exact image deployed on the Bill West cron.

An August 20 Active Days or Convos Powered by AA gap could have the same cause because it lies between the June 10 regression and September 17 merge. The date alone is insufficient: identify the customer, agent, timezone, Leaderboard filters, and ideally conversation IDs. Distinguish absent qualifying online events, Director-only presence, source exclusions, stale label assignment, and missing label rows.

## Replay mechanism and boundary

`cron-label-conversations` in `us-west-2-prod/cresta-cron` is enabled and scheduled every 30 minutes in checked-in Flux configuration. The task supports RFC3339 start/end overrides, a customer filter, and a batch size. It selects app `chats` by **`ended_at` in [start, end)** and writes `conversation_with_labels_d`. Therefore a local August 20 display day is not by itself the correct job interval: resolve the affected conversations' UTC `ended_at` values, then choose bounded UTC windows around them. The job's customer filter does not select an individual profile; confirm the customer's profile set before replay.

The local `conversation_with_labels` table is a `ReplicatedReplacingMergeTree` keyed by end-hour, agent, conversation ID, customer, profile, and message ID. A replay can leave stale rows when agent or end-hour changed, while message-level rows may use different keys. Establish the existing row set for the exact affected conversation IDs and an explicit cleanup/replay boundary. Run any required local-table delete on the cluster and wait for mutations to finish before replay. Do not delete a calendar day solely because a dashboard date looks wrong.

## Prepared one-shot command shape (not executed)

```sh
kubectl create job <unique-job-name> \
  --from=cronjob/cron-label-conversations \
  -n cresta-cron --context=us-west-2-prod_dev \
  --dry-run=client -o yaml > <draft-job.yaml>

kubectl set env --local -f <draft-job.yaml> \
  ENABLE_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=true \
  FILTER_CUSTOMER_IN_LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE=bill-west \
  LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_START_AT_RANGE_START=<UTC-RFC3339-start> \
  LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_CONV_END_AT_RANGE_END=<UTC-RFC3339-end> \
  LABEL_CONVERSATIONS_WITH_AGENT_ASSISTANCE_BATCH_SIZE=24h \
  -o yaml > <reviewed-job.yaml>
```

Before creating the job: confirm the actual CronJob namespace/context and image digest, that the image contains `772c0f2` or a descendant, and that the generated pod template has only the intended customer and time overrides. Submit one bounded window, wait for completion, then verify exact affected conversation IDs and the agent/day Active Days result. Keep subsequent windows sequential to avoid duplicate work and load.

## Evidence still required for the final job

1. Exact August 20 customer/agent/timezone/filter context from the user.
2. Read-only comparison of app conversations/messages, qualifying auth online intervals, and current label rows for that context.
3. Exact affected conversation IDs and UTC ended-at range; before counts and any stale/missing label IDs.
4. Live deployed CronJob image and post-merge release verification.
5. Cleanup scope and the final rendered manifest; post-run checks for label correctness, duplicate keys, and Leaderboard values.
