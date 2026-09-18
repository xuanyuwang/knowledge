from pathlib import Path
import subprocess,json
root=Path('/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/testdata')
out=Path('/tmp/convi7402-fixed')
ddl='''CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String, conversation_start_time DateTime64(6), moment_template_id String, metadata_string_value String, metadata_number_value Float64, metadata_bool_value Bool, update_time DateTime64(6)) ENGINE=Memory;
CREATE TABLE scorecard_d (scorecard_id String, scorecard_last_update_time DateTime64(6), conversation_id String, scorecard_submit_time DateTime64(6), manually_scored Bool, publish_time DateTime64(6), scorecard_time DateTime64(6), scorecard_template_id String) ENGINE=Memory;
CREATE TABLE score_d (conversation_id String, conversation_start_time DateTime64(6), agent_user_id String, scorecard_template_id String, scorecard_template_revision Int32, scorecard_id String, score_id String, criterion_id String, manually_scored Bool, numeric_value Float64, max_value Float64, percentage_value Float64, weight Float64, float_weight Float64, ai_value Float64, ai_scored Bool, auto_failed Bool, not_applicable Bool, scorecard_last_update_time DateTime64(6), scorecard_time DateTime64(6), scorecard_submit_time DateTime64(6), scorecard_publish_time DateTime64(6)) ENGINE=Memory;
'''
# Deliberately include revisions, duplicate annotations, absence, other values and an independent exclusion.
fixtures=[('selected','metadata-value-1',1,False),('other','other',1,False),('missing',None,1,False),('excluded','metadata-value-1',1,True),('old-selected-new-other','metadata-value-1',1,False),('old-other-new-selected','other',1,False),('outside-numeric','metadata-value-1',300,False)]
def insert(cid,template,val,num,ts='2021-01-03 00:00:00'):
 return f"INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('{cid}', '2021-01-02 00:00:00','{template}','{val}',{num},false,'{ts}');\n"
for kind in ['QaConversations','QAScoreStats']:
 q=(root/f'clickhouse_Retrieve{kind}_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql').read_text()
 setup=ddl
 for cid,val,num,excluded in fixtures:
  setup+=insert(cid,'moment_id_2','',num)
  if val is not None: setup+=insert(cid,'moment_id_1',val,0)
  if excluded: setup+=insert(cid,'moment_id_3','',0)
  template='TestRetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue'
  setup+=f"INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_template_id) VALUES ('s-{cid}','{cid}','2021-01-04 00:00:00','2021-01-02 00:00:00','{template}');\n"
  setup+=f"INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,conversation_start_time,scorecard_template_id,percentage_value,float_weight,agent_user_id) VALUES ('s-{cid}','{cid}','2021-01-04 00:00:00','2021-01-02 00:00:00','2021-01-02 00:00:00','{template}',80,1,'agent');\n"
 setup+=insert('old-selected-new-other','moment_id_1','other',0,'2021-01-05 00:00:00')+insert('old-other-new-selected','moment_id_1','metadata-value-1',0,'2021-01-05 00:00:00')+insert('selected','moment_id_1','metadata-value-1',0)
 for variant,query in [('exact-golden',q),('fixture-results',q.replace('LIMIT 50 OFFSET 50','LIMIT 50 OFFSET 0'))]:
  p=out/f'{kind}-{variant}.sql';p.write_text(setup+query+';')
  r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(p)],capture_output=True,text=True)
  (out/f'{kind}-{variant}.result').write_text(r.stdout+r.stderr)
  print(kind,variant,'exit',r.returncode,r.stdout,r.stderr[:1800])
