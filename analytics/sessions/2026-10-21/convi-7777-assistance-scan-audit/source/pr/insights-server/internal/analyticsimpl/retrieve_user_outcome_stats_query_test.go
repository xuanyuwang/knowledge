package analyticsimpl

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"
)

type RetrieveUserOutcomeStatsQueryTestSuite struct {
	suite.Suite
}

const (
	queryTestCustomerID = "testcustomer"
	queryTestProfileID  = "testprofile"
)

func TestRetrieveUserOutcomeStatsQuery(t *testing.T) {
	suite.Run(t, new(RetrieveUserOutcomeStatsQueryTestSuite))
}

func (s *RetrieveUserOutcomeStatsQueryTestSuite) TestBuildOutcomeStatsClickhouseQuery() {
	fixedStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	fixedEnd := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(fixedStart),
		EndTimestamp:   timestamppb.New(fixedEnd),
	}
	baseFilter := &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
		TimeRange: timeRange,
	}
	parent := &shared.CustomerProfile{CustomerID: queryTestCustomerID, ProfileID: queryTestProfileID}

	s.Run("TestBuildOutcomeStatsClickhouseQuery_WithUserFilter", func() {
		// given
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			Users:   []*userpb.User{{Name: userpb.UserName{CustomerID: queryTestCustomerID, UserID: "user_id_1"}.String()}},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
			},
		}

		// when
		query, queryArgs, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)

		// then
		s.Require().NoError(err)
		s.Require().Contains(query, "WITH outcomes_main AS (")
		s.Require().Contains(query, "user_id IN (?)")
		s.Require().Contains(query, "outcome_time >= ?")
		s.Require().Contains(query, "outcome_time < ?")
		s.Require().NotContains(query, "GLOBAL LEFT JOIN")
		s.Require().Contains(query, "GROUP BY user_id")
		s.Require().Len(queryArgs, 12)
		s.Require().Equal(queryTestCustomerID, queryArgs[0])
		s.Require().Equal(queryTestProfileID, queryArgs[1])
		s.Require().Equal(useroutcomes.UserOutcomeResourceTypeID, queryArgs[2])
		s.Require().Equal([]string{"user_id_1"}, queryArgs[3])
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[4])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[5])
		s.Require().Equal(queryTestCustomerID, queryArgs[6])
		s.Require().Equal(queryTestProfileID, queryArgs[7])
		s.Require().Equal(useroutcomes.UserOutcomeResourceTypeID, queryArgs[8])
		s.Require().Equal([]string{"user_id_1"}, queryArgs[9])
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[10])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[11])
		s.Require().Equal([]string{"user_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_WithFieldDefinitionFilter", func() {
		// given
		fieldDefName := metadatapb.FieldDefinitionName{
			CustomerID:        queryTestCustomerID,
			ProfileID:         queryTestProfileID,
			ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
			FieldDefinitionID: "field_definition_id_1",
		}.String()
		spec := &UserOutcomeQuerySpec{
			Parent:               parent,
			Filter:               baseFilter,
			FieldDefinitionNames: []string{fieldDefName},
			GroupBy:              &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
			},
		}

		// when
		query, queryArgs, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)

		// then
		s.Require().NoError(err)
		s.Require().Contains(query, "field_definition_id IN (?)")
		s.Require().Contains(query, "GROUP BY field_definition_id")
		s.Require().Len(queryArgs, 11)
		s.Require().Equal(queryTestCustomerID, queryArgs[0])
		s.Require().Equal(queryTestProfileID, queryArgs[1])
		s.Require().Equal(useroutcomes.UserOutcomeResourceTypeID, queryArgs[2])
		s.Require().Equal([]string{"field_definition_id_1"}, queryArgs[3])
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[4])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[5])
		s.Require().Equal(queryTestCustomerID, queryArgs[6])
		s.Require().Equal(queryTestProfileID, queryArgs[7])
		s.Require().Equal(useroutcomes.UserOutcomeResourceTypeID, queryArgs[8])
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[9])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[10])
		s.Require().Equal([]string{"field_definition_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_WithTimeRangeFilter", func() {
		// given
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
			},
		}

		// when
		query, queryArgs, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)

		// then
		s.Require().NoError(err)
		s.Require().Contains(query, "outcome_time >= ?")
		s.Require().Contains(query, "outcome_time < ?")
		s.Require().Len(queryArgs, 10)
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[3])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[4])
		s.Require().Equal("2026-01-01 00:00:00", queryArgs[8])
		s.Require().Equal("2026-01-02 00:00:00", queryArgs[9])
		s.Require().Equal([]string{"user_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByFieldDefinitionAndTimeRange", func() {
		spec := &UserOutcomeQuerySpec{
			Parent: parent,
			Filter: baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				Frequency: analyticspb.Frequency_DAILY,
			},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"bucket_time", "field_definition_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByFieldDefinitionAndAgentTier", func() {
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"field_definition_id", "user_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByTimeRange", func() {
		spec := &UserOutcomeQuerySpec{
			Parent: parent,
			Filter: baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				Frequency: analyticspb.Frequency_DAILY,
			},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
			},
		}
		query, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Contains(query, "bucket_time")
		s.Require().Equal([]string{"bucket_time"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByUserAndFieldDefinition", func() {
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"user_id", "field_definition_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByUser", func() {
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"user_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByUserAndTimeRange", func() {
		spec := &UserOutcomeQuerySpec{
			Parent: parent,
			Filter: baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				Frequency: analyticspb.Frequency_DAILY,
			},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"bucket_time", "user_id"}, groupByKeys)
	})

	s.Run("TestBuildOutcomeStatsClickhouseQuery_GroupByAgentTier", func() {
		spec := &UserOutcomeQuerySpec{
			Parent:  parent,
			Filter:  baseFilter,
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{},
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER,
			},
		}
		_, _, groupByKeys, err := buildOutcomeStatsClickhouseQuery(spec)
		s.Require().NoError(err)
		s.Require().Equal([]string{"user_id"}, groupByKeys)
	})
}
