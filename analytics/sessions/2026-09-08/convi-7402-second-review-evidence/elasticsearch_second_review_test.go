package elasticsearch
import (
 "testing"
 "encoding/json"
 "fmt"
 "os"
 "sort"
 analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
 metadata "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
 momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
)
func TestSecondReviewOverlapTypes(t *testing.T){
 for _,types:=range [][2]momentpb.Moment_Type{{momentpb.Moment_CONVERSATION_METADATA,0},{0,momentpb.Moment_CONVERSATION_METADATA}}{
  mg:=&analyticspb.MomentGroup{Moments:[]*momentpb.Moment{{Name:"customers/c/profiles/p/moments/A",Type:types[0]}},ExcludedMoments:[]*momentpb.Moment{{Name:"customers/c/profiles/p/moments/A",Type:types[1]}},MetadataValueAttributes:[]*analyticspb.MetadataValueAttribute{{MetadataValue:&metadata.MetadataValue{Value:&metadata.MetadataValue_StringValue{StringValue:"X"}}}}}
  inc,exc,err:=convertConvoMomentGroupToConvoFilters(mg)
  if err!=nil||inc==nil||exc!=nil||inc.Bool==nil||inc.Bool.Should==nil||len(*inc.Bool.Should)!=2||inc.Bool.MinimumShouldMatch!=1{t.Fatalf("types %v: inc=%+v exc=%+v err=%v",types,inc,exc,err)}
  t.Logf("types %v: selected-value OR missing, two branches, minimum_should_match=1",types)
 }
}

func TestSecondReviewNumericReference(t *testing.T){
 for _,lower:=range []bool{false,true}{for _,upper:=range []bool{false,true}{
  bin:=&metadata.NumericBin{FromValue:5,ToValue:10,FromValueIsExclusive:lower,ToValueIsInclusive:upper}
  filter:=ConvertNumericBinToESRange(bin,false)
  r:=filter.Range.MetadataNumberValueExplicitMapping;if r==nil{r=filter.Range.MetadataNumberValue}
  if r==nil{t.Fatal("no numeric range")}
  names:=[]string{"missing"}
  for _,fixture:=range []struct{name string;value float64}{{"five",5},{"six",6},{"ten",10}}{
   x:=fixture.value
   if (r.GT==nil||x>*r.GT)&&(r.GTE==nil||x>=*r.GTE)&&(r.LT==nil||x<*r.LT)&&(r.LTE==nil||x<=*r.LTE){names=append(names,fixture.name)}
  }
  sort.Strings(names);data,err:=json.Marshal(names);if err!=nil{t.Fatal(err)}
  err=os.WriteFile(fmt.Sprintf("/tmp/convi7402-review2/es-bounds-%t-%t.json",lower,upper),data,0600);if err!=nil{t.Fatal(err)}
 }}
}
