# Notification Catalog

This is the initial cross-workflow inventory. “Verified” means supported by the migrated 2026 source investigation; it does not guarantee the code has not changed since that review.

| Notification | Trigger | Recipient/owner | Destination | Suppression/routing | Evidence status |
|---|---|---|---|---|---|
| Internal job failure | Job state becomes failed, timed out, or partially succeeded | Job/team mentions based on job type; creator/owner for some test jobs | Configured job-error Slack channel or customer override | Child jobs ignored; silence flags and profile/job-type rules apply | Verified from `oncall` research |
| Scorecard backfill failure | Internal backfill job enters reportable failure state | QM/Coaching on-call user group is mentioned | Generic/customer-selected job error channel | Generic job notifier rules | Verified from `oncall` research |
| Aspect WFM import failure | Internal import job enters reportable failure state | QM/Coaching on-call user group is mentioned | Generic/customer-selected job error channel | Generic job notifier rules | Verified from `oncall` research |
| GroundCover/Grafana service alert | Alert rule fires | Owning PagerDuty service/on-call by `service` and `priority` | PagerDuty by default; approved Slack contact point for Slack-only routes | Notification policy and priority | Verified operational convention |
| Flux deployment health | Flux GitRepository/Kustomization alert | Cluster/deployment operators | Cluster info/error Slack channels | Flux provider configuration | Verified from deployment config review |
| Conversation Alert | User-configured alert condition | Channel selected in job configuration | Configured Slack channel | Missing channel may use dry-run behavior | Verified source path; exact current UX needs refresh |
| Virtual-agent scheduled test alert | Scheduled test alert condition | Channel selected in test configuration | Configured Slack channel | Explicit send-alert option | Verified source path; exact current UX needs refresh |
| Scorecard submit notification | Normal scorecard submission | Runtime recipients filtered by template/state permission | Product notification delivery channel(s) | Agent suppressed when explicit publish is required; may become published notification in auto-publish flow | Verified semantics; handler/channel inventory incomplete |
| Scorecard publish notification | Scorecard publish transition | Agent only when runtime view permission allows; other recipients follow current policy | Product notification delivery channel(s) | Template revision and publish state matter | Verified semantics; handler/channel inventory incomplete |
| Direct workflow Slack notification | Workflow-specific event | Workflow-specific owner/subteam | Hardcoded or payload/config channel | Ad hoc per workflow | Verified pattern; not necessarily QM/Coaching-owned |

## Catalog Maintenance Fields

Every future row should add:

- exact trigger/state transition;
- source code and configuration path;
- recipient-selection function and inputs;
- template/payload source;
- channel and destination-selection rule;
- suppression, dedupe, and retry behavior;
- success/failure observability;
- owner and escalation path;
- tests and known incidents.

## Priority Gaps

1. Complete the QM/Coaching product notification inventory beyond submit/publish.
2. Identify email vs in-product vs Slack delivery implementations and shared infrastructure.
3. Document retry/idempotency and whether delivery attempts are persisted.
4. Distinguish user-facing notifications from operational alerts in code/config naming.
