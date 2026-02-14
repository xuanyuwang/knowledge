# Feature Flag Infrastructure Analysis

**Created**: 2026-02-11  
**Updated**: 2026-02-11

## Overview

This document outlines how feature flags are implemented and managed in the go-servers codebase, with focus on understanding the patterns for gating the consolidated user filter rollout.

---

## 1. How `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` is Implemented

### 1.1 Definition and Initialization

**Location**: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/analyticsimpl.go` (lines 57, 161-165, 196)

The flag is defined as a service-level boolean field in `AnalyticsServiceImpl`:

```go
type AnalyticsServiceImpl struct {
    // ... other fields ...
    enableParseUserFilterForAnalytics bool
    // ... other fields ...
}
```

**Initialization Pattern** (lines 161-165):

```go
enableParseUserFilterForAnalytics := false
enableParseUserFilterForAnalyticsString := os.Getenv("ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS")
if enableParseUserFilterForAnalyticsString == TrueString {
    enableParseUserFilterForAnalytics = true
}
```

### 1.2 Where It's Read

The flag is sourced entirely from **environment variables** at service startup:
- Environment variable: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS`
- Expected value: `"true"` (string)
- Storage: Service struct field (immutable after init)

### 1.3 How It's Used

The flag is checked in multiple analytics endpoints to branch between old and new implementation paths:

**Files using this flag** (25+ occurrences):
- `retrieve_agent_stats.go` - Line 47 (new path) and Line 199 (old path)
- `retrieve_conversation_stats.go` - Lines 42, 164
- `retrieve_guided_workflow_stats.go` - Line 46
- `retrieve_hint_stats.go` - Lines 50, 175
- `retrieve_knowledge_assist_stats.go` - Lines 40, 155
- `retrieve_knowledge_base_stats.go` - Line 46
- `retrieve_live_assist_stats.go` - Lines 41, 158
- `retrieve_note_taking_stats.go` - Line 46
- `retrieve_qa_score_stats.go` - Lines 65, 226
- `retrieve_smart_compose_stats.go` - Lines 56, 175
- `retrieve_suggestion_stats.go` - Line 46
- `retrieve_summarization_stats.go` - Lines 39, 154

### 1.4 Code Pattern for Branching

```go
if a.enableParseUserFilterForAnalytics {
    // NEW PATH: Using consolidated ParseUserFilterForAnalytics
    result, err := ParseUserFilterForAnalytics(
        ctx,
        a.userClientRegistry,
        a.internalUserServiceClientRegistry,
        a.configClient,
        a.resourceACLHelperProvider.Get(),
        parent.CustomerID,
        req.GetProfileId(),
        // ... parameters ...
    )
    // Use result
} else {
    // LEGACY PATH: Using MoveFiltersToUserFilter or other old logic
    if !a.enableParseUserFilterForAnalytics {
        // Old implementation
    }
}
```

---

## 2. Other Feature Flag Patterns in Insights-Server

### 2.1 Environment Variable-Based Flags (Similar to ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS)

The insights-server uses **8 other environment-variable-based feature flags**, all following the same pattern:

| Flag | Env Var | Purpose |
|------|---------|---------|
| `enableAgentCountFromCrestaConversation` | `ENABLE_AGENT_COUNT_FROM_CRESTA_CONVERSATION` | Use cresta conversation count vs total count |
| `enableListUsersCache` | `ENABLE_LIST_USERS_MAPPED_TO_GROUPS_CACHE` | Cache user-to-group mappings |
| `enableNewACLIntersectionWithUserFilter` | `ENABLE_NEW_ACL_INTERSECTION_WITH_USER_FILTER` | New ACL filtering logic |
| `enableMetadataFilterFromNewViewInClickhouse` | `ENABLE_METADATA_FILTER_FROM_NEW_VIEW_IN_CLICKHOUSE` | Metadata filtering in Clickhouse |
| `enableFilteringByConvoFieldsInMsg` | `ENABLE_FILTERING_BY_CONVO_FIELDS_IN_MSG` | Filter by conversation fields in messages |
| `enableUsageOfExplicitMappingMomentAnnotationPayload` | `ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD` | Use explicit mapping for moment annotations |
| `enableACLFilteringInEvaluateAnalyticsChart` | `ENABLE_ACL_FILTERING_IN_EVALUATE_ANALYTICS_CHART` | ACL filtering in charts |
| `enableSimplifiedSubcategoryGroupByInEvaluateAnalyticsChart` | `ENABLE_SIMPLIFIED_SUBCATEGORY_GROUP_BY_IN_EVALUATE_ANALYTICS_CHART` | Simplified subcategory grouping |

