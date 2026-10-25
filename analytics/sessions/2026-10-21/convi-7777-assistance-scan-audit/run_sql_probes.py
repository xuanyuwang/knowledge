from pathlib import Path
import subprocess,json,re,uuid,datetime,collections,urllib.request,urllib.parse,urllib.error,types
ROOT=Path(__file__).resolve().parent
CONTAINER='go-servers-local-clickhouse-node1'
TOKEN=uuid.uuid4().hex[:8]
DBS={k:'assistance_scan_audit_'+TOKEN+'_'+k for k in ('conv','events')}
QUERIES=json.loads((ROOT/'generated-queries.json').read_text())
for q in json.loads((ROOT/'generated-queries-main-readers.json').read_text()):
 if q['Name'] in ('Summary','GenAI') and q['Mode'] in ('none','include','daily','two-day-daily'):
  q['Revision']='main';QUERIES.append(q)
RESULTS=[]

def call(query,db=None,multi=False):
 params={'allow_experimental_object_type':'1','join_use_nulls':'0','max_threads':'2'}
 if db:params['database']=db
 url='http://127.0.0.1:32768/?'+urllib.parse.urlencode(params)
 try:
  with urllib.request.urlopen(urllib.request.Request(url,data=query.encode()),timeout=20) as response: data=response.read().decode()
  return types.SimpleNamespace(returncode=0,stdout=data,stderr='')
 except urllib.error.HTTPError as error:
  return types.SimpleNamespace(returncode=error.code,stdout='',stderr=error.read().decode())

def execute(query,db=None,multi=False):
 result=call(query,db,multi)
 if result.returncode:raise RuntimeError(result.stderr[:1600])
 return result.stdout

def quote(value):
 if value is None:return 'NULL'
 if isinstance(value,bool):return '1' if value else '0'
 if isinstance(value,(float,int)):return str(value)
 if isinstance(value,list):return '['+','.join(quote(v) for v in value)+']'
 return "'"+str(value).replace('\\','\\\\').replace("'","\\'")+"'"
def bind(q):
 text=q['Query'];args=iter(q['Args'] or [])
 text=re.sub(r'\?',lambda _:quote(next(args)),text)
 assert next(args,None) is None
 return text

def insert(table,row,kind='conv'):
 sql='INSERT INTO '+table+' ('+','.join('`'+k+'`' for k in row)+') VALUES ('+','.join(quote(v) for v in row.values())+')'
 execute(sql,DBS[kind])

def schema():
 for kind,db in DBS.items():
  execute('CREATE DATABASE '+db)
  schema=(ROOT/'source/pr/shared/clickhouse/testing/schemas'/('ch_conv_schema.sql' if kind=='conv' else 'ch_events_schema.sql')).read_text()
  for table in ['conversation','message','action_annotation','moment_annotation','conversation_event','conversation_with_labels','analytics_event']:
   match=re.search(r'CREATE TABLE IF NOT EXISTS `'+table+r'`[^\n]*\n\((.*?)\n\)\nENGINE',schema,re.S)
   if not match:continue
   execute('CREATE TABLE '+table+'_d ('+match[1]+') ENGINE=Memory',db)
  execute('CREATE TABLE assistance_usage_population (conversation_id String,agent_user_id String) ENGINE=Memory',db)
  execute('CREATE TABLE assistance_usage_agents (user_id String) ENGINE=Memory',db)
  execute('CREATE TABLE moment_annotation_mv_by_metadata_d (conversation_id String,conversation_start_time DateTime64(6), moment_template_id String,metadata_string_value String,metadata_number_value Float64,metadata_bool_value Bool,update_time DateTime64(6)) ENGINE=Memory',db)

def reset():
 for db in DBS.values():
  tables=execute('SHOW TABLES',db).splitlines()
  for table in tables:execute('TRUNCATE TABLE '+table,db)

