WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (SELECT DISTINCT conversation_id, agent_user_id FROM (SELECT moment_annotation_d.conversation_id AS conversation_id,
    moment_annotation_d.conversation_start_time AS conversation_start_time,
    moment_annotation_d.agent_user_id AS agent_user_id,
    moment_annotation_d.moment_annotation_id AS moment_annotation_id,
    moment_annotation_d.moment_type AS moment_type,
    moment_annotation_d.knowledge_payload_knowledge_session_id AS knowledge_payload_knowledge_session_id FROM moment_annotation_d
   WHERE conversation_source IN (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND knowledge_payload_knowledge_session_id <> ''
     AND moment_type = 38  -- KA_GEN_AI_REPLY_USED

    AND ((agent_user_id IN (SELECT user_id FROM assistance_usage_agents))) -- Additional filters
) WHERE true)
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)