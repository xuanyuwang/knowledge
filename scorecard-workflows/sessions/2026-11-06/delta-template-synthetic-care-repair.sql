-- CONVI-7981: operator-run repair; not executed by Codex.
-- Expected: 5 revisions (3 POC / Agent Adherence, 2 Customer Experience).
BEGIN;
SET LOCAL lock_timeout = '5s';

UPDATE director.scorecard_templates
SET usecase_ids = ARRAY['synthetic-care']::text[]
WHERE customer = 'delta-air-lines'
  AND profile = 'us-west-2'
  AND resource_id IN (
    '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a',
    '01a1a9fc-4dbe-72b4-9e5e-8bfea0f3e902'
  )
  AND COALESCE(cardinality(usecase_ids), 0) = 0
  AND deleted_at IS NULL
  AND deactivated_at IS NULL
RETURNING resource_id, revision, title, usecase_ids;

-- Inspect the returned rows: expected UPDATE 5, all with {synthetic-care}.
-- Then run COMMIT; to persist, or ROLLBACK; if the result is unexpected.
