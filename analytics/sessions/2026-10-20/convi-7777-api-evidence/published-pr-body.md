## Summary

Implement `RetrieveConversationAssistanceUsageStats` for backend Phase I: return measured eligible-conversation and assisted-conversation counts, rather than the previous population-only response. Reuse the shared population and all seven feature row-query families, restrict evidence to eligible conversation/agent pairs, and fail the whole request if a required evidence read fails.

Overall counts deduplicate conversation IDs; Agent rows count distinct conversations per agent; Team rows sum member-agent counts, including shared conversations and overlapping membership. DAILY/WEEKLY buckets use population conversation start time and existing timezone/week semantics. Explicit agent/activity restrictions remain applied even when the shared unrestricted-user optimization would omit them.

Depends on #34044 for the final Summary/GenAI helpers. Existing feature predicates and feature RPCs remain unchanged. Conversation details explicitly return `Unimplemented`; Director is outside this PR. Production-scale performance is unverified: seven queries each receive the eligible cohort, with concurrency limited to three and no timestamp partition bounds.

## Test plan

- [x] Full API Bazel suite with fresh isolated ClickHouse containers: `bazel test //insights-server/internal/analyticsimpl:action_retrieve_conversation_assistance_usage_stats_test`.
- [x] Focused race checks: `go test -race ./insights-server/internal/analyticsimpl -run '^TestConversationAssistanceUsage($|Validation)' -count=1`.
- [x] Real ClickHouse coverage for all ten V1 categories, exclusions, duplicate/overlapping evidence, wrong-agent evidence, stale copied timestamps, user/ACL restrictions, message-bearing eligibility and bot policy.
- [x] Complete response comparisons for Agent/Team attribution, overlapping memberships, daily/weekly buckets and empty results; required query/scan/iteration failures discard the response.
- [x] Gazelle, gofmt and `git diff --check`.
- [ ] Staging integration and representative production-scale latency/read-volume/memory benchmarks.

## AI knowledge files

- [x] Updated `insights-server/internal/analyticsimpl/CLAUDE.md` for the complete Phase I counts pipeline and deferred details.
- No domain documentation under `docs/` changed.
