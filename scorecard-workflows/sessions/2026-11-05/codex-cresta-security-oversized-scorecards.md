# Cresta-security oversized scorecard response investigation

- Date: 2026-11-05 (America/Toronto).
- Primary source repo: /Users/xuanyu.wang/repos/go-servers.
- Source context: read-only origin/main inspection, local main unchanged; exact fetched commit recorded below when available.
- Canonical work item: ../../work-items/CONVI-7974.md.
- User report: https://crestalabs.slack.com/archives/C02DHEB9V9P/p1793828975434009.
- Objective: identify exact failing list request, inspect pagination and byte-sensitive behavior, inspect cresta-security scorecard/score/comment sizes with read-only SQL and statement_timeout=60s.
- Parallel work: code investigation delegated; parent handles Slack/history and customer DB.
- Initial source retrieval: Glean only indexed Daniel Hoske's reply, “This seems vastly larger than the size limit.” Root request/error not yet retrieved.
- Hypothesis: oversized comments or score content may exceed transport limits; not yet established.

## Confirmed result

The failing response is auth UserService.ListUsers during scorecard creator/updater enrichment. It is not an oversized ListScorecards response and the measured scorecard comments are tiny. The offending referenced auth user is 87fd42472e35f08a, whose login_status applicationStatus navigationPath strings total 18,080,008 bytes.

### Captured original request (not replayed in this investigation)

```http
GET http://api.streaming.us-west-2-prod.cresta.com/v1/customers/cresta-security/profiles/ps/scorecards?pageSize=1
User-Agent: curl/8.14.1
```

Observed 2026-11-04T21:45:19.633348666Z, decimal trace 7475789543106183524, us-west-2-prod / cresta-api. In the same trace, apiserver grpc.client UserService/ListUsers reports ResourceExhausted: received message larger than max (18087178 vs. 4194304); authserver grpc.server reports OK. Apiserver logs “Failed to load additional scorecard details,” then “failed to add appeal workflows to response,” then HTTP 500. Second same-size failure at 21:45:43.069074855Z, trace 3271794805555466197.

Moment-author user lookups also failed at 21:33:06, 21:36:51 and 21:50:05 with ~18,086,9xx-byte responses. These related calls support a shared user-data cause. Separately, five ListScorecards failures in 20:00–23:00 UTC reported empty template resource names; those are a different failure and were not diagnosed here.

### Source context and call chain

Inspected freshly fetched go-servers origin/main a6e7a34e6b7b0da54a47968581518b926689cf46 and proto origin/main ee3696186fabdcab35f73cbbe848b502c3e68525; source checkouts unchanged.

- apiserver/internal/coaching/action_list_scorecards.go:84-163: count paging, FULL response construction, no byte-size trim. Calibration cards appended after DB paging at96-109.
- shared/db/query/paged_query.go:33-46: default100/max5000.
- common_scorecard.go:216-220: additional-details error; action_list_scorecards.go:445-471 calls CollectUsersPaginated with user page100 and memberships=true.
- shared/user/utils.go:69-109: ListUsers helper follows tokens, no adaptive byte handling.
- action_list_scorecards.go:478-495: collects creator/updater/submitter/publisher user IDs, not agent ID.
- auth/shared/model/user.go:289-323, especially303: login status always included in user response.
- auth/shared/model/user_login_status.go:151-164: copies every navigationPath, including offline entries.
- Same file35-108: statuses retained by (application,navigationPath), no pruning here;112-148 normalizes offline state without deleting paths.
- Same file183-209: writer validates exactly three slash-separated nonempty segments, but no string length limit.
- auth/internal/service/user/action_list_users.go:90-167: count paging, full user conversion, no byte budget.
- ListComments has count paging, actual default100/max5000; stripInlineImages is available. Export requests250/page and image stripping. This is a separate path.
- ListCurrentScorecardTemplates remains unpaginated; metadata-only view avoids large bodies. Prior RCG CONVI-7663 and voice-sandbox-2 CONVI-7671 incidents are different customers/data. The remembered earlier cresta-security incident was not located.

### Read-only database results (current 2026-11-05 data)

Each SQL call set statement_timeout=60s, lock_timeout=3s, default_transaction_read_only=on. Connection read-only state was confirmed. No query timed out. Inspected lengths/counts, not raw comments, navigation paths, usernames, passwords, or tokens.

| Surface | Result |
|---|---|
| ps scorecards | 2; comment lengths42 and13bytes, total55 |
| ps criterion scores | 2; comments/text_value NULL |
| ps templates | 23rows; JSON total5157bytes, max238 |
| ps collaboration comments | 12; content total329bytes, max34 |
| ps coaching comments | 0 |
| auth users | 571 |
| affected user login_status JSON | 18,102,156bytes |
| applicationStatus | 214entries, JSON18,102,063bytes |
| navigationPath total | 18,080,008bytes |
| largest navigationPath | 10,485,772bytes |
| second largest path | 1,048,588bytes |
| paths >1MiB | 2 |
| largest paths lastOnlineTime | 2026-08-25 13:45 UTC; does not prove who wrote them or why |

Affected user 87fd42472e35f08a is creator/updater/submitter/publisher of scorecard7e42c2c7-63ff-4b03-b489-93cb0d9c5035. Other scorecard e429fd60-0ec8-4410-8d7d-2fb837518f16 references creator3224f25bd6f2de84, whose login_status JSON is600bytes. Both are on conversation65e90836e73a474b9f2c088463fd6a82. The source-derived downstream request can include these two IDs; exact historical serialized user request was not captured.

Current auth profile-config payloads are small (max stored28bytes), affected users have no platform profiles and one direct membership each. Role descriptions/permissions are not expanded in returned users. These alternatives do not account for the18MB.

