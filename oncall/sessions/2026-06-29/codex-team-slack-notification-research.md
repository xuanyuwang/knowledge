# Team Slack Notification Research

Date: 2026-06-29
Tool: Codex
Source repos:
- /Users/xuanyu.wang/repos/go-servers on main
- /Users/xuanyu.wang/repos/director on main
- /Users/xuanyu.wang/repos/flux-deployments on xwang/convi-7147-enable-scorecard-autoheal

## Objective

Research how existing code and deployment configuration route operational notifications to Slack channels, especially for:

- BE APIs
- Cron jobs or Temporal workflows
- FE
- GroundCover

The motivating use case is a dedicated Slack channel for Coaching and QM on-call notifications.

## Findings

### Backend APIs and Job Failure Notifications

The central backend path for operational job failure Slack notifications is:

- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/internaljob/publish_progress_update.go`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/internaljob/publish_slack_notifications.go`
- `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/internaljob/slack/module.go`

`publish_progress_update.go` calls `notifySlackViaWebhookOnActivityError` only on job state changes. The notifier reports `FAILED`, `TIMED_OUT`, and `PARTIALLY_SUCCEEDED` jobs, ignores nested child jobs, and posts with `PostMessageContextWithJoin`.

Slack channel routing is environment/customer based:

- `JOBS_ERROR_CHANNEL` is the fallback job error channel.
- `SLACK_JOB_BOT_TOKEN` configures the bot token.
- `JOBS_ERROR_SLACK_CHANNEL_FROM_CUSTOMER_CONFIG=true` lets nonpublic customer config `slack_channel_id` override `JOBS_ERROR_CHANNEL`.
- `(test)` and `(unstable)` display-name suffixes can route alerts to the creator/owner DM instead of the shared channel.
- Silencing flags exist for job types and partial-success profiles.

Team ownership is currently represented as Slack user group mentions, not per-team channels. `shared/slack/const.go` defines `QMCoachingOnCallID = "S0B942SJL8Z"`, and `publish_slack_notifications.go` maps these job types to the QM/Coaching user group:

- `JOB_TYPE_BACKFILL_SCORECARDS`
- `JOB_TYPE_ASPECT_WFM_IMPORT`

Implication: QM/Coaching already gets mentioned for these job types, but the messages still land in the selected job error channel, not a dedicated team channel, unless the customer config channel or deployment env var points there.

### Cron Jobs and On-call Sync

The `oncall-sync` service keeps Slack user groups in sync with PagerDuty escalation policies:

- Source: `/Users/xuanyu.wang/repos/go-servers/oncall-sync`
- Helm CronJob: `/Users/xuanyu.wang/repos/go-servers/charts/oncall-sync/templates/cronjob.yaml`
- Values: `/Users/xuanyu.wang/repos/go-servers/charts/oncall-sync/values.yaml`
- Mapping config: `/Users/xuanyu.wang/repos/go-servers/oncall-sync/internal/oncallsync/config.yaml`

It runs every 15 minutes (`*/15 * * * *`) with `concurrencyPolicy: Forbid`.

QM/Coaching already has an entry:

- PagerDuty escalation policy: `PZK4943`
- Name: `qm-coaching`
- Okta group: `slackgroup_qm-coaching-oncall`
- Escalation levels: `[1, 2]`

Implication: using the Slack user group is the current canonical pattern for "who is on call now"; the membership is driven by PagerDuty.

### Temporal Workflows and Scheduled Jobs

Most Temporal-backed internal jobs rely on generic job state publication, so failures flow through the backend job notifier above.

There are also direct Slack paths:

- Model deployment deleted-customer GC posts directly to hardcoded channel `C02V17X2G4R` from `/Users/xuanyu.wang/repos/go-servers/temporal/ai_services/modeldeploy/activity_send_slack_gc_notification.go`.
- That workflow can mention a subteam via `GC_DELETED_CUSTOMERS_MODELS_SLACK_NOTIFY_SUBTEAM`, defaulting in config to a platform on-call group.
- Orphaned model deployment warnings post to a hardcoded Slack workflow webhook from `/Users/xuanyu.wang/repos/go-servers/temporal/ai_services/modeldeploy/activity_gc_model_deployments_and_adapters.go`.
- Conversation alerts and scheduled virtual-agent tests accept a Slack channel through job payload notification settings and send workflow-specific Slack messages.

Implication: direct Slack routing exists, but it is ad hoc per workflow. For QM/Coaching, a reusable job-type-to-channel route would be cleaner than adding more hardcoded workflow channels unless the notification is truly workflow-specific.

### Flux Deployments

Flux Slack notifications are cluster/deployment health notifications, not team application on-call notifications:

