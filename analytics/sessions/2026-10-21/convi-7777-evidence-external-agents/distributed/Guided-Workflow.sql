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
			gw_conversation_events AS (
				SELECT DISTINCT
					agent_user_id,
					conversation_id,
					conversation_start_time,
					event_type,
					event_name,
					JSONExtractString(
						JSONExtractString(
							toJSONString(event_payload),
							'usedKbDocument'),
						'document') AS guided_workflow_name,
					JSONExtractString(
						JSONExtractString(
							toJSONString(event_payload),
							'usedKbDocument'),
						'docType') AS doc_type,
					JSONExtractString(
						JSONExtractString(
							JSONExtractString(
								toJSONString(event_payload),
								'usedKbDocument'),
							'sessionMetadata'),
						'sessionId') AS session_metadata_id
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
				guided_workflow_name AS guided_workflow_name,
				doc_type AS doc_type,
				session_metadata_id AS session_metadata_id
			FROM
				conversations JOIN gw_conversation_events USING (conversation_id)
			) WHERE event_type = 4 AND doc_type = '2' AND guided_workflow_name <> '')
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)