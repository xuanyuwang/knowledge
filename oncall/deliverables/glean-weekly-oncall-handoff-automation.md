# Glean Weekly Oncall Handoff Automation

Date: 2026-07-31

## Recommendation

Use a native scheduled Glean Agent as the primary implementation. Build it in **Workflow mode** so source collection, classification, and formatting are deterministic; schedule it for the end of the QM/Coaching rotation; and finish with a Slack message action to a restricted handoff channel.

For the current QM/Coaching rotation, the evidence shows handoff moved to Monday at 9:00 AM Pacific. A practical starting schedule is **Monday at 8:30 AM America/Los_Angeles**, leaving 30 minutes for the outgoing oncaller to review the generated draft. Confirm the actual handoff meeting time before activation.

Do not begin with a custom GitHub Actions integration unless native scheduling or the required Slack write action is unavailable. Glean now supports scheduled triggers directly, and Glean's own guidance recommends Workflow mode when a prescribed, repeatable sequence matters.

## What the automation should do

The agent should synthesize, not replace, the operational sources of truth:

1. Search the previous rotation window in the agreed QM/Coaching Slack channels for `@qm-coaching-oncall` mentions, incident threads, mitigations, customer impact, and unresolved asks.
2. Find QM/Coaching Linear tickets created, updated, or closed during the same window, using a dedicated oncall view or agreed labels rather than an unconstrained team-wide search.
3. Find PagerDuty incidents for the QM/Coaching service or escalation policy and preserve their current status and links.
4. Follow linked PRs, incident channels, postmortems, and documents only when needed to establish current status, owner, or next action.
5. Deduplicate the same issue across Slack, Linear, PagerDuty, and GitHub.
6. Produce one evidence-linked handoff with unresolved work first.
7. Post the result to the handoff channel, or retain it in Glean for review during the initial shadow period.

Glean should never become the authoritative status store. Linear remains authoritative for work status, PagerDuty for incident state, and the linked incident or Slack thread for live coordination.

## Required setup

### Admin prerequisites

- Enable scheduled triggers in **Admin console > Platform > Agents > Scheduled triggers** for the agent owner or an appropriate IdP group. The feature is off by default.
- Confirm the Slack connector and Slack Real Time Search are healthy.
- Allow the specific Slack message tool to **Run without user confirmation** if the scheduled agent will post automatically. Background writes cannot wait for an interactive approval.
- Confirm Agent Builder access for the builder and activation access for the schedule owner.

## Build steps

1. In Glean, open **Agents** and create a **Workflow mode** agent named `QM/Coaching Weekly Oncall Handoff`.
2. Use an **Input form trigger** and select **Allow agent to run on a schedule**.
3. Add inputs with defaults so later activation cannot pause for missing values:
   - rotation start and end rule;
   - Slack source channels;
   - Linear oncall view or labels;
   - PagerDuty service or escalation policy;
   - destination Slack channel.
4. Add explicit retrieval steps for Slack, Linear, and PagerDuty. Avoid one broad natural-language search across all company content.
5. Add a reasoning step that applies the output contract and guardrails below.
6. Add **Send Slack message to a channel** as the final step, bound to one concrete handoff channel. If background Slack writes are not allowed, omit this step for the pilot and review the result in Glean.
7. Save and publish the agent.
8. In the Agent library, open the agent, select **Set schedule**, configure the start date, `America/Los_Angeles`, weekly recurrence, and the agreed time, then select **Activate agent**.
9. Use **Run now** with a completed historical week. Compare the result with the outgoing oncaller's actual handoff before enabling unattended posting.

## Suggested agent instruction

```text
You create the weekly QM/Coaching oncall handoff for the rotation window
[[rotation_start]] through [[rotation_end]].

Search only the configured sources:
- Slack: [[slack_channels]], including threads that mention @qm-coaching-oncall
- Linear: [[linear_oncall_view_or_labels]]
- PagerDuty: [[pagerduty_service_or_policy]]
- Linked GitHub PRs, incident channels, postmortems, or docs needed to verify status

Build one issue record for each operational event. Deduplicate records that describe
the same issue. Prefer Linear for task state, PagerDuty for incident state, and the
latest timestamped incident or Slack update for live mitigation state.

Order the handoff as follows:
1. Immediate attention: unresolved P0-P2 incidents or customer-impacting issues
2. Active carryover: symptom, impact, current state, owner, next action, blocker
3. Resolved this rotation: root cause and mitigation in one or two bullets each
4. Changes worth watching: deploys, migrations, flags, or scheduled operations
5. Reliability follow-ups: noisy alerts, missing runbooks, repeated failure patterns
6. Coverage gaps: sources that could not be accessed or facts that could not be verified

For every issue, include direct evidence links and the timestamp of the latest relevant
update. Never invent an owner, priority, root cause, resolution, or due date. Write
"not established" when the evidence does not establish a fact. If two sources conflict,
show the conflict and ask the incoming oncaller to verify it. Keep the result concise
enough to review in five minutes.
```

