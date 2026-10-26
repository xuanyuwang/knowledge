# Legacy Source Index

The Notifications domain synthesizes cross-workflow behavior while preserving each source project's own responsibility.

| Source | Contribution | Canonical ownership after migration |
|---|---|---|
| `oncall/sessions/2026-06-29/codex-team-slack-notification-research.md` | Internal-job Slack routing, on-call sync, frontend job channels, Flux, GroundCover/Grafana conventions | Notifications owns routing synthesis; `oncall` retains the original incident/operations research |
| `scorecard-workflows/sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/investigation.md` | Current notification visibility and submit/publish behavior | `scorecard-workflows` owns permission semantics; Notifications owns recipient consumption/delivery view |
| `scorecard-workflows/sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/permission-history.md` | Evolution of `scorecard_viewers`, template revision behavior, submit/publish notification gates | Same shared boundary as above |
| [group-calibration-csv-comment-parity](../../scorecard-workflows/work-items/group-calibration-csv-comment-parity.md) | Reviewed for notification evidence | No notification-specific artifact found; catalog gap recorded |
| [appeal-resolution-comments-export](../../scorecard-workflows/work-items/appeal-resolution-comments-export.md) | Reviewed for notification evidence | No notification-specific artifact found; catalog gap recorded |

## Migration Rule

Do not move entire cross-domain source projects into Notifications. Promote only stable trigger, recipient, routing, and delivery knowledge, then link back to the source domain for business semantics or incident evidence.
