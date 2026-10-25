package analyticsimpl

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/metadataframework"
	metadatamock "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/metadataframework"

	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	useroutcomespb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/useroutcomes"

	useroutcomes "github.com/cresta/go-servers/shared/clickhouse/conversations/useroutcomes"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
)

type RetrieveUserOutcomeStatsTestSuite struct {
	AnalyticsTestSuite
	metadataGetResourceTypeResponse *metadatapb.GetResourceTypeResponse
	rows                            [][]any
	userA                           *userpb.User
	userB                           *userpb.User
	weekStarts                      []time.Time
}

func TestRetrieveUserOutcomeStats(t *testing.T) {
	suite.Run(t, new(RetrieveUserOutcomeStatsTestSuite))
}

func (s *RetrieveUserOutcomeStatsTestSuite) SetupSuite() {
	s.AnalyticsTestSuite.SetupSuite()
	s.UseMockedClickhouseConnections()
	s.mockConfigClient.EXPECT().
		Get(gomock.Any()).AnyTimes().
		Return(&config.CustomerConfig{}, nil)

	s.setupUserOutcomeStatsFieldsWithValues()
	ctrl := gomock.NewController(s.T())
	mockMetadataClient := metadatamock.NewMockMetadataServiceClient(ctrl)
	mockMetadataClient.EXPECT().
		GetResourceType(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, _ *metadatapb.GetResourceTypeRequest, _ ...any) (*metadatapb.GetResourceTypeResponse, error) {
			return s.metadataGetResourceTypeResponse, nil
		})
	s.server.metadataFrameworkClientRegistry = clientregistry.NewTestRegistry[metadatapb.MetadataServiceClient](mockMetadataClient)

}

func (s *RetrieveUserOutcomeStatsTestSuite) TestRetrieveUserOutcomeStats_Validations() {
	s.Run("TestRetrieveUserOutcomeStats_InvalidParent", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: "invalid_parent",
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "failed to parse parent: invalid_parent")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidFilter", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: nil,
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "filter is required.")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidGroupBy", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: nil,
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "group_by is required.")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidTimeRange", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: nil,
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
				AttributeTypes:  []analyticspb.UserOutcomeAttributeType{analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "time_range is required")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidAggregationMode", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_UNSPECIFIED,
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "aggregation_mode is required")
	})

	s.Run("TestRetrieveUserOutcomeStats_SumOnDisallowedField", func() {
		// Given: a field that did not opt into total aggregation, requested with SUM mode.
		nchName := metadatapb.FieldDefinitionName{
			CustomerID:        s.customerID,
			ProfileID:         s.profileID,
			ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
			FieldDefinitionID: "nch",
		}.String()
		originalResp := s.metadataGetResourceTypeResponse
		s.metadataGetResourceTypeResponse = &metadatapb.GetResourceTypeResponse{
			ResourceType: &metadatapb.ResourceType{
				Fields: []*metadatapb.FieldDefinition{
					{Name: nchName, DisplayName: "NCH", FieldMetadata: &metadatapb.FieldDefinition_Metadata{
						Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
							UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
								Frequency:             analyticspb.Frequency_DAILY,
								AllowTotalAggregation: false,
							},
						},
					}},
				},
			},
		}
		defer func() { s.metadataGetResourceTypeResponse = originalResp }()

		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeFieldDefinitionNames: []string{nchName},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				},
			},
		}

		// When
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// Then
		s.assertError(err, codes.InvalidArgument, "aggregation_mode SUM is not allowed")
		s.Require().Contains(err.Error(), "NCH")
	})

	s.Run("TestRetrieveUserOutcomeStats_NoGroupByAttributeTypes", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
				AttributeTypes:  []analyticspb.UserOutcomeAttributeType{},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "group_by_attribute_types is required.")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidAgentTierAndUserGroup", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_AGENT_TIER,
				},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "Not supporting agent tier attribute with user group attribute")
	})

	s.Run("TestRetrieveUserOutcomeStats_InvalidGroupByGroup", func() {
		// given: group + user is invalid (only time_range and outcome_field may be combined with group)
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_GROUP,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.assertError(err, codes.InvalidArgument, "when grouping by group, only time_range and outcome_field may be combined with group")
	})

	s.Run("TestRetrieveUserOutcomeStats_NoFieldDefinitionsAvailable", func() {
		// given: resource type has no field definitions, so filtering yields an empty list
		s.metadataGetResourceTypeResponse = &metadatapb.GetResourceTypeResponse{ResourceType: &metadatapb.ResourceType{}}
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "no field definitions found for the given field definition names")
		s.assertError(err, codes.InvalidArgument, "no field definitions found for the given field definition names")
	})

	s.Run("TestRetrieveUserOutcomeStats_NoFieldDefinitionsMatchedWithFieldDefinitionNames", func() {
		// given
		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeFieldDefinitionNames: []string{"invalid_field_definition_name"},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
				},
			},
		}

		// when
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.Require().Error(err)
		s.Require().Contains(err.Error(), "no field definitions found for the given field definition names")
		s.assertError(err, codes.InvalidArgument, "no field definitions found for the given field definition names")
	})
}

