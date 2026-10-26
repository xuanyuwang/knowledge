# CONVI-7208: Comment access roles reference

**Context**: Product decision for CONVI-7208 is to **not** apply comment access role filtering in the Group Calibration CSV export. This document explains what comment access roles are and where they are enforced elsewhere.

## Definition

On the coaching `Score` proto (`cresta-proto/cresta/v1/coaching/scorecard.proto`):

```proto
// Roles that can view the comment
repeated cresta.v1.auth.AuthProto.Role comment_access_roles = 10;
```

Stored in Postgres as `director.scores.comment_access_roles` (`smallint[]` of `AuthProto.Role` values, e.g. `AGENT`, `MANAGER`, `QA_ADMIN`).

A separate but related field exists on **collaboration comments** (`chat_comments.comment_access_roles`) for conversation transcript comments.

## Access rule

| `comment_access_roles` | Behavior |
|------------------------|----------|
| Empty / unset | Unrestricted — anyone who can load the score sees the comment text |
| Non-empty | Restricted — viewer must have **at least one** listed role to see comment text |

When the viewer lacks permission, the backend strips comment text (sets `Comment.Valid = false`); the score itself remains.

## Where enforced (go-servers)

### Scorecard criterion comments

**ListScorecards / GetScorecard** — `removeCommentsForUnauthorizedUsers` in `apiserver/internal/coaching/common_scorecard.go`:

- If `len(CommentAccessRoles) > 0` and viewer lacks any listed role → comment cleared before response

**Performance Insights QA conversations** — `fetchScoreCommentsFromDB` in `insights-server/.../retrieve_qa_conversations_clickhouse.go`:

- Filters out score records whose comments the viewer cannot access (unless super admin)

### Collaboration / transcript comments

**ListComments** — `apiserver/internal/collaboration/action_list_comments.go`:

- Filters comments unless viewer is super admin, has permitted role, is thread author, or comment is unrestricted

**ExportComment** — same role check before export

## Where NOT enforced

| Surface | Behavior |
|---------|----------|
| QM Report / Coaching Hub `ExportScorecards` | Reads comments directly from DB; no role filtering |
| Group Calibration session CSV (director) | Uses data from `ListScorecards` (may already be stripped); export does not re-check roles |
| Bulk scorecard export (temporal) | Includes `commentAccessRoles` in JSON; does not strip comments |

## How roles get set

- **On save**: persisted from API request via `mapScoresByCriterion` in `shared/scoring/scorecard_calculator.go`
- **Director scoring form**: `getScorecardDataFromForm` saves `comment` text but does **not** set `commentAccessRoles` — most scorecard criterion comments are stored with empty roles (unrestricted)
- **Collaboration comments**: users pick roles in `CreateCommentPopup` (`COMMENT_VIEW_ROLES`)

## CONVI-7208 implication

- No export-side role filtering in the CSV builder (consistent with QM export)
- If a score has restricted roles (e.g. `[AGENT]` only), `ListScorecards` may already strip the comment before the CSV builder sees it — exported cell would be blank for a QA admin downloader even without explicit export filtering
- Group calibration sessions are QA-admin-facing; most criterion comments have empty `comment_access_roles` in practice
