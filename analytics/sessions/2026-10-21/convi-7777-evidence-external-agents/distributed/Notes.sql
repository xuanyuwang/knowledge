WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (SELECT DISTINCT conversation_id, agent_user_id FROM (
			WITH
			conversations AS (
				SELECT DISTINCT
					conversation_id
				FROM
					conversation_d
				WHERE
					agent_user_id <> ''
					AND ((agent_user_id IN (SELECT user_id FROM assistance_usage_agents))) -- Additional filters
			),
			conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					event_name,
					event_id
				FROM
					conversation_event_d
				WHERE
					agent_user_id <> ''
					AND ((agent_user_id IN (SELECT user_id FROM assistance_usage_agents))) -- Additional filters
			)
			SELECT
				agent_user_id AS agent_user_id,
				conversation_id AS conversation_id,
				conversation_start_time AS conversation_start_time,
				event_type AS event_type,
				event_name AS event_name,
				event_id AS event_id
			FROM
				conversations
				JOIN conversation_events USING (conversation_id)
			) WHERE event_type = 5 AND event_name = 'VoiceAgentAssist.NoteTaking.Click.CopyEntity')
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)