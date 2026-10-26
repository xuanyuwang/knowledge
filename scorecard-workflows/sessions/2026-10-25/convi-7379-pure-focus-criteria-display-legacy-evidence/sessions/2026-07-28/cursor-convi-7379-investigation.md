# CONVI-7379 Investigation

## Ticket

[CONVI-7379](https://linear.app/cresta/issue/CONVI-7379/pure-coaching-plan-11-sessions-shows-raw-focus-criteria-ids-instead-of)

Pure Coaching Plan 1:1 Sessions shows raw focus criteria IDs instead of criterion names.

## Repro Case

- Namespace: `pure-us-east-1`, use case `care-voice`
- Agent: Jeff Sykes (`a53848957ec81e6b`)
- Coaching plan: `019952c4-501b-7485-8fdc-becca2fadda0`
- Session on 2026-07-24: `019f9542-63d0-708f-827f-06cb00e926b2`

## Frontend Flow

1. Coaching plan page loads sessions via `listCoachingSessions` and templates via `useGetScorecardTemplatesFilteredByPermissions`.
2. `SessionNotes.tsx` builds `criterionById` from **current** scorecard templates only.
3. `focusCriteriaOptions` only includes coaching-plan focus criteria whose `criterionId` exists in `criterionById`.
4. `FocusCriteriaSelect` uses Mantine `MultiSelect` with option values like `{criterionId} ({scorecardTemplateName})`.
5. When selected session criteria are missing from `focusCriteriaOptions`, Mantine renders the raw value string.

Relevant files:

- `director/packages/director-app/src/features/coaching-workflow/agent-coaching/agent-coaching-plan/session-notes/SessionNotes.tsx`
- `director/packages/director-app/src/features/coaching-workflow/agent-coaching/agent-coaching-plan/session-notes/FocusCriteriaSelect.tsx`
- `director/packages/director-app/src/features/coaching-workflow/agent-coaching/agent-coaching-plan/session-notes/utils.ts`

## Backend Storage

Focus criteria are stored in Postgres as `template_id/criterion_id` arrays:

- `director.coaching_plans.focus_criteria_ids`
- `director.coaching_sessions.focus_criteria_ids`

API conversion in `go-servers/apiserver/internal/coaching/transformers.go` rebuilds `scorecardTemplateName` and `criterionId` but does **not** populate `criterionDisplayName`, even though the proto supports it.

## DB Findings (Pure prod)

### Coaching plan

```sql
resource_id: 019952c4-501b-7485-8fdc-becca2fadda0
agent_user_id: a53848957ec81e6b  -- Jeff Sykes
created_at: 2025-09-16
focus_criteria_ids:
  - 0196dac5-6c3b-717e-bc9a-d95bb54c51b2/01974609-6c7d-744b-af34-b169f88b0b24
  - 0196dac5-6c3b-717e-bc9a-d95bb54c51b2/01974608-ce09-7798-94ca-7e31f0c8901d
```

### Stale criterion mapping

| Criterion ID | Historical display name | Present in latest template (`c61e5f34`)? |
|---|---|---|
| `01974609-6c7d-744b-af34-b169f88b0b24` | Verify Name | No |
| `01974608-ce09-7798-94ca-7e31f0c8901d` | Verify Member ID or Policy Number | No |
| `019b6b87-a72d-73a8-94aa-80601d4b3849` | Name of the Insured | Yes |

Criteria were removed when template revision `c24c43af` landed on 2025-12-19. Latest template (`MS Security`) now has different criteria entirely.

### Scope in Pure

3 active coaching plans still reference the removed criterion IDs.

## Why Sunbit Works

Sunbit is a control because its coaching plans still reference criterion IDs that exist in the current scorecard templates. This is customer-specific stale data plus a frontend lookup gap, not a universal coaching-session API regression.

## Fix Options

1. **Backend**: Populate `criterionDisplayName` on focus criteria by resolving criterion IDs across all revisions of the referenced template.
2. **Frontend**: Fall back to `criterionDisplayName` from API and/or historical template revision lookup; never show raw resource paths.
3. **Data migration (Pure-only mitigation)**: Update affected coaching plans/sessions to current criterion IDs after mapping old -> new semantics with customer input.

Recommended product fix: backend enrichment + frontend fallback. Pure data migration is optional short-term relief but risky after major template restructures.
