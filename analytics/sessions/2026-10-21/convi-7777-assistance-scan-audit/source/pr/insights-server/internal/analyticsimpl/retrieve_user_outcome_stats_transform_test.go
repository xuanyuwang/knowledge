package analyticsimpl

import (
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"
)

type RetrieveUserOutcomeStatsTransformTestSuite struct {
	suite.Suite
}

func TestRetrieveUserOutcomeStatsTransform(t *testing.T) {
	suite.Run(t, new(RetrieveUserOutcomeStatsTransformTestSuite))
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestTransformCHRowsToInternalUserOutcomeStats() {
	s.Run("TestTransformCHRowsToInternalUserOutcomeStats_MapsRows", func() {
		// given
		bucketTime := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
		rows := []CHOutcomeStatsRow{
			{
				BucketTime:        bucketTime,
				FieldDefinitionID: "1",
				UserID:            "123",
				RecordCount:       1,
				ValueSum:          1.0,
				WeightSum:         1.0,
				WeightedValueSum:  1.0,
			},
		}
		spec := &UserOutcomeQuerySpec{
			GroupByAttributeTypes: []analyticspb.UserOutcomeAttributeType{
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
				analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
			},
			Parent: &shared.CustomerProfile{
				CustomerID: "customer-1",
				ProfileID:  "profile-1",
			},
		}

		// when
		internalStats := transformCHRowsToInternalUserOutcomeStats(rows, spec)

		// then
		s.Require().Len(internalStats, 1)

		expectedUserName := userpb.UserName{
			CustomerID: "customer-1",
			UserID:     "123",
		}.String()
		s.Require().Equal(expectedUserName, internalStats[0].GroupBy.GetUser())

		expectedFieldDefName := metadatapb.FieldDefinitionName{
			CustomerID:        "customer-1",
			ProfileID:         "profile-1",
			ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
			FieldDefinitionID: "1",
		}.String()
		s.Require().Equal(expectedFieldDefName, internalStats[0].GroupBy.GetOutcomeFieldDefinitionName())
		s.Require().Equal(bucketTime, internalStats[0].GroupBy.GetTimeRange().GetStartTimestamp().AsTime())

		s.Require().Equal(1.0, internalStats[0].ValueSum)
		s.Require().Equal(1.0, internalStats[0].WeightSum)
		s.Require().Equal(1.0, internalStats[0].WeightedValueSum)
		s.Require().Equal(int32(1), internalStats[0].RecordCount)
	})

	s.Run("TestTransformCHRowsToInternalUserOutcomeStats_ReturnsEmptyForNoRows", func() {
		// given
		rows := []CHOutcomeStatsRow{}

		// when
		internalStats := transformCHRowsToInternalUserOutcomeStats(rows, &UserOutcomeQuerySpec{})

		// then
		s.Require().Empty(internalStats)
	})
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestTransformInternalUserOutcomeStatsToResponse() {
	s.Run("TestTransformInternalUserOutcomeStatsToResponse_ComputesSummaryAndAverageValues", func() {
		// given
		internalStats := []*internalUserOutcomeStats{
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User: userpb.UserName{
						CustomerID: "customer-1",
						UserID:     "1",
					}.String(),
				},
				RecordCount:      2,
				ValueSum:         6.0,
				WeightSum:        4.0,
				WeightedValueSum: 10.0,
			},
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					User: userpb.UserName{
						CustomerID: "customer-1",
						UserID:     "2",
					}.String(),
				},
				RecordCount:      3,
				ValueSum:         9.0,
				WeightSum:        6.0,
				WeightedValueSum: 18.0,
			},
		}

		// when
		response := transformInternalUserOutcomeStatsToResponse(
			internalStats,
			analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
		)

		// then
		s.Require().Len(response.GetUserOutcomeStats(), 2)
		s.Require().Equal(int32(5), response.GetUserOutcomeStatsSummary().GetRecordCount())
		s.Require().Equal(3.0, response.GetUserOutcomeStats()[0].GetValue())
		s.Require().Equal(3.0, response.GetUserOutcomeStats()[1].GetValue())
	})

	s.Run("TestTransformInternalUserOutcomeStatsToResponse_SortsByTimeRangeAscending", func() {
		// given
		t2 := timestamppb.New(time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC))
		t1 := timestamppb.New(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))
		internalStats := []*internalUserOutcomeStats{
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					TimeRange: &analyticspb.TimeRange{StartTimestamp: t2},
				},
				RecordCount: 1,
				ValueSum:    4.0,
			},
			{
				GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
					TimeRange: &analyticspb.TimeRange{StartTimestamp: t1},
				},
				RecordCount: 1,
				ValueSum:    2.0,
			},
		}

		// when
		response := transformInternalUserOutcomeStatsToResponse(
			internalStats,
			analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
		)

		// then
		s.Require().Len(response.GetUserOutcomeStats(), 2)
		s.Require().True(
			response.GetUserOutcomeStats()[0].GetGroupBy().GetTimeRange().GetStartTimestamp().AsTime().Before(
				response.GetUserOutcomeStats()[1].GetGroupBy().GetTimeRange().GetStartTimestamp().AsTime(),
			),
		)
	})
}

