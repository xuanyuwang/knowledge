package analyticsimpl

import (
	"context"
	"database/sql"
	"sort"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"gorm.io/gorm"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
)

const (
	// From director.coaching_sessions table
	coachingSessionTruncatedSubmittedAtQuery string = "DATE_TRUNC(" +
		"@frequency, " +
		"manager_submitted_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone " +
		"- INTERVAL '1 day' * @first_day_of_week_bias AS truncated_submitted_at"
	coachingSessionTruncatedSubmittedAtColumn string = "truncated_submitted_at"
	// Collapse a group/bulk coaching event (multiple agent rows sharing a group_session_id) to
	// one held session, while ungrouped 1:1 sessions still count individually ("sessions held"
	// model). The 'group:'/'session:' prefixes prevent a group_session_id from colliding with a
	// resource_id, mirroring dedupByGroupSession in retrieve_coaching_efficiency_stats.go.
	coachingSessionRawQuerySubmittedCount string = "COUNT(DISTINCT CASE " +
		"WHEN group_session_id IS NOT NULL AND group_session_id <> '' " +
		"THEN 'group:' || group_session_id ELSE 'session:' || resource_id END) " +
		"AS coaching_session_submitted_count"
	coachingSessionRawQueryManagerCount string = "COUNT(DISTINCT creator_user_id) AS coaching_session_manager_count"
)

type coachingSessionStatsRow struct {
	TruncatedSubmittedAt          time.Time
	CreatorUserID                 string
	CoachingSessionSubmittedCount int
	CoachingSessionManagerCount   int
}

func (a AnalyticsServiceImpl) RetrieveCoachingSessionStats(ctx context.Context, req *analyticspb.RetrieveCoachingSessionStatsRequest) (*analyticspb.RetrieveCoachingSessionStatsResponse, error) {
	if req.FilterByAttribute == nil {
		req.FilterByAttribute = &analyticspb.Attribute{}
	}
	shared.CountFilter(ctx, req.FilterByAttribute)
	parent, err := customerpb.ParseCustomerOrProfileName(req.Parent)
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
			return &analyticspb.RetrieveCoachingSessionStatsResponse{}, nil
		}
	}

	db := a.appsDB.DB(ctx)
	rows, err := a.queryCoachingSessionStatsFromRawTable(ctx, req, db, parent)
	if err != nil {
		return nil, err
	}
	return a.convertCoachingSessionStatsRowsToResponse(rows, parent.CustomerID, req.GroupByAttributeTypes, users), nil
}

func (a AnalyticsServiceImpl) queryCoachingSessionStatsFromRawTable(
	ctx context.Context, req *analyticspb.RetrieveCoachingSessionStatsRequest,
	db *gorm.DB, parent *customerpb.CustomerOrProfileName,
) ([]coachingSessionStatsRow, error) {
	coachingSessionQuery := db.Model(&dbmodel.CoachingSessions{}).
		Where("customer = ?", parent.CustomerID).
		Where("profile = ?", parent.ProfileID).
		Where("manager_submitted_at IS NOT NULL").
		Where("deleted_at IS NULL")

	// Filter by coaching session type (1:1 vs group), no-op for ALL/UNSPECIFIED.
	if sessionTypeCondition := coachingSessionTypeCondition(req.CoachingSessionFilter, "group_session_id"); sessionTypeCondition != nil {
		coachingSessionQuery = coachingSessionQuery.Scopes(sessionTypeCondition)
	}

	// Time range filter.
	startTime := req.FilterByTimeRange.StartTimestamp.AsTime()
	endTime := req.FilterByTimeRange.EndTimestamp.AsTime()
	coachingSessionQuery = coachingSessionQuery.
		Where("manager_submitted_at >= @start_ts_sec AND manager_submitted_at < @end_ts_sec",
			sql.Named("start_ts_sec", startTime),
			sql.Named("end_ts_sec", endTime))

	// Usecase filter.
	usecaseIDs := make([]string, 0, len(req.FilterByAttribute.UsecaseNames))
	for _, usecaseName := range req.FilterByAttribute.UsecaseNames {
		usecaseName, err := customerpb.ParseUsecaseName(usecaseName)
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}
	if len(usecaseIDs) > 0 {
		coachingSessionQuery = coachingSessionQuery.
			Where("usecase_id IN ?", usecaseIDs)
	}

	// Agent user filter.
	userClient, err := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}
	filteredAgents, err := a.getFilteredAgentUsers(ctx, userClient, parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}
	if len(filteredAgents) > 0 {
		coachingSessionQuery = coachingSessionQuery.
			Where("creator_user_id IN ?", fn.MapKeyToSlice(filteredAgents))
	}

	// Truncated time (required).
	selectQuery := ""
	args, err := postgres.ParseArgs(&req.Frequency, req.Metadata)
	if err != nil {
		return nil, err
	}
	selectQuery += coachingSessionTruncatedSubmittedAtQuery
	coachingSessionQuery = coachingSessionQuery.Group(coachingSessionTruncatedSubmittedAtColumn)

	// Manager creator user ID (optional).
	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
		selectQuery += ", creator_user_id"
		coachingSessionQuery = coachingSessionQuery.Group("creator_user_id")
	}

	// Coaching session stats.
	selectQuery += (", " + coachingSessionRawQuerySubmittedCount)
	selectQuery += (", " + coachingSessionRawQueryManagerCount)
	coachingSessionQuery = coachingSessionQuery.Select(selectQuery, args...)
	var coachingSessionStatsRows []coachingSessionStatsRow
	err = coachingSessionQuery.Scan(&coachingSessionStatsRows).Error
	if err != nil {
		return nil, err
	}

	return coachingSessionStatsRows, nil
}

