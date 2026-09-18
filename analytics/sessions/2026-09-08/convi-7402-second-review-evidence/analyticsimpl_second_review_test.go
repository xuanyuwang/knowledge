package analyticsimpl

import (
 "context"
 "fmt"
 "os"
 "testing"
 "time"
 analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
 metadata "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
 momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
 "google.golang.org/protobuf/types/known/timestamppb"
)

func TestSecondReviewOverlapTypes(t *testing.T) {
 for _, tc := range []struct{name string;included,excluded momentpb.Moment_Type}{
  {"included metadata excluded unspecified",momentpb.Moment_CONVERSATION_METADATA,momentpb.Moment_TYPE_UNSPECIFIED},
  {"included unspecified excluded metadata",momentpb.Moment_TYPE_UNSPECIFIED,momentpb.Moment_CONVERSATION_METADATA},
 }{t.Run(tc.name,func(t *testing.T){
  mg:=&analyticspb.MomentGroup{Moments:[]*momentpb.Moment{{Name:"customers/c/profiles/p/moments/A",Type:tc.included}},ExcludedMoments:[]*momentpb.Moment{{Name:"customers/c/profiles/p/moments/A",Type:tc.excluded}},MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}}}
  tr:=&analyticspb.TimeRange{StartTimestamp:timestamppb.New(time.Date(2021,1,1,0,0,0,0,time.UTC)),EndTimestamp:timestamppb.New(time.Date(2021,2,1,0,0,0,0,time.UTC))}
  _,inc,exc,_,err:=parseClickhouseFilter(context.Background(),tr,&analyticspb.Attribute{MomentGroups:[]*analyticspb.MomentGroup{mg}},[]ClickhouseTable{messageTable},false)
  if err==nil {t.Errorf("accepted unsupported overlap with %d includes and %d excludes",len(inc),len(exc))}
 })}
}

func TestSecondReviewOverlapIdentitySets(t *testing.T) {
 for i:=1;i<8;i++ {for e:=1;e<8;e++ {
  mg:=&analyticspb.MomentGroup{}
  for k:=0;k<3;k++ {m:=&momentpb.Moment{Name:fmt.Sprintf("customers/c/profiles/p/moments/%d",k),Type:momentpb.Moment_CONVERSATION_METADATA};if i&(1<<k)!=0{mg.Moments=append(mg.Moments,m)};if e&(1<<k)!=0{mg.ExcludedMoments=append(mg.ExcludedMoments,m)}}
  err:=validateNoMetadataMomentOverlap(mg)
  if (err!=nil)!=(i&e!=0){t.Errorf("masks %d %d err %v",i,e,err)}
 }}
}

func TestSecondReviewExport(t *testing.T) {
 m:=func(id string)*momentpb.Moment{return &momentpb.Moment{Name:"customers/c/profiles/p/moments/"+id,Type:momentpb.Moment_CONVERSATION_METADATA}}
 str:=func(s string)*analyticspb.MetadataValueAttribute{return &analyticspb.MetadataValueAttribute{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:s}}}}
 for _,withNulls:=range []bool{false,true}{
  var groups []*analyticspb.MomentGroup
  for _,id:=range []string{"A","B"}{groups=append(groups,&analyticspb.MomentGroup{Moments:[]*momentpb.Moment{m(id)},ExcludedMoments:[]*momentpb.Moment{m(id)},MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{str("X")}})}
  inc,exc,overlaps,err:=parseMomentConditionsForQAAttribute(nil,&analyticspb.QAAttribute{MomentGroups:groups},false,conversationStartTimeColumn);if err!=nil{t.Fatal(err)}
  score,_,err:=parseScoreConditionsForQAAttribute(nil,&analyticspb.QAAttribute{},scoreTable,false);if err!=nil{t.Fatal(err)}
  q,args:=qaConversationsClickhouseQuery(nil,nil,score,nil,inc,exc,overlaps,100,0,scoreTable)
  if withNulls{q+=" SETTINGS join_use_nulls=1"}
  if err:=os.WriteFile(fmt.Sprintf("/tmp/convi7402-review2/two-overlap-nulls-%t.sql",withNulls),[]byte(combineQueryAndArgs(q,args)),0600);err!=nil{t.Fatal(err)}
 }
 for _,lower:=range []bool{false,true}{for _,upper:=range []bool{false,true}{
  mg:=&analyticspb.MomentGroup{Moments:[]*momentpb.Moment{m("A")},ExcludedMoments:[]*momentpb.Moment{m("A")},MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{{NumericBin:&metadata.NumericBin{FromValue:5,ToValue:10,FromValueIsExclusive:lower,ToValueIsInclusive:upper}}}}
  _,_,over,err:=parseMomentConditionsForQAAttribute(nil,&analyticspb.QAAttribute{MomentGroups:[]*analyticspb.MomentGroup{mg}},false,conversationStartTimeColumn);if err!=nil{t.Fatal(err)}
  cte,join,args:=buildQAMomentFilterQuery(over[0].selectedValueFilter,0,"LEFT JOIN","base.conversation_id","cid")
  cond,ea:=concatConditionsAndArgs(over[0].existenceFilter.conditions)
  q:="WITH "+cte+" existence AS (SELECT DISTINCT conversation_id AS eid FROM moment_annotation_mv_by_metadata_d WHERE "+cond+") SELECT base.conversation_id FROM base "+join+" LEFT JOIN existence ON base.conversation_id=eid WHERE ifNull(cid,'')!='' OR ifNull(eid,'')='' ORDER BY base.conversation_id"
  os.WriteFile(fmt.Sprintf("/tmp/convi7402-review2/bounds-%t-%t.sql",lower,upper),[]byte(combineQueryAndArgs(q,append(args,ea...))),0600)
 }}
}
