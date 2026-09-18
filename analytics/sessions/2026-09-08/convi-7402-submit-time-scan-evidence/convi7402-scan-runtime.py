from pathlib import Path
import subprocess,re,json
p=Path('/tmp')
q=(p/'convi7402-scan-TIME_RANGE_FILTER_TARGET_SUBMIT_TIME.sql').read_text()
old=Path('/Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-09-08/convi-7402-fix-evidence/QaConversations-fixture-results.sql').read_text()
setup=old[:old.index('INSERT INTO')]
for cid in ['selected','missing','other','excluded','outside']:
 submit='2020-12-28' if cid=='outside' else '2021-01-05'
 setup+=f"INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-{cid}','{cid}','2021-01-05','2020-12-28','{submit}');\n"
 setup+=f"INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight,agent_user_id) VALUES ('s-{cid}','{cid}','2021-01-05','2020-12-28','{submit}','2020-12-28',80,1,'agent');\n"
for cid in ['selected','missing','other','excluded','outside','no-score']:
 for template,value in [('include','selected')]+([] if cid=='missing' else [('overlap','other' if cid=='other' else 'selected')])+([('exclude','selected')] if cid in ['excluded','outside','no-score'] else []):
  setup+=f"INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('{cid}','2020-12-28','{template}','{value}',0,false,'2021-01-05');\n"
# Older selected value must not override the latest nonmatching annotation.
setup+="INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('other','2020-12-28','overlap','selected',0,false,'2021-01-01');\n"
def run(name, sql):
 f=p/f'convi7402-scan-{name}.sql';f.write_text(setup+sql)
 r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(f)],capture_output=True,text=True)
 (p/f'convi7402-scan-{name}.result').write_text(r.stdout+r.stderr)
 assert r.returncode==0,r.stderr
 return r.stdout.strip()
result=run('full',q)
assert result=='agent\t160\t2\t2\t2',result
print('Full generated query: PASS; older selected and missing conversations retained; other, excluded, outside rejected')
prefix=q.rsplit('\n\t\tSELECT',1)[0]
ctes=re.findall(r'(t[12]_moment_annotation_filter_\d+|moment_annotation_exclude_filter_\d+) AS \(',q)
checks=[]
for name in ctes:
 col='conversation_id' if name.startswith('t') else 'conversation_id_exclude_'+name.rsplit('_',1)[1]
 checks.append(f"SELECT '{name}', countIf({col} IN ('outside','no-score')) FROM {name}")
result=run('sources',prefix+'\n'+' UNION ALL '.join(checks))
assert len(result.splitlines())==6,result
assert all(line.endswith('\t0') for line in result.splitlines()),result
print('All six source CTEs: PASS; out-of-window scorecards and conversations without scores excluded')
empty=q.replace("'2021-01-01 00:00:00'", "'2022-01-01 00:00:00'").replace("'2021-01-31 00:00:00'", "'2022-01-31 00:00:00'")
assert run('empty',empty)=='', 'expected no result'
print('Empty candidate set: PASS')
