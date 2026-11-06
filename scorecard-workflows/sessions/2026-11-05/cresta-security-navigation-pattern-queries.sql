-- Follow-up bounded path samples
SET statement_timeout='60s'; SET lock_timeout='3s'; SET default_transaction_read_only=on;
WITH paths AS MATERIALIZED (
SELECT e->>'navigationPath' AS path,e->>'lastOnlineTime' AS last_online_time FROM auth.users u CROSS JOIN LATERAL jsonb_array_elements(u.login_status->'applicationStatus') e WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a'
), biggest AS MATERIALIZED (SELECT * FROM paths ORDER BY octet_length(path) DESC NULLS LAST LIMIT 5)
SELECT octet_length(path) AS path_bytes,left(path,100) AS first_100_chars,right(path,100) AS last_100_chars,length(path)-length(replace(path,'/','')) AS slash_count,length(split_part(path,'/',1)) AS profile_chars,left(split_part(path,'/',1),60) AS profile_sample,length(split_part(path,'/',2)) AS usecase_chars,left(split_part(path,'/',2),60) AS usecase_sample,length(split_part(path,'/',3)) AS page_chars,left(split_part(path,'/',3),80) AS page_sample,last_online_time FROM biggest;

-- Follow-up whole-segment repetition checks
SET statement_timeout='60s'; SET lock_timeout='3s'; SET default_transaction_read_only=on;
WITH paths AS MATERIALIZED (
SELECT e->>'navigationPath' AS path,e->>'lastOnlineTime' AS last_online_time FROM auth.users u CROSS JOIN LATERAL jsonb_array_elements(u.login_status->'applicationStatus') e WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a'
), parts AS MATERIALIZED (SELECT split_part(path,'/',1) AS profile,split_part(path,'/',2) AS usecase,split_part(path,'/',3) AS page,last_online_time FROM paths WHERE path IS NOT NULL AND path<>'')
SELECT left(profile,40) AS profile,left(usecase,40) AS usecase,count(*) AS entries,count(*) FILTER (WHERE page<>'' AND translate(page,'A','')='') AS all_a_pages,sum(octet_length(page)) FILTER (WHERE page<>'' AND translate(page,'A','')='') AS all_a_page_bytes,min(last_online_time) FILTER (WHERE page<>'' AND translate(page,'A','')='') AS earliest_a_status,max(last_online_time) FILTER (WHERE page<>'' AND translate(page,'A','')='') AS latest_a_status FROM parts GROUP BY profile,usecase;
WITH paths AS MATERIALIZED (
SELECT e->>'navigationPath' AS path FROM auth.users u CROSS JOIN LATERAL jsonb_array_elements(u.login_status->'applicationStatus') e WHERE u.customer_id='cresta-security' AND u.user_id='87fd42472e35f08a'
)
SELECT octet_length(path) AS path_bytes,octet_length(translate(split_part(path,'/',3),'A','')) AS non_a_bytes FROM paths ORDER BY path_bytes DESC NULLS LAST LIMIT 10;
