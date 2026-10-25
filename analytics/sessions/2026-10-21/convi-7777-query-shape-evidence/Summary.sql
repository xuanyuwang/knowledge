WITH assistance_usage_evidence AS (
SELECT conversation_id, agent_user_id FROM (WITH convo_with_valid_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					action_annotation_d
				WHERE
					agent_user_id <> ''
					AND summarization_payload_is_valid_summary = 1
					AND ((agent_user_id IN ('1'))) -- Additional filters
			),
			convo_with_used_summarization AS (
				SELECT DISTINCT
					conversation_id, conversation_start_time, agent_user_id
				FROM
					conversation_event_d
				WHERE
					agent_user_id <> ''
					AND event_type = 16 -- 16: USED_SUMMARIZATION
					AND summarization_payload_data_type IN (1, 2)  -- 1: CopyAllNotes, 2: CopySummarizationNote
					AND ((agent_user_id IN ('1'))) -- Additional filters
			)
SELECT DISTINCT conversation_id, agent_user_id
FROM convo_with_valid_summarization
INNER JOIN convo_with_used_summarization
USING (conversation_id, conversation_start_time, agent_user_id))
GROUP BY conversation_id, agent_user_id
)
SELECT conversation_id, agent_user_id
FROM assistance_usage_evidence
INNER JOIN assistance_usage_population USING (conversation_id, agent_user_id)