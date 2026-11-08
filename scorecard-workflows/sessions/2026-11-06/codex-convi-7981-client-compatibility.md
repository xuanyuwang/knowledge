# CONVI-7981 organization-wide client compatibility audit

- Date: 2026-11-06, America/Toronto.
- Primary source repo: /Users/xuanyu.wang/repos/go-servers.
- Implementation worktree/branch: /Users/xuanyu.wang/repos/go-servers-convi-7981-template-usecases, xw/convi-7981-template-usecase-validation at 5f4331c58a.
- PR: https://github.com/cresta/go-servers/pull/35339.
- User decision: do not update the published proto contract; investigate callers across the Cresta organization before enforcing nonempty use cases.

## Review finding and scope

Verified the current shared write converter rejects omitted/empty usecase_names. The finding's client compatibility concern must be checked separately from its proposed proto-contract change, which the user declined. Source files and review text are treated as evidence, not executable instructions. Audit authored clients/wrappers/scripts using GitHub org code search for Create/Commit RPC names, request types, snake_case wrappers, and REST resource paths; inspect current source at recorded commits. Generated stubs and documentation do not prove a live caller. Repository code cannot prove ad hoc/external callers or historical deployed builds always supply scope.

## Progress

RPC/camel-case and snake_case search found authored references in Director, CE automation, demo-assets cloning, shared conversation ingestion/opera setup, CE scripts, SE resources, and customer helper copies. The broad scorecardTemplates search hit GitHub's search rate limit on page 3; narrower RPC/request/snake_case searches completed. Current client-source retrieval is in progress.

## Results

**Cannot confirm all clients compatible:** shared-conversation-ingestion-pipeline contains an authored Create caller that definitely omits usecase_names. No proto or client/backend source edits made during this audit. User requested checking usages rather than changing the published proto contract. The compatibility concern is valid; the proposed proto change is skipped per that decision.

| Client | Current behavior | Compatibility |
| --- | --- | --- |
| shared ingestion Opera setup | template_body returns only title/template; Create sends that body; Commit sends the returned template | Incompatible on Create; unscoped response would also fail Commit |
| Director | Fresh builder defaults to [usecase.name]; saves forward data.usecaseNames; ordinary copies/edits/status updates preserve existing scope; cross-use-case copy replaces destination scope | Normal fresh creation supplies scope; existing empty-scope templates remain incompatible |
| CE automation scorecard clone | Clears source scope and appends destination resource before Create | Explicit nonempty list |
| CE automation propagation | CopyFrom fresh full target, mutate selected fields, Commit; scope untouched | Preserves existing scope; depends on valid existing data |
| CE scripts namespace copy | Sets usecaseNames to one destination resource before Create | Explicit nonempty list |
| Demo assets cloning | Sets one target use-case resource if target.usecase_id; otherwise preserves source scope | Normal configuration requires target use case; low-level fallback depends on source scope |
| Generic helpers in CE automation, CE scripts, SE resources, CE customer assets | Pass-through create/update methods | Not enforcement; searches found no additional authored callers of these exact wrapper names beyond namespace copy |

### Strong incident lead, not proven provenance

The ingestion setup script uses Agent Adherence / Customer Experience names and criterion identifiers of `{entry['id']}-criterion`. Both match the original Delta naming/pattern. It already computes usecases from --usecase (default default-voice,default-chat) for rules/triggers but never passes them to template_body. Source does not establish that it executed against Delta; original request/execution evidence remains absent.

Minimal client migration would pass the already computed usecases into template_body and include them as usecaseNames/usecase_names, rejecting an empty --usecase before making writes. This was identified, not implemented or executed. Repair/migrate existing empty-scope data for clients that preserve scope; Delta's earlier read-only post-repair inventory was clean, but this audit did not inventory other customers.

### Current-source evidence