func (s *RetrieveUserOutcomeStatsTestSuite) TestRetrieveUserOutcomeStats_ValidRequest() {
	s.Run("TestRetrieveUserOutcomeStats_ValidRequest_GroupPerUserAndFieldDefinitionWeeklySum", func() {
		defer s.convoMockConn.AssertExpectationsAndReset(s.T())

		// given
		s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(&mockRowsWithErr{
			MockRows: clickhousetesting.NewMockRows(s.rows),
		}, nil)

		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)),
				},
				Users: []string{s.userA.Name, s.userB.Name},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
				Frequency:       analyticspb.Frequency_WEEKLY,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
				},
			},
		}

		// when
		resp, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Require().NotNil(resp.UserOutcomeStatsSummary)
		s.Require().Len(resp.UserOutcomeStats, 24)
		s.Require().Equal(int32(48), resp.UserOutcomeStatsSummary.RecordCount)

		statsByKey := make(map[string]*analyticspb.UserOutcomeStats, len(resp.UserOutcomeStats))
		for _, stat := range resp.UserOutcomeStats {
			key := stat.GroupBy.User + "|" +
				stat.GroupBy.GetOutcomeFieldDefinitionName() + "|" +
				stat.GroupBy.GetTimeRange().GetStartTimestamp().AsTime().Format(time.RFC3339)
			statsByKey[key] = stat
		}

		resourceFields := s.metadataGetResourceTypeResponse.GetResourceType().GetFields()
		scheduleAdherenceName := resourceFields[2].GetName()
		totalHoursName := resourceFields[3].GetName()
		nchName := resourceFields[4].GetName()

		for weekIdx, weekStart := range s.weekStarts {
			weekKey := weekStart.Format(time.RFC3339)

			userASchedule := statsByKey[s.userA.Name+"|"+scheduleAdherenceName+"|"+weekKey]
			userATotalHours := statsByKey[s.userA.Name+"|"+totalHoursName+"|"+weekKey]
			userANCH := statsByKey[s.userA.Name+"|"+nchName+"|"+weekKey]
			userBSchedule := statsByKey[s.userB.Name+"|"+scheduleAdherenceName+"|"+weekKey]
			userBTotalHours := statsByKey[s.userB.Name+"|"+totalHoursName+"|"+weekKey]
			userBNCH := statsByKey[s.userB.Name+"|"+nchName+"|"+weekKey]

			s.Require().NotNil(userASchedule)
			s.Require().NotNil(userATotalHours)
			s.Require().NotNil(userANCH)
			s.Require().NotNil(userBSchedule)
			s.Require().NotNil(userBTotalHours)
			s.Require().NotNil(userBNCH)

			s.Require().EqualValues(30+2*weekIdx, userASchedule.Value)
			s.Require().EqualValues(8+2*weekIdx, userATotalHours.Value)
			s.Require().EqualValues(18+2*weekIdx, userANCH.Value)
			s.Require().EqualValues(130+2*weekIdx, userBSchedule.Value)
			s.Require().EqualValues(28+2*weekIdx, userBTotalHours.Value)
			s.Require().EqualValues(58+2*weekIdx, userBNCH.Value)

			s.Require().Equal(int32(2), userASchedule.RecordCount)
			s.Require().Equal(int32(2), userBSchedule.RecordCount)
		}
	})

	s.Run("TestRetrieveUserOutcomeStats_ValidRequest_GroupPerUserAndFieldDefinitionWeeklyWeightedAverage", func() {
		defer s.convoMockConn.AssertExpectationsAndReset(s.T())

		// given
		s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(&mockRowsWithErr{
			MockRows: clickhousetesting.NewMockRows(s.rows),
		}, nil)

		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)),
				},
				Users: []string{s.userA.Name, s.userB.Name},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_WEIGHTED_AVERAGE,
				Frequency:       analyticspb.Frequency_WEEKLY,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_USER,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
				},
			},
		}

		// when
		resp, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// then
		s.Require().NoError(err)
		s.Require().NotNil(resp)
		s.Require().NotNil(resp.UserOutcomeStatsSummary)
		s.Require().Len(resp.UserOutcomeStats, 24)
		s.Require().Equal(int32(48), resp.UserOutcomeStatsSummary.RecordCount)

		statsByKey := make(map[string]*analyticspb.UserOutcomeStats, len(resp.UserOutcomeStats))
		for _, stat := range resp.UserOutcomeStats {
			key := stat.GroupBy.User + "|" +
				stat.GroupBy.GetOutcomeFieldDefinitionName() + "|" +
				stat.GroupBy.GetTimeRange().GetStartTimestamp().AsTime().Format(time.RFC3339)
			statsByKey[key] = stat
		}

		resourceFields := s.metadataGetResourceTypeResponse.GetResourceType().GetFields()
		scheduleAdherenceName := resourceFields[2].GetName()
		totalHoursName := resourceFields[3].GetName()
		nchName := resourceFields[4].GetName()

		for weekIdx, weekStart := range s.weekStarts {
			weekKey := weekStart.Format(time.RFC3339)

			userASchedule := statsByKey[s.userA.Name+"|"+scheduleAdherenceName+"|"+weekKey]
			userATotalHours := statsByKey[s.userA.Name+"|"+totalHoursName+"|"+weekKey]
			userANCH := statsByKey[s.userA.Name+"|"+nchName+"|"+weekKey]
			userBSchedule := statsByKey[s.userB.Name+"|"+scheduleAdherenceName+"|"+weekKey]
			userBTotalHours := statsByKey[s.userB.Name+"|"+totalHoursName+"|"+weekKey]
			userBNCH := statsByKey[s.userB.Name+"|"+nchName+"|"+weekKey]

			s.Require().NotNil(userASchedule)
			s.Require().NotNil(userATotalHours)
			s.Require().NotNil(userANCH)
			s.Require().NotNil(userBSchedule)
			s.Require().NotNil(userBTotalHours)
			s.Require().NotNil(userBNCH)

			s.Require().EqualValues(30+2*weekIdx, userASchedule.Value)
			s.Require().EqualValues((8+2*weekIdx)/2.0, userATotalHours.Value)
			s.Require().EqualValues((18+2*weekIdx)/2.0, userANCH.Value)
			s.Require().EqualValues(130+2*weekIdx, userBSchedule.Value)
			s.Require().EqualValues((28+2*weekIdx)/2.0, userBTotalHours.Value)
			s.Require().EqualValues((58+2*weekIdx)/2.0, userBNCH.Value)

			s.Require().Equal(int32(2), userASchedule.RecordCount)
			s.Require().Equal(int32(2), userBSchedule.RecordCount)
		}
	})

	s.Run("TestRetrieveUserOutcomeStats_ValidRequest_UserFilterAppliedWithoutUserGroupBy", func() {
		defer s.convoMockConn.AssertExpectationsAndReset(s.T())

		// Given: grouping by [bucket_time, field_definition_id] → 6 scan columns.
		mockRows := [][]any{
			{s.weekStarts[0], "total_hours", 8.0, 2.0, 8.0, uint64(1)},
		}
		s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(&mockRowsWithErr{
			MockRows: clickhousetesting.NewMockRows(mockRows),
		}, nil)

		req := &analyticspb.RetrieveUserOutcomeStatsRequest{
			Parent: s.profileResourceName,
			Filter: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestFilter{
				TimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC)),
				},
				Users: []string{s.userA.Name},
			},
			GroupBy: &analyticspb.RetrieveUserOutcomeStatsRequest_UserOutcomeStatsRequestGroupBy{
				AggregationMode: analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
				Frequency:       analyticspb.Frequency_WEEKLY,
				AttributeTypes: []analyticspb.UserOutcomeAttributeType{
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_TIME_RANGE,
					analyticspb.UserOutcomeAttributeType_USER_OUTCOME_ATTRIBUTE_TYPE_OUTCOME_FIELD,
				},
			},
		}

		// When
		_, err := s.server.RetrieveUserOutcomeStats(s.ctx, req)

		// Then
		s.Require().NoError(err)
		s.Require().NotEmpty(s.convoMockConn.Calls)
		query, ok := s.convoMockConn.Calls[0].Arguments.Get(1).(string)
		s.Require().True(ok)
		s.Require().Contains(query, "user_id IN", "user filter must be applied even without USER group-by key")
	})
}

