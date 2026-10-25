package analyticsimpl

import (
	"context"
	"database/sql"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/shared/db/condition"
)

type TeamUsecaseMappingRow struct {
	UsecaseID   sql.NullString
	TeamGroupID sql.NullString
	Total       int32
}

func (a AnalyticsServiceImpl) RetrieveTeamUsecaseConversationMappings(
	ctx context.Context, req *analyticspb.RetrieveTeamUsecaseConversationMappingsRequest,
) (*analyticspb.RetrieveTeamUsecaseConversationMappingsResponse, error) {
	profileName, err := customerpb.ParseProfileName(req.Profile)
	if err != nil {
		return nil, err
	}

	conditions := condition.New(
		"customer_id = ? AND profile_id = ? AND ended_at IS NOT NULL",
		profileName.GetCustomerID(),
		profileName.GetProfileID(),
	)
	if req.GetStartedSince() != nil {
		conditions = condition.AppendSQLAndClause(conditions, "started_at >= ?", req.StartedSince.AsTime())
	}
	if req.GetStartedBefore() != nil {
		conditions = condition.AppendSQLAndClause(conditions, "started_at < ?", req.StartedBefore.AsTime())
	}
	if len(req.Groups) > 0 {
		groupIDs, err := fn.MapE(req.Groups, func(rawGroupName string) (string, error) {
			groupName, err := userpb.ParseGroupName(rawGroupName)
			if err != nil {
				return "", err
			}
			return groupName.GroupID, nil
		})
		if err != nil {
			return nil, err
		}
		conditions = condition.AppendWhereIn(conditions, "team_group_id", groupIDs)
	}

	// by manual tests expecting that 1 mio records should still be doable within reasonable time
	limit := 1000000
	givenLimit := int(req.GetLimit())
	if givenLimit > 0 && givenLimit < limit {
		limit = givenLimit
	}

	var results []*TeamUsecaseMappingRow
	db := a.appsDB.DB(ctx)
	filteredConversationsSubQuery := db.
		Model(&dbmodel.Chats{}).
		Select("usecase_id", "team_group_id").
		Where(conditions.QueryString(), conditions.QueryParams()...).
		Order("started_at DESC").
		Limit(limit)

	err = db.Raw(`
		WITH filtered_conversations_subquery AS (?)
		SELECT s.team_group_id, s.usecase_id, COUNT(*) total
		FROM filtered_conversations_subquery s
		GROUP BY 1,2
	`, filteredConversationsSubQuery).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return &analyticspb.RetrieveTeamUsecaseConversationMappingsResponse{
		Mappings: fn.Map(results, func(row *TeamUsecaseMappingRow) *analyticspb.TeamUsecaseConversationMapping {
			return mapTeamUsecaseMappingToProto(row, profileName)
		}),
	}, nil
}

func mapTeamUsecaseMappingToProto(row *TeamUsecaseMappingRow, profileName customerpb.ProfileNameInterface) *analyticspb.TeamUsecaseConversationMapping {
	pb := &analyticspb.TeamUsecaseConversationMapping{
		Count: row.Total,
	}
	if row.TeamGroupID.Valid && row.TeamGroupID.String != "" {
		pb.Team = userpb.GroupName{
			CustomerID: profileName.GetCustomerID(),
			GroupID:    row.TeamGroupID.String,
		}.String()
	}
	if row.UsecaseID.Valid && row.UsecaseID.String != "" {
		pb.Usecase = customerpb.UsecaseName{
			CustomerID: profileName.GetCustomerID(),
			ProfileID:  profileName.GetProfileID(),
			UsecaseID:  row.UsecaseID.String,
		}.String()
	}
	return pb
}