- `/Users/xuanyu.wang/repos/flux-deployments/apps/flux-slack-notifications/releases/00-head/provider-slack-error-v2.yaml` sends to `k8s-${cluster_env}-error`.
- `/Users/xuanyu.wang/repos/flux-deployments/apps/flux-slack-notifications/releases/00-head/provider-slack-info-v2.yaml` sends to `k8s-${cluster_env}-info`.
- Alerts watch Flux `GitRepository` and `Kustomization` resources.
- `/Users/xuanyu.wang/repos/flux-deployments/README.md` documents the `#k8s-<cluster>-info` and `#k8s-<cluster>-error` convention.

Apiserver deployments provide the application job bot token and cluster-specific job error channel:

- `/Users/xuanyu.wang/repos/flux-deployments/apps/apiserver/releases/03-prod-main/helmrelease-apiserver.yaml` wires `SLACK_JOB_BOT_TOKEN`.
- `/Users/xuanyu.wang/repos/flux-deployments/releases/us-west-2-prod/cresta-api/patch-helmrelease-apiserver.yaml` sets `JOBS_ERROR_CHANNEL` to `C05KR433J1Z`.
- Staging/prod-like clusters set their own `JOBS_ERROR_CHANNEL` values in release patches.

There is an SDP `quality-assurance` team config pointing to `#qa-testing`, but it owns QA infra services like `1password-connect` and `reportportal`; it does not appear to represent QM/Coaching product on-call ownership.

### Frontend

Director does not have a general "team on-call channel" setting for backend job failures.

It does expose job-specific Slack channel inputs:

- Conversation Alert UI adds `config.notificationSettings.slack.channel` when a Slack channel is set.
- Backend `ConversationAlertHandler` reads that channel; without it, it defaults to dry-run.
- Run Virtual Agent Tests UI includes "Send Slack alert" and "Slack Channel"; backend converts it into scheduled test run alert config.

Director also has GroundCover observability integration, but this is inspection/tracking rather than Slack routing:

- Browser GroundCover SDK initialization in `/Users/xuanyu.wang/repos/director/packages/director-app/src/context/tracking/tracker/initialize.ts`.
- Job details page exposes GroundCover log links in `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/jobs/job-to-groundcover-logs/JobToGroundcoverLogs.tsx`.

### GroundCover and Internal Docs

Glean-read internal docs:

- PagerDuty: `https://coda.io/d/_dboaRXZbw4w/_su22g1if`
- Setting up alert rules in Grafana: `https://coda.io/d/_dboaRXZbw4w/_suNJKtk2`
- Voice Integration Groundcover alerts: `https://coda.io/d/_deNxzn_iL_p/_suIaPYpV`

Relevant conventions:

- GroundCover alerting is managed through Grafana alerts.
- PagerDuty is the default contact point via "Use notification policy".
- Routing is by `service` label, not directly by team.
- `priority` must be set to `P1`, `P2`, `P3`, or `P4`; missing priority defaults low.
- `P1` and `P2` are high urgency and notify the owning service's on-call.
- Direct Slack contact points are possible, but require adding the Slack channel to Terraform `environments/groundcover/alerting/slack-channels.yaml`, inviting the Grafana app to the channel, and getting infra approval.
- Slack/Grafana templates use Summary as the title and Description as the message body.

Implication: for service health alerts, create or use a PagerDuty service owned by QM/Coaching and set GroundCover/Grafana `service` + `priority` labels. Use direct Slack channel contact points only for lower-urgency broadcast-style alerts where paging is not appropriate.

## Recommendation

For a dedicated Coaching/QM Slack notification channel:

1. Keep PagerDuty as source of truth for "who is on call"; `oncall-sync` already keeps `slackgroup_qm-coaching-oncall` updated.
2. For existing job failures, decide whether the team wants:
   - mentions in the existing `#job-errors-*` channel only, which is already partly in place for scorecard backfill and Aspect WFM import;
   - customer-specific routing via nonpublic `slack_channel_id`, if notifications should follow customer/team configuration;
   - or a code/config change that routes selected QM/Coaching job types to the new dedicated channel.
3. For GroundCover alerts, route through PagerDuty service labels and priority first. Add direct Slack contact points through Terraform only when the alert is intentionally Slack-only.
4. For user/admin-created job-specific alerts, the FE already supports entering the new Slack channel for Conversation Alerts and Run Virtual Agent Tests.

The most coherent product-code change, if the channel should receive all team operational job failures, is to extend backend job Slack routing from "job type -> on-call mentions" to "job type/team -> destination channel + mentions", with deployment/config values for the channel IDs instead of hardcoding them.
