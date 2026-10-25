# Weekly wrap-up — 2026-10-19 to 2026-10-25

- **Primary home:** `train-for-staff` system project, with output in the established `weekly-summary/` destination.
- **Source repo:** `/Users/xuanyu.wang/repos/knowledge`.
- **Branch/worktree:** `main`, main checkout; no worktree created.
- **Range:** Monday October 19 through Sunday October 25, America/Toronto. Dated implementation records end October 23; PR state and public routing were checked October 25.

## Sources and method

- Read current operating/domain models, weekly-summary skill/template, daily-capture skill, project catalog, workspace registry, and the existing `train-for-staff` project metadata and declared `staff-project.md` home. No project scaffold was needed.
- Recursively inventoried seven daily logs and 59 dated session/evidence Markdown files, including 34 direct session notes and nested evidence. Read all seven logs across Analytics, Training Simulator, and development-environment work; read relevant work items, implementation/production sessions, and PR descriptions to reconcile current outcomes.
- Used [last week's summary](../../../weekly-summary/weekly-summary-2026-10-12-to-2026-10-18.md) to avoid repeating the original contract/reporting merge as new work. Memory supplied format and evidence-handling guidance only, verified against the current workflow.
- Searched Xuanyu-authored Cresta PRs updated October 19–25; returned 27 PRs. Refreshed 13 important PRs individually, including collaborator-authored dynamic-group and Renovate PRs.
- Main local sources: [CONVI-7777](../../../analytics/work-items/CONVI-7777.md), [CONVI-7879](../../../analytics/work-items/CONVI-7879.md), [CONVI-7874](../../../analytics/work-items/CONVI-7874.md), [production verification](../../../analytics/sessions/2026-10-23/codex-convi-7777-prod-api-verification.md), [quiz percentage log](../../../training-simulator/log/2026-10-22.md), and [dependency refresh](../../../dev-environment-tips/work-items/director-web-client-ci-retry.md).

## PR status refreshed October 25

| PR | Observed state | Meaning for the summary |
| --- | --- | --- |
| [go-servers#33993](https://github.com/cresta/go-servers/pull/33993) | Merged October 21 Toronto (`2026-10-22T03:07:41Z`), final head `6e870d73b8`. | Complete Phase I counts replace last week's population-only stage and its review concern. Optional tracing remains separate. |
| [go-servers#34330](https://github.com/cresta/go-servers/pull/34330) | Merged October 22; required CI success. | Source route and chart update merged; this alone does not prove live routing. |
| [director#24019](https://github.com/cresta/director/pull/24019), [#24020](https://github.com/cresta/director/pull/24020) | Both merged October 22 Toronto; approved, no observed failing/pending checks. | Query integration and Agent/Team column are merged, superseding local draft/open snapshots. |
| [director#24065](https://github.com/cresta/director/pull/24065) | Merged October 22. | Generated-client dependency update completed; not retained as next week's work. |
| [flux-deployments#349836](https://github.com/cresta/flux-deployments/pull/349836) | Merged October 21. | Metric rollout flag registered; no live targeting inferred. |
| [director#24097](https://github.com/cresta/director/pull/24097) | Merged October 23, approved. Historical QA metadata failures remain in check rollup. | Quiz percentage change is merged; do not claim all historical checks passed or deployed verification. |
| [go-servers#34417](https://github.com/cresta/go-servers/pull/34417) | Open, non-draft, approved; required CI success; Codeowner check pending; head `72ced1c78b`. | Tracing backend is implemented and awaiting remaining merge requirements. |
| [director#24150](https://github.com/cresta/director/pull/24150) | Open, non-draft, approved; Codeowner check pending; no observed failing checks; head `031c1b4558`. | Tracing frontend has been published, superseding the work item's local-only snapshot. |
| [flux-deployments#350964](https://github.com/cresta/flux-deployments/pull/350964) | Merged October 23. | Separate details flag defaults off in every stage; Director requires both metric and details flags. |
| [cresta-proto#10413](https://github.com/cresta/cresta-proto/pull/10413) | Open, non-draft, approved; Codeowner check pending; no observed failing checks; head `fc44bb8a9a`. | New CONVI-7810 export contract discovered through PR cross-check, absent from the dated logs. Backend and Director implementation remain follow-ups. |
| [go-servers#34439](https://github.com/cresta/go-servers/pull/34439) | Open, approved; required CI success; Codeowner check pending; authored by `wtyanan27`. | Credit Xuanyu for comparison/review work, not authoring the fix. Current head differs from the comparison's reviewed commit. |
| [director#23970](https://github.com/cresta/director/pull/23970) | Renovate-authored, merged October 22; approved; no observed failing checks. | Dependency-refresh support completed rather than still awaiting CI. |

Search also confirmed merged feature-refactor/regression PRs #34031/#34032/#34034/#34035/#34037/#34038/#34040/#34041/#34043/#34044, the recording-test helper #34080, and population regression #34001. These support the main metric outcome without becoming individual progress bullets. Original extraction #33992 is closed, not reported as merged. Director #23579 merged last week despite an in-range update. Closed older draft PRs #22149/#19897 are not treated as delivered features.

## Fresh production routing check

Sent two credential-free POSTs with `{}` and `Content-Type: application/json`:

- `https://api.us-west-2-prod.cresta.ai/v1/customers/cresta/profiles/default/conversationAssistanceUsageStats:retrieve` → **HTTP 401**, code 16, `UNAUTHENTICATED`, `No valid credential is provided.`
- Same customer/profile path with `conversationStats:retrieve` → **HTTP 401**, same response.

Friday's assistance-usage probe returned 405/method-not-registered while the control returned 401. That failure no longer reproduces. The October 25 probe reaches authentication, but does not prove an authenticated request succeeds, that the customer UI is deployed/enabled, or that production counts are correct. No chart, ingress, deployment, or flag configuration was changed. The cause/timing of recovery was not investigated; the earlier ingress admission-policy hypothesis remains unconfirmed as the historical cause.

## Synthesis decisions and supporting findings

- Report the metric's backend/leaderboard code as merged, with production validation still pending. Do not carry forward last week's incomplete counts or today's stale work-item headers.
- The merged backend PR body still says scans have no timestamp bounds, while October 21 logs describe later requested-range and attribute-propagation changes as local. This summary does not claim those particular local follow-ups shipped or independently verify the final source diff. Preserve the production benchmark and inherited-correctness follow-ups.
- The [analytics issue register](../../../analytics/deliverables/convi-7777-assistance-analytics-issue-register.md) records inherited defects separately. Synthetic metadata-exclusion evidence on production infrastructure establishes behavior under that connection's settings, not measured customer-report prevalence.
- Tracing provides opt-in, complete eligible conversation/agent feature details, including no-use entries. Its PR description specifies a separate experimental default-off flag and says live backend end-to-end testing has not been done. No customer exposure claimed.
- CSV contract #10413 defines synchronous session/lesson/module exports preserving all attempts and retakes, unioned rule columns across exported revisions, blank cells for absent rules, and recorded N/A separately. Session uses manager/admin reporting roles; lesson/module retain admin reporting roles. Complete CSV generation, integration, and deployment are not implemented by this contract PR.
- Developer-environment support: diagnosed the missing model-map entry behind #23970's failure, refreshed the branch through Renovate, and now confirmed merge. Retained this evidence here rather than expanding the compact summary with a minor dependency-maintenance bullet.
- Dynamic-group contribution is the documented comparison against Oportun's earlier fix, not a fresh review of the changed latest head or a customer-validation claim.

## Limits, output, and access

- No exhaustive Slack/Linear refresh, authenticated production request, live flag evaluation, workload benchmark, or production data query in this wrap-up. Existing test results are cited as prior-session evidence; product tests were not rerun.
- [Weekly summary](../../../weekly-summary/weekly-summary-2026-10-19-to-2026-10-25.md) and [daily movement](../../log/2026-10-25.md) preserve the output and evidence. Validation passed for the exact three-section structure, local links, and whitespace in all three new files, including untracked-file checks.
- Existing `gh` self-authentication used for read-only GitHub requests. HTTP probes used no credentials. No explicit AWS, Okta, Azure, or SSH credential/profile was selected or read.
- Unrelated working-tree changes preserved. No source edits, deployments, external messages, stage, commit, push, or publication.
