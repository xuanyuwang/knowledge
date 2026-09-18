from pathlib import Path
import subprocess
p=Path('/tmp/convi7402-fixed')
# Reuse only the synthetic schema from the full-query fixture.
s=(p/'QaConversations-fixture-results.sql').read_text();ddl=s[:s.index('INSERT INTO')]
setup=ddl+"""INSERT INTO moment_annotation_mv_by_metadata_d VALUES ('old-other','2020-12-28','A','Y',0,false,'2021-01-05'),('old-selected','2020-12-28','A','X',0,false,'2021-01-05');
"""
for cid in ['old-other','old-selected','missing']:
 setup+=f"INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time) VALUES ('s-{cid}','{cid}','2021-01-05','2020-12-28','2021-01-05');\n"
 setup+=f"INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,scorecard_time,scorecard_submit_time,conversation_start_time,percentage_value,float_weight) VALUES ('s-{cid}','{cid}','2021-01-05','2020-12-28','2021-01-05','2020-12-28',80,1);\n"
for kind in ['conversations','stats']:
 f=p/f'execute-submit-{kind}.sql';f.write_text(setup+(p/f'submit-{kind}.sql').read_text()+';')
 r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(f)],capture_output=True,text=True)
 (p/f'submit-{kind}.result').write_text(r.stdout+r.stderr)
 print(kind,r.returncode,r.stdout,r.stderr[:1000])
