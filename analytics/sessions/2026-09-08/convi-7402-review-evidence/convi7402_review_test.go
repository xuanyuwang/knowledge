package analyticsimpl

import (
 "context"
 "os"
 "testing"
 "time"
 "strings"
 analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
 metadata "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
 momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
 "google.golang.org/protobuf/types/known/timestamppb"
)

func TestReview7402SharedMultiOverlap(t *testing.T) {
 a:=&momentpb.Moment{Name:"customers/c/profiles/p/moments/A",Type:momentpb.Moment_CONVERSATION_METADATA}
 b:=&momentpb.Moment{Name:"customers/c/profiles/p/moments/B",Type:momentpb.Moment_CONVERSATION_METADATA}
 tr:=&analyticspb.TimeRange{StartTimestamp:timestamppb.New(time.Date(2021,1,1,0,0,0,0,time.UTC)),EndTimestamp:timestamppb.New(time.Date(2021,1,31,0,0,0,0,time.UTC))}
 for _,tc:=range []struct{name string;incl,excl []*momentpb.Moment}{
  {"one-include-two-exclude",[]*momentpb.Moment{a},[]*momentpb.Moment{a,b}},
  {"two-include-one-exclude",[]*momentpb.Moment{a,b},[]*momentpb.Moment{a}},
  {"two-include-two-exclude",[]*momentpb.Moment{a,b},[]*momentpb.Moment{a,b}},
 } { t.Run(tc.name,func(t *testing.T){
  mg:=&analyticspb.MomentGroup{Moments:tc.incl,ExcludedMoments:tc.excl,MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}}}
  _,inc,exc,_,err:=parseClickhouseFilter(context.Background(),tr,&analyticspb.Attribute{MomentGroups:[]*analyticspb.MomentGroup{mg}},[]ClickhouseTable{messageTable},false)
  if err==nil { t.Errorf("unsupported OR accepted: include filters=%d exclude filters=%d; must reject or preserve OR",len(inc),len(exc)) }
 })}
}

func TestReview7402ExportOverlapSQL(t *testing.T){
 a:=&momentpb.Moment{Name:"customers/c/profiles/p/moments/A",Type:momentpb.Moment_CONVERSATION_METADATA}
 cases:=[]struct{name string;attrs []*analyticspb.MetadataValueAttribute;col string}{
  {"string",[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}},conversationStartTimeColumn},
  {"singleton",[]*analyticspb.MetadataValueAttribute{{NumericBin:&metadata.NumericBin{FromValue:5,ToValue:5,ToValueIsInclusive:true}}},conversationStartTimeColumn},
  {"exclusive",[]*analyticspb.MetadataValueAttribute{{NumericBin:&metadata.NumericBin{FromValue:5,ToValue:10,FromValueIsExclusive:true,ToValueIsInclusive:true}}},conversationStartTimeColumn},
  {"empty-oneof",[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{}}},conversationStartTimeColumn},
  {"raw-string",[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}},conversationEndTimeColumn},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
  _,_,overlap,err:=parseMomentConditionsForQAAttribute(nil,&analyticspb.QAAttribute{MomentGroups:[]*analyticspb.MomentGroup{{Moments:[]*momentpb.Moment{a},ExcludedMoments:[]*momentpb.Moment{a},MetadataValueAttributes:tc.attrs}}},false,tc.col)
  if err!=nil{t.Fatal(err)}
  cte,join,args:=buildQAMomentFilterQuery(overlap[0].selectedValueFilter,0,"LEFT JOIN","base.conversation_id","conversation_id_0")
  cond,existArgs:=concatConditionsAndArgs(overlap[0].existenceFilter.conditions)
  query:="WITH "+cte+" existence AS (SELECT DISTINCT conversation_id AS eid FROM "+overlap[0].existenceFilter.source.tableName()+" WHERE "+cond+") SELECT base.conversation_id FROM base "+join+" LEFT JOIN existence ON base.conversation_id=existence.eid WHERE ifNull(conversation_id_0,'')!='' OR ifNull(eid,'')='' ORDER BY base.conversation_id"
  args=append(args,existArgs...)
  if err:=os.WriteFile("/tmp/convi7402-review/generated-"+tc.name+".sql",[]byte(combineQueryAndArgs(query,args)),0600);err!=nil{t.Fatal(err)}
  if tc.name=="empty-oneof"{t.Error("empty metadata value accepted instead of InvalidArgument")}
  if tc.name=="singleton" && strings.Contains(query,"metadata_number_value < ?"){t.Error("inclusive singleton emitted exclusive upper bound")}
 })}
}

func TestReview7402SubmitTimeSQL(t *testing.T) {
 tr:=&analyticspb.TimeRange{StartTimestamp:timestamppb.New(time.Date(2021,1,1,0,0,0,0,time.UTC)),EndTimestamp:timestamppb.New(time.Date(2021,1,31,0,0,0,0,time.UTC))}
 m:=&momentpb.Moment{Name:"customers/c/profiles/p/moments/A",Type:momentpb.Moment_CONVERSATION_METADATA}
 attr:=&analyticspb.QAAttribute{MomentGroups:[]*analyticspb.MomentGroup{{Moments:[]*momentpb.Moment{m},ExcludedMoments:[]*momentpb.Moment{m},MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}}}}}
 opts,err:=qaTimeRangeFilterTargetOptions(analyticspb.TimeRangeFilterTarget_TIME_RANGE_FILTER_TARGET_SUBMIT_TIME,0);if err!=nil{t.Fatal(err)}
 common,_,err:=parseCommonConditionsForQAAttribute(tr,attr,scoreTable,false,opts...);if err!=nil{t.Fatal(err)}
 card,err:=parseScorecardConditionsForQAAttribute(attr);if err!=nil{t.Fatal(err)}
 score,_,err:=parseScoreConditionsForQAAttribute(tr,attr,scoreTable,false,opts...);if err!=nil{t.Fatal(err)}
 conv,_,err:=parseConversationConditionsForQAAttribute(context.Background(),nil,attr,false);if err!=nil{t.Fatal(err)}
 for _,kind:=range []string{"conversations","stats"} {
  // Mirror the argument passed by each production read method at this PR head.
  momentTR:=tr;if kind=="stats"{momentTR=nil}
  inc,exc,overlap,err:=parseMomentConditionsForQAAttribute(momentTR,attr,false,conversationStartTimeColumn);if err!=nil{t.Fatal(err)}
  var query string;var args []any
  if kind=="conversations"{query,args=qaConversationsClickhouseQuery(common,card,score,conv,inc,exc,overlap,50,0,scoreTable)}else{query,args=qaScoreStatsClickhouseQueryWithMetadataView(common,card,score,conv,inc,exc,overlap,nil,nil,scoreTable,false,false)}
  if err:=os.WriteFile("/tmp/convi7402-review/submit-"+kind+".sql",[]byte(combineQueryAndArgs(query,args)),0600);err!=nil{t.Fatal(err)}
 }
}