## Output contract

The Slack message should follow this shape:

```md
# QM/Coaching Oncall Handoff — <start> to <end>
Outgoing: <name or not established> | Incoming: <name or not established>

## Immediate attention
- [P?] <issue> — <impact>; <current state>; next: <action> (<owner>)
  Evidence: <Linear> · <PagerDuty> · <Slack>

## Active carryover
- <same evidence-linked structure>

## Resolved this rotation
- <issue> — root cause: <fact>; mitigation: <fact>

## Changes worth watching
- <change, risk, observation window, owner>

## Reliability follow-ups
- <alert/runbook/repeated-pattern improvement>

## Coverage gaps
- <unavailable source or unverified fact>
```

## Security and correctness guardrails

- Post only to a channel whose membership is appropriate for every source the scheduled owner can retrieve. A scheduled write can otherwise move private source content into a broader destination.
- Prefer a private team handoff channel during the pilot.
- Do not use public, channel-visible agent publishing as a substitute for the scheduled Slack action. Public Slack responses use stricter broadly-shared-content rules and can omit team-restricted evidence.
- Always include source URLs. A claim without evidence should be marked unverified or omitted.
- Keep unresolved items separate from resolved history; do not infer resolution from silence.
- Treat the generated message as a draft during the first two rotations. The outgoing oncaller should correct it before handoff.

## Pilot and validation

Run a two-rotation shadow pilot:

1. Generate the handoff 30 minutes before rotation.
2. Have the outgoing oncaller compare it with the manual record.
3. Track missed issues, stale status, duplicate issues, incorrect ownership, unsupported conclusions, and inaccessible sources.
4. Tighten source filters and prompt rules after each run.
5. Enable unattended channel posting only after the agent consistently captures all active carryover and makes no permission-sensitive disclosures.

Success criteria:

- every active customer-impacting issue appears;
- every item has at least one direct source link;
- no resolved issue is presented as active, or vice versa;
- the outgoing oncaller needs less than five minutes to correct the draft;
- the incoming oncaller can identify the next action and owner without replaying the week.

## Operating caveats

- Glean allows at most 10 active background agents per user.
- Scheduled execution time can vary under load.
- Adding or renaming a required input, changing the trigger away from Input form, removing access, or disabling scheduling can pause subscriptions and require manual reactivation.
- Background tools are limited to capabilities admins permit without user confirmation.
- Exact execution limits apply; constrain source queries and avoid unbounded historical searches.
- One Slack channel action is normally bound to one concrete destination. Prefer one canonical handoff channel instead of broadcasting copies.

## Fallback: GitHub Actions plus the Glean Agents API

Use the internal `glean-daily-summary-support-agent` pattern only if native scheduled triggers or background Slack writes cannot be enabled.

The current implementation:

- schedules a Python runner with GitHub Actions cron;
- uses a user-scoped Glean API token with `AGENTS` scope;
- invokes `POST /rest/api/v1/agents/runs/stream` and consumes server-sent events;
- posts the assembled result through a Slack incoming webhook;
- stores `GLEAN_API_TOKEN`, `GLEAN_SERVER_URL`, `GLEAN_AGENT_ID`, and `SLACK_WEBHOOK_URL` as GitHub Actions secrets;
- retries failures and aborts rather than posting an empty handoff.

Important: the repository README is stale in two ways. It still describes `/agents/runs/wait`, while the current script uses `/agents/runs/stream` to avoid idle timeouts; and the current workflow file has the cron schedule commented out, so copying the repository does not create a scheduled job until the schedule is explicitly restored. GitHub cron is UTC and can drift relative to local time unless daylight-saving behavior is handled.

## Evidence

- [Glean schedule triggers](https://docs.glean.com/agents/concepts/schedule-triggers)
- [Glean publishing to Slack](https://docs.glean.com/agents/concepts/publish-slack)
- [Glean Slack connector setup](https://docs.glean.com/connectors/native/slack/setup/slack-connector)
- [Cresta/Glean weekly-channel-summary guidance](https://crestalabs.slack.com/archives/C0909QZADBN/p1775080604553269?thread_ts=1775060192.359559&cid=C0909QZADBN)
- [Internal Glean Daily Summary Agent](https://github.com/cresta/glean-daily-summary-support-agent/blob/main/README.md)
- [Current API runner](https://github.com/cresta/glean-daily-summary-support-agent/blob/main/daily_agent_report.py)
- [Current GitHub Actions workflow](https://github.com/cresta/glean-daily-summary-support-agent/blob/main/.github/workflows/daily_report.yml)
- [QM/Coaching Monday rotation change](https://github.com/cresta/terraform/pull/12304)
- [QM/Coaching oncall sync](https://github.com/cresta/go-servers/pull/28553)
- [Existing oncall handoff model](https://docs.superhuman.com/d/_dDRLf7PHWP1/_su0OuK1P)