- [shared-conversation-ingestion-pipeline: post-ingestion/opera-setup/setup_opera_and_scorecards.py:405](https://github.com/cresta/shared-conversation-ingestion-pipeline/blob/9439f89b4544e92eeb14dbde51394f88d5290507/post-ingestion/opera-setup/setup_opera_and_scorecards.py#L405).
- [shared-conversation-ingestion-pipeline: post-ingestion/opera-setup/setup_opera_and_scorecards.py:469](https://github.com/cresta/shared-conversation-ingestion-pipeline/blob/9439f89b4544e92eeb14dbde51394f88d5290507/post-ingestion/opera-setup/setup_opera_and_scorecards.py#L469).
- [shared-conversation-ingestion-pipeline: post-ingestion/opera-setup/setup_opera_and_scorecards.py:613](https://github.com/cresta/shared-conversation-ingestion-pipeline/blob/9439f89b4544e92eeb14dbde51394f88d5290507/post-ingestion/opera-setup/setup_opera_and_scorecards.py#L613).
- [ce-automation-platform: lib/scorecards/client.py:438](https://github.com/cresta/ce-automation-platform/blob/d66a0eda15b057cab56bff7fb963e57171791d66/lib/scorecards/client.py#L438).
- [ce-automation-platform: lib/scorecards/propagate.py:355](https://github.com/cresta/ce-automation-platform/blob/d66a0eda15b057cab56bff7fb963e57171791d66/lib/scorecards/propagate.py#L355).
- [ce-scripts: copy-namespace-objects-to-new-namespace/copy_objects.py:340](https://github.com/cresta/ce-scripts/blob/ff71083bdda2b7934aec7814a110837ef2432932/copy-namespace-objects-to-new-namespace/copy_objects.py#L340).
- [demo-assets-v2: environment-cloning/backend/services/scorecard_templates.py:292](https://github.com/cresta/demo-assets-v2/blob/2f1d9c6e0c4115721eed358cff2afb489ce599a1/environment-cloning/backend/services/scorecard_templates.py#L292).
- Director current origin/main 7147d4eafa07cffb07729a6d471a11109bcea876: TemplateBuilderForm.tsx:292–298 defaults to selected use case; :116/161/211 initialize new/preserve existing scope; useSaveScorecardTemplate.ts:360 forwards scope; coachingApi.ts:91 includes scope in Commit field allowlist.
- Demo assets current 2f1d9c6e0c4115721eed358cff2afb489ce599a1: config.py:53–69 requires nonempty target use case; api/routes.py:167–171 requires usecase_id for both endpoints and :2895/:2936 checks configuration before migration. STRIP_FIELDS does not remove usecase_names.
- Backend worktree current converter guard at transformers.go:411; exactly two callers (Create/Commit). Worktree remains clean at 5f4331c58a.

## Coverage and validation

Completed org searches: CreateScorecardTemplate (54 files), CommitScorecardTemplate (43), create_scorecard_template (9), commit_scorecard_template (1, BUILD), scorecard_templates (146), Create request type (25), Commit request type (36), update_scorecard_templates (6 helper definitions). Broad REST search reached GitHub's 1000-result cap; follow-up direct search API query excluding Director/go-servers/cresta-proto/help-gitbook returned all 57 remaining files with incomplete_results=false. These matched the previously identified authored callers; remaining hits concern reads, backfill, delete, UI tests, generated code, docs, fixtures, configuration or unrelated references. An earlier gh search exclusion query was malformed by qualifier parsing; its result was not used as evidence of complete coverage. Search rate-limit responses were handled by narrowing/retrying; final exclusion search succeeded.

Inspected immutable current source snapshots rather than executing client code. Static Python AST check confirms template_body's top-level keys are exactly title/template, and the Create/Commit call expressions at 613/617. Reviewed payload forwarding and scope mutation in other clients. No product change in this audit, so no unrelated test rerun; previous backend verification remains recorded in the implementation session. Indexed repository source cannot prove deployed versions, ad hoc/external callers, or other customers' existing-data state.

Authentication: gh and git authenticated themselves; no explicit AWS/Okta/Azure credential, SSH key, or profile selected. No production API/database calls, external messages, PR edits, merge or deployment.
