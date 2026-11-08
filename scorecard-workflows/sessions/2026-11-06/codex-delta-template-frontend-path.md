# Delta empty template scope: frontend creation-path investigation

- Date: 2026-11-06, America/Toronto
- Primary source repo: /Users/xuanyu.wang/repos/director
- Context: main checkout inspected read-only through fetched origin/main `7fcbe35c8102ce2887c7f2ff1e93eeac421242fb`; checkout HEAD was behind origin/main, so owning sources were read with git show/grep against origin/main.
- Work item: [Delta target failure](../../work-items/CONVI-7981.md)
- Request: carefully determine whether frontend template creation can produce empty usecase_names, given the established assumption that a template has at least one use case.
- Related backend/data evidence: [earlier investigation](codex-delta-organization-target-not-found.md).

## Conclusion

No path was found in the inspected current frontend that turns a normally selected use case into an empty list during fresh template creation. A concrete UI path can propagate existing empty scope: ordinary Duplicate preserves the source template's empty list and calls CreateScorecardTemplate with it. Editing and status changes likewise preserve existing empty scope. Cross-use-case duplication instead sets a required selected destination.

The oldest stored Delta revision already has empty scope and human-readable criterion IDs. Current builder fresh criteria and copied criteria receive UUIDs, so this record does not match the normal current builder's fresh-create/copy output. An external API caller or import is a hypothesis, not an identified root cause. Current source is not proof of the historical deployed client or original request.

## Complete frontend call inventory

`git grep` of tracked packages found one production create call site: `useSaveScorecardTemplate.ts:379`, through `coachingApi.ts:359-375`. Commit call sites are that save hook, the template status hook, and useUpdateScorecardTemplate used by Opera cascading policy operations. No separate frontend template-import create caller was found.

### Fresh creation

1. AddScorecardTemplatePopover invokes the supplied creation callback; ScorecardTemplates.tsx:218-237 navigates to the builder without an existing template resource.
2. useCurrentUseCaseConfig throws if context/currentConfig is absent. CustomerConfigContext.tsx:154-166 supplies a resource-name fallback when config is unavailable, rather than an empty use-case list.
3. TemplateBuilderForm.tsx:290-299 passes `[usecase.name]` into calculateDefaultFormValues. Its new-template branch at lines 95-121 keeps this list unchanged. There is no isUseCasesSuperset-to-empty conversion here.
4. All-use-case selection is also nonempty: useCaseUtils.ts:38-105 builds a name ending `/usecases/*`; the form still wraps this name in a one-element array. This investigation does not claim target compatibility for wildcard scope.
5. useForm uses defaultValues and does not enable shouldUnregister. No form writer for usecaseNames or user-editable scope selector was found in the builder. The access tab displays a read-only label only when the existing list is nonempty (TemplateBuilderFormAccessStep.tsx:214-225).
6. useSaveScorecardTemplate.ts:350-379 forwards data.usecaseNames directly. CoachingApi.createScorecardTemplate spreads the object into the request; the commit pick list at coachingApi.ts:87-97 explicitly includes usecaseNames. No omission at that adapter boundary.
7. CrestaApiBaseService.wrapRequest adds headers/routing and does not rewrite this request body. Configured web-client pre-serialization and response middleware lists are empty; its serialized middleware adds client version headers (services/web-client/opts.ts:19-30).

### Copy and edit

- ScorecardTemplates.tsx:241-254 ordinary Duplicate sets copy-from and optional copy-as type but no target-use-case override.
- ScorecardTemplateBuilder.tsx:91-112 spreads the fetched template, clears identity, and preserves usecaseNames. It only overwrites scope when targetUsecase exists. calculateDefaultFormValues receives that copied object as an existing template, so the current-selection default does not apply. This can create a new template with empty scope if the source was already empty.
- DuplicateScorecardTemplateModal.tsx:119-139 requires a nonempty destination before navigation; the builder then sets `[targetUsecase]`. It does not produce an empty list through that flow.
- Editing initializes from template.usecaseNames at TemplateBuilderForm.tsx:161,211 and forwards it unchanged.
- Status commits send the whole template. Opera cascading operations clone the whole template and mutate only its JSON structure; usecaseNames is retained (usePerformanceConfigAndBackfillOperations.ts:182-264).
- transformScorecardTemplateToModel defaults a missing API field to [] (director-api coaching/transformers.ts:172). A missing scope in an API response can therefore become an empty list on edit/copy, but this is a conditional mechanism; no missing-field response from Delta was captured.
- frontendFormValidation validates template items only (TemplateBuilderForm.tsx:67-74); it does not reject an empty scope or restore one.

## Local behavioral verification

Used Node with TypeScript to extract the actual calculateDefaultFormValues and submittedEditorsToSelection functions from origin/main. Supplied unrelated enum/permission constants locally and used the installed react-hook-form 7.90.0 createFormControl/handleSubmit. Template fixtures had empty item lists so criterion-conversion code was not mocked/exercised. Registered title only, leaving usecaseNames unregistered, to check the hidden-field concern.

All six checks passed:

