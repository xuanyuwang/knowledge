# Implementation Considerations for User Filter Consolidation

**Created**: 2026-02-09
**Purpose**: Identify all considerations for building a reliable, company-wide shared user filter package, and outline an implementation approach.

---

## Considerations

### 1. No Regression (Critical)

The consolidated implementation must behave identically to the existing paths for all currently-correct behaviors. Two strategies:

#### Strategy A: Behavioral Test Suite First

Write comprehensive tests against the **behavioral standard** before touching any production code. Tests encode expected behaviors as assertions. Any refactoring that breaks a test is a regression.

**Pros**: Gold standard for safety. Tests are permanent assets.
**Cons**: Test setup is heavy — needs mocks for UserService, InternalUserService, ACL, ConfigService. Current test coverage is ~4/10 for behavioral scenarios — significant gap to fill.

**Current test infrastructure** (from exploration):
- `shared/user-filter/user_filter_test.go` (~970 lines) — uses gomock for gRPC, testify/mock for config/ACL
- `insights-server/internal/analyticsimpl/common_user_filter_test.go` (~1443 lines) — suite-based, good mock helpers (`setupACLMock`, `setupAllUsersMock`, `setupExpandGroupsToUsersMock`)
- Missing: deactivated + ACL + groups combinations, nested group hierarchy, UNION semantics edge cases, profile scoping, many B-SF/B-GS/B-GM behaviors

#### Strategy B: Shadow Mode (Dual-Write Comparison)

Run both old and new paths in production, compare results, log differences. Only switch to new path when difference rate drops to zero.

**Pros**: Catches regressions that tests miss (unexpected data patterns, edge cases in production data). Real-world validation.
**Cons**: 2x latency during shadow period. Complex comparison logic. May be noisy if old path has known bugs (Divergence 5: INTERSECTION vs UNION).

#### Recommended: Both

1. Write behavioral tests first (blocks nothing — can be done independently)
2. Add shadow mode comparison when migrating callers (catches production edge cases)

---

### 2. Smooth Transition (No Downtime)

The consolidation must not cause any API to become temporarily unavailable.

**Current state**: ~12 APIs use `ParseUserFilterForAnalytics`, ~17 use old pattern. Nobody uses the shared `Parse` directly for analytics.

**Approach**:
- Build the new unified implementation in `shared/user-filter/` as **new functions** alongside existing code
- Migrate callers one-by-one (same incremental approach as the current migration)
- Old code is not deleted until all callers are migrated and validated
- Feature flag gates which path each caller uses

**Assumption**: "No one else is using the two implementations" — verify by searching all import sites. If someone is, they need to be considered.

---

### 3. Small PRs

Big PRs are hard to review and risky. Target: **<300 lines of production code per PR** (tests and docs excluded from this count).

**Natural PR boundaries**:
1. Tests only (can be large — test-only PRs are safe)
2. New types/interfaces (no callers yet)
3. New implementation (behind feature flag, no callers yet)
4. Per-caller migration (each `retrieve_*_stats.go` is one PR)
5. Cleanup (remove old code after all callers migrated)

---

### 4. Feature Flags

**Current infrastructure**:
- `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` — env var, global to cluster, requires pod restart
- Proto-based config service — per-customer, runtime-updatable, no restart needed

**Current limitation**: The env var is all-or-nothing. If the new code has a bug for one customer, rolling back affects all customers.

**Recommended**: Use proto-based config flag for the consolidated implementation. Add a field to `cresta/v1/config/features/insights.proto`:

```proto
// When true, use the unified user filter implementation (shared/user-filter).
// When false, use the existing ParseUserFilterForAnalytics / old pattern.
bool enable_unified_user_filter = <next_field_number>;
```

This enables:
- Per-customer rollout (enable for low-risk customers first)
- Instant rollback (no pod restart)
- Gradual rollout (enable for 1 customer → 10 → all)

---

### 5. Debuggability

**Existing observability infrastructure**:
- **Logging**: `shared-go/framework/log` — context-aware structured logging (`logger.Infof(ctx, ...)`)
- **Metrics**: `shared-go/framework/stats` — Datadog-compatible counters/gauges
- **Timing**: `shared/performance/Stopwatch` — duration tracking
- **Tracing**: `shared-go/framework/tracer` — OpenTelemetry-compatible, integrated with gRPC
- **Request context**: RequestID, StreamID, GrpcMethod, Principal auto-propagated

