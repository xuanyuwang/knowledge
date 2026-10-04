WITH sc AS (SELECT scorecard_id,scorecard_template_id,argMax(conversation_id,scorecard_last_update_time) AS cid
FROM scorecard_d
WHERE customer_id='sunbit' AND profile_id='us-east-1' AND usecase_id='care' AND scorecard_template_id IN ('019cffb8-041b-7748-837b-ebe698d36c08','019a87fd-2f52-75ef-b4e5-e5d7f1e50111') AND scorecard_time>='2026-09-01' AND scorecard_time<'2026-09-15'
GROUP BY 1,2), cv AS (SELECT DISTINCT conversation_id
FROM conversation_d
WHERE customer_id='sunbit' AND profile_id='us-east-1' AND usecase_id='care' AND conversation_end_time>='2026-09-01' AND conversation_end_time<'2026-09-15')
SELECT 'interaction time / no join' AS mode,countIf(scorecard_template_id='019cffb8-041b-7748-837b-ebe698d36c08') AS digital_support,countIf(scorecard_template_id='019a87fd-2f52-75ef-b4e5-e5d7f1e50111') AS resolutions
FROM sc
UNION ALL
SELECT 'conversation-ended / inner join',countIf(scorecard_template_id='019cffb8-041b-7748-837b-ebe698d36c08'),countIf(scorecard_template_id='019a87fd-2f52-75ef-b4e5-e5d7f1e50111')
FROM sc GLOBAL INNER JOIN cv ON sc.cid=cv.conversation_id FORMAT TSVWithNames;
