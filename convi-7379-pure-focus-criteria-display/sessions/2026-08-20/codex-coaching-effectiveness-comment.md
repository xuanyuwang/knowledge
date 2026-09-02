# CONVI-7379 coaching-effectiveness comment

## Context

- Ticket: CONVI-7379
- Linear comment: `7a1367fa-d41d-4b04-87b1-b40d291a9152`
- Source repo: `/Users/xuanyu.wang/repos/go-servers-convi-7379`
- Branch: `convi-7379-focus-criteria-display-names-be`
- Mode: read-only investigation; no Linear reply or product-code change

## Question

When a focus criterion is removed, do historical 1:1 sessions still include it in coaching-effectiveness calculations, or are their values recalculated?

## Finding

The behavior depends on where the criterion is removed:

- Historical coaching sessions retain their own `focus_criteria_ids`. Removing a criterion only from the current coaching plan does not rewrite historical sessions. If the criterion still exists in the current scorecard template, on-read effectiveness calculation continues to use it for those historical sessions.
- Effectiveness is calculated on read from the session's stored focus criteria and the agent's before/after performance windows; it is not a frozen value stored on the session.
- The calculation currently classifies and admits criteria using the current scorecard-template definition. If a historical session's criterion has also been removed from the current template, it is absent from the current criterion metadata and is excluded by the performance-category filter. The resulting effectiveness can therefore change because it is recomputed without that criterion.
- CONVI-7379 does not change these calculation semantics. Its accepted scope preserves stored identifiers and recovers historical display names on coaching plan/session reads. The frontend also continues hiding removed criteria from the coaching-plan trends surface while showing historical/deactivated labels in session-note surfaces.

## Evidence

- `insights-server/internal/analyticsimpl/retrieve_coaching_efficiency_stats.go` reads `director.coaching_sessions.focus_criteria_ids`, joins each stored criterion, fetches current scorecard templates, filters performance criteria that lack current category metadata, and computes effectiveness from before/after score windows.
- `shared/coaching/session.go` requests performance-category coaching efficiency grouped by coaching session.
- The CONVI-7379 work item and decision record explicitly constrain the fix to read-time label recovery and preserve persistence, filtering, and calculation behavior.

## Suggested answer

Historical sessions keep the focus criteria that were saved on the session, so removing a criterion from the current coaching plan alone does not rewrite those sessions. Coaching effectiveness is calculated on read rather than stored as a frozen value. If the criterion still exists in the current scorecard template, it remains part of the historical session's calculation. If it was also removed/deactivated from the scorecard template—as in this ticket—the current calculation excludes it, so the displayed effectiveness is recomputed without it. CONVI-7379 only fixes the historical label display and does not change that calculation behavior.

## Origin/main frontend styling follow-up

Inspected Director `origin/main` at `18370ed8fd7da72d706ab22f0f37e966bb9ba4ca`.

- The coaching-plan criterion card has a muted state only when the entire scorecard template is deactivated: `templateDeactivated` applies the `deactivated` CSS class, which changes the criterion title and receptivity to `--content-disabled`, replaces adherence with `--`, hides target/delta, and displays a `Deactivated` notice with a tooltip.
- There is no strike-through style for coaching criteria in the Coaching Hub or coaching-plan/session code paths.
- A criterion removed from the current template is not given that muted template state. The coaching-plan focus-criteria selector filters criteria missing from the current criterion display-name map, so they are hidden instead of rendered muted.
- Coaching-session pills use the normal `--content-primary` color and `--background-section` background; `origin/main` has no removed/deactivated criterion-specific pill styling.
