# Assistance Usage API production verification

- Date: 2026-10-23 (America/Toronto)
- Primary source repo: /Users/xuanyu.wang/repos/go-servers
- Context: main checkout; source/deployment investigation only, no product or deployment changes.
- Request: verify RetrieveConversationAssistanceUsageStats on us-west-2-prod and frontend accessibility.

## Result

Not available through the production public HTTP API. A credential-free POST to `https://api.us-west-2-prod.cresta.ai/v1/customers/cresta/profiles/default/conversationAssistanceUsageStats:retrieve` with `{}` returned HTTP 405 and the gateway message `the requested gRPC method is either not registered, or registered with a different HTTP method`. The same request to `conversationStats:retrieve` returned HTTP 401 / UNAUTHENTICATED, establishing a working control route. These probes establish routing behavior, not successful authenticated data retrieval.

## Source and deployment evidence

- Local flux-deployments HEAD, `apps/insights-server/releases/03-prod-main/helmrelease-insights-server.yaml`, specifies chart `0.24.109` and image `main-20261021_092833z-79d4425e`. Groundcover live HelmRelease and sampled pod checks independently corroborated the deployed configuration; live ingress inspection lacked the new route.
- `git cat-file -e 79d4425e:insights-server/internal/analyticsimpl/action_retrieve_conversation_assistance_usage_stats.go` failed because the implementation file is absent. The revision also has no assistance_usage source files in analyticsimpl. The implementation file first appears on origin/main at `7be97d707f` (October 22).
- Source route fix PR #34330 / commit `31fdfb77497883f52c5662a455e53ab5bf4f173a` adds `/v1/customers/[^/]+/profiles/[^/]+/conversationAssistanceUsageStats.*`; chart version `0.24.110`.
- Director origin/main contains `InsightsApi.retrieveConversationAssistanceUsageStats` at `packages/director-api/src/services/cresta-api/insights/insightsApi.ts:321`, calling `AnalyticsService.RetrieveConversationAssistanceUsageStats`.
- Director `useConversationAssistanceUsageMetric.ts` requires resolved true evaluation of `enableConversationAssistanceUsageMetric`. Production frontend bundle deployment and customer-specific flag evaluation were not checked because the API is already blocked at backend/routing.

## Next action

Promote a backend image containing the implementation and chart `0.24.110` or later to us-west-2-prod, then repeat the routing probe and an authenticated frontend request. Verify the rollout flag for the intended customer/profile after the API works. No deployment performed.

## Access

Groundcover used its existing connector authentication. Public HTTP probes used no credentials. No AWS, Okta, Azure, or SSH credential was selected or read.

## Follow-up: current release tools

Checked current cresta/cresta-cli main files via GitHub API and installed CLI help. `cresta-cli releaser promote-apps <stage> insights-server` remains the explicitly supported command. `promote-v2 insights-server --to-stage <stage>` exists but remains marked experimental. UI: https://cresta-releaser.ops2.internal.cresta.ai/ (VPN). The July UI announcement and current October docs confirm this URL; some old prose still refers to Appsmith.

Both chart version and Docker tag live in the same insights-server HelmRelease and are promoted together. Local flux-deployments HEAD has 00-head chart 0.24.110 and image main-20261023_144814z-af21f495; 01-staging and 02-prod-early retain the older pair. Normal topology: 00-head -> 01-staging -> 02-prod-early -> 03-prod-main. `releases/us-west-2-prod/cresta-api/kustomization.yaml:30` selects 03-prod-main, so promoting there affects all consumers of that stage, not just this cluster. The 02-prod-early releaser metadata requires sync-apiserver-schema and insights_server_e2e_test workflows before production promotion merge. No release command executed. GitHub used existing gh authentication; CLI help did not authenticate.

Sources: https://github.com/cresta/cresta-cli/blob/main/cmd/releaser/promote_apps.go ; https://github.com/cresta/cresta-cli/blob/main/cmd/releaser/promote.go ; https://docs.superhuman.com/d/_dboaRXZbw4w/_suH8M0jE ; https://crestalabs.slack.com/archives/C08M88ES630/p1784304949056559 .

