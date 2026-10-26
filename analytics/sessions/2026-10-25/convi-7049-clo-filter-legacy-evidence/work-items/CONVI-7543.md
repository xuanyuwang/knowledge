# CONVI-7543: GA enable CLO filter

**Status:** active — repair merged and fleet replay succeeded; effective read-back verification pending
**Primary domain:** Analytics
**Primary subdomain:** Insights User Filter
**Official ticket:** [CONVI-7543](https://linear.app/cresta/issue/CONVI-7543/ga-enable-clo-filter-for-all-customers)
**Last updated:** 2026-08-23

## Objective and Impact

- **Objective:** Enable the released Performance Insights CLO filter for standard production customers.
- **Customer/system impact:** Makes CLO filtering available across eligible production profile and use-case scopes after successful config review and deployment.
- **Role:** implemented

## Scope

**In scope**

- Standard production customer-history feature maps.
- Existing NCLH, Holiday Inn, and Rivo Holdings pilot entries remain enabled.

**Non-goals**

- Comcast and Schwab configs, which follow separate release plans.
- The separate ClickHouse CLO materialized-view optimization flag.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/config`
- **Worktrees:** `/Users/xuanyu.wang/repos/config-convi-7543`, `/Users/xuanyu.wang/repos/config-convi-7543-repair`
- **Branches:** `xw/convi-7543-enable-clo-filter-all-customers`, `xw/convi-7543-repair-clo-enablement`
- **PRs/commits:** merged production [config#152031](https://github.com/cresta/config/pull/152031), merged staging [config#152040](https://github.com/cresta/config/pull/152040), merged repair [config#152115](https://github.com/cresta/config/pull/152115), production replay [run 32655059116](https://github.com/cresta/config/actions/runs/32655059116), `9c843eca8c`, `92db84ab2f`, `bfed642b8b`, `b4432a033b5de6d9bd507a2416973542f7f27856`

## Current Understanding

Merged PR #152031 added `enableCLOFilters: true` to the 1,831 standard production history maps that were not already covered by pilot PRs. The 117 maps in 17 Comcast/Schwab config files remain unchanged for their separate release plans. Follow-up PR #152040 merged with the remaining 173 staging history additions. The post-merge workflow reported successful persistence, but current ConfigService-derived `configv3` and Cresta Admin state show that the GA values did not become durable effective state. Admin reports 249 disabled profiles in its current scope; examples such as Uber, Twilio, and eBay remain true in history but absent in generated effective config. Later whole-customer writers have removed 100 of those history-only values and introduced another 16 absent Woolworths maps, which explains the narrower 11-customer Git regression count but not effective rollout coverage.

## Findings and Decisions

- GA follows the established expanded `customer_config_history/prod` pattern; there is no wildcard production default.
- Reused the established standard-production eligibility set from the global Director flag rollout.
- The customer-facing `enableCLOFilters` flag remains independent from `insights.useConversationOutcomeMomentAnnotationMaterializedView`; customers without the optimization flag continue using the raw query path.

## Blockers and Dependencies

- Production PR #152031 and staging PR #152040 are merged.
- Production effective state has 249 disabled profiles in the current Admin scope despite GA history entries.
- The batch sync reported success but effective/read-back state did not retain the flag; this persistence/read-back failure must be understood before repair.
- Subsequent Config Wizard/profile-health-check/whole-customer writes omit the absent effective flag and gradually erase the history-only values.

## Validation and Rollout

- 352 changed YAML files parsed successfully.
- Semantic checks verified exactly one enabled CLO flag in every eligible map and no duplicates.
- Diff contains 1,831 additions, zero deletions, and only `enableCLOFilters: true` additions.
- `git diff --check` passed.
- Local AJV validation could not start because the isolated worktree has no Yarn dependency state; PR CI provides the owning schema and ConfigService checks.
- PR CI passed, including ConfigService dry-run, schema proto, config consistency, JavaScript/Python YAML lint, credential scan, and Schwab no-change validation.
- Staging follow-up validation parsed all 28 changed YAML files and verified exactly one enabled CLO flag in all 174 staging maps; its diff contains 173 additions and zero deletions.

## Next Actions

1. Verify final effective state in Admin and through targeted ConfigService-to-`configv3` read-back.
2. Guard whole-customer writers and verify generated diffs preserve CLO plus unrelated settings such as Spruce Spanish.
3. Handle Comcast and Schwab production through their separate release plans.

## Timeline

- 2026-08-20 — Opened config#152031 with standard production coverage and explicit Comcast/Schwab exclusions. Evidence: `log/2026-08-20.md`, `sessions/2026-08-20/codex-convi-7543-ga-rollout.md`.
- 2026-08-20 — Production PR #152031 merged; opened staging follow-up #152040 for all 174 staging maps.
- 2026-08-20 — Staging PR #152040 merged.
- 2026-08-21 — Spruce Spanish recovery PR #152086 removed both Spruce CLO entries due divergent Admin/ConfigService state; recorded follow-up in `sessions/2026-08-21/codex-spruce-config-collision.md`.
- 2026-08-21 — Corrected the initial history-only audit: Admin reports 249 disabled profiles, and Uber/Twilio/eBay prove history can say true while effective `configv3` remains absent. The 11-customer count only covers later Git-history rewrites.
- 2026-08-21 — Opened repair PR #152115 with 116 additions across 11 customers; validated 1,889 enabled maps and exactly 117 intentional Comcast/Schwab omissions. Prepared an explicit 354-customer replay list.
- 2026-08-22 — Repair PR #152115 merged at `b4432a033b5de6d9bd507a2416973542f7f27856`; concurrent master changes expanded the repair to 180 additions across 17 files.
- 2026-08-23 — Verified `origin/master` has 1,891 eligible true maps and exactly 117 intentional omissions, then manually replayed all 354 eligible customers. [Workflow run 32655059116](https://github.com/cresta/config/actions/runs/32655059116) succeeded in every production region.
