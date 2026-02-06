# CONVI-6192: Allow Conversation Source to be Configurable

**Linear**: https://linear.app/cresta/issue/CONVI-6192

## Problem

Alaska Airlines has agents who don't use the Five9 extension. Their chats come in via a postcall job instead of real-time streaming.

**Current behavior on Leaderboard:**
- Reservations Chat: Agents without extension show as **0** (correctly identifies they didn't use Cresta)
- Customer Care Chat: Agents without extension show as **1** (incorrectly appears they used Cresta)

**Impact**: Alaska cannot identify which agents are not using Cresta features because postcall conversations are being counted in Agent Leaderboard metrics.

## Goal

Add a per-customer configuration flag to control which conversation sources are included in `RetrieveAgentStats` API.

## Investigation Findings

### Conversation Source Enum

From `cresta-proto/cresta/v1/conversation/conversation.proto:280-329`:

| Enum | Value | Description |
|------|-------|-------------|
| `SOURCE_UNSPECIFIED` | 0 | Real-time production traffic (agents using Cresta) |
| `INGESTION_PIPELINE` | 8 | Postcall/onboarding ingestion (agents NOT using extension) |
| `BOT` | 7 | Bot service conversations |

### Current Query Logic

In `go-servers/insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go:168,219`:

```sql
conversation_source in (0, 8[, 7])
```

- By default: includes `SOURCE_UNSPECIFIED` (0) and `INGESTION_PIPELINE` (8)
- With `enable_stats_for_conversation_source_bot=true`: also includes `BOT` (7)

### Existing Config Pattern

`cresta-proto/cresta/v1/config/features/insights.proto:85`:

```protobuf
// Whether to enable insights stats calculation for conversation source of
// CONVERSATION_SOURCE_BOT (enum: 7). By default, we only calculate for
// CONVERSATION_SOURCE_UNSPECIFIED (enum: 0) and
// CONVERSATION_SOURCE_INGESTION_PIPELINE (enum: 8).
bool enable_stats_for_conversation_source_bot = 13;
```

Usage in `go-servers/insights-server/internal/shared/common.go:1090-1101`:

```go
func IncludeConversationSourceBot(
    ctx context.Context, configClient config.Client,
    customerID, profileID string,
) (bool, error) {
    profile, err := configClient.GetProfileConfig(customerID, profileID)
    if err != nil {
        return false, err
    }
    return profile.GetFeatures() != nil &&
        profile.GetFeatures().GetInsights() != nil &&
        profile.GetFeatures().GetInsights().EnableStatsForConversationSourceBot, nil
}
```

## Proposed Solution

### Option 1: Add exclusion flag (Recommended)

Add new config flag `exclude_ingestion_pipeline_from_agent_stats` in `insights.proto`:

```protobuf
// Whether to exclude conversation source INGESTION_PIPELINE (enum: 8) from
// insights stats calculation. When true, only SOURCE_UNSPECIFIED (0) and
// optionally BOT (7) are included. Useful for customers who want to
// distinguish between agents using Cresta (realtime) vs those whose
// conversations are ingested via postcall jobs.
bool exclude_ingestion_pipeline_from_agent_stats = 19;
```

**Pros:**
- Simple, single-purpose flag
- Intuitive naming - clearly describes what it does
- No breaking changes

### Option 2: Explicit include list

Add `included_conversation_sources` repeated field:

```protobuf
// Explicit list of conversation sources to include in stats.
// If empty, defaults to [SOURCE_UNSPECIFIED, INGESTION_PIPELINE].
repeated cresta.v1.conversation.Conversation.Source included_conversation_sources = 19;
```

**Pros:**
- More flexible for future needs
- Single field controls all sources

**Cons:**
- More complex to implement
- Requires migration strategy for existing configs

## Implementation Plan

### Files to Modify

**cresta-proto:**
- `cresta/v1/config/features/insights.proto` - Add new field (field 20)
- `cresta/config/features.proto` - Add same field to legacy proto (keep in sync)

**config:**
- `json-schema/configv3/Insights.json` - **Auto-generated from proto** (no manual edit needed)

**go-servers:**
- `insights-server/internal/shared/common.go` - Add `ExcludeIngestionPipeline()` helper
- `insights-server/internal/analyticsimpl/retrieve_agent_stats.go` - Pass flag to query
- `insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go` - Modify SQL query
- `insights-server/internal/analyticsimpl/retrieve_conversation_stats.go` - Pass flag (if needed)
- `insights-server/internal/analyticsimpl/retrieve_hint_stats_clickhouse.go` - Pass flag (if needed)
- `insights-server/internal/analyticsimpl/retrieve_adherences_clickhouse.go` - Pass flag (if needed)

### SQL Query Change

From:
```sql
conversation_source in (0, 8[, 7])
```

To (when `exclude_ingestion_pipeline_from_agent_stats=true`):
```sql
conversation_source in (0[, 7])
```

## Affected Use Cases (Alaska)
- Customer Care Chat
- Cargo Chat
- Central Baggage Chat

## Setting the Flag for Customers

### Option 1: Config Wizard (Recommended)

1. Go to https://cresta-admin.cresta.ai/config/editor
2. Select the customer (e.g., Alaska Airlines)
3. Navigate to **Insights** section
4. Enable `excludeIngestionPipelineFromAgentStats`
5. Changes auto-sync to ConfigService DB

### Option 2: Edit customerv2 YAML (Legacy)

File: `config/customerv2/alaska-air-us-east-1.yaml`

```yaml
environments:
- features:
    insights:
      excludeIngestionPipelineFromAgentStats: true  # add this line
      enable: true
      useCrestaConversationCount: true
      # ... other existing flags
```

Then commit via PR - GitHub Actions will sync to ConfigService.

### Config Sync Flow

```
Config Wizard UI
    ↓ (auto-sync)
ConfigService DB
    ↓ (GitHub Action: sync_from_config_service)
customer_config_history/*.yaml
    ↓ (GitHub Action: sync_to_config_service_batch)
All regions (staging, us-west-2, us-east-1, etc.)
```

Note: `json-schema/configv3/Insights.json` is auto-generated from proto - no manual edit needed.

### Existing Flag Example (for reference)

Customers using `enableStatsForConversationSourceBot`:
- `kiavi-us-east-1.yaml`
- `mm-ohio-us-east-1.yaml`
- `brinks-care-voice.yaml`
- `walter-dev.yaml`

## Related Files

| File | Purpose |
|------|---------|
| `cresta-proto/cresta/v1/config/features/insights.proto` | Config proto definition (new location) |
| `cresta-proto/cresta/config/features.proto` | Config proto definition (legacy, keep in sync) |
| `cresta-proto/cresta/v1/conversation/conversation.proto:280` | Source enum |
| `config/json-schema/configv3/Insights.json` | Auto-generated from proto (no manual edit) |
| `config/customerv2/alaska-air-us-east-1.yaml:45` | Alaska customer config |
| `go-servers/insights-server/internal/shared/common.go:1090` | `IncludeConversationSourceBot()` helper pattern |
| `go-servers/insights-server/internal/analyticsimpl/retrieve_agent_stats.go:224` | Flag usage in RetrieveAgentStats |
| `go-servers/insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go:168` | SQL query with source filter |

### APIs Using Conversation Source Filter

| API | File | Uses Bot Flag |
|-----|------|---------------|
| `RetrieveAgentStats` | `retrieve_agent_stats.go:224` | Yes |
| `RetrieveConversationStats` | `retrieve_conversation_stats.go:187` | Yes |
| `RetrieveHintStats` | `retrieve_hint_stats_clickhouse.go:286` | Yes |
| `RetrieveAdherences` | `retrieve_adherences_clickhouse.go:99` | Yes |
| `RetrieveAssistanceStats` | `retrieve_assistance_stats_clickhouse.go` | No (hardcoded) |
