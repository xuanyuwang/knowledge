# CONVI-6862 Local FE Test Plan

## Summary

Use this checklist to validate the current frontend behavior in `/Users/xuanyu.wang/repos/director-convi-6862`.

This plan assumes:

- backend is already merged
- local validation is FE-only
- the current FE source of truth is branch `xwang/convi-6862-submitted-scorecard-editors-v2`

## Prerequisites

1. Confirm you are on the feature branch:

```bash
git status --short --branch
```

2. Ensure dependencies exist. If `node_modules` is missing or stale:

```bash
yarn install
```

3. Prepare local E2E auth if needed:

```bash
yarn workspace @cresta/director-testkit auth
```

4. Note that Playwright local smoke tests will auto-start the app through the testkit config.

## Fast Static Checks

Run these first:

```bash
yarn workspace @cresta/director-app tsc
```

```bash
yarn biome check \
  packages/director-app/src/features/admin/coaching/template-builder/TemplateBuilderForm.tsx \
  packages/director-app/src/features/admin/coaching/template-builder/useSaveScorecardTemplate.ts \
  packages/director-app/src/features/admin/coaching/template-builder/steps/access/TemplateBuilderFormAccessStep.tsx \
  packages/director-app/src/features/admin/coaching/template-builder/steps/access/template-builder-advanced/TemplateBuilderAdvanced.tsx \
  packages/director-app/src/features/admin/coaching/template-builder/steps/access/PermissionInputWithLabel.tsx \
  packages/director-app/src/components/filters/user-team-group/UserTeamGroupSelect.tsx \
  packages/director-testkit/src/pages/admin/performance-config/views/ScorecardAccessProcessTemplateTabView.ts \
  packages/director-testkit/src/tests/admin/performance-template/process/perf-config-edit-process-template-smoke.spec.ts
```

Optional heavier check:

```bash
yarn workspace @cresta/director-testkit lint
```

## Targeted E2E Smoke Checks

Run the same high-signal specs that covered this feature in CI:

```bash
yarn workspace @cresta/director-testkit test:e2e src/tests/admin/performance-template/process/perf-config-edit-process-template-smoke.spec.ts
```

```bash
yarn workspace @cresta/director-testkit test:e2e src/tests/admin/performance-template/conversation/perf-config-edit-conv-template-ui.spec.ts
```

```bash
yarn workspace @cresta/director-testkit test:e2e src/tests/admin/performance-config-ui.spec.ts
```

If you want one command for all three:

```bash
yarn workspace @cresta/director-testkit test:e2e \
  src/tests/admin/performance-template/process/perf-config-edit-process-template-smoke.spec.ts \
  src/tests/admin/performance-template/conversation/perf-config-edit-conv-template-ui.spec.ts \
  src/tests/admin/performance-config-ui.spec.ts
```

## Detailed Manual Validation

### A. Process template: create a new template and inspect the access UI

1. Start the app if you want to test manually outside Playwright:

```bash
yarn develop:client
```

2. Open Director locally at `http://localhost:3100/director/`.
3. Navigate to `Admin > Performance Config`.
4. Click `Add New`.
5. Choose `Process Template`.
6. Wait for the template builder to open.
7. Verify the default title is `New Template`.
8. Click the `Scorecard access` tab.
9. Scroll to the bottom of the access page.
10. Verify there is an `Advanced` section.
11. Under `Advanced`, verify there is a row labeled `Scorecard editors`.
12. Verify the helper text is:
    - `Who can edit this scorecard after submission (e.g. change the scorecard evaluation)`
13. Verify the selector is visible.
14. Without selecting anything, verify the placeholder reads `All users`.

### B. Process template: verify selector contents

1. Open the `Scorecard editors` selector.
2. Verify the picker includes:
   - users
   - teams
   - groups
3. Verify it is not restricted to users only.
4. Verify it is not empty just because `Who can use this scorecard` is unchanged.
5. Close and reopen the selector to confirm it remains stable.

### C. Process template: verify it is disconnected from `Who can use this scorecard`

1. In `Who can use this scorecard`, note the initial selected roles.
2. Open `Scorecard editors`.
3. Select at least:
   - one user
   - one team
   - one group
4. Close the selector and confirm the selected chips or summary appear on the tile.
5. Change `Who can use this scorecard` by adding or removing a role.
6. Verify the `Scorecard editors` tile still retains the previously selected user/team/group values.
7. Reopen the selector and confirm the same selections are still checked.
8. Verify the available users/teams/groups are not obviously reduced because of the grader-role change.

### D. Process template: verify save + reload round-trip

1. With the selected user/team/group still present, click `Save`.
2. Return to the Performance Config list.
3. Reopen the same template.
4. Go back to `Scorecard access`.
5. Scroll to `Advanced`.
6. Verify the `Scorecard editors` tile still shows the saved selection summary.
7. Open the selector and confirm the same user/team/group selections are preserved.
8. Change the selection if possible:
   - remove one item
   - add one different item
9. Save again.
10. Reopen again and confirm the updated selection persisted.

### E. Process template: verify Cresta-only disables the control

1. In the same template, locate `Who can edit this template`.
2. Switch it to the Cresta-only option.
3. Verify the `Scorecard editors` selector becomes disabled.
4. Verify you cannot open or change the picker while disabled.
5. Switch `Who can edit this template` back to the customer-visible option.
6. Verify the `Scorecard editors` selector becomes enabled again.

### F. Conversation template: verify Advanced placement still works with publish controls

1. Go back to `Admin > Performance Config`.
2. Open an existing conversation template, or create one if needed.
3. Navigate to `Scorecard access`.
4. Scroll to `Advanced`.
5. Verify both are present:
   - `Scorecard editors`
   - `Require publish`, if the publish feature is enabled in your environment
6. Verify `Scorecard editors` appears in the intended Advanced layout with the publish controls.
7. Verify the `Scorecard editors` placeholder is also `All users`.
8. Select a user/team/group, save, and reopen to confirm the same round-trip behavior on conversation templates.

### G. Regression checks

1. Verify `Target agents` still works and still shows users/teams/groups as before.
2. Verify `Who can use this scorecard` still opens and shows its expected role list.
3. Verify `Who can view this scorecard` still opens and shows its expected role list.
4. On conversation templates, verify publish controls still behave as before.
5. On process templates, verify the access page still renders correctly even if publish controls are absent.

## Pass Criteria

The feature is locally validated if all of the following are true:

- docs consistently describe the merged `submitted_scorecard_editors` contract
- no active doc still claims the FE selector is user-only or grader-filtered
- submitted editors can be configured with users, teams, and groups
- changing `Who can use this scorecard` does not clear submitted-editor selections
- `Scorecard editors` appears in `Advanced` for both process and conversation templates
- empty-state text is `All users`
- saved user/team/group selections persist after reopening the template
- targeted smoke E2E specs pass locally

## Known Local Caveats

- local E2E requires auth bootstrap via `yarn workspace @cresta/director-testkit auth`
- Playwright local smoke will auto-start the app using the testkit config
- if `yarn workspace @cresta/director-app tsc` surfaces unrelated workspace issues, use the targeted biome check plus the targeted smoke specs as the higher-signal validation for this feature
- this plan is FE-only because the BE PR is already merged
