# CONVI-7663 frontend trigger and request scope

## Context

- Date: 2026-09-10 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/director`
- Source context: Director main checkout, read-only code tracing
- Question: identify the frontend scenario that triggered the oversized request and quantify the templates/use cases fetched.

## Frontend path

The oversized request is owned by the shared `useGetScorecardTemplatePermissions` hook, not by one page-specific data loader. The hook calls `useCurrentScorecardTemplates` with:

- active and inactive templates;
- archived templates excluded;
- `usecaseNames = []`, which deliberately bypasses the current-use-case fallback;
- no explicit `view`, which selects the default full-body response.

On a conversation page, both `ScorecardFormLoader` and `useConversationCoachingDetails` mount this permission hook. The returned checker filters visible templates/scorecards and controls edit, grade, appeal, and publish behavior. React Query uses the same request key, so these consumers share/deduplicate the same all-usecase permission request.

## Observed product scenarios

Read-only frontend-error telemetry for the 2026-09-09 15:20-18:00 UTC incident window attributed the 79,860,161-byte failure to multiple pages because the permission hook is shared. The largest named page groups were:

- Closed Conversations: 749 failures;
- Live Conversations: 38;
- Coaching Hub: 25;
- Leaderboard: 15;
- Opera: 12;
- Performance Insights: 12.

One repeated frontend session was specifically on a Closed Conversations conversation-detail URL. The supported primary scenario is therefore opening/viewing a conversation and loading its scorecard/coaching details; Performance Insights and Opera were real but small secondary trigger surfaces.

## Exact request scope

A read-only as-of query against RCG `us-east-1`, capped at the timestamp of the original measurement, reproduced the permission-hook selection:

- 3,330 distinct template resource IDs fetched;
- 98 distinct individual use cases;
- 98 distinct `usecase_ids` sets;
- 3,330 total template-to-use-case assignments;
- every selected template row had exactly one use case;
- zero selected templates had no use case;
- 294 distinct titles.

The 3,330 records are therefore not 3,330 templates shared across 98-use-case arrays. They are 3,330 distinct template resources, each attached to one of 98 use cases, with many repeated human-readable titles.

## Distribution by use case

The incident-time distribution is flat rather than dominated by one use case:

| Rank | Use case | Templates | Share of templates | Stored JSON bytes | Share of stored JSON |
| --- | --- | ---: | ---: | ---: | ---: |
| 1 | `rcg-default` | 126 | 3.78% | 2,788,297 | 3.87% |
| 2 | `emergency-travel` | 84 | 2.52% | 1,889,028 | 2.62% |
| 3 | `air2sea` | 83 | 2.49% | 1,821,657 | 2.53% |
| 4 | `access` | 80 | 2.40% | 1,794,623 | 2.49% |
| 5 | `access-corr` | 78 | 2.34% | 1,762,743 | 2.45% |

- Top five use cases: 13.54% of templates and 13.97% of stored JSON.
- Top ten: 22.22% of templates and 22.47% of stored JSON.
- Average: 33.98 templates per exact use-case ID; median: 41.5; minimum: 1.
- The 98 exact IDs collapse to 95 under case normalization. Three case-only pairs exist; the largest merged pair has 119 templates, still below `rcg-default` at 126. Case normalization therefore does not change the conclusion.

Adding the current conversation's use-case name to the permission request would reduce the observed worst-case selection from 3,330 templates to 126 for `rcg-default` (a 96.22% row reduction). Its stored JSON portion was about 2.79 MB rather than 71.98 MB. This is strong evidence for the narrow conversation-page patch, although the shared hook still needs a defined behavior on pages that intentionally operate across multiple or all use cases.

## Feasibility and regression assessment

### Conclusion

Use-case scoping is feasible and low risk when applied at each consumer using the same effective use-case set as the scorecard/template data being filtered. Changing `useGetScorecardTemplatePermissions` itself from an explicit all-usecase default to an implicit selected-usecase default is not safe: the all-usecase behavior was introduced in March 2025 for the notification-preferences selector, and several administrative workflows legitimately span all use cases.

The recommended quick patch is therefore call-site alignment, not a global default change.

### Why scoped permission lookup preserves behavior

- `useGetScorecardTemplatePermissions` only evaluates template-local role arrays. It does not combine permissions across use cases.
- Callers pass a template resource name to the checker; the permission decision is needed only for templates already present in that caller's scorecard/template result.
- The backend use-case predicate is overlap-based, so templates assigned to multiple use cases remain included when any requested use case matches.
- The backend also includes templates whose `usecase_ids` is null or empty, so global templates are not lost by adding a use-case filter.
- RCG has no ambiguous current template identity: all 3,330 selected resources have exactly one current use-case set, and no resource ID appears in multiple sets.
- RCG also has no historical use-case reassignment among those current resources as of the incident snapshot, so conversation scoping does not make an existing RCG scorecard lose its current permission record.

### Consumer classification

| Consumer | Data being permission-filtered | Safe scope |
| --- | --- | --- |
| `ScorecardFormLoader` | One conversation and its templates/scorecards | The conversation use case already passed to `useConversationCoachingDetails` |
| `useConversationCoachingDetails` | Scorecards and current templates for one conversation | `options.usecaseNames`, already used by both underlying list calls |
| `ScorecardForm` | One selected template | `scorecardTemplate.usecaseNames` |
| Scorecard `ResetButton` | One selected template | `selectedScorecardTemplate.usecaseNames` |
| Agent coaching `useScorecards` | Scorecards queried with selected use cases | Existing `useUsecaseNamesForFilter()` result |
| `useGetScorecardTemplatesFilteredByPermissions` | Template list queried for a use-case set | Resolve the effective selected use cases once and pass the same set to both list and permission hooks |
| `ScorecardTemplatesFilter` | Template list with explicit `usecaseNames` | Already aligned |
| AI Agent `PerformanceTemplatePicker` | Templates queried for `useCaseName` | Pass that same use case through the picker/operation hook |
| Opera attachment/backfill | Templates queried for selected policy use cases | Pass the same selected/original use-case union through the permission consumers |
| Scorecard-template level selector | Selected-usecase or explicit all-usecase list | Pass its computed use-case set; retain all-usecase behavior when explicitly requested |
| Admin scorecard-template list | Selected use case or all-usecase superset | Scope only in selected-usecase mode; retain all-usecase mode |
| Notification-preference selectors | Intentionally grouped across all use cases | Retain explicit all-usecase behavior |

### Bugs a naive patch could introduce

1. **Global-default regression:** replacing `usecaseNames = []` with `undefined` would silently make all-usecase admin/notification selectors depend on the current header use case and hide valid templates.
2. **Mismatched list and permission scopes:** scoping only one of the two queries can make `buildTemplatePermissionChecker` return `NO_PERMISSIONS` for templates present in the list, hiding or disabling them after loading.
3. **Historical reassignment:** outside RCG, a scorecard can theoretically reference a template resource whose latest lineage moved to another use case. Since a missing template becomes `NO_PERMISSIONS`, conversation behavior must be tested with a historical scorecard and a reassigned template. A full RCG scorecard-to-template scan exceeded the bounded read-only query timeout, so this risk is ruled out for RCG template lineage but not globally.
4. **Multi-usecase and all-usecase modes:** callers must pass all effective use cases, not arbitrarily choose the first. The backend overlap predicate handles multi-usecase templates correctly.
5. **Existing selector expression:** `useScorecardTemplatesLevelSelect` computes `usecases` with an ambiguous/incorrect conditional expression that resolves to all available use cases whenever explicit `usecaseNames` is truthy. That should be corrected and covered while aligning its permission scope.

### Required validation

- Unit-test that each scoped consumer sends the same use-case names to the permission hook and its data-list request.
- Test one-usecase, multiple-usecase, global-template (null/empty assignment), and explicit all-usecase selector modes.
- Test a historical scorecard whose template is inactive and one whose template's latest lineage has a different use-case assignment; define the expected permission source before release.
- Test that a missing scoped template does not cause a visible template/scorecard to disappear because the list and permission scopes diverged.
- Add a request-shape regression test proving a conversation page no longer emits the empty/all-usecase permission request.
- Validate the RCG conversation flow with the serialized response comfortably below the transport limit, not only by counting returned records.

## Evidence boundary

- Frontend telemetry identifies the page active when each client error was emitted; it does not expose a JavaScript component stack beyond the shared hook's error site.
- Source tracing establishes the conversation-page consumers and why they request the all-usecase full view.
- The production query used a read-only DB identity. AWS profiles `us-east-1-prod_ro` and `us-east-1-prod_dev` were individually inspected and cleared; the read-only AWS profile's generated DB identity failed PAM authentication, while the standard profile successfully obtained the helper-enforced read-only DB identity.
