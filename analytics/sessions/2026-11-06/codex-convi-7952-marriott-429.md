# CONVI-7952 Marriott Insights 429 investigation

Date: 2026-11-06 America/Toronto
Primary source repo: /Users/xuanyu.wang/repos/go-servers
Source context: main checkout at 35c364ff698e86e4d8eef77f4504eb85dc76c5cc; read-only investigation. Related config: origin/master 2950821491a909db3f923742767574d436d5f7b0. No product code edits.
Primary domain: analytics; primary subdomain: shared-analytics-platform.

## Outcome

Direct read-only reproduction establishes an oversized auth ListUsers response for the affected users' combined management hierarchy. This exceeds the default 4 MiB gRPC client receive limit and provides a concrete explanation for the analytics ResourceExhausted surface. The ticket's rate/concurrency-limit hypothesis should be corrected. No quota increase, permission change, or server deployment was performed.

## Access setup

Followed https://crestalabs.slack.com/archives/C08M88ES630/p1793885099585279. cresta-cli 0.162.0 already supports --auth=cresta. Installed HashiCorp Vault CLI 2.2.1 with brew tap hashicorp/tap and brew install hashicorp/tap/vault. Ran cresta-cli startmyday --skip-brew --skip-docker --skip-helm --skip-codeartifact --skip-gcloud; AWS SSO login completed. Initial Vault request denied marriott-prod because the current identity lacked the customer-space Okta group. User manually obtained Marriott access and both subsequent database probes succeeded.

- Auth: configv3/prod/marriott/config.yaml -> AWS us-east-1-prod, cluster auth-us-east-1-prod, database marriott, DB user marriott-iam.
- App: configv3/prod/marriott/us-east-1/config.yaml deployment.sqlConn -> AWS us-east-1-prod, cluster us-east-1-prod, database marriott-us-east-1, DB user marriott-us-east-1-iam.
- Both connections used cresta-cli connstring --auth=cresta --read-only. PGOPTIONS additionally forced default_transaction_read_only=on and a 20-second statement timeout; SELECT current_setting('transaction_read_only') returned on. No app tables were queried beyond this connection check.
- Credentials used: CLI-managed AWS SSO and Vault-brokered Marriott read-only DB identities; CLI-issued Marriott OAuth token for read-only auth RPCs. The default AWS SSO profile/session entry was read and cleared before setup. No explicit alternative AWS profile was selected. No SSH key was selected or opened. Tokens/connection URIs were held in process memory and excluded from output, files, clipboard, and command arguments; grpcurl expanded the bearer header from a process environment variable.
- Targeted git fetch origin master refreshed config after a broad fetch encountered unrelated case-colliding remote branch names; checkout unchanged.

## Affected accounts verified in PostgreSQL and GetUser

- Mary Tubbs active: user 90a3669c9bd81d5, username mary.tubbs@marriott.com. Inactive duplicate: 516d52dcacfe287f, username metub088@marriott.com, same email, active_state 2. The duplicate does not define the active account's scope.
- Heather Ganey active: user 58bc185181ea031, username heather.ganey@marriott.com.
- Both have Manager 2nd and Insights Viewer on US & CAN CEC and Monica Bookout; Manager 2nd on Bethany Medeiros Czurak. Heather also has Manager on her own team. Marriott Associate is the customer role agent-no-library-4038797530 inheriting custom_agent.
- Recursive union of the management roots spans 121 groups and 2,426 distinct users for each active account. US & CAN CEC alone spans 89 groups / 1,973 users, so testing that one team alone misses the combined-scope failure.
- Management roots used in direct reproduction: 019abcc6-6631-7a46-a6cc-0b67a7775c0f (US & CAN CEC), 019abcc6-69d6-7209-b901-c554b12106a4 (Monica Bookout), 019e850d-f603-7913-af80-d984a1257bd7 (Bethany Medeiros Czurak).

## Direct reproduction and controlled verification

