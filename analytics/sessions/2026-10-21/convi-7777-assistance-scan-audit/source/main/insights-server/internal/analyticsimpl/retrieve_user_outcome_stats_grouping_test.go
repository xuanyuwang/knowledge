package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"
)

type RetrieveUserOutcomeStatsGroupingTestSuite struct {
	suite.Suite
}

func TestRetrieveUserOutcomeStatsGrouping(t *testing.T) {
	suite.Run(t, new(RetrieveUserOutcomeStatsGroupingTestSuite))
}

var (
	TestCustomerID = "testcustomer"
	TestProfileID  = "testprofile"
)

func (s *RetrieveUserOutcomeStatsGroupingTestSuite) TestGroupInternalUserOutcomeStats() {
	s.Run("TestGroupInternalUserOutcomeStats_EmptyUserStats", func() {
		// given
		perUserStats := []*internalUserOutcomeStats{}
		groups := []*userpb.Group{
			{
				Name: userpb.GroupName{CustomerID: TestCustomerID, GroupID: "group_id_1"}.String(),
			},
		}
		userNameToGroupNamesMap := map[string][]string{}

		// when
		perGroupStats, err := groupInternalUserOutcomeStats(perUserStats, groups, userNameToGroupNamesMap)

		// then
		s.Require().NoError(err)
		s.Require().Empty(perGroupStats)
	})

	s.Run("TestGroupInternalUserOutcomeStats_GroupByGroupAndFieldDefinition", func() {
		// given two per user stats (same group, same outcome field) aggregate by group and field definition
		userName := userpb.UserName{CustomerID: TestCustomerID, UserID: "user_id_1"}.String()
		groupName := userpb.GroupName{CustomerID: TestCustomerID, GroupID: "group_id_1"}.String()
		fieldDefName := metadatapb.FieldDefinitionName{
			CustomerID:        TestCustomerID,
			ProfileID:         TestProfileID,
			ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
			FieldDefinitionID: "field_definition_id",
		}.String()

		perUserStats := []*internalUserOutcomeStats{
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User:                       userName,
					OutcomeFieldDefinitionName: fieldDefName,
				},
				RecordCount:      2,
				ValueSum:         10,
				WeightSum:        2,
				WeightedValueSum: 20,
			},
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User:                       userName,
					OutcomeFieldDefinitionName: fieldDefName,
				},
				RecordCount:      3,
				ValueSum:         15,
				WeightSum:        3,
				WeightedValueSum: 30,
			},
		}
		groups := []*userpb.Group{{Name: groupName}}
		userNameToGroupNamesMap := map[string][]string{userName: {groupName}}

		// when
		perGroupStats, err := groupInternalUserOutcomeStats(perUserStats, groups, userNameToGroupNamesMap)

		// then
		s.Require().NoError(err)
		s.Require().Len(perGroupStats, 1)
		s.Require().Empty(perGroupStats[0].GroupBy.GetUser())
		s.Require().Equal(groupName, perGroupStats[0].GroupBy.GetGroup())
		s.Require().Equal(fieldDefName, perGroupStats[0].GroupBy.GetOutcomeFieldDefinitionName())
		s.Require().Equal(int32(5), perGroupStats[0].RecordCount)
		s.Require().Equal(25.0, perGroupStats[0].ValueSum)
		s.Require().Equal(5.0, perGroupStats[0].WeightSum)
		s.Require().Equal(50.0, perGroupStats[0].WeightedValueSum)
	})

	s.Run("TestGroupInternalUserOutcomeStats_GroupByGroup", func() {
		// given per user stats with only user set one user in one group
		userName := userpb.UserName{CustomerID: TestCustomerID, UserID: "user_id_1"}.String()
		groupName := userpb.GroupName{CustomerID: TestCustomerID, GroupID: "group_id_1"}.String()
		perUserStats := []*internalUserOutcomeStats{
			{
				GroupBy:          &analyticspb.UserOutcomeStatsGroupBy{User: userName},
				RecordCount:      1,
				ValueSum:         5,
				WeightSum:        1,
				WeightedValueSum: 5,
			},
		}
		groups := []*userpb.Group{{Name: groupName}}
		userNameToGroupNamesMap := map[string][]string{userName: {groupName}}

		// when
		perGroupStats, err := groupInternalUserOutcomeStats(perUserStats, groups, userNameToGroupNamesMap)

		// then
		s.Require().NoError(err)
		s.Require().Len(perGroupStats, 1)
		s.Require().Empty(perGroupStats[0].GroupBy.GetUser())
		s.Require().Equal(groupName, perGroupStats[0].GroupBy.GetGroup())
		s.Require().Equal(int32(1), perGroupStats[0].RecordCount)
		s.Require().Equal(5.0, perGroupStats[0].ValueSum)
	})

	s.Run("TestGroupInternalUserOutcomeStats_GroupByGroupAndTimeRange", func() {
		// given per user stats with user and time range aggregate to per group with time range in key
		userName := userpb.UserName{CustomerID: TestCustomerID, UserID: "user_id_1"}.String()
		groupName := userpb.GroupName{CustomerID: TestCustomerID, GroupID: "group_id_1"}.String()
		t1 := timestamppb.New(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		t2 := timestamppb.New(time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC))
		perUserStats := []*internalUserOutcomeStats{
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User:      userName,
					TimeRange: &analyticspb.TimeRange{StartTimestamp: t1},
				},
				RecordCount: 2,
				ValueSum:    20,
			},
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User:      userName,
					TimeRange: &analyticspb.TimeRange{StartTimestamp: t2},
				},
				RecordCount: 1,
				ValueSum:    10,
			},
		}
		groups := []*userpb.Group{{Name: groupName}}
		userNameToGroupNamesMap := map[string][]string{userName: {groupName}}

		// when
		perGroupStats, err := groupInternalUserOutcomeStats(perUserStats, groups, userNameToGroupNamesMap)

		// then
		s.Require().NoError(err)
		s.Require().Len(perGroupStats, 2)
		byTime := make(map[string]*internalUserOutcomeStats)
		for _, stat := range perGroupStats {
			s.Require().Empty(stat.GroupBy.GetUser())
			s.Require().Equal(groupName, stat.GroupBy.GetGroup())
			ts := stat.GroupBy.GetTimeRange().GetStartTimestamp().AsTime()
			byTime[ts.Format(time.RFC3339)] = stat
		}
		key1 := t1.AsTime().Format(time.RFC3339)
		key2 := t2.AsTime().Format(time.RFC3339)

		s.Require().Contains(byTime, key1)
		s.Require().Contains(byTime, key2)
		s.Require().Equal(int32(2), byTime[key1].RecordCount)
		s.Require().Equal(20.0, byTime[key1].ValueSum)
		s.Require().Equal(int32(1), byTime[key2].RecordCount)
		s.Require().Equal(10.0, byTime[key2].ValueSum)
	})
}
