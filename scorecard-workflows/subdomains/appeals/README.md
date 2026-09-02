# Appeals

## Purpose

Own appeal request and resolution workflows, their relationship to the original scorecard, and how final values and comments appear downstream.

## Semantics and Invariants

- Appeal request and appeal resolution are distinct scorecard types and permission contexts.
- The original evaluation, requested change, resolution, and final effective result must remain distinguishable.
- An approved appeal reason may be the correct exported criterion comment when it explains the final effective value.
- Appeal visibility and action permission must be evaluated separately.
- The persisted relationship chain is original scorecard <- original replica <- appeal request <- optional appeal resolve.
- The replica is the appeal-time snapshot used to decide which criteria changed; the mutable original receives the final resolved values.
- A submitted appeal request is the reporting boundary. The original and replica may still be unsubmitted, while an unsubmitted resolve is treated as absent.

## Appeals on Unsubmitted Auto-Scored Scorecards

Source: [go-servers PR #31265](https://github.com/cresta/go-servers/pull/31265) (`CONVI-7485`).

- Appeal creation already accepts an unsubmitted original. `createAppealScorecard` validates the template's appeal-request edit permission but does not require the referenced original to be submitted.
- Submitting the resolve finalizes an unsubmitted original: it copies the resolve's `submitted_at` and `submitter_user_id`, applies the resolved score and criterion values, and sets `manually_scored = true`.
- `manually_scored = true` is required even when `ai_scored_at` remains set. Auto-score backfills classify that original as manual and do not overwrite the resolved values.
- If the original was already submitted, appeal resolution updates its values without persisting submission fields. This preserves the original submitter and timestamp and avoids the known `SubmitScorecard` stale-write race (`CONVI-6076`) in the common path.
- Resolution does not set `published_at`, `publisher_user_id`, or a missing `creator_user_id` on the original.
- Appeal analytics includes the workflow once the appeal request is submitted, even while the original and replica are unsubmitted. Reviewer filters and QA-analyst grouping still use the original's submitter, so a never-submitted auto score has no QA analyst for those dimensions until finalization.

## Architecture and Source Map

- **Frontend:** appeal request/resolve scorecard experiences; `enableAppealOnAutoScorecards` controls whether the auto-score appeal action is exposed
- **Backend:** coaching appeal creation and resolve submission, related-scorecard assembly, permissions, and export behavior
- **Analytics:** `retrieveAppealWorkflows` admits submitted requests with unsubmitted originals/replicas and ignores draft resolves
- **Storage:** original and appeal scorecards/scores plus comments and relationships

## Operational Knowledge

- Trace both the original and appeal scorecards when the UI/export disagrees with the final decision.
- Confirm whether a consumer wants historical original data or the effective appealed result.
- Do not enable appeals on unsubmitted auto scores where `enableScorecardPublish` is active until publish semantics are implemented. Finalization makes the original manual but leaves it unpublished, so agent-only reads and list queries hide it.
- Customer rollout also requires the template's `scorecardAppealers` to include the agent role, Director appeal role access, and the appropriate appeal-request notification configuration.
- The API still permits states that the UI prevents: appealing a manually edited unsubmitted draft, repeated appeals, and an agent appealing another agent's scorecard by direct ID when template permissions allow it.

## Review Risks from PR #31265

- The new finalization path is not concurrency-safe against a simultaneous normal submission of the original. It decides to persist submission fields from an unlocked read; a normal submit that commits between that read and the resolve update can have its submitter and timestamp overwritten. The write should lock or conditionally update the original and re-check `submitted_at`.
- The new finalization and analytics eligibility branches have no dedicated regression coverage. Existing resolve tests use an already-submitted original and therefore exercise only the preservation path.
- Required coverage: finalizing an unsubmitted auto-scored original, preserving publish fields, preventing concurrent submission metadata loss, including submitted requests with unsubmitted original/replica, and excluding unsubmitted requests.

## Legacy Sources and Cases

- `export-appeal-comments/`
- `scorecard-template/deliverables/workflow-map.md`
- `scorecard-permission-policy/`

## Open Questions

- Define the canonical effective-value resolver for UI, exports, analytics, and notifications.
- Document partial approval and repeated-appeal behavior.
- Decide publish and agent-visibility semantics when an appeal resolve finalizes an unsubmitted original.
- Decide whether appeal finalization should populate a missing original creator or deliberately retain the auto-score provenance.
