# Validate PR #33993 restricted-user external-table review

## Request and verdict

Xuanyu requested validating [Cursor review 5604472273](https://github.com/cresta/go-servers/pull/33993#pullrequestreview-5604472273), specifically [Restricted users skip ext-table path](https://github.com/cresta/go-servers/pull/33993#discussion_r4340544803). Primary domain/subdomain: analytics / assistance-insights. Verified local and remote PR head both equal 0e6919128928f99c5601f1582e03fb9b630694df. Verdict: valid query-size failure, with a reasonable narrowly scoped population-query fix. The initial validation did not change product code or publish a GitHub reply; see the subsequently approved implementation below.

## State trace

1. FilterToAgentsOnly or ExcludeDeactivatedUsers makes the endpoint clone the resolved user filter and force ShouldQueryAllUsers=false (action file lines 73–77). This is required to retain the restricted user set and the empty-audience early return.
2. ApplyUserFilterFromResult copies FinalUsers into the request attribute when ShouldQueryAllUsers=false (common_user_filter.go lines 740–757).
3. The endpoint passes only enableExtTableForUserFilter to both ApplyUserFilterFromResult and buildConversationPopulationQuery (action file lines 78 and 90). The service flag defaults false unless its environment setting is true; deployed configuration was not inspected.
4. With false, buildUsersConditionAndArgs uses columnIn/IN (?) rather than an external user table (common_clickhouse.go lines 408–442). The shared population builder places user conditions in the message and conversation sides. clickhouse-go v2.34.0 expands ordinary positional slice arguments into SQL literals; these are not native typed server parameters.
5. Workflow automation already uses useExternalUsers = enableExtTableForUserFilter || restrictUserPopulation at both application and SQL construction (action_retrieve_workflow_automation_stats.go lines 58–66). This validates the cited sibling pattern. The historical claim that this was the original incident was not independently traced to a production incident; current shared-helper comments explicitly identify query-size avoidance as the purpose of the unrestricted-user optimization.

## Local reproduction

A temporary diagnostic method on the existing real-ClickHouse population suite supplied an AgentOnly directory of 8,001 eligible users: one seeded user and 8,000 synthetic UUID-length user IDs. The actual readConversationAssistanceUsagePopulation method ran with an otherwise identical request in two subtests. With the flag false it failed with ClickHouse code 62, Max query size exceeded, at position 262123. With the flag true it succeeded and returned exactly the seeded eligible conversation/agent row. Both expected outcomes passed; go test completed in 18.110s against the repository’s ClickHouse 23.8.16.40 test image. This proves a local size-triggered failure, not customer prevalence or a timeout benchmark. At initial validation, existing regression tests used only one eligible user, covering restriction correctness without reaching the size limit.

The [diagnostic fixture and filtered outcomes](convi-7777-restricted-users-review-evidence/) are retained here. The temporary source test was removed afterward; the API worktree is clean.

## Fix boundary and caveat

Recommend the proposed local useExternalUsers = flag || restrictedAudience value for both ApplyUserFilterFromResult and buildConversationPopulationQuery, retaining the existing forced ShouldQueryAllUsers=false and empty-audience behavior. That preserves the user set while changing how IDs reach ClickHouse. Add persistent regression coverage for large restricted lists with the global flag off, both role/activity restrictions and the empty audience.

This is not a complete endpoint-scale fix: action_retrieve_conversation_assistance_usage_stats_clickhouse.go lines 30–34 still builds inline agent_user_id IN (?) predicates for all feature families using agents from the selected conversation population. A large user directory does not necessarily mean many agents have selected conversations, but a sufficiently large actual eligible-agent set can independently grow evidence-query SQL. Address that transport separately while preserving the reduced-relation join design and avoiding a broad GLOBAL cohort broadcast. This related evidence-query risk was traced in source, not independently reproduced in this diagnostic.

Git/gh used their normal authentication; no AWS, Okta, Azure or SSH credential entry was selected or read.

## Existing API examples verified in current source

Xuanyu asked for other APIs using external user tables. Verified four concrete paths at PR head 0e6919128928f99c5601f1582e03fb9b630694df: RetrieveWorkflowAutomationStats (action lines 58–66; ClickHouse reader lines 61–70) forces the path for role/activity restrictions even when the shared flag is false; RetrieveConversationStats (ClickHouse reader lines 89–94 and 122) passes the shared flag into the same population builder and attaches its external tables; RetrieveKnowledgeBaseStats (ClickHouse reader lines 56 and 78) obtains the users table from parseClickhouseFilter and attaches it; RetrieveUserOutcomeStats (handler lines 216–225; query helper lines 221–227) explicitly builds the external users table when UseExtTable is true and references it from SQL. The latter three are flag controlled, not automatic list-size detection. All reuse buildUsersExtTable / attachExtTablesToContext and the agent_filter(user_id String) pattern. The existing ext_table_integration_test.go also defines a 10,000-seeded-user / 5,000-selected-user correctness case and a separately opt-in performance comparison; neither was rerun for this source lookup. No product changes or external communication.

## Approved adaptation

Xuanyu approved adapting the proposed fix. The endpoint computes restrictUserPopulation from agents-only or exclude-deactivated-users, preserves the forced ShouldQueryAllUsers=false, and passes useExternalUsers = shared flag OR restrictUserPopulation to both ApplyUserFilterFromResult and buildConversationPopulationQuery. No shared parser or other API changed; the seven feature-family queries are unchanged.

Extended the existing real-ClickHouse restriction test to six cases with the shared flag disabled: small agents-only and active-only audiences, large agents-only and active-only directories (8,001 eligible users each), and empty agents-only and active-only audiences. Both large-directory cases failed before implementation, then all six passed after it (focused Go run 17.925s). Exact population assertions still exclude the seeded nonselected users. The test is renamed to TestUserRestrictionsWithExternalTableFlagDisabled because the effective transport is now external despite the disabled shared flag. Filtered red/green outcomes are saved beside the original diagnostic. Gazelle and gofmt ran. Final full API Bazel validation and publication are recorded below.

The separate inline evidence-agent-list scalability concern remains open; this adaptation targets the reviewed population-query failure. The follow-up request/query-count optimization candidate remains independent. No GitHub review reply or thread-resolution action is performed.

Final validation: full API Bazel target passed in 37.3s (50.920s command elapsed). Gazelle produced no BUILD changes; gofmt and git diff --check passed. Commit `9cd953dddc7319f5fedac1e99e9efa5cb0baf9c3` contains only the population-reader transport change and regression tests.

Publication verified: #33993 is open at `9cd953dddc7319f5fedac1e99e9efa5cb0baf9c3`; the API worktree is clean, and its description retains the counting example and adds the restricted-user policy/regressions. At the final snapshot Codeowner check is pending; local full API Bazel success is independent of remote CI. The construction report was refreshed with current source snapshots and pins, and its embedded JavaScript passed syntax checks.