**Initialization Location**: `analyticsimpl.go` lines 121-165

**All follow identical pattern**:
1. Initialize to `false`
2. Read from env var
3. Check if equals `"true"` string
4. Store in struct field

### 2.2 Characteristics of Environment Variable Flags

**Advantages**:
- Simple deployment-time control
- No runtime config needed
- Fast lookup (no RPC calls)
- Easy to enable/disable per environment or pod

**Limitations**:
- **Global to entire service** - Cannot be per-customer or per-profile
- Requires pod restart to change
- Not suitable for gradual rollouts

---

## 3. Proto-Based Config Feature Flags

### 3.1 The Config Service Pattern

The Cresta infrastructure has a **Config Service** that manages customer/profile/usecase-level configuration through proto definitions stored in customer config YAML.

**Key Files**:
- `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features.proto` - Top-level features
- `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features/insights.proto` - Insights-specific features
- `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features/conversation_features.proto` - Conversation features
- Config service client: `/Users/xuanyu.wang/repos/go-servers/config/shared/grpcclient/config/`

### 3.2 Feature Flag Definition in Proto

**Location**: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features/insights.proto`

Example of config-based flags:

```proto
message Insights {
  bool enable = 1;
  bool use_cresta_conversation_count = 2;
  bool use_float_weight_in_qa = 3;
  bool match_conversations_with_latest_moment_annotation = 5;
  bool suspend_historic_analytics = 6;
  bool include_empty_conversation_in_elasticsearch_query = 8;
  int32 ai_analysis_report_weekly_quota = 14;
  bool enable_read_from_typed_percentage_score_field = 15;
  int32 precision_threshold_for_elasticsearch_cardinality_agg = 16;
  bool enable_moment_prerequirements_usage = 17;
  bool enable_predict_csat_v3 = 18;
  bool enable_dashboard_builder_caching = 19;
  bool exclude_ingestion_pipeline_from_agent_stats = 20;
}
```

### 3.3 How Proto-Based Flags Are Read at Runtime

**Location**: `/Users/xuanyu.wang/repos/go-servers/config/shared/grpcclient/config/util.go`

The pattern uses the config client to fetch configuration and fail-open:

```go
// GetFeaturesFailOpen gets features config with graceful degradation
func GetFeaturesFailOpen(ctx context.Context, client Client, profileName customerpb.ProfileNameInterface, opts ...GetFeaturesConfigOption) *configpb.Features {
    features, err := GetFeatures(ctx, client, profileName, opts...)
    if err != nil {
        logger.With(logger.EveryN(1000)).Errorf(ctx, "%v", err)
        return &configpb.Features{}  // Return empty config on error
    }
    return features
}

// GetFeatures fetches config from usecase or profile level
func GetFeatures(ctx context.Context, client Client, profileName customerpb.ProfileNameInterface, opts ...GetFeaturesConfigOption) (*configpb.Features, error) {
    // If usecase ID provided, fetch usecase config
    if opt.usecaseID != nil {
        cfg, err := client.GetUsecaseConfig(profileName.GetCustomerID(), profileName.GetProfileID(), *opt.usecaseID)
        if err == nil {
            return cfg.Proto().GetFeatures(), nil
        }
        if !opt.usecaseOptional {
            return nil, fmt.Errorf("failed to get config for usecase %v: %w", ...)
        }
    }
    
    // Fallback to profile config
    cfg, err := client.GetProfileConfig(profileName.GetCustomerID(), profileName.GetProfileID())
    if err != nil {
        return nil, fmt.Errorf("failed to get config for profile %v: %w", ...)
    }
    return cfg.GetRawProto().GetFeatures(), nil
}
```

**Hierarchy**:
1. Try usecase-level config if `WithUsecaseID()` option provided
2. Fallback to profile-level config
3. Return empty config on error (fail-open pattern)

### 3.4 Example: Reading Config-Based Features

**Location**: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/aianalysis/action_check_ai_analysis_quota.go` (lines 55-74)

