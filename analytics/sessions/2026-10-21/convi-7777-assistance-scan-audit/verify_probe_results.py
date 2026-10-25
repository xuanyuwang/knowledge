from pathlib import Path
import json
root=Path(__file__).resolve().parent
rows=json.loads((root/'probe-results.json').read_text())
checks=[]
def check(condition,name):
 assert condition,name
 checks.append(name)
def get(scenario,feature,mode='new',revision='pr'):
 return next(r for r in rows if (r['scenario'],r['feature'],r['mode'],r['revision'])==(scenario,feature,mode,revision))
check(all(not row['error'] for row in rows),'all captured SQL probes execute without errors')
features=['KB search','Chat Suggestions','GenAI','Summary','Hints','Notes','Guided Workflow']
for scenario in ['Aligned','BeforeStartMicrosecond','BeforeEndMicrosecond','AtEnd','CopiedTimePlusSevenDays','CopiedTimeMinusFourteenDays','ArrivalFourteenDaysLater']:
 for feature in features:check(len(get(scenario,feature)['rows'])==1,f'{scenario}: new {feature} preserves eligible pair')
for scenario in ['BeforeStartMicrosecond','AtEnd','CopiedTimePlusSevenDays','CopiedTimeMinusFourteenDays']:
 for feature,key in [('KB','total_kb_search_count'),('Suggestions','total_suggestions_count'),('Notes','total_note_taking_used_count'),('Workflow','total_gw_used_count'),('GenAI','gen_ai_answer_used_count'),('Summary','used_summarization_count')]:
  values=get(scenario,feature,'none')['rows']
  check(sum(int(v.get(key,0)) for v in values)==0,f'{scenario}: legacy {feature} filters copied start outside range')
check(int(get('SummarySplitTimes','Summary','none')['rows'][0]['used_summarization_count'])==0,'Summary exact-time internal join loses mismatched copy in legacy')
check(not get('SummarySplitTimes','Summary')['rows'],'Summary exact-time internal join loses mismatched copy in new API')
check(int(get('CrossDayCopies','GenAI','two-day-daily')['rows'][0]['gen_ai_answer_used_count'])==0,'GenAI usage day without message bucket disappears despite two-day scan')
check(get('CrossDayCopies','Notes','two-day-daily')['rows'][0]['truncated_time']=='2026-10-02 00:00:00','Notes legacy bucket comes from evidence copy rather than population')
check(get('CrossDayCopies','Summary','two-day-daily')['rows'][0]['used_summarization_count']=='0','Summary usage day without conversation bucket disappears')
for feature,legacy,key in [('Notes','Notes','total_note_taking_used_count'),('Guided Workflow','Workflow','total_gw_used_count')]:
 check(not get('MissingEventsConversation',feature)['rows'],feature+' new depends on Events conversation copy')
 check(not get('MissingEventsConversation',legacy,'none')['rows'],feature+' unfiltered legacy depends on Events conversation copy')
 check(get('MissingEventsConversation',legacy,'include')['rows'][0][key]=='1',feature+' matching metadata switches to Conversations and recovers row')
check(not get('MissingMessage','Population')['rows'],'Population remains message-anchored')
check(bool(get('LatestConversationMovedOutside','Population')['rows']),'Population prefilters before newest-row selection and retains older in-range version')
check(get('FractionalBeforeStart','KB','fractional')['rows'][0]['total_kb_search_count']=='1','Fractional lower bound floors and admits too-early evidence')
check(not get('FractionalBeforeEnd','KB','fractional')['rows'],'Fractional upper bound floors and drops still-eligible evidence')
for scenario in ['Aligned','SummarySplitTimes','CrossDayCopies']:
 for feature in ['GenAI','Summary']:
  for mode in ['none']+(['daily','two-day-daily'] if scenario in ('Aligned','CrossDayCopies') else []):
   check(get(scenario,feature,mode)['rows']==get(scenario,feature,mode,'main')['rows'],f'{scenario}: {feature} {mode} main reader matches PR reader')
check(bool(get('MetadataDoesNotMatch','Adherences','include')['rows']),'Adherences result survives nonmatching metadata request')
check(get('MetadataDoesNotMatch','Adherences','include')['rows']==get('MetadataDoesNotMatch','Adherences','none')['rows'],'Adherences discards parsed metadata predicates')
pruning=json.loads((root/'scan-pruning-results.json').read_text())['records']
check(all(r['rows']==pruning[0]['rows'] for r in pruning),'scan pruning experiment retains exactly the same eligible pair in all variants')
read=[int(r['local_metrics'][0]['read_rows']) for r in pruning]
check(read[0]>read[2]>read[1],'local read volume: unbounded exceeds padded exceeds exact')
(root/'verification.json').write_text(json.dumps({'sql_executions':len(rows),'checks_passed':len(checks),'checks':checks},indent=2)+'\n')
print(json.dumps({'sql_executions':len(rows),'checks_passed':len(checks),'local_read_rows':read}))
