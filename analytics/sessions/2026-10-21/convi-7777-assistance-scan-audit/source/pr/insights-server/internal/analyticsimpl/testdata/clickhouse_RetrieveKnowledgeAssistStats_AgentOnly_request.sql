WITH messages AS (
				SELECT
					agent_user_id,
					COUNT(DISTINCT agent_user_id) AS user_count
				FROM
					(SELECT message_d.conversation_id AS conversation_id,
    message_d.conversation_start_time AS conversation_start_time,
    message_d.agent_user_id AS agent_user_id,
    message_d.message_id AS message_id FROM message_d
   WHERE conversation_source IN (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''

    AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00') AND (agent_user_id IN ('1','2'))) -- Additional filters
) AS message_d
				GROUP BY agent_user_id
			),
			action_annotations AS (
				SELECT
					agent_user_id,
					COUNT(DISTINCT CASE WHEN action_type = 22 THEN action_annotation_id END) AS question_extraction_count,
					COUNT(DISTINCT CASE WHEN action_type = 24 THEN knowledge_payload_knowledge_session_id END) AS retrieval_article_count
				FROM
					(SELECT action_annotation_d.conversation_id AS conversation_id,
    action_annotation_d.conversation_start_time AS conversation_start_time,
    action_annotation_d.agent_user_id AS agent_user_id,
    action_annotation_d.action_annotation_id AS action_annotation_id,
    action_annotation_d.action_type AS action_type,
    action_annotation_d.knowledge_payload_knowledge_session_id AS knowledge_payload_knowledge_session_id FROM action_annotation_d
   WHERE conversation_source IN (0, 8)
    AND is_dev_user = 0
    AND agent_user_id <> ''
    AND knowledge_payload_knowledge_session_id <> ''
     AND action_type in (22, 24)  -- (RETRIEVAL_QUESTION, RETRIEVAL_ARTICLE)

    AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00') AND (agent_user_id IN ('1','2'))) -- Additional filters
) AS action_annotation_d
				GROUP BY agent_user_id
			),
			moment_annotations AS (
				SELECT
					agent_user_id,
					COUNT(DISTINCT knowledge_payload_knowledge_session_id) AS answer_used_count
				FROM
					(SELECT moment_annotation_d.conversation_id AS conversation_id,
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

    AND ((conversation_start_time >= '2021-01-01 00:00:00') AND (conversation_start_time < '2024-01-01 00:00:00') AND (agent_user_id IN ('1','2'))) -- Additional filters
) AS moment_annotation_d
				GROUP BY agent_user_id
			)
			SELECT
				messages.agent_user_id,
				action_annotations.retrieval_article_count AS gen_ai_answer_viewed_count,
				moment_annotations.answer_used_count AS gen_ai_answer_used_count,
				action_annotations.question_extraction_count AS knowledge_moments_detected_count,
				messages.user_count
			FROM
				messages
				LEFT JOIN action_annotations ON messages.agent_user_id = action_annotations.agent_user_id
				LEFT JOIN moment_annotations ON messages.agent_user_id = moment_annotations.agent_user_id