**What to add for the consolidated user filter**:

```
Logging (at INFO level):
- Entry: customer/profile, flag values, selection counts
- Base population: user count fetched
- ACL: users/groups before → after, diff
- Selection filtering: users/groups matched
- Exit: final counts, ShouldQueryAllUsers, duration

Metrics:
- user_filter.parse.duration_ms — total parse time
- user_filter.parse.base_population_count — base population size
- user_filter.parse.final_users_count — result size
- user_filter.parse.early_return — counter for no-access returns
- user_filter.parse.error — error counter by type

Comparison (shadow mode):
- user_filter.compare.match — old and new agree
- user_filter.compare.mismatch — old and new disagree
- user_filter.compare.mismatch_details — structured log of differences
```

---

### 6. Rollback Strategy

If a bug is discovered after rollout:

| Scenario | Rollback mechanism |
|----------|-------------------|
| Bug affects one customer | Disable proto config flag for that customer (instant) |
| Bug affects all customers | Disable proto config flag globally via wildcard (instant) |
| Bug in shared package code | Revert PR, deploy (standard deploy cycle) |
| Shadow mode shows divergence | Don't promote to primary — investigate first |

**Key**: The feature flag must be checked at the entry point of each caller, not deep inside the shared package. This ensures the flag cleanly switches between old path and new path.

---

### 7. Shadow Mode / Comparison

Run both implementations and compare results before switching.

**How it works**:
1. Caller calls the new unified implementation (primary)
2. Caller also calls the old implementation (shadow)
3. Compare `FinalUsers`, `UserToDirectGroups`, `AllGroups`, `ShouldQueryAllUsers`
4. Log any differences as structured comparison events
5. Return the **old** implementation's result (safe — no behavior change)
6. Once mismatch rate is zero for N days, switch to returning the new result

**Known expected differences** (not bugs):
- Divergence 5: UNION vs INTERSECTION — new is correct, old has known bug
- Divergence 6: sort order (resource name vs FullName)
- Divergence 9: hasAgentAsGroupByKey differences

These should be excluded from comparison or logged at DEBUG level.

**Practical concern**: Shadow mode doubles the RPC calls (two `ListUsersForAnalytics` calls). Options:
- Only enable shadow mode for a subset of requests (e.g., 10% sampling)
- Only enable for specific customers during validation
- Accept the cost temporarily during rollout period

---

### 8. Performance

The consolidated implementation should not be slower than the existing paths.

**Baseline**: Measure current latency of `ParseUserFilterForAnalytics` and old pattern per-caller before starting migration. Use the `Stopwatch` utility.

**Risks**:
- Additional map construction (two user-to-groups maps instead of one)
- Struct-based outputs (`LiteUser`/`LiteGroup`) have slightly more memory than string-based

**Mitigations**:
- Benchmark tests for large user sets (1000+, 10000+)
- Profile memory allocation
- The extra map is cheap — it's just pointer assignments, not data copies

---

### 9. API Ergonomics

The shared package will be used by 30+ callers across insights-server and coaching-server. The API must be easy to use correctly and hard to use incorrectly.

**Current pain points**:
- `ParseUserFilterForAnalytics` has 15+ parameters — easy to mix up
- `hasAgentAsGroupByKey` leaks caller concern into the filter
- Dependencies passed as parameters every time

**Recommended pattern**: Options struct for per-call parameters. All parameters are named fields — IDE autocompletion shows what's available, and you can't accidentally swap two bools.

```go
type ParseOptions struct {
    CustomerID              string
    ProfileID               string
    SelectedUsers           []string
    SelectedGroups          []string
    ExcludeDeactivatedUsers bool
    DirectMembershipsOnly   bool
    ListAgentOnly           bool
    // ... other per-call fields
}

func (p *Parser) Parse(ctx context.Context, opts ParseOptions) (*ParseResult, error)
```

**Dependencies** (UserService client, ACL helper, config client, logger): Inject via constructor, not per-call. These are the same across all calls within a service — no reason to pass them every time.

```go
type Parser struct {
    userClient    userpb.InternalUserServiceClient
    configClient  config.Client
    aclHelper     auth.ResourceACLHelper
    logger        log.Logger
}

func NewParser(deps ...) *Parser { ... }
```

**Why not functional options?** Functional options (`WithDirectMembershipsOnly()`, etc.) are better for optional configuration with sensible defaults. But most of the user filter parameters are required per-call and vary between calls — a plain struct is simpler and more explicit. No hidden defaults, no order-dependent option application.