// mockRowsWithErr wraps conversations MockRows and implements driver.Rows.Err() (returns nil).
type mockRowsWithErr struct {
	*clickhousetesting.MockRows
}

func (m *mockRowsWithErr) Err() error { return nil }

func (s *RetrieveUserOutcomeStatsTestSuite) setupUserOutcomeStatsFieldsWithValues() {
	s.metadataGetResourceTypeResponse = s.userOutcomeStatsThreeMetricsMetadata()

	s.userA = s.agentsAndGroups.agentUserA
	s.userB = s.agentsAndGroups.agentUserB

	s.weekStarts = []time.Time{
		time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 19, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 26, 0, 0, 0, 0, time.UTC),
	}
	fieldIDs := []string{"schedule_adherence", "total_hours", "nch"}

	s.rows = make([][]any, 0, 24)
	for weekIdx, weekStart := range s.weekStarts {
		for _, user := range []*userpb.User{s.userA, s.userB} {
			parsed, err := userpb.ParseUserName(user.Name)
			s.Require().NoError(err)
			userID := parsed.UserID
			isUserB := user.Name == s.userB.Name

			for _, fieldID := range fieldIDs {
				valueSum := 0.0
				weightSum := 2.0
				weightedValueSum := 0.0
				switch fieldID {
				case "schedule_adherence":
					valueSum = float64(30 + 2*weekIdx)
					if isUserB {
						valueSum += 100
					}
					totalHoursValueSum := float64(8 + 2*weekIdx)
					if isUserB {
						totalHoursValueSum += 20
					}
					weightSum = totalHoursValueSum
					weightedValueSum = valueSum * totalHoursValueSum
				case "total_hours":
					valueSum = float64(8 + 2*weekIdx)
					if isUserB {
						valueSum += 20
					}
					weightedValueSum = valueSum
				case "nch":
					valueSum = float64(18 + 2*weekIdx)
					if isUserB {
						valueSum += 40
					}
					weightedValueSum = valueSum
				}

				s.rows = append(s.rows, []any{weekStart, userID, fieldID, weightedValueSum, weightSum, valueSum, uint64(2)})
			}
		}
	}
}

