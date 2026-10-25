from pathlib import Path
import subprocess,json,re,uuid,urllib.request,urllib.parse
root=Path(__file__).resolve().parent
name='assistance_scan_cost_'+uuid.uuid4().hex[:8]
container='go-servers-local-clickhouse-node1'
def query(sql,query_id=None,default_db=False):
 params={'log_queries':'1','max_threads':'2'}
 if not default_db:params['database']=name
 if query_id:params['query_id']=query_id
 url='http://127.0.0.1:32768/?'+urllib.parse.urlencode(params)
 with urllib.request.urlopen(urllib.request.Request(url,data=sql.encode()),timeout=20) as response:return response.read().decode()
query('CREATE DATABASE '+name,default_db=True)

try:
 query('CREATE TABLE conversation_event_d (conversation_id String,agent_user_id String,conversation_start_time DateTime64(6),event_id String,event_type Int32,action_policy_id String) ENGINE=MergeTree ORDER BY (toStartOfHour(conversation_start_time),agent_user_id,action_policy_id,conversation_id,event_id) SETTINGS index_granularity=1024')
 query("INSERT INTO conversation_event_d SELECT toString(number),toString(number%3+1),toDateTime64('2026-05-01 00:00:00',6)+toIntervalSecond(intDiv(number,1000)*86400+number%1000),toString(number),13,'' FROM numbers(180000)")
 query('CREATE TABLE assistance_usage_population (conversation_id String,agent_user_id String) ENGINE=Memory')
 query("INSERT INTO assistance_usage_population SELECT conversation_id,agent_user_id FROM conversation_event_d WHERE conversation_start_time >= '2026-10-01' AND conversation_start_time < '2026-10-02' AND agent_user_id='1' LIMIT 1")
 query('CREATE TABLE assistance_usage_agents (user_id String) ENGINE=Memory');query("INSERT INTO assistance_usage_agents VALUES ('1')")
 all_queries=json.loads((root/'generated-queries.json').read_text())
 new=next(q['Query'] for q in all_queries if q['Name']=='KB search' and q['Mode']=='new')
 # Keep the same eligible-agent predicate for all variants to isolate scan bounds.
 bounded=new.replace('agent_user_id IN (SELECT user_id FROM assistance_usage_agents)',"agent_user_id IN (SELECT user_id FROM assistance_usage_agents) AND conversation_start_time >= '2026-10-01 00:00:00' AND conversation_start_time < '2026-10-02 00:00:00'")
 padded=new.replace('agent_user_id IN (SELECT user_id FROM assistance_usage_agents)',"agent_user_id IN (SELECT user_id FROM assistance_usage_agents) AND conversation_start_time >= '2026-09-24 00:00:00' AND conversation_start_time < '2026-10-09 00:00:00'")
 variants={'new-unbounded':new,'same-cohort-exact-bounds':bounded,'same-cohort-seven-day-padding':padded}
 records=[]
 for label,sql in variants.items():
  plan=query('EXPLAIN indexes=1 '+sql)
  (root/(label+'.plan.txt')).write_text(plan)
  (root/(label+'.sql')).write_text(sql+'\n')
  query_id='scan-audit-'+uuid.uuid4().hex
  rows=query(sql+' FORMAT JSONEachRow',query_id)
  records.append({'variant':label,'query_id':query_id,'rows':[json.loads(line) for line in rows.splitlines()]})
 query('SYSTEM FLUSH LOGS')
 for record in records:
  stats=query("SELECT read_rows,read_bytes,result_rows,memory_usage,query_duration_ms FROM system.query_log WHERE type='QueryFinish' AND query_id='"+record['query_id']+"' FORMAT JSONEachRow")
  record['local_metrics']=[json.loads(line) for line in stats.splitlines()]
 (root/'scan-pruning-results.json').write_text(json.dumps({'version':query('SELECT version()').strip(),'row_count':180000,'engine':'single-node MergeTree synthetic fixture','records':records},indent=2)+'\n')
 print(json.dumps(records,indent=2))
finally:
 query('DROP DATABASE IF EXISTS '+name,default_db=True)
