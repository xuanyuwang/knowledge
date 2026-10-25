WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (SELECT DISTINCT conversation_id, agent_user_id FROM (SELECT moment_annotation_d.moment_annotation_id AS moment_annotation_id, moment_annotation_d.create_time AS create_time, moment_annotation_d.update_time AS update_time, moment_annotation_d.conversation_id AS conversation_id, moment_annotation_d.agent_user_id AS agent_user_id, moment_annotation_d.conversation_start_time AS conversation_start_time, moment_annotation_d.behavior_id AS behavior_id, moment_annotation_d.policy_id AS policy_id, moment_annotation_d.adherence_action_annotation_id AS adherence_action_annotation_id, moment_annotation_d.adherence_type AS adherence_type, moment_annotation_d.adherence_action_type AS adherence_action_type, moment_annotation_d.adherence_action_adherence_type AS adherence_action_adherence_type, moment_annotation_d.adherence_action_policy_id AS adherence_action_policy_id FROM moment_annotation_d 
 
 WHERE adherence_type = 2
 AND adherence_action_annotation_id <> ''
 AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
 AND adherence_action_policy_source_type=1 -- TODO: add policy_source_type into moment_annotation table
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND ((agent_user_id IN (SELECT user_id FROM assistance_usage_agents))) -- Additional filters
 ) WHERE (adherence_action_type = 1 AND adherence_action_adherence_type = 1) OR adherence_action_type = 21 UNION ALL SELECT DISTINCT conversation_id, agent_user_id FROM (SELECT conversation_event_d.event_id AS event_id, conversation_event_d.create_time AS create_time, conversation_event_d.update_time AS update_time, conversation_event_d.conversation_id AS conversation_id, conversation_event_d.agent_user_id AS agent_user_id, conversation_event_d.conversation_start_time AS conversation_start_time, conversation_event_d.action_policy_id AS action_policy_id, conversation_event_d.action_annotation_id AS action_annotation_id, conversation_event_d.event_type AS event_type, conversation_event_d.action_type AS action_type, conversation_event_d.action_adherence_type AS action_adherence_type FROM conversation_event_d 
 
 WHERE event_type = 9
 AND (conversation_source in (0, 8, 7) OR conversation_source IS NULL)
 AND action_policy_source_type=1
 AND is_dev_user = 0
 AND agent_user_id <> ''
 AND ((agent_user_id IN (SELECT user_id FROM assistance_usage_agents))) -- Additional filters
 ) WHERE action_type IN (13, 14))
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)