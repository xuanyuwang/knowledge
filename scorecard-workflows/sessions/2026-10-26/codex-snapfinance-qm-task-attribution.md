# Snapfinance QM task attribution — ZD #24606

- Date: 2026-10-26 (America/Toronto)
- Primary source repo/worktree: `/Users/xuanyu.wang/repos/go-servers`, branch `main`; read-only source review.
- Request: https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793000511846639
- Customer asks who created the replacement “TASK - Service Standards” QM task in Coll Loan Voice around early October, and who archived the original, with timestamps. Latest updater alone is insufficient if later edits occurred.

## Current status

Live lookup is pending AWS SSO refresh. No database rows retrieved. The connect-customer-app-db skill fetched current config and resolved `origin/master:configv3/prod/snapfinance/us-west-2/config.yaml`, AWS account `us-west-2-prod`, DB cluster `us-west-2-prod`, database `snapfinance-us-west-2`. The program authenticated itself; no credential or AWS profile was explicitly selected. `cresta-cli connstring -i --read-only` failed refreshing its cached SSO token with `InvalidGrantException`. Asked user to refresh their normal login.

## Source evidence

- `apiserver/sql-schema/director/director-schema.sql:291`: `director.tasks` contains task ID, display name, created/updated timestamps, creator/last updater IDs, status, usecase ID, task content config, and last status change timestamp.
- `apiserver/internal/coaching/action_update_director_task.go:83-96`: every edit sets `updated_at` and `last_updater_user_id`; only a status transition changes `last_status_changed_at`. No separate historical actor column in the checked-in table. No task history/audit match found in the focused director-schema/coaching search; this does not establish absence of external audit logs or live DB history.

## Prepared first query

```sql
BEGIN READ ONLY;
SET LOCAL statement_timeout = '15s';
SELECT customer_id, profile_id, task_id, task_display_name, usecase_id,
       task_type, task_status, created_at, creator_user_id,
       updated_at, last_updater_user_id, last_status_changed_at,
       task_content_config
FROM director.tasks
WHERE customer_id = 'snapfinance'
  AND task_display_name ILIKE '%Service Standards%'
ORDER BY created_at;
COMMIT;
```

Next: retry connection after login; check live schema and matched task/template/usecase records; resolve user IDs; compare updated/status timestamps and inspect available history/audit evidence before attributing archive. Do not substitute latest updater for archival actor when a later edit exists. No customer identities or dates have been confirmed. No Slack reply posted.

## Successful retry after SSO refresh

User refreshed SSO and authorized retry. Connected successfully using the same skill's read-only connection; no credential/profile explicitly selected. Queried task records, selected non-secret user identity columns, schema metadata, task history entries, and triggers. Closed psql afterwards.

### Confirmed DB results

All four Coll Loan Voice (`col-loan-voice`) matches link to template `d164e950-580e-4796-960f-0feecd89a97b` and have name `TASK - Service Standards task`. Status enum: 2 active, 4 archived.

| Task ID | Current status | Creator | Created UTC | Latest updater | Last status change UTC | Updated UTC |
|---|---|---|---|---|---|---|
| 0197e5fc-2254-704f-8541-b29b97b4c7b9 | active | Fressy Vargas Alvarez | 2025-07-07 17:43:15.795446 | Karina Monestel Araya | NULL | 2026-10-15 21:42:51.221801 |
| 0199ca88-e2a7-7b20-9d3c-6e39efa97946 | archived | Maria Jose Jimenez Bolanos | 2025-10-09 19:53:12.615115 | Karina Monestel Araya | 2026-10-15 21:42:09.338630 | 2026-10-15 21:42:09.339363 |
| 0199ca8c-1796-7d96-adb0-3e8af6d93747 | archived | Maria Jose Jimenez Bolanos | 2025-10-09 19:56:42.774387 | Karina Monestel Araya | 2026-10-15 21:42:05.508265 | 2026-10-15 21:42:05.508808 |
| 0199cafe-323a-7cc2-bd22-9f3ee3c07282 | archived | Maria Jose Jimenez Bolanos | 2025-10-09 22:01:20.698324 | Karina Monestel Araya | 2026-10-15 21:42:02.214312 | 2026-10-15 21:42:02.214655 |

Identity mappings in app.users:
- f66d96f8236e7fb6: Fressy Vargas Alvarez, fvargas@snapfinance.com
- bb9f48db39cfaa81: Maria Jose Jimenez Bolanos, mjimenez@snapfinance.com
- 8b751f5b2bae03f8: Karina Monestel Araya, kmonestel@snapfinance.com

No matching October 2026 creation. The active row predates all archived rows, so current data does not establish the reported archive-and-recreate sequence. It would be inaccurate to call the active task an October replacement without a specific task ID or further evidence.

### Archive attribution limits

Archived rows' latest updates follow last status changes by only 733, 543 and 343 microseconds respectively, consistent with one save operation for each status change. Karina is the recorded latest updater. This is strong row-level evidence, but not an independently recorded historical actor. `app.history` query for these four IDs/source_table=director.tasks returned zero rows. A broader source_table ILIKE '%task%' query also returned zero. `pg_trigger` reports no non-internal triggers on director.tasks. Do not claim independent audit confirmation. No task audit history available in the inspected application DB.

### Additional read-only SQL

```sql
SELECT user_id, username, display_name, full_name, email
FROM app.users
WHERE customer_id = 'snapfinance'
  AND user_id IN ('f66d96f8236e7fb6','bb9f48db39cfaa81','8b751f5b2bae03f8');

SELECT source_table, count(*)
FROM app.history
WHERE customer_id = 'snapfinance' AND profile_id = 'us-west-2'
  AND source_table ILIKE '%task%'
GROUP BY source_table;

SELECT tgname, pg_get_triggerdef(oid)
FROM pg_trigger
WHERE tgrelid = 'director.tasks'::regclass AND NOT tgisinternal;
```

Lookup complete with explicit attribution and chronology limitations. No DB mutations or Slack posts.
