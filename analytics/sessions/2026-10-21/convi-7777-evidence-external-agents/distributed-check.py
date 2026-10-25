from pathlib import Path
import subprocess, re, json, time
D=['docker','--host','unix:///Users/xuanyu.wang/.colima/default/docker.sock']
def run(*args,input=None):
 p=subprocess.run(D+list(args),input=input,text=True,capture_output=True)
 if p.returncode: raise RuntimeError(p.stderr)
 return p.stdout
network='convi-7777-query-check'
nodes=['convi-7777-plan-a','convi-7777-plan-b']
root=Path('/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage')
out=Path('/tmp/convi-7777-external-agents-distributed-evidence');out.mkdir(exist_ok=True)
config='''<clickhouse><listen_host>0.0.0.0</listen_host><remote_servers><two_shards>'''+''.join(f'<shard><replica><host>{n}</host><port>9000</port></replica></shard>' for n in nodes)+'''</two_shards></remote_servers></clickhouse>'''
(out/'cluster.xml').write_text(config)
queries={}
for line in Path('/tmp/convi-7777-external-agents-sql.log').read_text().splitlines():
 if line.startswith('DIAGNOSTIC_SQL '):
  decoder=json.JSONDecoder();name,end=decoder.raw_decode(line[len('DIAGNOSTIC_SQL '):]);query,_=decoder.raw_decode(line[len('DIAGNOSTIC_SQL ')+end:].strip());queries[name]=query.replace('?',"'1'")
assert len(queries)==7,queries
run('network','create',network)
try:
 for n in nodes:
  run('run','--rm','-d','--name',n,'--network',network,'-e','CLICKHOUSE_SKIP_USER_SETUP=1','clickhouse/clickhouse-server:23.8.16.40-alpine')
 for n in nodes:
  for i in range(60):
   try: run('exec',n,'clickhouse-client','--query','SELECT 1');break
   except RuntimeError: time.sleep(1)
  else: raise RuntimeError('server did not start')
  run('cp',str(out/'cluster.xml'),n+':/etc/clickhouse-server/config.d/query-check.xml')
  run('exec',n,'clickhouse-client','--query','SYSTEM RELOAD CONFIG')
 def sql(n,q):return run('exec','-i',n,'clickhouse-client','--allow_experimental_object_type=1','--multiquery',input=q)
 for n in nodes:
  for schema,db in [('ch_conv_schema.sql','convs'),('ch_events_schema.sql','events')]:
   text=(root/'shared/clickhouse/testing/schemas'/schema).read_text()
   # Only raw and Distributed tables used by the seven evidence adapters.
   statements=[]
   for statement in text.split(';'):
    if not re.search(r'CREATE TABLE IF NOT EXISTS `(conversation|conversation_d|conversation_event|conversation_event_d|action_annotation|action_annotation_d|moment_annotation|moment_annotation_d)`',statement):continue
    statement=re.sub(r" ON CLUSTER '[^']+'",'',statement)
    statement=re.sub(r"ReplicatedReplacingMergeTree\('[^']*', '[^']*', ([^)]+)\)",r'ReplacingMergeTree(\1)',statement)
    statement=statement.replace("'{cluster}'","'two_shards'").replace('DATABASE_NAME_TO_REPLACE',db)
    statements.append(statement+';')
   sql(n,'CREATE DATABASE '+db+'; USE '+db+';\n'+'\n'.join(statements))
 # Evidence is on both shards; Summary generation and copy deliberately span shards.
 sql(nodes[0],"INSERT INTO convs.conversation_event (conversation_id,agent_user_id,conversation_start_time,event_type) VALUES ('eligible','1','2021-01-08',13),('ineligible','1','2021-01-08',13),('eligible','2','2021-01-08',13); INSERT INTO convs.action_annotation (conversation_id,agent_user_id,conversation_start_time,summarization_payload_is_valid_summary) VALUES ('summary','1','2021-01-08',1);")
 sql(nodes[1],"INSERT INTO convs.conversation_event (conversation_id,agent_user_id,conversation_start_time,event_type,summarization_payload_data_type) VALUES ('eligible','1','2021-01-08',13,0),('remote-only','1','2021-01-08',13,0),('summary','1','2021-01-08',16,1);")
 for db in ('convs','events'):
  sql(nodes[0],f'CREATE TABLE {db}.assistance_usage_population (conversation_id String, agent_user_id String) ENGINE=Memory; CREATE TABLE {db}.assistance_usage_agents (user_id String) ENGINE=Memory;')
 population='eligible\t1\nremote-only\t1\nsummary\t1\n'
 run('exec','-i',nodes[0],'sh','-c','cat > /tmp/population.tsv',input=population)
 run('exec','-i',nodes[0],'sh','-c','cat > /tmp/agents.tsv',input='1\n'+''.join(f'00000000-0000-4000-8000-{i:012d}\n' for i in range(8000)))
 def external(q,db): return run('exec',nodes[0],'clickhouse-client','--database',db,'--allow_experimental_object_type=1','--distributed_product_mode=deny','--prefer_localhost_replica=0','--query',q,'--external','--file=/tmp/population.tsv','--name=assistance_usage_population','--structure=conversation_id String, agent_user_id String','--format=TabSeparated','--external','--file=/tmp/agents.tsv','--name=assistance_usage_agents','--structure=user_id String','--format=TabSeparated')
 for name,q in queries.items():
  db='events' if name in ('Notes','Guided Workflow') else 'convs'
  (out/(name.replace(' ','-')+'.sql')).write_text(q)
  (out/(name.replace(' ','-')+'-plan.txt')).write_text(sql(nodes[0], 'USE '+db+'; EXPLAIN PLAN '+q+' SETTINGS prefer_localhost_replica=0, distributed_product_mode=\'deny\';'))
  result=external(q,db)
  (out/(name.replace(' ','-')+'-result.tsv')).write_text(result)
  if name=='KB search':assert set(result.splitlines())=={'eligible\t1','remote-only\t1'},result
  if name=='Summary':assert result.strip()=='summary\t1',result
  print(name,repr(result),flush=True)
 for n in nodes:sql(n,'SYSTEM FLUSH LOGS;')
 remote=sql(nodes[1],"SELECT query FROM system.query_log WHERE is_initial_query=0 AND type='QueryFinish' AND query NOT LIKE '%system.query_log%' FORMAT JSONEachRow")
 (out/'remote-queries.jsonl').write_text(remote)
 assert 'assistance_usage_population' not in remote,remote
 assert 'assistance_usage_agents' in remote,remote
 print('PASS: all seven queries execute with distributed_product_mode=deny; remote queries do not reference the eligible-pair table.',flush=True)
finally:
 for n in nodes:
  subprocess.run(D+['rm','-f',n],capture_output=True)
 subprocess.run(D+['network','rm',network],capture_output=True)