func (a AnalyticsServiceImpl) convertCoachingSessionStatsRowsToResponse(
	coachingSessionStatsRows []coachingSessionStatsRow,
	customerID string,
	attributeTypes []analyticspb.AttributeType,
	users []*userpb.User,
) *analyticspb.RetrieveCoachingSessionStatsResponse {
	if len(coachingSessionStatsRows) == 0 {
		return &analyticspb.RetrieveCoachingSessionStatsResponse{}
	}

	managerUserIDToCoachingSessionStatsResult := make(map[string]*analyticspb.CoachingSessionStatsResult)
	managerUserIDToUserCount := make(map[string]int32)
	for _, row := range coachingSessionStatsRows {
		coachingSessionStats := analyticspb.CoachingSessionStats{
			TimeRange:                               &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.TruncatedSubmittedAt)},
			CoachingSessionsSubmitted:               int32(row.CoachingSessionSubmittedCount),
			AverageCoachingSessionsSubmittedPerUser: safeDivideFloat(int32(row.CoachingSessionSubmittedCount), int32(row.CoachingSessionManagerCount)),
		}

		managerUserID := "*"
		if len(row.CreatorUserID) > 0 {
			managerUserID = row.CreatorUserID
		}

		if _, exists := managerUserIDToCoachingSessionStatsResult[managerUserID]; !exists {
			managerUserIDToCoachingSessionStatsResult[managerUserID] = &analyticspb.CoachingSessionStatsResult{}
			managerUserIDToUserCount[managerUserID] = int32(0)
		}
		managerUserIDToCoachingSessionStatsResult[managerUserID].CoachingSessionStats = append(managerUserIDToCoachingSessionStatsResult[managerUserID].CoachingSessionStats, &coachingSessionStats)
		managerUserIDToUserCount[managerUserID] += int32(row.CoachingSessionManagerCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	managerUserIDToAttribute := map[string]*analyticspb.Attribute{}
	attributes := map[*analyticspb.Attribute]bool{}
	for managerUserID := range managerUserIDToCoachingSessionStatsResult {
		attr := &analyticspb.Attribute{}
		if managerUserID != "*" {
			attr.Users = append(attr.Users, &userpb.User{
				Name: userpb.UserName{CustomerID: customerID, UserID: managerUserID}.String(),
			})
		}
		if len(attr.Users) > 0 {
			if user, exists := usernameToUserMap[attr.Users[0].Name]; exists {
				attr.Users[0].FullName = user.FullName
				attr.Users[0].Username = user.Username
			}
		}

		managerUserIDToAttribute[managerUserID] = attr
		attributes[attr] = true
	}

	totalUserCount := int32(0)
	resp := &analyticspb.RetrieveCoachingSessionStatsResponse{}
	for k, v := range managerUserIDToCoachingSessionStatsResult {
		v.Attribute = managerUserIDToAttribute[k]
		for _, stats := range v.CoachingSessionStats {
			v.CoachingSessionsSubmitted += stats.CoachingSessionsSubmitted
			v.AverageCoachingSessionsSubmittedPerUser += stats.AverageCoachingSessionsSubmittedPerUser
		}
		// Sort result based on timestamp.
		sort.Slice(v.CoachingSessionStats, func(i, j int) bool {
			return v.CoachingSessionStats[i].TimeRange.StartTimestamp.AsTime().Before(v.CoachingSessionStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserCount += managerUserIDToUserCount[k]
		resp.CoachingSessionsSubmitted += v.CoachingSessionsSubmitted
		resp.CoachingSessionStatsResults = append(resp.CoachingSessionStatsResults, v)
	}
	resp.CoachingSessionStatsResults = sortResults(resp.CoachingSessionStatsResults)
	if shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
		totalUserCount = int32(len(resp.CoachingSessionStatsResults))
	}
	resp.AverageCoachingSessionsSubmittedPerUser = safeDivideFloat(resp.CoachingSessionsSubmitted, totalUserCount)
	return resp
}
