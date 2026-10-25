package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveManagerStats(ctx context.Context, req *analyticspb.RetrieveManagerStatsRequest) (*analyticspb.RetrieveManagerStatsResponse, error) {
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)
	err := shared.ValidateGroupByAttributes(req.GroupByAttributeTypes, req.Frequency)
	if err != nil {
		return nil, err
	}

	parent, err := shared.ParseCustomer(req.Parent)
	if err != nil {
		return nil, err
	}
	if len(req.GetProfileId()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "`profile_id` is required.")
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
			req.GetProfileId(),
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
		req.FilterByAttribute, err = shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			req.GetProfileId(), req.FilterByAttribute, includeDirectGroupMembershipsOnly,
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		if len(req.FilterByAttribute.Users) == 0 {
			return &analyticspb.RetrieveManagerStatsResponse{}, nil
		}
	}

	return a.readManagerStatsFromClickhouse(ctx, req, req.GetProfileId(), users)
}