func (s *RetrieveUserOutcomeStatsTestSuite) userOutcomeStatsThreeMetricsMetadata() *metadatapb.GetResourceTypeResponse {
	outcomeTimeName := metadatapb.FieldDefinitionName{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
		FieldDefinitionID: useroutcomes.TimeFieldID,
	}.String()
	userIDName := metadatapb.FieldDefinitionName{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
		FieldDefinitionID: useroutcomes.UserIDFieldID,
	}.String()
	totalHoursName := metadatapb.FieldDefinitionName{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
		FieldDefinitionID: "total_hours",
	}.String()
	scheduleAdherenceName := metadatapb.FieldDefinitionName{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
		FieldDefinitionID: "schedule_adherence",
	}.String()
	nchName := metadatapb.FieldDefinitionName{
		CustomerID:        s.customerID,
		ProfileID:         s.profileID,
		ResourceTypeID:    useroutcomes.UserOutcomeResourceTypeID,
		FieldDefinitionID: "nch",
	}.String()

	return &metadatapb.GetResourceTypeResponse{
		ResourceType: &metadatapb.ResourceType{
			Fields: []*metadatapb.FieldDefinition{
				{Name: outcomeTimeName},
				{Name: userIDName},
				{Name: scheduleAdherenceName, FieldMetadata: &metadatapb.FieldDefinition_Metadata{
					Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
						UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
							Frequency:             analyticspb.Frequency_DAILY,
							WeightFieldDefinition: totalHoursName,
							AllowTotalAggregation: true,
						},
					},
				}},
				{Name: totalHoursName, FieldMetadata: &metadatapb.FieldDefinition_Metadata{
					Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
						UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
							Frequency:             analyticspb.Frequency_DAILY,
							AllowTotalAggregation: true,
						},
					},
				}},
				{Name: nchName, FieldMetadata: &metadatapb.FieldDefinition_Metadata{
					Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
						UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
							Frequency:             analyticspb.Frequency_DAILY,
							AllowTotalAggregation: true,
						},
					},
				}},
			},
		},
	}
}

