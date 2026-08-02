# Glean Weekly Oncall Handoff Investigation

Date: 2026-07-31
Tool: Codex
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch: `main`

## Objective

Determine how to use Glean to automatically summarize a QM/Coaching oncall rotation and hand it to the incoming oncaller each week.

## Sources reviewed

- Official Glean documentation for scheduled triggers, publishing agents to Slack, Slack connector setup, and channel response configuration.
- Internal Glean support guidance for a weekly scheduled Slack-channel summarizer.
- `cresta/glean-daily-summary-support-agent` README, Python runner, agent-ID helper, and GitHub Actions workflow.
- Existing Cresta oncall handoff documents and internal search results.
- QM/Coaching rotation and Slack-group changes in `cresta/terraform#12304` and `cresta/go-servers#28553`.

## Findings

### Native scheduling is the preferred path

Glean scheduled triggers are available and support daily or weekly recurrence. They are disabled by default at the tenant level and can be enabled for everyone, selected users, or supported IdP groups.

Auto mode configures its schedule directly in Agent Builder. Workflow mode uses an Input form trigger with scheduling enabled, then requires the user to set and activate the schedule after publishing. Glean recommends Auto mode by default, but Workflow mode is a better fit for this handoff because the source sequence and output contract should remain deterministic.

### Slack delivery needs separate permissions

A scheduled agent should produce a background outcome such as a Slack message. Any write tool used during a scheduled run must be allowed to run without user confirmation. The Slack connector and Slack Real Time Search must be healthy, and the Glean app may need to be present in the destination channel.

Private-channel search is permission-aware and requires the scheduled owner to opt into private Slack/DM access. That is separate from adding the Glean app to a private Slack channel. Scheduled output must not be posted into a destination broader than the retrieved source material.

### The schedule is per user

Users activate their own scheduled agents, and scheduled execution uses the subscriber's access. This creates ownership and offboarding risk; the team should record a stable owner and backup maintainer.

### Current QM/Coaching timing

The QM/Coaching weekly rotation was changed from Tuesday to Monday at 9:00 AM Pacific. The deliverable therefore proposes Monday at 8:30 AM Pacific as a reviewable draft time, subject to confirmation of the actual handoff meeting.

### API fallback exists but the example is stale

The internal `glean-daily-summary-support-agent` is a useful fallback pattern: GitHub Actions invokes a Glean Agent and posts its output to Slack. The current implementation uses `/rest/api/v1/agents/runs/stream`, not the README's older `/runs/wait` description, because streaming avoids idle timeouts. The checked-in workflow currently has its cron schedule commented out, despite the README describing an active weekday schedule.

## Conclusion

Build a native Glean Workflow mode agent, activate a weekly schedule, and send one evidence-linked draft to a restricted handoff channel. Pilot for two rotations with human review. Use GitHub Actions plus the Agents API only if admins cannot enable native scheduling or the Slack action for background execution.

## Artifact

- `oncall/deliverables/glean-weekly-oncall-handoff-automation.md`
