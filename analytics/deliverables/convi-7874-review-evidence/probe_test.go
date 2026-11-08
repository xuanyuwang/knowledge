package shared

import (
 "context"
 "testing"
 "slices"
 npconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
 internaluser "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
 analytics "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
 user "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
 "github.com/cresta/go-servers/config/shared/grpcclient/config"
 "github.com/cresta/go-servers/shared/auth"
 "github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
 "google.golang.org/grpc"
 "google.golang.org/protobuf/encoding/prototext"
)
type reviewConfig struct { config.Client; cfg *config.CustomerConfig }
func (c reviewConfig) Get(string) (*config.CustomerConfig,error) { return c.cfg,nil }
type reviewACL struct { auth.ResourceACLHelper; acl *auth.ResourceACL }
func (a reviewACL) GetResourceACL(context.Context,string,...auth.ResourceACLOption)(*auth.ResourceACL,error){return a.acl,nil}
type reviewUser struct {user.UserServiceClient}
func(reviewUser)ListGroups(context.Context,*user.ListGroupsRequest,...grpc.CallOption)(*user.ListGroupsResponse,error){return &user.ListGroupsResponse{Groups:[]*user.Group{{Name:"customers/c/groups/team",Type:user.Group_TEAM},{Name:"customers/c/groups/dynamic",Type:user.Group_DYNAMIC_GROUP}}},nil}
type reviewInternalUser struct {internaluser.InternalUserServiceClient}
func(reviewInternalUser)ListUsersForAnalytics(_ context.Context,r *internaluser.ListUsersForAnalyticsRequest,_ ...grpc.CallOption)(*internaluser.ListUsersForAnalyticsResponse,error){
 ids:=[]string{"A","C"} // Active users; C is not in selected dynamic group.
 if len(r.GroupIds)>0 { ids=[]string{"A"}; if r.GroupIds[0]=="empty-team" { ids=nil } }
 out:=&internaluser.ListUsersForAnalyticsResponse{}
 for _,id:=range ids{out.Users=append(out.Users,&internaluser.LiteUser{UserId:id})}
 return out,nil
}
func reviewUsers(ids ...string)[]*user.User{var r []*user.User;for _,id:=range ids{r=append(r,&user.User{Name:"customers/c/users/"+id})};return r}
func TestReview34439(t *testing.T){
 var pc npconfig.CustomerConfig
 if err:=prototext.Unmarshal([]byte(`customer:"customers/c" auth_config:{enable_conversation_access_control:true}`),&pc);err!=nil{t.Fatal(err)}
 cfg,err:=config.NewCustomerConfig(&pc);if err!=nil{t.Fatal(err)}
 cases:=[]struct{name string;root bool;attr *analytics.Attribute;members []string;want []string;wantEmpty bool}{
 {name:"legacy_dynamic_scoped_positive",attr:&analytics.Attribute{Groups:[]*user.Group{{Name:"customers/c/groups/dynamic"}}},want:[]string{"customers/c/users/A"}},
 {name:"legacy_mixed_dynamic_survives_inaccessible_team",attr:&analytics.Attribute{Groups:[]*user.Group{{Name:"customers/c/groups/team"},{Name:"customers/c/groups/dynamic"}}},want:[]string{"customers/c/users/A"}},
 {name:"split_root_positive",root:true,attr:&analytics.Attribute{DynamicGroups:[]*user.Group{{Name:"customers/c/groups/dynamic"}}},members:[]string{"A"},want:[]string{"customers/c/users/A"}},
 {name:"split_dynamic_survives_inaccessible_team",attr:&analytics.Attribute{Groups:[]*user.Group{{Name:"customers/c/groups/team"}},DynamicGroups:[]*user.Group{{Name:"customers/c/groups/dynamic"}}},members:[]string{"A"},want:[]string{"customers/c/users/A"}},
 {name:"split_root_dynamic_survives_empty_team",root:true,attr:&analytics.Attribute{Groups:[]*user.Group{{Name:"customers/c/groups/empty-team"}},DynamicGroups:[]*user.Group{{Name:"customers/c/groups/dynamic"}}},members:[]string{"A"},want:[]string{"customers/c/users/A"}},
 {name:"split_root_explicit_active_user_does_not_readd_inactive_dynamic_member",root:true,attr:&analytics.Attribute{Users:reviewUsers("A"),DynamicGroups:[]*user.Group{{Name:"customers/c/groups/dynamic"}},ExcludeDeactivatedUsers:true},members:[]string{"A","B"},want:[]string{"customers/c/users/A"}},
 {name:"split_root_excludes_inactive_and_unselected_users",root:true,attr:&analytics.Attribute{DynamicGroups:[]*user.Group{{Name:"customers/c/groups/dynamic"}},ExcludeDeactivatedUsers:true},members:[]string{"A","B"},want:[]string{"customers/c/users/A"}},
 }
 for _,tc:=range cases{t.Run(tc.name,func(t *testing.T){
 empty,err:=ApplyResourceACLToAttributeFilter(context.Background(),reviewConfig{cfg:cfg},reviewACL{acl:&auth.ResourceACL{IsRootAccess:tc.root,ManagedUserIDs:[]string{"A","C"}}},clientregistry.NewTestRegistry[user.UserServiceClient](reviewUser{}),clientregistry.NewTestRegistry[internaluser.InternalUserServiceClient](reviewInternalUser{}),"c","p",tc.attr,true,append(append([]*user.User{},tc.attr.Users...),reviewUsers(tc.members...)...),true)
 if err!=nil{t.Fatal(err)};var got []string;for _,u:=range tc.attr.Users{got=append(got,u.Name)};slices.Sort(got)
 t.Logf("empty=%v actual_users=%v expected_users=%v",empty,got,tc.want)
 if empty!=tc.wantEmpty||!slices.Equal(got,tc.want){t.Errorf("selected audience was not preserved")}
 })}
}