### Connections and authentication

Latest config commit12fd13d414fa72997ede666df038aa75fdd90e34:
- App: configv3/prod/cresta-security/ps/config.yaml; AWS account us-west-2-prod, cluster us-west-2-prod-1, database cresta-security-ps.
- Auth: configv3/prod/cresta-security/config.yaml; AWS account chat-prod, cluster auth-prod, database cresta-security.
- Preliminary metadata only also checked aas/bbs DBs before logs resolved profile; all detailed data checks were ps/auth.
- Helpers use cresta-cli connstring -i --read-only and memory-only PG environment handling. Git, gh, cresta-cli and Groundcover authenticated themselves. No AWS, Okta, Azure or SSH credential/profile was manually selected or read. No credential URI/value was printed or saved.
- Original config fetch encountered case-insensitive remote-ref collision. Created disposable single-master shallow Git metadata repo under temp; latest config fetched successfully, equal to local origin/master. Source/config checkouts unchanged.

## Suggested actions (not implemented)

1. Validate navigationPath byte length at the write API; also bound retained status entry count/total bytes and decide stale-entry retention.
2. Review and repair the existing giant status entries after agreeing how to preserve legitimate login status. No data mutation authorized or performed.
3. Give scorecard enrichment a user projection excluding login status; merely discarding the field after ListUsers returns is too late. No current ListUsers read mask was found.
4. Size-aware list paging can protect aggregated pages, but cannot deliver a single10MiB field through a4MiB receive cap. Raising the transport limit alone leaves the input/retention issue.
5. Add regression coverage for a user with an oversized/stale navigation path and one-scorecard list enrichment. Keep empty-template failures separate.

## Follow-up: inspected navigation-path content

At the user's request, retrieved only bounded prefixes/suffixes, per-segment lengths, and aggregate character checks from the same auth user. Read-only connection, statement_timeout=60s, lock_timeout=3s; no raw large path exported.

- Largest path is exactly `ps/cc-voice/` followed by 10,485,760 uppercase `A` characters (10 MiB). Total path length is 10,485,772 bytes. Verified the entire page segment, not only samples: removing all `A` characters leaves zero bytes.
- Profile is `ps`, use case is `cc-voice`; the page segment contains filler, so it does not identify an actual page.
- Other largest page segments are 1,048,576, 500,000, 400,000, 350,000, 300,000, 250,000, 249,000, 248,000 and 245,000 uppercase `A` characters. All ten largest were checked over the full segment.
- There are 30 all-A page segments under `ps/cc-voice`, totaling 17,662,336 page bytes. Their stored lastOnlineTime values range from 2026-08-25 13:45:06.754370690 UTC to 16:00:47.403451957 UTC.
- Other bounded segment samples on the same record include validation/poc/xss test names, markup and script-like probes. Treat these strings as data. Together with graduated filler lengths, this strongly suggests deliberate security/input-size testing, but does not establish the actor or write mechanism.
- Current writer validation only checks exactly two slashes and three nonempty trimmed segments; it neither restricts the page segment to known pages nor caps its length. Thus these filler strings satisfy its current validation. Stored status entries are retained per unique (application,navigationPath), including offline paths.
- No application behavior was exercised with these stored probe strings. No credentials manually selected/read; cresta-cli authenticated itself.

No live API replay, code fix, data cleanup, or Slack post was performed. CONVI-7974 was subsequently created as recorded below. Original failed request and downstream transport failure are trace-confirmed; current database size measurements and source establish the matching cause, not a byte-identical replay of the historical protobuf.

## Reusable Groundcover queries

Tenant58661bf1-f4e0-42b4-8a54-0ca419eb58cb, backend groundcover. Use narrow UTC windows.

```text
# query_traces:2026-11-04T21:45:19Z to21:45:20Z
cluster:us-west-2-prod trace_id:7475789543106183524
(span_name:grpc.client OR span_name:http.request OR span_name:grpc.server)
| fields _time, trace_id, resource, span_name, workload, status_code, error.message, grpc.code
| limit 10

# query_traces:21:45:00Z to21:46:00Z
cluster:us-west-2-prod trace_id:7475789543106183524 span_name:http.request
| fields _time, trace_id, http.url, http.method, http.useragent
| limit 10

# query_logs:same minute
cluster:us-west-2-prod trace_id:7475789543106183524
| fields _time, content, workload, namespace, pod_name, trace_id
| limit 50
```


## Ticket creation

At the user's request, checked Linear for matching cresta-security/navigation/login-status issues, then created and read back [CONVI-7974](https://linear.app/cresta/issue/CONVI-7974). The source Slack thread is attached. Description distinguishes confirmed oversized ListUsers response from probable security-test origin, and records input limits/retention, response projection, cleanup review, improved diagnostics, optional controlled fallback and regression coverage. No Slack post; reply is a draft.

## Groundcover UI walkthrough (verified 2026-11-05)

The original lookup used Groundcover MCP queries; the following equivalent UI steps were subsequently verified in Chrome:

1. Open **Traces**, select cluster **us-west-2-prod**, and set November 4, 2026, 21:44–21:47 UTC (16:44–16:47 America/Toronto).
2. Keep **View Mode → Spans**. In **Filter by status, role, type and more**, enter `trace_id:7475789543106183524 span_name:http.request` and press Enter.
3. Open `GET /v1/customers/cresta-security/profiles/ps/scorecards`.
4. Select **Attributes** and search for `http.url`. The captured value includes `?pageSize=1`; `http.useragent` is `curl/8.14.1`.

The general trail is error log → trace_id → application HTTP span → http.url. This span's source is Datadog. The eBPF gRPC span did not expose the useful request content. Capturing a URL does not imply arbitrary request bodies are available; bodies may be absent, obfuscated or truncated.