```go
func (s AiAnalysisServiceImpl) getQuotaLimit(ctx context.Context, customerID, profileID string) (int, error) {
    // Fetch profile configuration from config service
    profile, err := s.configClient.GetProfileConfig(customerID, profileID)
    if err != nil {
        return 0, err
    }
    
    perCustomerProfileQuota := 0
    if profile.GetFeatures() != nil &&
        profile.GetFeatures().GetInsights() != nil {
        // Read ai_analysis_report_weekly_quota from config
        perCustomerProfileQuota = int(profile.GetFeatures().GetInsights().AiAnalysisReportWeeklyQuota)
    }
    
    // Apply defaults and return
    if perCustomerProfileQuota < 0 {
        return math.MaxInt32, nil  // -1 means infinite
    }
    return perCustomerProfileQuota, nil
}
```

**Pattern**: Every request fetches config via RPC to config service, extracts the needed feature flag.

---

## 4. Feature Flag Management: How to Toggle Flags

### 4.1 Environment Variable Flags (Current Approach for ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS)

**Current Process**:
1. Set environment variable in Kubernetes deployment
2. Restart pod to pick up new value
3. No per-customer control

**Location**: Kubernetes manifests or `values.yaml` in Helm charts (not shown, but referenced in `/Users/xuanyu.wang/repos/go-servers/charts/configserver/values.yaml`)

### 4.2 Config-Based Flags (Recommended for Per-Customer Rollout)

#### Setting Feature Flags via BatchSetFeatureFlags API

**Location**: `/Users/xuanyu.wang/repos/go-servers/config/internal/service/config/action_batch_set_feature_flags.go`

**Pattern**: Config service provides gRPC endpoint to update feature flags

```proto
service ConfigService {
    rpc BatchSetFeatureFlags(BatchSetFeatureFlagsRequest) returns (BatchSetFeatureFlagsResponse);
}

message BatchSetFeatureFlagsRequest {
    // Resource path with wildcards: /customers/-/profiles/-/usecases/-
    string usecase = 1;
    
    // Which levels to apply: LEVEL_ALL, LEVEL_PROFILE_ONLY, LEVEL_USECASE_ONLY
    ApplicationLevel application_level = 2;
    
    // List of flags to set
    repeated FeatureFlag feature_flags = 3;
}

message FeatureFlag {
    string name = 1;
    bool value = 2;
}
```

**Scope Examples**:
- `/customers/-/profiles/-/usecases/-` → All customers, profiles, usecases
- `/customers/cust123/profiles/-/usecases/-` → Single customer, all profiles
- `/customers/cust123/profiles/prof456/usecases/uc789` → Single usecase

**Response**: Returns a review URI for config changes (requires approval)

### 4.3 Config File-Based Updates (Lower-Level)

Customer configurations are stored in YAML files in customer config repositories. These can be updated and deployed through normal config management:

- Profile-level config: `featureFlags` in `profileConfig`
- Usecase-level config: `featureFlags` in `usecaseConfig`

Example structure:
```yaml
profileConfig:
  profile:
    desktopAppConfig:
      legacyPublicConfig:
        featureFlags:
          # Feature flags defined here
          enableParseUserFilterForAnalytics: true
```

---

## 5. Hierarchy and Resolution Order

### 5.1 Config Resolution Hierarchy

1. **Usecase-level** (highest priority) - `GetUsecaseConfig(customerID, profileID, usecaseID)`
2. **Profile-level** (fallback) - `GetProfileConfig(customerID, profileID)`
3. **Customer-level** (rarely used for feature flags)
4. **Defaults** (empty proto or hardcoded defaults)

