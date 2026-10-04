# Walter-dev submitted scorecard permissions

- Date: 2026-09-23
- Primary domain: scorecard-workflows / permissions-and-visibility
- Source repo: /Users/xuanyu.wang/repos/go-servers
- Branch/worktree: main, existing checkout; no product edits.
- Requested scope: query scorecard `customers/cresta/profiles/walter-dev/scorecards/01a0cf24-657c-757f-9a3e-8311ffc133c9` and its template configuration before diagnosing why updates remain possible.
- Config: `origin/master:configv3/staging/cresta/walter-dev/config.yaml`; AWS account/cluster `voice-staging`, database `walter-dev`, derived from `deployment.sqlConn`.
- Refreshed origin/master with a targeted fetch because fetching every ref encounters a case-insensitive filesystem ref collision. No checkout changes.
- Credentials used: GitHub SSH identity `~/.ssh/id_ed25519` for config fetch; AWS SSO profiles `voice-staging_ro` (attempt) and `voice-staging_dev` (successful read-only connection). Reviewed their configurations before use. Connection URI remained in process memory only.
- Initial blocker was expired SSO. After user refreshed authentication, `_ro` reached PostgreSQL but failed PAM authentication for `walter-dev-iam`. The CLI's documented default `_dev` profile succeeded with `--read-only`, plus PostgreSQL `default_transaction_read_only=on` and a 30-second statement timeout.
- Scorecard and template query completed. No root cause established and no application data changed.

## Live database findings

Scorecard `01a0cf24-657c-757f-9a3e-8311ffc133c9` in `cresta/walter-dev`:

| Field | Value |
| --- | --- |
| created_at | 2026-09-23 16:41:04.171609 UTC |
| submitted_at / published_at | 2026-09-23 17:56:54.243328 UTC |
| updated_at | 2026-09-23 20:37:56.835711 UTC |
| creator_user_id / last_updater_user_id | d634c90a8748bcb2 |
| submitter_user_id / publisher_user_id | 9e151f0863143402 |
| agent_user_id | c831701043ba2dc7 |
| template_id | 0199ba42-d3af-76b9-95db-5611ef8d15e3 |
| template_revision | 98cfcf4c |
| conversation_id | 38698255-867b-454f-b40e-d1eada2a63d3 |
| manually_scored / score | true / 100 |
| scorecard_type / reference_scorecard_id | NULL / NULL |

`director.scorecard_templates` has 25 rows for this template. Ordered by `created_at DESC`, the newest is revision `98cfcf4c`, created 2026-09-19 02:26:58.731012 UTC, title `06.10.25`. This matches the scorecard's pinned revision. Its permissions are:

```json
{
  "templateEditors": [4, 7],
  "scorecardGraders": [3, 7, 8, 2, 14],
  "submittedScorecardEditors": {
    "users": ["customers/cresta/users/a07b887c425b17f4"]
  }
}
```

The latest template row is active (`status=1`, `type=1`, no deletion/deactivation). The separate `director.scorecard_template_revisions` row with resource_id `98cfcf4c` has NULL permissions. Do not infer a bypass from that NULL: local `action_update_scorecard.go` uses `dao.GetScorecardTemplates` and notes latest-template authorization; permission evaluator reads `ScorecardTemplates.Permissions`.

Query scope: exact scorecard customer/profile/resource_id; templates matched by its template ID; revision matched by template ID and pinned revision; compact follow-up listed template revision/created_at/title/permissions ordered newest first. All SQL was SELECT-only.

## Interpretation and next step

- A non-empty post-submit user allowlist is present, and the recorded last updater is not its sole permitted user.
- `updated_at` is later than submission, but these rows alone do not establish which API or authorization path made the change.
- Local main source `apiserver/internal/coaching/scorecards/permission_evaluator.go` gates the restriction on applicable scorecard scope and `disableEditingOnSubmittedScorecardsEnabled`. Local source/deployed staging equivalence is unverified.
- Next investigation: effective runtime flag, authenticated updater identity/roles, and deployed update authorization path. Stop this phase at the requested data/config inspection; do not assert a root cause yet.

## Follow-up: live flag and permission evaluation

User authorized checking the effective flag and calling the card permission endpoint. Completed read-only validation on 2026-09-23 using fresh customer OAuth via `cresta-cli cresta-token voice-staging cresta --bearer`. Bearer token was captured in memory; grpcurl received it via environment expansion, not a literal token argument. No AWS/SSH credentials were used in this follow-up. The shared helper allowlist only covers Training Simulator methods, so these explicitly requested, source-verified read-only methods were invoked through a scoped caller without changing the helper or its allowlist.

