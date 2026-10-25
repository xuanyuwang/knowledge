package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveScorecardStats(
	ctx context.Context, req *analyticspb.RetrieveScorecardStatsRequest,
) (*analyticspb.RetrieveScorecardStatsResponse, error) {
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	// Get resource ACL based on the caller identity.
	req.FilterByAttribute.Users, req.FilterByAttribute.Groups, err = shared.ApplyResourceACL(ctx, a.configClient, parent.CustomerID, a.resourceACLHelperProvider.Get(), req.FilterByAttribute.Users, req.FilterByAttribute.Groups, false)
	if err != nil {
		return nil, err
	}

	var (
		users                             = []*userpb.User{}
		hasAgentAsGroupByKey              = shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT)
		includeDirectGroupMembershipsOnly = shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	)
	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) ||
		shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
		_, _, users, err = shared.ListUsersMappedToGroups(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterByAttribute.GetGroups(),
			hasAgentAsGroupByKey,
			includeDirectGroupMembershipsOnly,
			a.enableListUsersCache,
			*a.listUsersCache,
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
	}

	if req.FilterByAttribute != nil && (len(req.FilterByAttribute.Groups) > 0 || req.FilterByAttribute.ExcludeDeactivatedUsers) {
		includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
		req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterByAttribute,
			includeDirectGroupMembershipsOnly,
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		if len(req.FilterByAttribute.Users) == 0 {
			return &analyticspb.RetrieveScorecardStatsResponse{}, nil
		}
	}

	return a.readScorecardStatsFromClickhouse(ctx, req, users)
}
