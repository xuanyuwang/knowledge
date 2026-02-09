# Coaching Service Feature Flag Inventory

**Created:** 2026-02-09
**Updated:** 2026-02-09

## Summary

- **18 envflags** found across coaching service code
- **All** use the `envflag` package (no raw `os.Getenv`)
- **Config service** is used but only for customer-level settings (e.g., ACL), not feature flags we'd clean up
- **8 prod clusters** checked via flux-deployments

## Prod Clusters

| Group | Clusters |
|-------|----------|
| **prod-main** (03) | us-east-1-prod, us-west-2-prod, schwab-prod, voice-prod |
| **prod-early** (02) | eu-west-2-prod, ca-central-1-prod, ap-southeast-2-prod, chat-prod |

## Classification

### Cleanable: Uniformly Enabled (can hardcode `true` and remove flag)

These flags are `true` on **all** prod clusters (set in base HelmRelease for both prod-early and prod-main).

| Flag | Default | Prod Value | File | Action |
|------|---------|-----------|------|--------|
| `COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE` | `false` | `"true"` all clusters | `constants.go:23` | Remove flag, hardcode `true` behavior |
| `ENABLE_EVALUATION_PERIOD_END_TIME` | `false` | `"true"` all clusters | `action_retrieve_evaluation_periods.go:13` | Remove flag, hardcode `true` behavior |
| `ENABLE_ON_FLY_TARGET_QA_SCORE` | `false` | `"true"` all clusters | `action_retrieve_coaching_progresses.go:37` | Remove flag, hardcode `true` behavior |

### Cleanable: Uniformly Disabled / Default (can remove flag and dead code)

These flags are **not set anywhere** in flux-deployments, meaning all clusters use the code default.

| Flag | Default | Prod Value | File | Action |
|------|---------|-----------|------|--------|
| `DISABLE_APPEAL_NOTIFICATIONS` | `false` | default (`false`) all clusters | `action_submit_scorecard.go:42` | Remove flag and the conditional; appeal notifications stay enabled |
| `COACHING_SERVICE_OVERRIDDEN_SESSION_NOTE_FORMAT` | `""` | default (`""`) all clusters | `constants.go:22` | Remove flag; empty string = no override, so remove override logic |
| `EXPORT_COACHING_OVERVIEWS_VERBOSE_LOG` | `false` | default (`false`) all clusters | `action_export_coaching_overviews.go:27` | Remove flag and verbose logging code |
| `COACHING_SERVICE_COACHING_PLAN_LOG` | `false` | default (`false`) except voice-prod (`true`) | `constants.go:20` | **Borderline** — only voice-prod has it on. Could discuss with voice team. |

### Not Cleanable: Varies by Cluster

These flags differ between clusters and still gate behavior.

| Flag | Default | Cluster Overrides | File |
|------|---------|-------------------|------|
| `COACHING_VERBOSE_LOG` | `false` | us-west-2-prod: `true`, voice-prod: `false` | `constants.go:19` |
| `COACHING_SERVICE_PAGINATED_QUERY` | `false` | voice-prod: `true` | `constants.go:21` |
| `ENABLE_OUTCOME_TARGETS` | `false` | voice-prod: `true` | `action_create_target.go:22` |

### Configuration Values (not boolean feature flags)

These are tuning parameters, not feature toggles. They use the same default everywhere (or a uniform override). Cleaning them is lower priority but possible — hardcode the value and remove the envflag.

| Flag | Default | Prod Override | File | Notes |
|------|---------|--------------|------|-------|
| `UPDATE_SCORECARD_ASYNC_WORK_TIMEOUT` | `10m` | `30m` all clusters | `action_update_scorecard.go:39` | Could hardcode `30m` |
| `COACHING_SERVICE_TRANSACTION_RETRY_COUNT` | `3` | not set (default) | `constants.go:24` | Could hardcode `3` |
| `NUMBER_DOWNWARD_TREND_CONCURRENT_QUERY` | `5` | not set (default) | `action_suggest_coaching_opportunities.go:27` | Could hardcode `5` |
| `NUMBER_SCORECARD_EXPORT_WORKERS` | `10` | not set (default) | `action_export_scorecards.go:104` | Could hardcode `10` |
| `SCORECARD_EXPORT_TIME_BUCKET_SIZE_NUM_DAYS` | `3` | not set (default) | `action_export_scorecards.go:105` | Could hardcode `3` |
| `NUMBER_COACHING_PROGRESS_CONCURRENT_QUERY` | `15` | not set (default) | `action_retrieve_coaching_progresses.go:38` | Could hardcode `15` |
| `LIST_CURRENT_SCORECARD_TEMPLATES_STATS_DURATION` | `7d` | not set (default) | `action_list_current_scorecard_templates.go:27` | Could hardcode `7d` |
| `SHARED_USER_GROUP_FILTER_DEFAULT_PAGE_SIZE` | `5000` | not set (default) | `utils/user_group_filter_util.go:19` | Could hardcode `5000` |

## Recommended Cleanup Priority

### High Priority (clear wins, no risk)

1. **`ENABLE_EVALUATION_PERIOD_END_TIME`** — all prod = `true`, simple boolean gate
2. **`ENABLE_ON_FLY_TARGET_QA_SCORE`** — all prod = `true`, simple boolean gate
3. **`COACHING_SERVICE_ENABLE_SYNC_HISTORIC_SCORECARD_WRITE`** — all prod = `true`, simple boolean gate
4. **`DISABLE_APPEAL_NOTIFICATIONS`** — never enabled anywhere, dead code path

### Medium Priority (safe but more code to touch)

5. **`COACHING_SERVICE_OVERRIDDEN_SESSION_NOTE_FORMAT`** — never set, remove override logic
6. **`EXPORT_COACHING_OVERVIEWS_VERBOSE_LOG`** — never enabled, remove debug logging
7. **`UPDATE_SCORECARD_ASYNC_WORK_TIMEOUT`** — uniform `30m` everywhere, hardcode it

### Low Priority (config tuning values — keep for operational flexibility)

8-15. The numeric/duration configuration values. These are useful operational knobs even if currently at default. Consider keeping unless there's a push to reduce envflag count.

## Source Locations

### flux-deployments

- Base configs: `apps/apiserver/releases/{02-prod-early,03-prod-main}/helmrelease-apiserver.yaml`
- Per-cluster overrides: `releases/<cluster>/cresta-api/patch-helmrelease-apiserver.yaml`
- Global patches: `releases/<cluster>/_kustomizations/patch-helmrelease-global.yaml`

### go-servers

- Coaching service: `apiserver/internal/coaching/`
- envflag package: `github.com/cresta/shared-go/util/envflag`