func TestValidateAggregationModeAgainstFieldDefinitions(t *testing.T) {
	allowedField := &metadatapb.FieldDefinition{
		Name:        "allowed_field",
		DisplayName: "Allowed",
		FieldMetadata: &metadatapb.FieldDefinition_Metadata{
			Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
				UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
					AllowTotalAggregation: true,
				},
			},
		},
	}
	disallowedField := &metadatapb.FieldDefinition{
		Name:        "disallowed_field",
		DisplayName: "Disallowed",
		FieldMetadata: &metadatapb.FieldDefinition_Metadata{
			Config: &metadatapb.FieldDefinition_Metadata_UserOutcomesFieldMetadataConfig{
				UserOutcomesFieldMetadataConfig: &useroutcomespb.UserOutcomesFieldMetadataConfig{
					AllowTotalAggregation: false,
				},
			},
		},
	}

	t.Run("SUM with all fields allowed returns no error", func(t *testing.T) {
		// When
		err := validateAggregationModeAgainstFieldDefinitions(
			analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
			[]*metadatapb.FieldDefinition{allowedField, allowedField},
		)
		// Then
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("SUM with at least one disallowed field returns InvalidArgument", func(t *testing.T) {
		// When
		err := validateAggregationModeAgainstFieldDefinitions(
			analyticspb.AggregationMode_AGGREGATION_MODE_SUM,
			[]*metadatapb.FieldDefinition{allowedField, disallowedField},
		)
		// Then
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if got := status.Code(err); got != codes.InvalidArgument {
			t.Fatalf("expected InvalidArgument, got %v", got)
		}
		if !strings.Contains(err.Error(), "Disallowed") {
			t.Fatalf("expected error to contain disallowed field display name, got %q", err.Error())
		}
	})

	t.Run("AVERAGE with disallowed field returns no error", func(t *testing.T) {
		// When
		err := validateAggregationModeAgainstFieldDefinitions(
			analyticspb.AggregationMode_AGGREGATION_MODE_AVERAGE,
			[]*metadatapb.FieldDefinition{disallowedField},
		)
		// Then
		if err != nil {
			t.Fatalf("expected no error for AVERAGE mode, got %v", err)
		}
	})

	t.Run("WEIGHTED_AVERAGE with disallowed field returns no error", func(t *testing.T) {
		// When
		err := validateAggregationModeAgainstFieldDefinitions(
			analyticspb.AggregationMode_AGGREGATION_MODE_WEIGHTED_AVERAGE,
			[]*metadatapb.FieldDefinition{disallowedField},
		)
		// Then
		if err != nil {
			t.Fatalf("expected no error for WEIGHTED_AVERAGE mode, got %v", err)
		}
	})
}
