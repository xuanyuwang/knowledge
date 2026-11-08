-- Read-only inventory and criterion comparison for Delta's four templates.
-- Traverse JSON objects/arrays once, avoiding lax recursive JSONPath duplication.
WITH RECURSIVE revisions AS (
  SELECT t.*,
         row_number() OVER (PARTITION BY profile, resource_id ORDER BY created_at DESC) AS latest_rank
  FROM director.scorecard_templates t
  WHERE customer = 'delta-air-lines'
), nodes AS (
  SELECT profile, resource_id, revision, template AS node FROM revisions
  UNION ALL
  SELECT n.profile, n.resource_id, n.revision, child.value
  FROM nodes n
  CROSS JOIN LATERAL (
    SELECT value FROM jsonb_each(
      CASE WHEN jsonb_typeof(n.node) = 'object' THEN n.node ELSE '{}'::jsonb END
    )
    UNION ALL
    SELECT value FROM jsonb_array_elements(
      CASE WHEN jsonb_typeof(n.node) = 'array' THEN n.node ELSE '[]'::jsonb END
    )
  ) child
), criteria AS (
  SELECT profile, resource_id, revision,
         node->>'identifier' AS criterion_id,
         node->>'type' AS criterion_type
  FROM nodes
  WHERE jsonb_typeof(node) = 'object'
    AND node ? 'identifier' AND node ? 'type'
), criterion_sets AS (
  SELECT profile, resource_id, revision,
         count(*) AS criterion_objects,
         count(DISTINCT criterion_id) AS distinct_criterion_ids,
         count(*) FILTER (
           WHERE criterion_id ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
         ) AS uuid_criterion_objects,
         count(*) FILTER (
           WHERE criterion_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
         ) AS non_uuid_criterion_objects,
         array_agg(DISTINCT criterion_id ORDER BY criterion_id) AS ids,
         array_agg(DISTINCT criterion_id ORDER BY criterion_id) FILTER (
           WHERE criterion_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
         ) AS non_uuid_ids
  FROM criteria
  GROUP BY profile, resource_id, revision
), summaries AS (
  SELECT r.profile, r.resource_id, r.revision, r.title, r.type, r.status,
         r.usecase_ids, r.created_at, r.creator_user_id, r.deleted_at, r.deactivated_at,
         r.latest_rank, md5(r.template::text) AS template_json_md5,
         c.criterion_objects, c.distinct_criterion_ids,
         c.uuid_criterion_objects, c.non_uuid_criterion_objects,
         c.ids, c.non_uuid_ids
  FROM revisions r
  LEFT JOIN criterion_sets c USING (profile, resource_id, revision)
), pairs AS (
  SELECT a.title AS title_a, a.resource_id AS template_a,
         b.title AS title_b, b.resource_id AS template_b,
         a.template_json_md5 = b.template_json_md5 AS identical_template_json,
         ARRAY(SELECT unnest(a.ids) INTERSECT SELECT unnest(b.ids) ORDER BY 1) AS shared_criterion_ids
  FROM summaries a
  JOIN summaries b ON a.profile = b.profile AND a.resource_id < b.resource_id
  WHERE a.latest_rank = 1 AND b.latest_rank = 1
)
SELECT 'latest_templates' AS result_kind,
       jsonb_agg(jsonb_build_object(
         'resource_id', resource_id, 'revision', revision, 'title', title,
         'usecase_ids', usecase_ids, 'criterion_count', criterion_objects,
         'uuid_criteria', uuid_criterion_objects, 'non_uuid_criteria', non_uuid_criterion_objects,
         'non_uuid_ids', non_uuid_ids
       ) ORDER BY resource_id) AS result
FROM summaries WHERE latest_rank = 1
UNION ALL
SELECT 'template_history', jsonb_agg(to_jsonb(h) ORDER BY first_created_at)
FROM (
  SELECT resource_id, count(*) AS revision_count,
         min(created_at) AS first_created_at, max(created_at) AS last_created_at,
         count(*) FILTER (WHERE coalesce(cardinality(usecase_ids),0) = 0) AS empty_scope_revisions,
         min(uuid_criterion_objects) AS min_uuid_criteria, max(uuid_criterion_objects) AS max_uuid_criteria,
         min(non_uuid_criterion_objects) AS min_non_uuid_criteria,
         max(non_uuid_criterion_objects) AS max_non_uuid_criteria,
         (array_agg(creator_user_id ORDER BY created_at))[1] AS first_creator,
         array_agg(DISTINCT creator_user_id) AS all_creators
  FROM summaries GROUP BY resource_id
) h
UNION ALL
SELECT 'empty_scope_revisions', jsonb_agg(jsonb_build_object(
  'resource_id',resource_id,'revision',revision,'title',title,
  'created_at',created_at,'creator',creator_user_id,
  'criteria',criterion_objects,'non_uuid_criteria',non_uuid_criterion_objects,
  'template_json_md5',template_json_md5
) ORDER BY created_at)
FROM summaries WHERE coalesce(cardinality(usecase_ids),0) = 0
UNION ALL
SELECT 'latest_pairwise_comparison', jsonb_agg(to_jsonb(p) ORDER BY template_a, template_b)
FROM pairs p
UNION ALL
SELECT 'poc_ids_shared_with_other_template_history', to_jsonb(count(DISTINCT c.criterion_id))
FROM criteria c
WHERE c.resource_id <> '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a'
  AND c.criterion_id IN (
    SELECT criterion_id FROM criteria
    WHERE resource_id = '01a1a9fc-4b07-70c7-ae85-ae2ded1a923a'
  );
