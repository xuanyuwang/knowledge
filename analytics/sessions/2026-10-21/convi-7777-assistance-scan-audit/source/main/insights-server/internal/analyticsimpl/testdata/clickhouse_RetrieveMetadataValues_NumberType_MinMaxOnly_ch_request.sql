SELECT 
    min(metadata_number_value) AS min,
    max(metadata_number_value) AS max
FROM 
    metadata_moment_value_count_mv_d
WHERE 
    moment_template_id = '1'
