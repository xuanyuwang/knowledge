SELECT 
	metadata_string_value, uniqMerge(metadata_value_count)
FROM
	metadata_moment_value_count_mv_d
WHERE
	moment_template_id = '1'
	AND ((usecase_id IN ('usecase_id_1'))) -- USECASE_FILTER
GROUP BY 1
ORDER BY 2 DESC
LIMIT 1000
