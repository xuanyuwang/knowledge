# Notifications Operational Playbook

## Classify the Symptom

- **Not triggered:** the business/job state transition did not call notification logic.
- **Suppressed:** a child-job, silence, publish, permission, test, or workflow rule intentionally prevented delivery.
- **Wrong recipients:** identity, role, template revision, state, or on-call mapping is wrong.
- **Wrong destination:** fallback/customer/team/payload channel routing selected the wrong place.
- **Rendering failure:** payload/template/link construction failed or produced misleading content.
- **Delivery failure:** provider/API call failed, timed out, or lacked credentials/channel membership.
- **Duplicate/noisy:** retries or repeated state transitions are not deduplicated.
- **Invisible failure:** delivery failed without durable logs, metrics, or escalation.

## Internal Job Slack Checklist

1. Confirm the job state is reportable: failed, timed out, or partially succeeded.
2. Confirm it is not a nested child job.
3. Check job-type/profile silence settings and test/unstable behavior.
4. Determine destination precedence:
   - customer-config override when enabled;
   - environment fallback job-error channel.
5. Check job-type→team mention mapping separately from destination.
6. Verify bot credentials and channel membership without copying secrets into notes.
7. Check state-change publication logs and Slack API response/error evidence.

## On-Call Routing Checklist

1. Confirm PagerDuty escalation-policy ownership.
2. Confirm `oncall-sync` recently updated the Slack user group.
3. Distinguish “correct people mentioned” from “correct channel selected.”
4. Avoid static person lists as a workaround.

## GroundCover/Grafana Checklist

1. Confirm alert rule query and firing state.
2. Verify `service` and `priority` labels.
3. Confirm the expected notification policy/contact point.
4. Use PagerDuty for paging urgency; use direct Slack only when intentionally non-page.
5. Confirm any direct Slack channel is infrastructure-configured and the app is invited.

## Scorecard Product Notification Checklist

1. Identify scorecard type and transition: submit vs publish vs appeal/calibration.
2. Load the scorecard's template revision, not only the latest template.
3. Evaluate static viewer/publisher roles and runtime visibility separately.
4. Confirm whether submit should suppress agent delivery until publish.
5. Verify recipient identity and message channel/template.
6. Check whether retries can duplicate delivery.

## Evidence to Capture

- exact trigger/job/scorecard ID and timestamp;
- expected and actual recipients/destination;
- relevant state and configuration without secrets;
- suppression/routing branch selected;
- provider response or error;
- retry/dedupe behavior;
- mitigation and permanent fix;
- how recurrence will be detected.

## Known Operational Gaps

- No complete retry/idempotency model is documented across product notification channels.
- No single inventory currently proves all QM/Coaching triggers are covered.
- Generic job errors can mention the correct team while landing in a non-team-specific channel.
- Direct workflow routes can bypass shared routing conventions.