def seed(evidence_time,create_time='2026-10-01 00:00:00',summary_copy=None,event_conversation=True,message=True,metadata=True,metadata_value='yes'):
 canonical='2026-10-01 00:00:00'
 for kind in DBS:
  if kind=='conv' or event_conversation:
   insert('conversation_d',{'conversation_id':'c','agent_user_id':'1','conversation_start_time':canonical,'conversation_end_time':'2026-10-01 01:00:00','update_time':canonical},kind)
  insert('assistance_usage_population',{'conversation_id':'c','agent_user_id':'1'},kind)
  insert('assistance_usage_agents',{'user_id':'1'},kind)
  if metadata:insert('moment_annotation_mv_by_metadata_d',{'conversation_id':'c','conversation_start_time':canonical,'moment_template_id':'audit','metadata_string_value':metadata_value,'update_time':canonical},kind)
 if message:insert('message_d',{'conversation_id':'c','agent_user_id':'1','conversation_start_time':canonical,'message_id':'message'})
 base={'conversation_id':'c','agent_user_id':'1','conversation_start_time':evidence_time,'create_time':create_time}
 insert('action_annotation_d',dict(base,action_annotation_id='summary-generation',summarization_payload_is_valid_summary=1,action_type=24,policy_source_type=1,knowledge_payload_knowledge_session_id='session'))
 insert('moment_annotation_d',dict(base,moment_annotation_id='genai',moment_type=38,knowledge_payload_knowledge_session_id='session'))
 insert('moment_annotation_d',dict(base,moment_annotation_id='hint',adherence_moment_annotation_id='sdx',adherence_moment_adherence_type=1,adherence_type=2,adherence_action_annotation_id='hint-action',adherence_action_policy_source_type=1,adherence_action_type=1,adherence_action_adherence_type=1))
 for ident,event_type,extras in [('kb',13,{}),('suggestion',4,{}),('hint-used',9,{'action_type':13,'action_policy_source_type':1}),('summary-copy',16,{'summarization_payload_data_type':1})]:
  row=dict(base,event_id=ident,event_type=event_type,**extras)
  if ident=='summary-copy' and summary_copy:row['conversation_start_time']=summary_copy
  insert('conversation_event_d',row)
 notes=dict(base,event_id='notes',event_type=5,event_name='VoiceAgentAssist.NoteTaking.Click.CopyEntity',event_payload='{}')
 insert('conversation_event_d',notes,'events')
 workflow=dict(base,event_id='workflow',event_type=4,event_payload=json.dumps({'usedKbDocument':{'document':'doc','docType':'2','sessionMetadata':{'sessionId':'session'}}}))
 insert('conversation_event_d',workflow,'events')
 # The metadata paths use the conversations projection with typed payload columns.
 insert('conversation_event_d',dict(base,event_id='notes-conv',event_type=5,**{'payload.used_note_taking.data_type':1}))
 insert('conversation_event_d',dict(base,event_id='workflow-conv',event_type=4,**{'payload.used_kb_document.doc_id':'doc','payload.used_kb_document.doc_type':2,'payload.used_kb_document.session_id':'session'}))
 insert('analytics_event_d',{'agent_user_id':'1','event_time':evidence_time,'event_name':'ChatAgentAssist.SmartCompose.Insert.SmartComposeText','event_id':'smart','event_payload':json.dumps({'smartComposeInserted':{'smartComposeText':'text'}})},'events')

def run_query(q,scenario):
 metadata=q['Mode'] in ('include','exclude','both')
 kind='events' if q['Name'] in ('Notes','Workflow','SmartCompose','Guided Workflow') and not metadata else 'conv'
 sql=bind(q)
 result=call(sql+' FORMAT JSONEachRow',DBS[kind])
 rows=[json.loads(line) for line in result.stdout.splitlines()] if not result.returncode else []
 record={'scenario':scenario,'revision':q.get('Revision','pr'),'feature':q['Name'],'mode':q['Mode'],'cluster':kind,'rows':rows,'error':result.stderr.strip() if result.returncode else None}
 RESULTS.append(record)
 return record

def run():
 schema()
 cases=[('Aligned','2026-10-01 00:00:00',{}),('MetadataDoesNotMatch','2026-10-01 00:00:00',{'metadata_value':'no'}),('BeforeStartMicrosecond','2026-09-30 23:59:59.999999',{}),('BeforeEndMicrosecond','2026-10-01 23:59:59.999999',{}),('AtEnd','2026-10-02 00:00:00',{}),('CopiedTimePlusSevenDays','2026-10-08 00:00:00',{}),('CopiedTimeMinusFourteenDays','2026-09-17 00:00:00',{}),('ArrivalFourteenDaysLater','2026-10-01 00:00:00',{'create_time':'2026-10-15 00:00:00'}),('SummarySplitTimes','2026-10-01 00:00:00',{'summary_copy':'2026-10-01 01:00:00'}),('MissingEventsConversation','2026-10-01 00:00:00',{'event_conversation':False}),('MissingMessage','2026-10-01 00:00:00',{'message':False}),('CrossDayCopies','2026-10-02 00:00:00',{})]
 for scenario,stamp,kwargs in cases:
  reset();seed(stamp,**kwargs)
  for q in QUERIES:
   if q.get('Revision')=='main' and scenario not in ('Aligned','SummarySplitTimes','CrossDayCopies'):continue
   if q['Mode'] in ('none','new') or (scenario in ('Aligned','CrossDayCopies') and q['Mode'] in ('daily','two-day-daily')) or (scenario in ('Aligned','MissingEventsConversation','MetadataDoesNotMatch') and q['Mode'] in ('include','exclude','both')):
    run_query(q,scenario)
 # Corrected latest conversation version is outside the request, but old in-range row survives prefiltering.
 reset();seed('2026-10-01 00:00:00')
 insert('conversation_d',{'conversation_id':'c','agent_user_id':'1','conversation_start_time':'2026-10-03 00:00:00','conversation_end_time':'2026-10-03 01:00:00','update_time':'2026-10-04 00:00:00'})
 run_query(next(q for q in QUERIES if q['Name']=='Population'),'LatestConversationMovedOutside')
 for scenario,stamp in [('FractionalBeforeStart','2026-10-01 00:00:00.250000'),('FractionalBeforeEnd','2026-10-01 00:00:01.250000')]:
  reset();seed(stamp)
  run_query(next(q for q in QUERIES if q['Mode']=='fractional'),scenario)
 # Millisecond boundaries lose their fractions in the shared SQL formatter (source-level experiment).
 (ROOT/'probe-results.json').write_text(json.dumps(RESULTS,indent=2)+'\n')
 errors=[r for r in RESULTS if r['error']]
 print(json.dumps({'queries':len(RESULTS),'errors':len(errors),'databases':DBS,'error_examples':errors[:2]},indent=2))

try:run()
finally:
 for db in DBS.values():call('DROP DATABASE IF EXISTS '+db)
