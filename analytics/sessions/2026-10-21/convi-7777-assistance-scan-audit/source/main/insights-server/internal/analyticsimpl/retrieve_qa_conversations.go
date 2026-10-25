package analyticsimpl

import (
	"context"
	"strconv"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/shared-go/fn"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/qa"
)

func (a AnalyticsServiceImpl) RetrieveQAConversations(
	ctx context.Context,
	req *analyticspb.RetrieveQAConversationsRequest,
) (*analyticspb.RetrieveQAConversationsResponse, error) {
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.QAAttribute{}
	}

	// Parse the page size and offset.
	pageSize := 20
	if req.PageSize > 0 {
		pageSize = int(req.PageSize)
	}
	offset := 0
	if req.PageToken != "" {
		if offset, err = strconv.Atoi(req.PageToken); err != nil {
			return nil, err
		}
	}

	includeDirectGroupMembershipsOnly := shared.IsIncludeDirectGroupMembershipsOnly(req.GetFilterByAttribute().GetGroupMembershipFilter())
	excludeDeactivatedUsers := req.FilterByAttribute != nil && req.FilterByAttribute.GetExcludeDeactivatedUsers()

	result, err := ParseUserFilterForAnalytics(
		ctx,
		a.logger,
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		parent.CustomerID,
		parent.ProfileID,
		req.FilterByAttribute.GetUsers(),
		req.FilterByAttribute.GetGroups(),
		false, // hasAgentAsGroupByKey
		includeDirectGroupMembershipsOnly,
		false, // listAgentOnly
		false, // includePeerUserStats
		excludeDeactivatedUsers,
	)
	if err != nil {
		return nil, err
	}
	if shouldEarlyReturn := ApplyUserFilterFromResult(result, &req.FilterByAttribute.Users, &req.FilterByAttribute.Groups, a.enableExtTableForUserFilter); shouldEarlyReturn {
		return &analyticspb.RetrieveQAConversationsResponse{}, nil
	}
	if _, shouldEarlyReturn, err := a.resolveScorecardReviewerAudience(ctx, parent, req.FilterByAttribute, includeDirectGroupMembershipsOnly); err != nil {
		return nil, err
	} else if shouldEarlyReturn {
		return &analyticspb.RetrieveQAConversationsResponse{}, nil
	}

	if req.GetRespectTemplateQaScoreConfig() {
		if len(req.GetFilterByAttribute().GetScorecardTemplates()) != 1 {
			return nil, status.Errorf(codes.InvalidArgument, "must have one scorecard template filter set when RespectTemplateQaScoreConfig is enabled.")
		}
		if len(req.GetFilterByAttribute().GetConversationDurationBuckets()) > 0 {
			return nil, status.Errorf(codes.InvalidArgument, "should not have conversation duration bucket filter set when RespectTemplateQaScoreConfig is enabled.")
		}

		profileName, err := customerpb.ParseProfileName(req.GetParent())
		if err != nil {
			return nil, err
		}
		scorecardTemplateIDs, err := qa.ExtractScorecardTemplateIDs(req.FilterByAttribute.ScorecardTemplates)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "the given scorecard template names are invalid: %v", err)
		}
		activeScorecardTemplateIDToTemplateMap, err := qa.ListCurrentScorecardTemplateIDs(
			a.appsDB.DB(ctx), profileName, req.GetFilterByAttribute().GetUsecaseNames(), scorecardTemplateIDs, false /* filterActive */)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "failed to list the scorecard template IDs: %v", err)
		}

		templates := fn.MapValueToSlice[string, *model.ScorecardTemplates](activeScorecardTemplateIDToTemplateMap)
		if len(templates) != 1 {
			a.logger.Warnf(ctx, "there is no active scorecard template found: %v", req)
		} else {
			durationBuckets := []*analyticspb.ConversationDurationBucket{}
			if templates[0].QaScoreConfig.Valid {
				protoQaScoreConfig, err := qa.ConvertDBToServiceQAScoreConfig(templates[0])
				if err != nil {
					return nil, err
				}
				durationBuckets = protoQaScoreConfig.GetConversationFilters().GetConversationDurationBuckets()
			}
			req.GetFilterByAttribute().ConversationDurationBuckets = durationBuckets
		}

	}

	if req.ScoreResource != analyticspb.RetrieveQAConversationsRequest_QA_SCORE_RESOURCE_SCORECARD {
		criteriaIDFilters := req.GetFilterByAttribute().GetCriterionIdentifiers()
		if len(criteriaIDFilters) == 0 {
			scoreableCriteria, err := a.getScoreableCriteria(
				ctx,
				&customerpb.ProfileName{
					CustomerID: parent.CustomerID,
					ProfileID:  parent.ProfileID,
				},
				req.GetFilterByAttribute().GetScorecardTemplates(),
				req.GetFilterByAttribute().GetUsecaseNames())
			if err != nil {
				return nil, err
			}
			req.FilterByAttribute.CriterionIdentifiers = scoreableCriteria
		}
	}

	// Read QA conversations from clickhouse.
	return a.readQaConversationsFromClickhouse(ctx, req, parent, pageSize, offset)
}