---

### 10. Testing Strategy for Shared Package

Since this is shared across the company, tests must be thorough.

**Test layers**:
1. **Unit tests** (in `shared/user-filter/`) — mock all external services, test every behavior from the behavioral standard
2. **Comparison tests** (in `insights-server/`) — for each caller, assert that old path and new path produce same results given same inputs
3. **Integration tests** (optional) — test with real services in staging environment

**Test organization**: One test per behavior ID from the behavioral standard (B-ACL-1, B-SF-1, etc.). This creates a 1:1 mapping between spec and tests.

---

### 11. Documentation

For a company-wide shared package:
- **Package-level godoc** explaining purpose, usage, and examples
- **Caller migration guide** — step-by-step instructions for each caller pattern (agent leaderboard, team leaderboard, time-range, coaching)
- **The behavioral standard** itself serves as the spec

---

### 12. Monitoring After Rollout

Use the same `stats.NewMetric` pattern used elsewhere in the codebase (e.g., `action_reset_scorecard.go`):

```go
var (
    parseRequestCounter  = stats.NewMetric("shared.user_filter.parse.request.count")
    parseErrorCounter    = stats.NewMetric("shared.user_filter.parse.error.count")
    parseDurationMetric  = stats.NewMetric("shared.user_filter.parse.duration_ms")
)
```

The `request.count` / `error.count` / `duration_ms` triplet covers core monitoring — error rate and latency percentiles are derivable from these. More granular metrics (base population size, early return rate) can be added later if needed.

Keep metrics permanently. Set up alerts for:
- Error rate spike (> 0.1% of requests)
- Latency spike (p99 > 2x baseline)
- Early return rate change (could indicate ACL misconfiguration)

---

## Implementation Phases (High-Level)

### Phase 1: Behavioral Test Suite
- Write comprehensive tests in `shared/user-filter/` covering all B-* behaviors
- Test against existing `Parse` implementation
- These tests are the regression guard for all future changes
- **PR pattern**: test-only PRs, can be large

### Phase 2: New Types and Interface
- Define `ParseOptions`, `ParseResult`, `LiteUser`, `LiteGroup` types
- Define new `Parser` struct with constructor
- Define post-processing utilities (`StripRootAndDefaultGroups`, `UserIDs`, `GroupNamesToStrings`, `ApplyToQuery`)
- No implementation yet — just types
- **PR pattern**: small, types-only

### Phase 3: Unified Implementation
- Implement the new `Parse` method in `shared/user-filter/`
- Wire up to `ListUsersForAnalytics`, ACL, group expansion
- All behavioral tests must pass
- Behind feature flag — no callers use it yet
- **PR pattern**: implementation PR (may be larger, but behind flag)

### Phase 4: Add Proto Config Flag
- Add `enable_unified_user_filter` to insights proto
- Wire up config reading in insights-server
- **PR pattern**: small config change

### Phase 5: Shadow Mode Infrastructure
- Add comparison logic that runs both paths and logs differences
- Enable for specific test customers first
- **PR pattern**: small, shadow mode plumbing

### Phase 6: Caller Migration (per-caller PRs)
- For each `retrieve_*_stats.go`:
  1. Add new-path call behind feature flag
  2. Add shadow mode comparison (optional per caller)
  3. Enable flag for test customers
  4. Validate — promote to all customers
- ~29 callers × 1 PR each = ~29 PRs
- **PR pattern**: small, per-caller (~50-150 lines each)

### Phase 7: Cleanup
- Remove old `ParseUserFilterForAnalytics` and inline pattern code
- Remove feature flag
- Remove shadow mode comparison code
- **PR pattern**: deletion PRs (safe, can be large)

---

## Open Questions

1. **Shadow mode cost**: Is 2x RPC cost acceptable during validation? Alternative: offline comparison using recorded request/response pairs.
2. **Migration order**: Which callers to migrate first? Lowest-traffic APIs for safety, or highest-traffic for maximum coverage?
3. **Timeline for feature flag removal**: How long should the flag stay active after 100% rollout? (Suggested: 2 sprints of stability)
4. **Coaching callers**: The current `Parse` in `shared/user-filter/` uses `ListUsers` (not `ListUsersForAnalytics`). Coaching callers will need to switch to the new API. Is this acceptable?