| Input | Submitted usecaseNames |
| --- | --- |
| Fresh template, selected synthetic-care | [selected resource name] |
| Fresh template, All use-cases | [resource name ending /*] |
| Edit template with scope | Existing nonempty list |
| Edit template with empty scope | [] |
| Ordinary-copy-shaped template with empty scope | [] |
| Cross-use-case-copy-shaped template with explicit target | [target resource name] |

This was a local source-function/form-control check, not a mounted-browser or network replay. The copy fixtures mirror the inspected copy transformation; the full copy component was not rendered. No API writes or product code changes.

The latest origin/main ancestor with commit date before Delta's earliest stored revision was `a9206df01664713bcdef0be79095a12f6aa0017e` (2026-11-05 02:42:12Z). Diffing its relevant builder, context, use-case utility, API adapter/transformer, template-list, and update-hook paths against inspected origin/main produced no changes. This reduces source-timing uncertainty but does not establish which build was deployed or used.

## Additional read-only production metadata

Used the customer app DB skill and cresta-cli self-authentication with the same config/account/cluster/database as the earlier investigation. No AWS credential/profile or SSH key explicitly selected; no secrets printed/persisted.

```sql
SELECT revision, created_at, creator_user_id, type,
       template->>'version' AS structure_version,
       jsonb_path_query_array(template, '$.**.identifier') AS identifiers
FROM director.scorecard_templates
WHERE customer = 'delta-air-lines' AND profile = 'us-west-2'
  AND resource_id = '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a'
ORDER BY created_at;
```

- Oldest stored revision 2496e40f: 2026-11-05 04:34:06.216123 UTC, creator_user_id e2f03fc79f3250aa.
- Next a95546ec: 04:34:06.536737 UTC, same creator, roughly 321 ms later.
- Latest 7508e489: 20:34:22.611306 UTC, creator_user_id 1722a4291580a19f.
- All are type 1, structure version 2. All retain chapter identifier agent-adherence-chapter and descriptive criterion identifiers including both reported criteria.
- The recursive lax JSONPath repeats some returned values; this is not evidence of duplicate stored identifiers.
- TemplateBuilderFormConfigurationStep.tsx:282-295 generates new criterion identifiers with uuidv7. ScorecardTemplateBuilder.tsx:208-219 regenerates copied top-level/chapter criterion identifiers with uuid7. Delta's descriptive IDs differ from these normal current creation paths.

## Remaining evidence needed

Identify the earliest revision's actual creation request/client (creator above, timestamp above), or ask its author which tool/workflow created the template. A script/import/nonstandard API client is plausible, but not established. Do not attribute the problem to ordinary frontend fresh creation, normal copying from a populated template, config-loading race, hidden use-case field, or API adapter field loss without new evidence. No logs/HTTP payload or historical browser build was captured in this follow-up.

## Live Chrome comparison: reported zero versus use-case field

User proposed that Delta's use-case setup could explain a missing field and a literal `0` on Scorecard access. Inspected the two existing user Chrome tabs through CUA, preserving their forms:

| Page | Actual state observed | Use-case area |
| --- | --- | --- |
| Delta user tab, template/01a1a9fc-4b07-70c7-ae85-ae2ded1a923a@7508e489 | Editing POC Template, selected voice-care | `0` between Target agents and Template status |
| Walter Dev staging user tab, template without ID | Creating New Template | Use case field shows Walter Dev |
| Separate temporary Delta tab, template without ID, voice-care | Creating New Template | Use case field shows voice-care |
| Same temporary Delta tab after selecting synthetic-care and reopening template without ID | Creating New Template | Use case field shows synthetic-care |

The initial comparison was existing-template editing versus new-template creation, not two new-template forms. Delta's live global selector listed All use-cases, synthetic-care, and voice-care. A fresh form displayed the right use case under both actual use cases. No Save was clicked, no criteria or template fields were edited, and no template was created. The temporary tab's selected use case was restored to voice-care and the temporary tab was closed. Existing user tabs were left on their original forms. Authentication used existing browser sessions; no credentials were retrieved or selected.

The apparent dropdown is a read-only text field in this component. DOM inspection of the existing Walter field and the fresh Delta field returned a readonly attribute. Live screenshots of the temporary fresh Delta voice-care and synthetic-care forms were emitted through CUA.

### Exact meaning of the zero

TemplateBuilderFormAccessStep.tsx:214 renders:

```tsx
{!hasSingleUsecase && form.getValues('usecaseNames').length && (
  <DirectorFormTextInput value={usecaseNames} readOnly /* other props */ />
)}
```

For a multi-use-case customer with an empty template scope, this expression evaluates to numeric 0, which React renders as text. The display-name map at lines 124-128 is evaluated separately. If a nonempty resource-name list failed display-name resolution, the condition would remain truthy and the input would render with a blank value; it would not produce this numeric 0. This observation therefore identifies empty form scope, not a missing use-case display-name mapping.

Four checks using installed React/react-dom server rendering and the exact condition/mapping shape passed: empty scope -> `<div>0</div>`; unknown display mapping with a nonempty list -> blank readonly input; known mapping -> populated readonly input; hasSingleUsecase true -> no field/zero.

Conclusion: the visible zero is a rendering defect triggered by the existing template's empty scope. The current Delta configuration supplies both use cases successfully to fresh forms. This narrows the present symptom to template-specific data; it does not prove the use-case setup was correct at the historical creation time or identify the original creation client. No product fix or customer-data mutation was performed.