Using a CLI-issued Marriott operator token, grpcurl called read-only cresta.v1.user.UserService/ListUsers on auth.us-east-1-prod.internal.cresta.ai:443. Request: parent customers/marriott; filter.groups equal the three manager root resource names above; filter.includeIndirectGroupMemberships true. No impersonation of either customer user was performed. This reproduces their hierarchy's payload size under the operator identity; it is not an end-to-end customer-session replay.

| Client request | Result |
| --- | --- |
| US & CAN CEC alone, pageSize 5000, default receive cap | Success: 1,973 users; JSON output 7,140,530 bytes. JSON size differs from protobuf wire size. |
| Combined manager roots, pageSize 5000, default receive cap | ResourceExhausted: grpc: received message larger than max (4826132 vs. 4194304). |
| Combined roots, pageSize 5000, local receive cap 8388608 | Success: 2,426 users, no next page; JSON output 8,893,914 bytes. |
| Combined roots, pageSize 1000, receive cap 4194304 | Success through all three pages: 2,426 total users / 2,426 unique names. First page returns a next-page token. |

Only the local grpcurl client's receive limit/request page size changed. All calls were reads; no production configuration changed. Payloads were captured in process memory only and summarized as counts/bytes. Tests demonstrate two mitigation mechanisms; neither is a shipped fix or browser validation.

## Owning code and why root role overrides miss it

- shared/auth/acl_helper.go:144: GetResourceACL immediately returns root access for server/super-admin identities, skipping management expansion.
- shared/auth/acl_helper.go:170: a cache miss fetches the full ACL, including managed and peer users, before applying caller-specific filters. RESOURCE_ACL_USE_CACHE is true in the checked-in standard production Insights HelmRelease.
- shared/auth/acl_helper.go:342: managed-user expansion calls public ListUsers with PageSize 5000 and follows next-page tokens; the peer-user branch also hardcodes 5000. It needs user names/IDs but downloads full user resources.
- insights-server/internal/backendservices/authuser.go:12: the ACL helper's injected UserServiceClient uses grpcClient.DefaultGRPCServiceConfig without a specific larger response allowance.
- shared-go v0.2.599 framework/client/grpc/client_option.go:22 (locally available dependency source): default MAX_GRPC_CLIENT_BYTES is 4 MiB. Larger call options alone may still be constrained by the service config; WithServiceConfigMaxResponseBytes documents this explicitly.
- insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go:105 calls ParseUserFilterForAnalytics, which invokes the resource ACL helper before running score aggregation.
- insights-server/internal/analyticsimpl/retrieve_closed_conversations.go:69 invokes ApplyResourceACLToAttributeFilter before Elasticsearch Query.
- flux-deployments/apps/insights-server/releases/03-prod-main/helmrelease-insights-server.yaml configures MAX_GRPC_BYTES to 52428800 for the Insights server/local gateway, but that is distinct from an outbound auth client's receive limit. The Flux checkout was read only; live reconciliation was not inspected.

Thus smaller successful analytics responses do not establish anything about the size of an internal auth response. Empty application response bodies also cannot exclude failed inbound gRPC response handling.

## Recommendation and remaining checks

Prefer reducing ACL user-list page size, with regression coverage that proves all pages preserve the exact allowed user set. A page size of 1,000 passed this current real-data fixture. An endpoint-specific larger receive allowance on the injected auth client is another immediate mitigation; 8 MiB passed locally. Avoid a global receive-limit increase without assessing other callers. Longer term, use an ID-only or slim ACL membership endpoint instead of downloading complete user resources for ID extraction.

Before shipping: select the mitigation, cover pagination and ACL filtering for managed/peer users, verify the actual client service-config limit, deploy, then retest Mary and Heather's real customer sessions on both affected surfaces. Smaller pages are not a permanent byte guarantee for arbitrary future user-resource growth. Customer-session replay, original historical trace attribution, production rollout, and browser validation remain pending.

## Sources and status

