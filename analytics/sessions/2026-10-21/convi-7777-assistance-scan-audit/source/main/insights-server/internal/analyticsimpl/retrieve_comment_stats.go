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
	// From app.chat_comments table
	commentTruncatedCreatedAtQuery string = "DATE_TRUNC(" +
		"@frequency, " +
		"chat_comments.created_at AT TIME ZONE @time_zone + INTERVAL '1 day' * @first_day_of_week_bias) AT TIME ZONE @time_zone " +
		"- INTERVAL '1 day' * @first_day_of_week_bias AS truncated_created_at"
	commentTruncatedCreatedAtColumn   string = "truncated_created_at"
	commentRawQueryCommentPlacedCount string = "COUNT(chat_comments.comment_id) AS comments_placed_count"
	commentRawQueryManagerCount       string = "COUNT(DISTINCT chat_comments.user_id) AS comments_manager_count"
)

type commentStatsRow struct {
	TruncatedCreatedAt   time.Time
	ManagerUserID        string
	CommentsPlacedCount  int
	CommentsManagerCount int
}

// nolint:dupl
func (a AnalyticsServiceImpl) RetrieveCommentStats(
	ctx context.Context, req *analyticspb.RetrieveCommentStatsRequest,
) (*analyticspb.RetrieveCommentStatsResponse, error) {
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
			return &analyticspb.RetrieveCommentStatsResponse{}, nil
		}
	}

	db := a.appsDB.DB(ctx)
	rows, err := a.queryCommentStatsFromRawTable(ctx, req, db, parent)
	if err != nil {
		return nil, err
	}
	return a.convertCommentStatsRowsToResponse(rows, parent.CustomerID, req.GroupByAttributeTypes, users), nil
}

func (a AnalyticsServiceImpl) queryCommentStatsFromRawTable(
	ctx context.Context, req *analyticspb.RetrieveCommentStatsRequest,
	db *gorm.DB, parent *customerpb.CustomerOrProfileName,
) ([]commentStatsRow, error) {
	commentStatsQuery := db.Model(&dbmodel.ChatComments{}).
		Where("chat_comments.customer_id = ?", parent.CustomerID).
		Where("chat_comments.profile_id = ?", parent.ProfileID)

	// Time range filter.
	startTime := req.FilterByTimeRange.StartTimestamp.AsTime()
	endTime := req.FilterByTimeRange.EndTimestamp.AsTime()
	commentStatsQuery = commentStatsQuery.
		Where("chat_comments.created_at >= @start_ts_sec AND chat_comments.created_at < @end_ts_sec",
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
		commentStatsQuery = commentStatsQuery.
			Joins("INNER JOIN app.chats ON chat_comments.customer_id = chats.customer_id AND chat_comments.profile_id = chats.profile_id AND chat_comments.conversation_id = chats.conversation_id").
			Where("chats.usecase_id IN ?", usecaseIDs)
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
		commentStatsQuery = commentStatsQuery.
			Where("chat_comments.user_id IN ?", fn.MapKeyToSlice(filteredAgents))
	}

	// Truncated time (required).
	selectQuery := ""
	args, err := postgres.ParseArgs(&req.Frequency, req.Metadata)
	if err != nil {
		return nil, err
	}
	selectQuery += commentTruncatedCreatedAtQuery
	commentStatsQuery = commentStatsQuery.Group(commentTruncatedCreatedAtColumn)

	// Manager user ID (optional).
	if shared.ContainsAttributeType(req.GroupByAttributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
		selectQuery += ", chat_comments.user_id AS manager_user_id"
		commentStatsQuery = commentStatsQuery.Group("manager_user_id")
	}

	// Comment stats.
	selectQuery += (", " + commentRawQueryCommentPlacedCount)
	selectQuery += (", " + commentRawQueryManagerCount)
	commentStatsQuery = commentStatsQuery.Select(selectQuery, args...)
	var commentStatsRows []commentStatsRow
	err = commentStatsQuery.Scan(&commentStatsRows).Error
	if err != nil {
		return nil, err
	}

	return commentStatsRows, nil
}

func (a AnalyticsServiceImpl) convertCommentStatsRowsToResponse(
	commentStatsRows []commentStatsRow,
	customerID string,
	attributeTypes []analyticspb.AttributeType,
	users []*userpb.User,
) *analyticspb.RetrieveCommentStatsResponse {
	if len(commentStatsRows) == 0 {
		return &analyticspb.RetrieveCommentStatsResponse{}
	}

	managerUserIDToCommentStatsResult := make(map[string]*analyticspb.CommentStatsResult)
	managerUserIDToUserCount := make(map[string]int32)
	for _, row := range commentStatsRows {
		commentStats := analyticspb.CommentStats{
			TimeRange:                         &analyticspb.TimeRange{StartTimestamp: shared.ConvertTimeToAPITimestamp(row.TruncatedCreatedAt)},
			TotalCommentsPlacedCount:          int32(row.CommentsPlacedCount),
			AverageCommentsPlacedCountPerUser: safeDivideFloat(int32(row.CommentsPlacedCount), int32(row.CommentsManagerCount)),
		}

		managerUserID := "*"
		if len(row.ManagerUserID) > 0 {
			managerUserID = row.ManagerUserID
		}

		if _, exists := managerUserIDToCommentStatsResult[managerUserID]; !exists {
			managerUserIDToCommentStatsResult[managerUserID] = &analyticspb.CommentStatsResult{}
			managerUserIDToUserCount[managerUserID] = int32(0)
		}
		managerUserIDToCommentStatsResult[managerUserID].CommentStats = append(managerUserIDToCommentStatsResult[managerUserID].CommentStats, &commentStats)
		managerUserIDToUserCount[managerUserID] += int32(row.CommentsManagerCount)
	}

	usernameToUserMap := make(map[string]*userpb.User, len(users))
	for _, u := range users {
		usernameToUserMap[u.Name] = u
	}
	managerUserIDToAttribute := map[string]*analyticspb.Attribute{}
	attributes := map[*analyticspb.Attribute]bool{}
	for managerUserID := range managerUserIDToCommentStatsResult {
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
	resp := &analyticspb.RetrieveCommentStatsResponse{}
	for k, v := range managerUserIDToCommentStatsResult {
		v.Attribute = managerUserIDToAttribute[k]
		for _, stats := range v.CommentStats {
			v.TotalCommentsPlacedCount += stats.TotalCommentsPlacedCount
			v.AverageCommentsPlacedCountPerUser += stats.AverageCommentsPlacedCountPerUser
		}
		// Sort result based on timestamp.
		sort.Slice(v.CommentStats, func(i, j int) bool {
			return v.CommentStats[i].TimeRange.StartTimestamp.AsTime().Before(v.CommentStats[j].TimeRange.StartTimestamp.AsTime())
		})
		totalUserCount += managerUserIDToUserCount[k]
		resp.TotalCommentsPlacedCount += v.TotalCommentsPlacedCount
		resp.CommentStatsResults = append(resp.CommentStatsResults, v)
	}
	resp.CommentStatsResults = sortResults(resp.CommentStatsResults)
	if shared.ContainsAttributeType(attributeTypes, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) {
		totalUserCount = int32(len(resp.CommentStatsResults))
	}
	resp.AverageCommentsPlacedCountPerUser = safeDivideFloat(resp.TotalCommentsPlacedCount, totalUserCount)
	return resp
}