// helperFindTier finds the aggregated stats entry for a given tier and outcome field definition name.
func helperFindTier(stats []*internalUserOutcomeStats, tier analyticspb.AgentTier, outcomeFieldDefName string) *internalUserOutcomeStats {
	for _, s := range stats {
		if s.GroupBy.GetAgentTier() == tier && s.GroupBy.GetOutcomeFieldDefinitionName() == outcomeFieldDefName {
			return s
		}
	}
	return nil
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestAggregateTopAgentsInternalUserOutcomeStats_LowerIsBetter() {
	// For a lower-is-better field (e.g., AHT), agents with LOW values should be in TOP_AGENTS tier.
	// We create 4 agents with equal volume (RecordCount=10) so partitioning is purely by metric value.
	// With bottomAgentPercentage=0.25 and AvgAgentPercentage=0.5:
	//   ascending sort by negated metric -> agent with highest real value is first (most negative)
	//   partition[0]=BOTTOM (25%) -> 1 agent (highest AHT, worst)
	//   partition[1]=AVERAGE (50%) -> 2 agents
	//   partition[2]=TOP (25%) -> 1 agent (lowest AHT, best)
	ahtFieldDef := "customers/c1/profiles/p1/resourceTypes/user_outcome/fieldDefinitions/aht"

	internalStats := []*internalUserOutcomeStats{
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-low-aht",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    50.0, // avg AHT = 5 (best)
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid1-aht",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    100.0, // avg AHT = 10
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid2-aht",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    150.0, // avg AHT = 15
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-high-aht",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    200.0, // avg AHT = 20 (worst)
		},
	}

	lowerIsBetterFieldDefs := set.NewFromSlice([]string{ahtFieldDef})

	result, err := aggregateTopAgentsInternalUserOutcomeStats(
		internalStats,
		analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
		lowerIsBetterFieldDefs,
	)
	s.Require().NoError(err)
	s.Require().Len(result, 3) // BOTTOM, AVERAGE, TOP

	topTier := helperFindTier(result, analyticspb.AgentTier_TOP_AGENTS, ahtFieldDef)
	bottomTier := helperFindTier(result, analyticspb.AgentTier_BOTTOM_AGENTS, ahtFieldDef)
	avgTier := helperFindTier(result, analyticspb.AgentTier_AVERAGE_AGENTS, ahtFieldDef)

	s.Require().NotNil(topTier)
	s.Require().NotNil(bottomTier)
	s.Require().NotNil(avgTier)

	// TOP tier should contain the agent with the lowest AHT (best performer).
	// With 1 agent in TOP: ValueSum=50, RecordCount=10
	s.Require().Equal(50.0, topTier.ValueSum)
	s.Require().Equal(int32(10), topTier.RecordCount)

	// BOTTOM tier should contain the agent with the highest AHT (worst performer).
	// With 1 agent in BOTTOM: ValueSum=200, RecordCount=10
	s.Require().Equal(200.0, bottomTier.ValueSum)
	s.Require().Equal(int32(10), bottomTier.RecordCount)

	// AVERAGE tier should contain the 2 middle agents: ValueSum=100+150=250, RecordCount=20
	s.Require().Equal(250.0, avgTier.ValueSum)
	s.Require().Equal(int32(20), avgTier.RecordCount)
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestAggregateTopAgentsInternalUserOutcomeStats_HigherIsBetter_Unchanged() {
	// For a higher-is-better field (not in lowerIsBetterFieldDefs), agents with HIGH values
	// should still be in TOP_AGENTS tier — same as before the directionality change.
	revenueFieldDef := "customers/c1/profiles/p1/resourceTypes/user_outcome/fieldDefinitions/revenue"

	internalStats := []*internalUserOutcomeStats{
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-low-rev",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    50.0, // avg = 5 (worst)
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid1-rev",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    100.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid2-rev",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    150.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-high-rev",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    200.0, // avg = 20 (best)
		},
	}

	// revenue is NOT lower-is-better; pass an empty set.
	lowerIsBetterFieldDefs := set.New[string]()

	result, err := aggregateTopAgentsInternalUserOutcomeStats(
		internalStats,
		analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
		lowerIsBetterFieldDefs,
	)
	s.Require().NoError(err)
	s.Require().Len(result, 3)

	topTier := helperFindTier(result, analyticspb.AgentTier_TOP_AGENTS, revenueFieldDef)
	bottomTier := helperFindTier(result, analyticspb.AgentTier_BOTTOM_AGENTS, revenueFieldDef)

	s.Require().NotNil(topTier)
	s.Require().NotNil(bottomTier)

	// TOP tier: highest revenue agent (ValueSum=200)
	s.Require().Equal(200.0, topTier.ValueSum)
	// BOTTOM tier: lowest revenue agent (ValueSum=50)
	s.Require().Equal(50.0, bottomTier.ValueSum)
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestAggregateTopAgentsInternalUserOutcomeStats_MixedDirectionality() {
	// When the request includes multiple outcome fields — one lower-is-better (AHT) and one
	// higher-is-better (revenue) — each field group should be partitioned independently
	// with the correct directionality.
	ahtFieldDef := "customers/c1/profiles/p1/resourceTypes/user_outcome/fieldDefinitions/aht"
	revenueFieldDef := "customers/c1/profiles/p1/resourceTypes/user_outcome/fieldDefinitions/revenue"

	internalStats := []*internalUserOutcomeStats{
		// AHT field — lower is better
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-a",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    50.0, // avg AHT=5 (best)
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-b",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    100.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-c",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    150.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-d",
				OutcomeFieldDefinitionName: ahtFieldDef,
			},
			RecordCount: 10,
			ValueSum:    200.0, // avg AHT=20 (worst)
		},
		// Revenue field — higher is better
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-a",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    50.0, // avg rev=5 (worst)
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-b",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    100.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-c",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    150.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-d",
				OutcomeFieldDefinitionName: revenueFieldDef,
			},
			RecordCount: 10,
			ValueSum:    200.0, // avg rev=20 (best)
		},
	}

	lowerIsBetterFieldDefs := set.NewFromSlice([]string{ahtFieldDef})

	result, err := aggregateTopAgentsInternalUserOutcomeStats(
		internalStats,
		analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
		lowerIsBetterFieldDefs,
	)
	s.Require().NoError(err)
	s.Require().Len(result, 6) // 3 tiers x 2 field groups

	// AHT (lower-is-better): lowest AHT agent in TOP
	ahtTop := helperFindTier(result, analyticspb.AgentTier_TOP_AGENTS, ahtFieldDef)
	ahtBottom := helperFindTier(result, analyticspb.AgentTier_BOTTOM_AGENTS, ahtFieldDef)
	s.Require().NotNil(ahtTop)
	s.Require().NotNil(ahtBottom)
	s.Require().Equal(50.0, ahtTop.ValueSum)     // lowest AHT in TOP
	s.Require().Equal(200.0, ahtBottom.ValueSum) // highest AHT in BOTTOM

	// Revenue (higher-is-better): highest revenue agent in TOP
	revTop := helperFindTier(result, analyticspb.AgentTier_TOP_AGENTS, revenueFieldDef)
	revBottom := helperFindTier(result, analyticspb.AgentTier_BOTTOM_AGENTS, revenueFieldDef)
	s.Require().NotNil(revTop)
	s.Require().NotNil(revBottom)
	s.Require().Equal(200.0, revTop.ValueSum)   // highest revenue in TOP
	s.Require().Equal(50.0, revBottom.ValueSum) // lowest revenue in BOTTOM
}

