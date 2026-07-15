# Notifications Architecture and Routing

## Domain Model

Treat a notification as a pipeline with independently testable decisions:

```text
business/operational trigger
  -> eligibility and suppression
    -> recipient or owning team
      -> destination/channel selection
        -> payload/template rendering
          -> delivery attempt
            -> retry/deduplication
              -> observability and escalation
```

A notification bug should be classified at one of these boundaries instead of treated as a generic “Slack/email did not work” issue.

## Operational Job Failure Path

The central internal-job Slack path is in `go-servers`:

- `apiserver/internal/internaljob/publish_progress_update.go`
- `apiserver/internal/internaljob/publish_slack_notifications.go`
- `apiserver/internal/internaljob/slack/module.go`

The 2026-06-29 source review found:

- notification is triggered on job state change;
- failed, timed-out, and partially succeeded jobs are reportable;
- nested child jobs are suppressed;
- a cluster/environment fallback job-error channel is configured by `JOBS_ERROR_CHANNEL`;
- nonpublic customer config can override the channel when `JOBS_ERROR_SLACK_CHANNEL_FROM_CUSTOMER_CONFIG=true`;
- test/unstable jobs and silence settings can change destination or suppress noise;
- selected job types mention the QM/Coaching on-call Slack user group.

Team ownership mention and destination channel are separate decisions. Mentioning the QM/Coaching on-call group does not by itself route the message to a team-specific channel.

## On-Call Identity

PagerDuty is the source of truth for who is on call. `go-servers/oncall-sync` periodically maps PagerDuty escalation policies to Slack user-group membership. The QM/Coaching mapping already exists, so new notification designs should normally reuse the managed user group rather than maintain a static person list.

## GroundCover/Grafana Path

Service-health alerts are configured through Grafana/GroundCover alerting:

- routing uses a `service` label;
- `priority` distinguishes paging urgency;
- the default contact policy routes through PagerDuty;
- direct Slack contact points require infrastructure-managed channel configuration and are better suited to non-page broadcast alerts.

Use PagerDuty ownership for high-urgency service health. Do not replace paging with a Slack-only route merely because Slack is more visible.

## User-Configured Job Alerts

Some frontend-created jobs carry their own Slack destination:

- Conversation Alerts accept a Slack channel in notification settings.
- scheduled virtual-agent tests can enable Slack alerts and provide a channel.

These are job-specific product configurations, not the same routing layer as generic internal-job failures.

## Direct Workflow Notifications

Some workflows post directly to hardcoded or workflow-specific Slack destinations. They are architectural precedents but also evidence of routing fragmentation. Prefer a shared team/job-type routing abstraction when the behavior is broadly operational; retain direct routing only when a workflow has genuinely unique recipients or payload semantics.

## Source-Repo Map

| Concern | Primary source |
|---|---|
| Internal-job failure state and message construction | `go-servers/apiserver/internal/internaljob/` |
| Slack client/module and team mention mapping | `go-servers/apiserver/internal/internaljob/slack/`, `go-servers/shared/slack/` |
| On-call group membership sync | `go-servers/oncall-sync/` and chart/config |
| Job-specific Slack channel input | `director` job configuration UI plus matching backend handlers |
| Cluster/deployment health Slack | `flux-deployments/apps/flux-slack-notifications/` |
| GroundCover/Grafana→PagerDuty routing | infrastructure alert configuration and service labels |
| Scorecard submit/publish recipients | coaching scorecard notification code and template permission/state evaluation |

## Architecture Gaps

- No canonical team/job-type destination-channel abstraction was identified; generic channel selection and on-call mentions are separate.
- Direct workflow routes are inconsistent.
- Retry, deduplication, delivery persistence, and failure-observability semantics are not yet inventoried across all channels.
- The complete scorecard notification handler/source map still needs a fresh source-code pass.
