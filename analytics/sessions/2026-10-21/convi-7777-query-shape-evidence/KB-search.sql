WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (SELECT DISTINCT conversation_id, agent_user_id FROM (
			SELECT
				conversation_event_d.conversation_id AS conversation_id,
				conversation_event_d.agent_user_id AS agent_user_id,
				conversation_start_time, event_id, event_type
			FROM
				conversation_event_d
			WHERE
				agent_user_id <> ''
				AND ((agent_user_id IN ('1'))) -- Additional filters
			) WHERE event_type = 13)
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)