- Ticket: https://linear.app/cresta/issue/CONVI-7952 (Todo, assigned Xuanyu Wang when read).
- Support account context: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793883238738489.
- Access setup: https://crestalabs.slack.com/archives/C08M88ES630/p1793885099585279.
- Primary proof in this note is the independent direct auth RPC reproduction, database identity/hierarchy reads, and local source inspection. Observability was inspected separately in chat; raw telemetry is not included in this record.
- No source/config/customer-data changes, issue edits, Slack messages, commits, pushes, or deployments were made. Knowledge files only were updated in the repository.


## Follow-up: evaluate ListUsersForAnalytics migration

User proposed evaluating whether an ongoing user-API migration would fix the payload failure. Read the current CONVI-7871 description: it targets ListGroups/ListGroupsForAnalytics and explicitly preserved public managed-user/peer-user lookups. User confirmed no existing user-migration plan and agreed it can be added to that ticket.

Read-only live comparison used the same three Marriott management roots with transitive membership:

- Public ListUsers, pageSize 1000: pages of 1000, 1000, 426; 2426 unique IDs.
- Internal ListUsersForAnalytics, includeIndirectGroupMemberships=true, includeInactiveUsers=true, agentOnly=false, profileIds omitted: 2426 unique IDs, one page, no next offset.
- Set comparison: identical; zero IDs unique to either result.
- Repeated the internal call with an explicit grpcurl -max-msg-sz 4194304: success, 2426 users, no next offset. This establishes that this response fits the same receive cap that rejected the public 5000-user request. JSON output size is not wire protobuf size and is not used as evidence of the receive threshold.
- CLI-issued operator token only; no customer impersonation. Credentials stayed in process memory/environment. No raw user response was persisted or printed. No additional AWS profile or SSH credentials selected.

Conclusion: migrating the shared ACL managed-user and peer-user branches can address the reproduced Marriott response-size failure. A group-only migration or parser-only migration cannot remove those calls. The parser initial population already calls ListUsersForAnalytics (common_user_filter.go:625-633), and later calls the shared helper (line829). Both QA and Closed Conversations can benefit from a shared helper migration.

Implementation requirements identified from current source:

1. Managed users: existing managedGroupIDs, transitive=true; peers: direct existing agentOnlyGroupIDs, transitive=false. Retain optional userIDs filters and caller inclusion/selected-audience behavior.
2. Include inactive=true, agentOnly=false, no profileIds, follow all nextPageOffset values. Existing public default state is active plus inactive; analytics defaults active-only.
3. Internal service requires server authentication. Public ListUsers runs appendUserAccessFilters based on propagated caller; internal implementation does not perform equivalent caller filtering. Verify caller-scope parity, especially ordinary members and custom roles; operator ID equality does not prove that generally.
4. Analytics implementation hardcodes DEV_USER/EXTERNAL_PARTY_USER exclusions and does not inspect user_types in the checked source. Review intended differences rather than assume full API interchangeability.
5. Analytics uses a default five-minute cache, and LiteUser still includes memberships. Validate freshness and large membership fanout; there is no universal response-byte guarantee.

Sources: shared/auth/acl_helper.go:327-400; auth/internal/service/user/action_list_users.go:72-83,323-337; auth/internal/service/user/access_validation_util.go:32; auth/internal/service/nonpublic/user/action_list_users_for_analytics.go:86-255; cresta-proto/cresta/nonpublic/user/internal_user_service.proto:32,102-155. Main source context remains unchanged. No product implementation or deployment performed.


User authorized adding the user migration to CONVI-7871. Updated its summary, planned managed/peer lookup flow and data-source table, and added section 6 with the Marriott comparison and migration requirements. The save call returned a transport error; a fresh get_issue confirmed every intended edit was persisted (updatedAt 2026-11-06T16:04:52.507Z). No retry mutation was needed. Official ticket status/title/assignee unchanged; CONVI-7952 itself was not edited. Earlier no-issue-edit statements in this note describe the initial investigation, before this follow-up authorization.