### Effective profile flag

Called `POST https://auth.us-west-2-staging.cresta.ai/v1/config:queryCustomerConfigs` (the regional auth/config route used by Director):

```json
{"customer":"customers/cresta","profiles":["customers/cresta/profiles/walter-dev"],"pageSize":1}
```

Response projection:

```json
{
  "versionId": "183c59e8-f6db-5603-8283-939022a064bc",
  "profile": "customers/cresta/profiles/walter-dev",
  "desktopAppConfig.legacyPublicConfig.featureFlags.disableEditingOnSubmittedScorecards": true
}
```

The evaluator's `GetProfileConfig` reads this profile-level flag (local source `permission_evaluator.go:372`). This is a live ConfigService read, not only Git configuration. The apiserver's process cache was not inspected.

### Permission endpoint

Endpoint: `grpc-cresta-api.voice-staging.internal.cresta.ai:443`.
Method: `cresta.v1.coaching.CoachingService/EvaluateScorecardsPermissions`.

```json
{
  "parent": "customers/cresta/profiles/walter-dev",
  "scorecardNames": ["customers/cresta/profiles/walter-dev/scorecards/01a0cf24-657c-757f-9a3e-8311ffc133c9"],
  "permissions": ["SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD"],
  "requesterUserName": "customers/cresta/users/d634c90a8748bcb2"
}
```

Complete response:

```json
{
  "results": [{
    "scorecardName": "customers/cresta/profiles/walter-dev/scorecards/01a0cf24-657c-757f-9a3e-8311ffc133c9",
    "permissions": [{
      "permission": "SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD",
      "allowed": true
    }]
  }]
}
```

Repeated with requester `customers/cresta/users/a07b887c425b17f4` (allowlisted user); identical response (`allowed: true`). The authenticated caller remains the CLI user; `requesterUserName` explicitly selects the evaluated identity. No impersonated mutation was issued.

### Live user roles

Called `cresta.v1.user.UserService/GetUser` on the same apiserver endpoint with `{"name":"customers/cresta/users/<id>","profileId":"walter-dev"}`. Authentication factors were not requested. Results projected to name/roles:

- Last updater `d634c90a8748bcb2`: AGENT, MANAGER, ADMIN, **SUPER_ADMIN**, USER_ADMIN, CI_ADMIN, QA_ADMIN, QA_SPECIALIST, OPERA_ADMIN, MANAGER_2ND.
- Allowlisted user `a07b887c425b17f4`: AGENT, ADMIN, **SUPER_ADMIN**, USER_ADMIN, CI_ADMIN, QA_ADMIN, QA_SPECIALIST, OPERA_ADMIN, MANAGER_2ND, PROCESS_SPECIALIST, CONFIG_ADMIN.
- Submitter `9e151f0863143402`: MANAGER, **SUPER_ADMIN**, QA_ADMIN, QA_SPECIALIST, VA_ADMIN, MANAGER_2ND, AI_AGENT_TESTER.

### Explanation

The recorded updater is a SUPER_ADMIN. That role explicitly bypasses the submitted-editor allowlist at `apiserver/internal/coaching/scorecards/permission_evaluator.go:223-228`, then receives an unconditional SUPER_ADMIN grant from `edit_permission.go:92-94`. `action_update_scorecard.go:128` calls the same permission adapter/evaluator as the evaluation RPC via `submitted_scorecard_permissions.go`.

This is intentional behavior from commit `58b737c63b142f99ebced1f5897ec3ad8dce6009`, September 2, 2026, `[CONVI-7612] Let Cresta admins bypass the submitted scorecard editor allowlist (#31889)`. Inspected local main HEAD `6e9999f0ad45c0e900c3860cbdb48ae2862fe40c`; deployed commit was not separately resolved.

Verdict: live permission result agrees with the documented SUPER_ADMIN exemption even with the flag enabled. Both tested users are SUPER_ADMIN, so this does not provide negative-control coverage for non-admin users. We did not replay UpdateScorecard, modify roles/config/data, or retrieve the original update trace; historical requester roles/cache state at 20:37 UTC were not independently reconstructed.

### Routing observations

- ConfigService reflection on the apiserver endpoint was available, but a valid call returned `Unimplemented: unknown service`; this was a routing issue, not a disabled flag.
- Internal ConfigService gRPC endpoints in voice/chat staging timed out during reflection.
- The older `api-config.staging.cresta.ai` README route returned HTTP 404. The regional endpoint derived from the CLI token issuer and Director's `AuthServiceBase` worked.
- GetUser requires `profileId` on this serving path; initial calls without it returned InvalidArgument.