### 5.2 Example: Fetching Config with Fallback

```go
// Try usecase config first if ID provided
result, err := config.GetFeatures(ctx, configClient, profileName, 
    config.WithOptionalUsecaseID(usecaseID))  // Returns profile config if usecase not found

// Or require usecase config
result, err := config.GetFeatures(ctx, configClient, profileName,
    config.WithUsecaseID(usecaseID))  // Error if usecase not found
```

---

## 6. Current Limitations and Recommendations for User Filter Rollout

### 6.1 Issues with Environment Variable Approach

**Current state**: `ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS` is an **environment variable flag**

**Problems**:
1. **No per-customer control** - Entire cluster behaves the same
2. **Requires pod restarts** - Can't enable/disable gradually
3. **No gradual rollout** - Either on for everyone or off for everyone
4. **Hard to audit** - Changes require deployment records

### 6.2 Recommended Approach for Consolidated User Filter Rollout

**Option 1: Add Proto-Based Feature Flag (Recommended)**

Add to `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features/insights.proto`:

```proto
message Insights {
  // ... existing fields ...
  
  // Enable consolidated user filter for analytics APIs
  // When enabled, uses ParseUserFilterForAnalytics for consistent filter handling
  bool enable_parse_user_filter_for_analytics = 21;
}
```

Then modify insights-server to read from config:

```go
// In Init() function
configClient := NewConfigClient(...)
// Later in request handlers
features := config.GetFeaturesFailOpen(ctx, configClient, profileName)
if features.GetInsights().GetEnableParseUserFilterForAnalytics() {
    // New path
} else {
    // Legacy path
}
```

**Advantages**:
- Per-customer, per-profile control
- Fail-open (defaults to safe behavior)
- Can be updated via `BatchSetFeatureFlags` API
- No service restart needed
- Gradual rollout capability

**Option 2: Keep Environment Variable + Add Config Override**

Support both env var (for backward compatibility) and proto-based override:

```go
// Check environment variable first (for existing deployments)
enableFromEnv := os.Getenv("ENABLE_PARSE_USER_FILTER_FOR_ANALYTICS") == "true"

// In request handler, try to override from config
features := config.GetFeaturesFailOpen(ctx, configClient, profileName)
enable := enableFromEnv
if features.GetInsights() != nil {
    // Config takes precedence over env var
    enable = features.GetInsights().GetEnableParseUserFilterForAnalytics()
}
```

**Advantages**:
- Backward compatible
- Can run environment variable as baseline, config for per-customer overrides
- Flexible deployment strategy

---

## 7. Key Configuration Files and APIs

### 7.1 Proto Definitions

- **Top-level Features**: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features.proto`
- **Insights Features**: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/features/insights.proto`
- **Config Service API**: `/Users/xuanyu.wang/repos/cresta-proto/cresta/v1/config/config_service.proto`

### 7.2 Config Client Implementation

- **Utilities**: `/Users/xuanyu.wang/repos/go-servers/config/shared/grpcclient/config/util.go`
- **Feature Flag Setter**: `/Users/xuanyu.wang/repos/go-servers/config/internal/service/config/action_batch_set_feature_flags.go`

### 7.3 Usage Patterns

- **Insights Service**: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/aianalysis/action_check_ai_analysis_quota.go`
- **API Server Conversation Service**: `/Users/xuanyu.wang/repos/go-servers/apiserver/internal/conversation/conversationserviceimpl.go`

---

## Summary

1. **Current Pattern**: Environment variable flags are simple but inflexible (service-wide, requires restart)
2. **Proto-Based Pattern**: Config service provides per-customer, runtime-updatable feature flags
3. **Recommended Strategy**: Add `enable_parse_user_filter_for_analytics` to the proto-based `Insights` config for gradual, per-customer rollout
4. **Fail-Safe**: Use `GetFeaturesFailOpen()` to gracefully handle config service errors
5. **Migration Path**: Support both env var (baseline) and proto config (override) during transition