func (s *RetrieveUserOutcomeStatsTransformTestSuite) TestAggregateTopAgentsInternalUserOutcomeStats_NilLowerIsBetterSet() {
	// When lowerIsBetterFieldDefinitions is nil (e.g., error building the set),
	// behavior should be the same as higher-is-better for all fields.
	fieldDef := "customers/c1/profiles/p1/resourceTypes/user_outcome/fieldDefinitions/metric"

	internalStats := []*internalUserOutcomeStats{
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-low",
				OutcomeFieldDefinitionName: fieldDef,
			},
			RecordCount: 10,
			ValueSum:    50.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid1",
				OutcomeFieldDefinitionName: fieldDef,
			},
			RecordCount: 10,
			ValueSum:    100.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-mid2",
				OutcomeFieldDefinitionName: fieldDef,
			},
			RecordCount: 10,
			ValueSum:    150.0,
		},
		{
			GroupBy: &analyticspb.UserOutcomeStatsGroupBy{
				User:                       "customers/c1/users/agent-high",
				OutcomeFieldDefinitionName: fieldDef,
			},
			RecordCount: 10,
			ValueSum:    200.0,
		},
	}

	// nil set — graceful fallback to higher-is-better behavior
	result, err := aggregateTopAgentsInternalUserOutcomeStats(
		internalStats,
		analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
		nil,
	)
	s.Require().NoError(err)
	s.Require().Len(result, 3)

	topTier := helperFindTier(result, analyticspb.AgentTier_TOP_AGENTS, fieldDef)
	bottomTier := helperFindTier(result, analyticspb.AgentTier_BOTTOM_AGENTS, fieldDef)
	s.Require().NotNil(topTier)
	s.Require().NotNil(bottomTier)

	// Higher-is-better: highest value in TOP
	s.Require().Equal(200.0, topTier.ValueSum)
	s.Require().Equal(50.0, bottomTier.ValueSum)
}
