-- Read-only checks against auth DB advent-health, profile us-east-1.
-- Run with the connect-customer-auth-db skill; no connection string belongs here.
SET default_transaction_read_only = on;
SET statement_timeout = '15s';

-- Exact ticket examples: direct memberships matching ListUsersForAnalytics profile/group rules.
SELECT u.full_name, u.username, m.group_id, m.group_display_name, m.role_id, m.role_ids
FROM auth.users u JOIN auth.user_group_memberships_all m USING(customer_id,user_id)
WHERE u.customer_id='advent-health'
  AND u.user_id IN ('2f8a738e43909145','e1994597854e5a82','63c4a5971f5ab6fc','d9b6bbfc43480903','c178a5e0ff34fa00','aab8df84999ddc82')
  AND NOT m.is_indirect AND m.group_type=1 AND NOT m.cresta_managed
  AND (m.group_profile_ids IS NULL OR m.group_profile_ids && ARRAY['us-east-1'])
ORDER BY u.full_name,m.group_id;

-- Potential label mismatches, not a count of users proven visible in a date-filtered Leaderboard.
-- Actual UserType exclusions are DEV_USER=3 and EXTERNAL_PARTY_USER=6.
WITH memberships AS (
  SELECT m.* FROM auth.user_group_memberships_all m JOIN auth.users u USING(customer_id,user_id)
  WHERE m.customer_id='advent-health' AND NOT m.is_indirect AND m.group_type=1 AND NOT m.cresta_managed
    AND u.type NOT IN (3,6)
    AND (m.group_profile_ids IS NULL OR m.group_profile_ids && ARRAY['us-east-1'])
    AND EXISTS(SELECT 1 FROM auth.user_profile_configs p WHERE p.customer_id=u.customer_id AND p.user_id=u.user_id AND p.profile_id='us-east-1' AND p.profile_status=1)
), counts AS (
  SELECT user_id,count(*) AS direct_team_count,
    count(*) FILTER(WHERE '1'=ANY(role_id)) AS agent_team_count,
    (array_agg('1'=ANY(role_id) ORDER BY group_id))[1] AS first_team_has_agent_role
  FROM memberships GROUP BY user_id
)
SELECT count(*) FILTER(WHERE agent_team_count>0) AS active_users_with_agent_team,
  count(*) FILTER(WHERE agent_team_count=1 AND direct_team_count>1) AS single_agent_team_plus_other_teams,
  count(*) FILTER(WHERE agent_team_count=1 AND direct_team_count>1 AND NOT first_team_has_agent_role) AS predicted_wrong_single_agent_team,
  count(*) FILTER(WHERE agent_team_count>1) AS multiple_agent_teams
FROM counts;

-- Parent/child relationship explains the separate manager-only expectation.
SELECT parent.display_name AS parent_team, child.display_name AS child_team
FROM auth.group_group_memberships gg
JOIN auth.groups parent ON parent.customer_id=gg.customer_id AND parent.group_id=gg.parent_group_id
JOIN auth.groups child ON child.customer_id=gg.customer_id AND child.group_id=gg.child_group_id
WHERE gg.customer_id='advent-health'
  AND (child.display_name IN ('Brittni Howard','Anita Soltis','Angie Luckey-Succes') OR parent.display_name IN ('Christina Gusby','Angie Luckey-Succes'))
ORDER BY parent_team,child_team;
