from pathlib import Path
import subprocess, json, collections
root=Path(__file__).parent
# Deliberately independent membership oracle; tuples are (string, number, boolean).
X=('X',5,True); Y=('Y',10,False); E=('',0,False)
fixtures=[
 ('selected',[X],[X],False), ('other',[Y],[X],False),
 ('stale',[X,Y],[X],False), ('fresh',[Y,X],[X],False),
 ('missing',[],[],False), ('empty',[E],[Y],False),
 ('duplicate',[X,X],[X],False), ('voicemail',[X],[X],False),
 ('old_other',[Y],[X],False), ('old_selected',[X],[X],False),
 ('outwindow',[X],[X],False), ('excluded',[X],[X],True),
 ('wrong_b',[X],[Y],False), ('missing_a_good_b',[],[X],False),
]
convs=[]; annotations=[]
for i,(cid,a,b,c) in enumerate(fixtures):
 start='2025-12-28 12:00:00' if cid.startswith('old_') else '2026-01-05 12:00:00'
 end='2026-02-05 13:00:00' if cid=='outwindow' else '2026-01-05 13:00:00'
 convs.append(dict(id=cid,agent='u'+str(i%2),start=start,end=end,vm=int(cid=='voicemail'),handle=(i+1)*10,values={'A':a,'B':b,'C':[X] if c else []}))
 for template,values in [('A',a),('B',b),('C',[X] if c else [])]:
  for j,(s,n,v) in enumerate(values):
   annotations.append([cid,start,end,f'2026-01-06 00:00:0{j}',19,template,s,n,v])
# Duplicate an identical latest annotation and a message: sums must not fan out.
annotations.append(annotations[0][:])
def lit(v):
 if isinstance(v,bool):return str(int(v))
 if isinstance(v,str):return "'"+v.replace("'","''")+"'"
 return str(v)
def insert(table,rows):return 'INSERT INTO '+table+' VALUES '+','.join('('+','.join(map(lit,row))+')' for row in rows)+';\n'
schema='''CREATE TABLE moment_annotation_d (conversation_id String,conversation_start_time DateTime,conversation_end_time DateTime,create_time DateTime,moment_type Int32,moment_template_id String,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool) ENGINE=Memory;
CREATE TABLE message_d (conversation_id String,agent_user_id String,conversation_start_time DateTime,conversation_end_time DateTime,handle_time_secs Int64,is_voice_mail UInt8,is_dev_user UInt8,conversation_source Int32) ENGINE=Memory;
CREATE TABLE conversation_d (conversation_id String,agent_user_id String,conversation_start_time DateTime,conversation_end_time DateTime,is_dev_user UInt8,conversation_source Int32,update_time DateTime) ENGINE=Memory;
'''
messages=[[c['id'],c['agent'],c['start'],c['end'],c['handle'],c['vm'],0,0] for c in convs]
messages.append(messages[0][:]); messages[-1][4]=1
schema+=insert('moment_annotation_d',annotations)+insert('message_d',messages)+insert('conversation_d',[[c['id'],c['agent'],c['start'],c['end'],0,0,'2026-01-07 00:00:00'] for c in convs])
(root/'fixture.sql').write_text(schema)
def oracle(case,ended,stale,grouped):
 def test(v):
  s,n,b=v
  return {'String':s=='X','EmptyString':s=='','MultiValue':s in ('X','Y'),'BoolTrue':b,'BoolFalse':not b,'Zero':n==0,'Singleton':n==5,'ExclusiveInclusive':5<n<=10,'MultipleGroups':s=='X','MixedGroups':s=='X','IncludeOnly':s=='X','MissingOnly':False}[case]
 def match(values):return any(test(v) for v in (values if stale else values[-1:]))
 buckets=collections.defaultdict(list)
 for c in convs:
  if c['vm'] or not ('2026-01-01'<=c['end' if ended else 'start']<'2026-02-01'):continue
  a,b,cc=(c['values'][x] for x in ('A','B','C'))
  ok=not a or match(a)
  if case=='IncludeOnly':ok=match(a)
  if case=='MissingOnly':ok=not a
  if case=='MultipleGroups':ok=ok and (not b or match(b))
  if case=='MixedGroups':ok=ok and match(b) and not cc
  if ok:buckets[(c['start' if not ended else 'end'][:10]+' 00:00:00',c['agent']) if grouped else ()].append(c)
 out=[]
 for key,cs in buckets.items():
  row={'convo_count':len(cs),'user_count':len({c['agent'] for c in cs}),'aht_sec':sum(c['handle'] for c in cs)}
  if grouped:row.update(truncated_time=key[0],agent_user_id=key[1])
  out.append(row)
 return sorted(out,key=lambda r:json.dumps(r,sort_keys=True))
results=[]
queries=json.loads((root/'generated-queries.json').read_text())
for name,query in sorted(queries.items()):
 case,end,stale,grouped=name.split('_')
 ended=end=='Endtrue'; stale=stale=='Staletrue'; grouped=grouped=='Grouptrue'
 # New OR joins are checked with both ClickHouse missing-join representations.
 for nulls in [0,1]:
  if nulls and case in ('MissingOnly','MixedGroups'):continue # Independent missing-only null behavior is unchanged.
  sql=schema+query+f' SETTINGS join_use_nulls={nulls} FORMAT JSONEachRow;'
  proc=subprocess.run(['clickhouse','local','--multiquery'],input=sql,text=True,capture_output=True)
  expected=oracle(case,ended,stale,grouped)
  if proc.returncode:
   results.append({'case':name,'nulls':nulls,'error':proc.stderr[:1000]});continue
  rows=[{k:int(v) if k in ('convo_count','user_count','aht_sec') else v for k,v in json.loads(line).items()} for line in proc.stdout.splitlines()]
  rows.sort(key=lambda r:json.dumps(r,sort_keys=True))
  results.append({'case':name,'nulls':nulls,'pass':rows==expected,'actual':rows,'expected':expected})
(root/'runtime-results.json').write_text(json.dumps(results,indent=2)+'\n')
fails=[r for r in results if not r.get('pass')]
print(f'{len(results)-len(fails)}/{len(results)} runtime checks passed')
for r in fails[:5]: print(json.dumps(r))
raise SystemExit(bool(fails))
