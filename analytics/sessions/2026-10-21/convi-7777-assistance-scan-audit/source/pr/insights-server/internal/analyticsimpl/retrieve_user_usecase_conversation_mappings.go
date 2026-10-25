package analyticsimpl

import (
	"context"
	"database/sql"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/shared-go/fn"

	"github.com/cresta/go-servers/shared/db/condition"
)

type UserUsecaseMappingRow struct {
	AgentUserID string
	UsecaseID   sql.NullString
	Total       int32
}

func (a AnalyticsServiceImpl) RetrieveUserUsecaseConversationMappings(
	ctx context.Context, req *analyticspb.RetrieveUserUsecaseConversationMappingsRequest,
) (*analyticspb.RetrieveUserUsecaseConversationMappingsResponse, error) {
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
	if len(req.Users) > 0 {
		userIDs, err := fn.MapE(req.Users, func(rawUserName string) (string, error) {
			userName, err := userpb.ParseUserName(rawUserName)
			if err != nil {
				return "", err
			}
			return userName.UserID, nil
		})
		if err != nil {
			return nil, err
		}
		conditions = condition.AppendWhereIn(conditions, "agent_user_id", userIDs)
	} else {
		conditions = condition.AppendSQLAndClause(conditions, "agent_user_id IS NOT NULL")
	}

	// by manual tests expecting that 1 mio records should still be doable within reasonable time
	limit := 1000000
	givenLimit := int(req.GetLimit())
	if givenLimit > 0 && givenLimit < limit {
		limit = givenLimit
	}

	var results []*UserUsecaseMappingRow
	db := a.appsDB.DB(ctx)
	filteredConversationsSubQuery := db.
		Model(&dbmodel.Chats{}).
		Select("usecase_id", "agent_user_id").
		Where(conditions.QueryString(), conditions.QueryParams()...).
		Order("started_at DESC").
		Limit(limit)

	err = db.Raw(`
		WITH filtered_conversations_subquery AS (?)
		SELECT s.agent_user_id, s.usecase_id, COUNT(*) total
		FROM filtered_conversations_subquery s
		GROUP BY 1,2
	`, filteredConversationsSubQuery).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	return &analyticspb.RetrieveUserUsecaseConversationMappingsResponse{
		Mappings: fn.Map(results, func(row *UserUsecaseMappingRow) *analyticspb.UserUsecaseConversationMapping {
			return mapUserUsecaseMappingToProto(row, profileName)
		}),
	}, nil
}

func mapUserUsecaseMappingToProto(row *UserUsecaseMappingRow, profileName customerpb.ProfileNameInterface) *analyticspb.UserUsecaseConversationMapping {
	pb := &analyticspb.UserUsecaseConversationMapping{
		Count: row.Total,
		User: userpb.UserName{
			CustomerID: profileName.GetCustomerID(),
			UserID:     row.AgentUserID,
		}.String(),
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
