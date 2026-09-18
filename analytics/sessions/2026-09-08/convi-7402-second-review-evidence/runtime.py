from pathlib import Path
import subprocess,json
p=Path('/tmp/convi7402-review2')
prior=Path('/Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-09-08/convi-7402-fix-evidence')
s=(prior/'QaConversations-fixture-results.sql').read_text();ddl=s[:s.index('INSERT INTO')]
setup=ddl
expected=[]
for av in ['X','Y',None]:
 for bv in ['X','Y',None]:
  cid=f'{av or "absent"}-{bv or "absent"}'
  if av in ('X',None) and bv in ('X',None):expected.append(cid)
  setup+=f"INSERT INTO scorecard_d (scorecard_id,conversation_id,scorecard_last_update_time) VALUES ('s-{cid}','{cid}','2021-01-01');\n"
  setup+=f"INSERT INTO score_d (scorecard_id,conversation_id,scorecard_last_update_time,percentage_value) VALUES ('s-{cid}','{cid}','2021-01-01',80);\n"
  for template,val in [('A',av),('B',bv)]:
   if val is not None:
    setup+=f"INSERT INTO moment_annotation_mv_by_metadata_d (conversation_id,moment_template_id,metadata_string_value,update_time) VALUES ('{cid}','{template}','{val}','2021-01-01');\n"
for n in ['false','true']:
 f=p/f'execute-two-overlap-nulls-{n}.sql';f.write_text(setup+(p/f'two-overlap-nulls-{n}.sql').read_text())
 r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(f)],capture_output=True,text=True)
 (p/f'two-overlap-nulls-{n}.result').write_text(r.stdout+r.stderr)
 assert r.returncode==0,r.stderr
 actual=[line.split('\t')[0] for line in r.stdout.splitlines()]
 assert actual==sorted(expected),(actual,expected)
 print('two overlaps join_use_nulls='+n,'PASS',actual)
# Numeric range reference comes from the independent ES production converter.
s=(prior/'execute-generated-exclusive.sql').read_text();ddl=s[:s.index('WITH ')]
for lo in ['false','true']:
 for hi in ['false','true']:
  f=p/f'execute-bounds-{lo}-{hi}.sql';f.write_text(ddl+(p/f'bounds-{lo}-{hi}.sql').read_text())
  r=subprocess.run(['clickhouse','local','--multiquery','--queries-file',str(f)],capture_output=True,text=True)
  (p/f'bounds-{lo}-{hi}.result').write_text(r.stdout+r.stderr)
  assert r.returncode==0,r.stderr
  actual=r.stdout.splitlines();expected=json.loads((p/f'es-bounds-{lo}-{hi}.json').read_text())
  assert actual==expected,(actual,expected)
  print('bounds',lo,hi,'PASS versus ES range',actual)
