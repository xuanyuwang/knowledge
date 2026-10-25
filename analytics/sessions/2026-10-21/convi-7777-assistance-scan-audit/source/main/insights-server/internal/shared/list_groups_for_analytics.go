package shared

import (
	"context"
	"fmt"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/framework/client/grpc/registry"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/go-servers/shared/fnutils"
)

var useInternalListGroupsForAnalytics = envflag.Bool("ANALYTICS_USE_INTERNAL_LIST_GROUPS_FOR_ANALYTICS", false, "Fetch analytics groups through the internal auth ListGroupsForAnalytics RPC instead of the public user service ListGroups RPC.")

// listGroupsForAnalytics fetches groups through the internal auth
// ListGroupsForAnalytics RPC, which serves a cached flat group list without the
// membership and linked-user fan-out of the public user service ListGroups.
// Child teams of filtered parent teams are expanded from the response's group
// memberships, matching the member expansion of the public path.
func listGroupsForAnalytics(
	ctx context.Context,
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	groups []*userpb.Group,
	onlyFilteredGroups bool,
) ([]*userpb.Group, error) {
	internalUserClient, err := internalUserClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get internal user client for customer %v profile %v", customerID, profileID)
	}

	request := &internaluserpb.ListGroupsForAnalyticsRequest{
		Parent:     Customer{CustomerID: customerID}.String(),
		ProfileIds: []string{profileID},
		// Child teams are only expanded when a group filter is present, matching
		// the IncludeGroupMemberships behavior of the public user service path.
		IncludeGroupMemberships: !onlyFilteredGroups && len(groups) > 0,
	}
	for _, group := range groups {
		groupName, err := userpb.ParseGroupName(group.GetName())
		if err != nil {
			return nil, err
		}
		request.GroupIds = append(request.GroupIds, groupName.GroupID)
	}

	var (
		groupResults    = []*userpb.Group{}
		nextPageOffset  int32
		groupIDSet      = map[string]struct{}{}
		listedGroupByID = map[string]*userpb.Group{}
	)
	for {
		request.PageOffset = nextPageOffset
		response, err := internalUserClient.ListGroupsForAnalytics(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("listing groups for analytics for customer %v profile %v: %w", customerID, profileID, err)
		}
		for _, liteGroup := range response.GetGroups() {
			group := convertLiteGroupToGroup(customerID, liteGroup)
			groupResults = append(groupResults, group)
			groupIDSet[liteGroup.GetGroupId()] = struct{}{}
			listedGroupByID[liteGroup.GetGroupId()] = group
		}
		// Child teams of the listed parent teams, matching the GROUP-type member
		// expansion of the public user service path. Memberships whose parent or
		// child is not among the listed groups are skipped: the child expansion
		// only appends children of the returned groups.
		for _, membership := range response.GetGroupMemberships() {
			parentGroupID := membership.GetParentGroupId()
			childGroupID := membership.GetChildGroup().GetGroupId()
			if childGroupID == consts.RootGroupID {
				// Skip the `root` group as member.
				continue
			}
			if _, isListed := groupIDSet[parentGroupID]; !isListed {
				continue
			}
			if _, isListed := listedGroupByID[childGroupID]; isListed {
				// The child is already part of the group list.
				continue
			}
			childGroup := convertLiteGroupToGroup(customerID, membership.GetChildGroup())
			groupResults = append(groupResults, childGroup)
			listedGroupByID[childGroupID] = childGroup
		}
		if response.GetNextPageOffset() == 0 {
			break
		}
		nextPageOffset = response.GetNextPageOffset()
	}

	return fnutils.DedupeBy(groupResults, func(g *userpb.Group) string { return g.GetName() }), nil
}

func convertLiteGroupToGroup(customerID string, liteGroup *internaluserpb.LiteGroup) *userpb.Group {
	group := &userpb.Group{
		Name:        userpb.GroupName{CustomerID: customerID, GroupID: liteGroup.GetGroupId()}.String(),
		DisplayName: liteGroup.GetDisplayName(),
	}
	switch liteGroup.GetGroupType() {
	case internaluserpb.LiteGroup_TEAM:
		group.Type = userpb.Group_TEAM
	case internaluserpb.LiteGroup_DYNAMIC_GROUP:
		group.Type = userpb.Group_DYNAMIC_GROUP
	}
	return group
}