## Post-release recheck

User reported completing release of image `main-20261023_144814z-af21f495`. Live ready-container checks confirmed that image. Current GitHub master production HelmRelease now requests both that image and chart `0.24.110`; both were included in the desired production release. However, repeated credential-free public POST probes still return HTTP 405 / method-not-registered. Live ingress inspection confirms the new route has not appeared. Groundcover returned no HelmRelease entity during this recheck, so reconciliation progress/failure reason could not be established. Do not assume another promotion is needed: desired Git configuration is already updated. Next investigate Flux/Helm application of the chart and repeat route verification. No production changes made. Existing Groundcover/GitHub authentication only; public probes credential-free.

## Glean documentation search: ingress updates

Found and read these references through Glean MCP:

- Internal Helm chart publishing/versioning: https://docs.superhuman.com/d/_dboaRXZbw4w/_suX00aBk . Publish a versioned chart and reference it through HelmRelease spec.chart.spec.version.
- GitOps promotion/deployment: https://github.com/cresta/flux-deployments/blob/master/README.md and https://docs.superhuman.com/d/_dboaRXZbw4w/_suitvCFz . Flux applies the promoted HelmRelease configuration.
- Failed HelmRelease runbook: https://github.com/cresta/claude-code-marketplace/blob/main/plugins/oncall-debug/skills/oncall-debug/runbooks/debug-failed-helmrelease.md . Explicitly lists ingress admission denials during business hours as a possible blocker. Inspect actual HelmRelease/events before considering reconciliation.
- Indexed policy: https://github.com/cresta/infra-deployments/blob/main/apps/kyverno-policies/releases/03-prod-stable/cluster/oneoff-clusterpolicy-block-ingress-changes-during-business-hours.yaml . Enforces denial of non-internal-waf-ingress spec/annotation changes and deletes during UTC hours 13-23 and 0-2. This is an indexed policy document, not verification of the effective us-west-2-prod policy or current rejection.
- Release reference: https://github.com/cresta/claude-code-marketplace/blob/main/plugins/flux-deployments/skills/release/SKILL.md . Its indexed text names a known us-west-2-prod yodawg/insights-server vanity-ingress block from this policy. Historical precedent strengthens the hypothesis but does not establish this release's current root cause.

Interpretation: the chart update/promotion is already requested in Git; a time-based ingress admission restriction is a concrete documented explanation to check next, rather than another image release. No policy, ingress, reconciliation, credentials, or automation changed in this documentation lookup. Two-hour monitoring continues.

## Two-hour monitoring closure — 15:39 Toronto

Completed the requested checks every ten minutes from 13:39 through 15:39 Toronto. The independently probed public Assistance Usage endpoint returned HTTP 405 / method-not-registered throughout, including the final check. No routing recovery observed. The monitor is now PAUSED through the native automation tool, ID `monitor-production-assistance-usage-ingress`. No deployment or credentials changed. Groundcover HelmRelease data was intermittent; its absence on the final check does not establish recovery or a new failure. The next operational step is to inspect the failed upgrade and arrange a permitted chart retry with Infra.

## Monitoring extension — 19:15 Toronto

User requested two more hours. Reactivated the existing heartbeat, every ten minutes through October 23 at 21:15 Toronto (October 24 01:15 UTC), with unchanged-state silence and final check/pause. Immediate public probe still returns HTTP 405.

Verified current infra-deployments main: us-west-2-prod kyverno-policies kustomization selects the 03-prod-stable business-hours ingress policy without a regional time patch. The source blocks UTC hours 13-23 and 0-2, without a weekday condition. Thus during EDT it blocks 09:00-23:00 Eastern daily; permitted window is 23:00-09:00. During EST the same fixed UTC rule blocks 08:00-22:00 Eastern. The next permitted window today starts at 23:00 Toronto, beyond the additional two-hour monitoring window. This is source configuration; release recovery may still require an explicit retry after prior retry exhaustion. No deployment or credentials changed.
