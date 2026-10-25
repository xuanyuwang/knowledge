// nolint:dupl
package analyticsimpl

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
)

type RetrieveAnnotatedTimeSeriesTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) TearDownTest() {
	// Reset flag state after each test to prevent cross-test pollution
	envutil.SetEnv("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "false")
	s.Require().NoError(envflag.Set("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "false"))
	envutil.SetEnv("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "false")
	s.Require().NoError(envflag.Set("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "false"))
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeries() {
	// Scope gomock expectations to this test method
	s.mockESServer.SubTest(s.T())

	// Set flag in both OS env and envflag to keep them synced
	envutil.SetEnv("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true")
	s.Require().NoError(envflag.Set("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true"))

	moment1Values := []float64{
		106, 107, 108, 1, 110, 111, 112,
		113, 114, 115, 116, 117, 118, 119,
		120, 125, 130, 135, 140, 145, 150,
		155, 160, 165, 170, 175, 180, 185,
		190, 195, 200, 205, 210, 215, 220,
	}

	moment1TrendLineValues := []float64{
		50, 55, 60, 65, 70, 75, 80,
		85, 90, 95, 100, 105, 110, 115,
		120, 125, 130, 135, 140, 145, 150,
		155, 160, 165, 170, 175, 180, 185,
		190, 195, 200, 205, 210, 215, 220,
	}

	moment2Values := []float64{
		226, 222, 218, 1, 210, 206, 202,
		198, 194, 190, 186, 182, 178, 174,
		170, 166, 162, 158, 154, 150, 146,
		142, 138, 134, 130, 126, 122, 118,
		114, 110, 106, 102, 98, 94, 90,
	}

	moment3Values := []float64{
		150, 150, 150, 1, 150, 150, 150,
		150, 150, 150, 150, 150, 150, 150,
		150, 150, 150, 150, 150, 150, 150,
		150, 150, 150, 150, 150, 150, 150,
		150, 150, 150, 150, 150, 150, 150,
	}

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock Elasticsearch responses and query expectations
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_numerator_es_response.json"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)

	// When
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 29, 0, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 8, 2, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 8, 5, 0, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 8, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 8, 5, 0, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  2,
		FlexibleTrendMinimumChange: 20,
		MinTotalCount:              10,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name: fmt.Sprintf("%s/moments/cluster-1", parent),
								Type: momentpb.Moment_TOPIC,
							},
							{
								Name: fmt.Sprintf("%s/moments/cluster-2", parent),
								Type: momentpb.Moment_TOPIC,
							},
							{
								Name: fmt.Sprintf("%s/moments/cluster-3", parent),
								Type: momentpb.Moment_TOPIC,
							},
						},
					},
				},
			},
		},
	})

	// Then
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")

	wantDataPoints := func(values []float64, trendLineValues []float64) []*analyticspb.AnnotatedTimeSeriesDataPoint {
		dataPoints := []*analyticspb.AnnotatedTimeSeriesDataPoint{}
		for index, value := range values {
			if index != 3 { // 3 is the index of the day with less than MinTotalCount conversations
				dataPoints = append(dataPoints, &analyticspb.AnnotatedTimeSeriesDataPoint{
					TimeRange: &analyticspb.TimeRange{
						StartTimestamp: timestamppb.New(time.Unix(int64(1656633600+86400*index), 0)),
						EndTimestamp:   timestamppb.New(time.Unix(int64(1656633600+86400*(index+1)), 0)),
					},
					Value:               float32(value),
					PredictedRangeLower: float32(100),
					PredictedRangeUpper: float32(200),
					PredictedValue:      float32(150),
					AnomalyScore:        float32(math.Max(0.0, math.Max(value-200, 100-value)) / 100.0),
					ValueNumerator:      int32(value),
					ValueDenominator:    int32(1),
					TrendLineValue:      float32(trendLineValues[index]),
				})
			}
		}
		return dataPoints
	}

	want := &analyticspb.RetrieveAnnotatedTimeSeriesResponse{
		AnnotatedTimeSeries: []*analyticspb.AnnotatedTimeSeries{
			{
				Attribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("%s/moments/cluster-1", parent),
									Type: momentpb.Moment_TOPIC,
								},
							},
						},
					},
				},
				MaxAnomalyScore: 0.2,
				SlopeOfTrend:    5.0,
				DataPoints:      wantDataPoints(moment1Values, moment1TrendLineValues),
				TrendInfo: &analyticspb.TrendInfo{
					TrendTimeRange: &analyticspb.TimeRange{
						StartTimestamp: timestamppb.New(time.Date(2022, 7, 15, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2022, 8, 5, 0, 0, 0, 0, time.UTC)),
					},
					ValueChange: 105.0,
				},
			},
			{
				Attribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("%s/moments/cluster-2", parent),
									Type: momentpb.Moment_TOPIC,
								},
							},
						},
					},
				},
				MaxAnomalyScore: 0.1,
				SlopeOfTrend:    -4.0,
				DataPoints:      wantDataPoints(moment2Values, moment2Values),
				TrendInfo: &analyticspb.TrendInfo{
					TrendTimeRange: &analyticspb.TimeRange{
						StartTimestamp: timestamppb.New(time.Date(2022, 7, 8, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2022, 8, 5, 0, 0, 0, 0, time.UTC)),
					},
					ValueChange: -112.0,
				},
			},
			{
				Attribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("%s/moments/cluster-3", parent),
									Type: momentpb.Moment_TOPIC,
								},
							},
						},
					},
				},
				MaxAnomalyScore: 0.0,
				SlopeOfTrend:    0.0,
				DataPoints:      wantDataPoints(moment3Values, moment3Values),
				TrendInfo: &analyticspb.TrendInfo{
					TrendTimeRange: &analyticspb.TimeRange{
						StartTimestamp: timestamppb.New(time.Date(2022, 7, 22, 0, 0, 0, 0, time.UTC)),
						EndTimestamp:   timestamppb.New(time.Date(2022, 8, 5, 0, 0, 0, 0, time.UTC)),
					},
					ValueChange: 0.0,
				},
			},
		},
	}

	if diff := cmp.Diff(want, resp, protocmp.Transform()); diff != "" {
		s.FailNow("RetrieveAnnotatedTimeSeries return unexpected response with diff", "(-want +got):\n %v", diff)
	}
}

func TestRetrieveAnnotatedTimeSeries(t *testing.T) {
	suite.Run(t, &RetrieveAnnotatedTimeSeriesTestSuite{})
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesMessageLevel() {
	// Scope gomock expectations to this test method
	s.mockESServer.SubTest(s.T())

	// Enable message-level optimization flag
	envutil.SetEnv("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "true")
	s.Require().NoError(envflag.Set("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "true"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	compositeMomentName := fmt.Sprintf("%s/moments/composite-1", parent)
	includedKeywordMomentName := fmt.Sprintf("%s/moments/included-keyword-1", parent)
	excludedKeywordMomentName := fmt.Sprintf("%s/moments/excluded-keyword-1", parent)

	// Given - Mock moment service for composite moment and its children
	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			// First call: returns the composite moment definition
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: includedKeywordMomentName,
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			// Second call: returns the child keyword moments
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: includedKeywordMomentName,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "transfer",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "my",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "prescription",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "send",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "it",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "to",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "another",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "pharmacy",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "move",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "my",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "meds",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "switch",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "pharmacies",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: excludedKeywordMomentName,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Create an account in the CVS Pharmacy app or at CVS dot com",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Did you know that you can go online to refill a mail order prescription get an order status, check medication costs, get other pharmacy benefit information at Aetna Medicare dot com",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "To hear this again, say repeat",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	// Given - Mock ListMoments for aggregate resolution (returns the composite moment with data = not an aggregate)
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...any) (*momentpb.ListMomentsResponse, error) {
			// Return the composite moment which has moment data (so it's not an aggregate)
			return &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: includedKeywordMomentName,
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
				NextPageToken: "",
			}, nil
		})

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock Elasticsearch responses and query expectations
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_numerator_es_response.json"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When - time ranges chosen so endTime.Hour() == 10 (Hawaii timezone)
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  2,
		FlexibleTrendMinimumChange: 20,
		MinTotalCount:              10,
		FilterConversationByAttribute: &analyticspb.Attribute{
			UsecaseNames: []string{fmt.Sprintf("%s/usecases/initial", parent)},
		},
		FilterMessageByAttribute: &analyticspb.Attribute{
			SpeakerRoles: []string{"visitor"},
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name: compositeMomentName,
							Type: momentpb.Moment_COMPOSITE,
						},
					},
				},
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name: compositeMomentName,
								Type: momentpb.Moment_COMPOSITE,
							},
						},
					},
				},
			},
		},
	})

	// Then
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)

	// Verify we got exactly one time series (grouped by composite moment)
	s.Require().Len(resp.AnnotatedTimeSeries, 1, "Expected one annotated time series")

	timeSeries := resp.AnnotatedTimeSeries[0]

	// Verify the moment attribute
	s.Require().NotNil(timeSeries.Attribute)
	s.Require().Len(timeSeries.Attribute.MomentGroups, 1)
	s.Require().Len(timeSeries.Attribute.MomentGroups[0].Moments, 1)
	s.Equal(compositeMomentName, timeSeries.Attribute.MomentGroups[0].Moments[0].Name)

	// Verify we have 3 data points (one per day: Dec 24, 25, 26 in Hawaii time)
	s.Require().Len(timeSeries.DataPoints, 3, "Expected 3 daily data points")

	// Expected values from ES mock responses (using RELATIVE quantity type):
	// - Numerator (messages with moment): 100, 110, 120 conversations
	// - Denominator (all messages): 1000, 1000, 1000 conversations
	// - Ratios (numerator/denominator): 0.1, 0.11, 0.12
	expectedDataPoints := []struct {
		startTimestamp int64
		value          float32
		numerator      int32
		denominator    int32
	}{
		{1735030800, 0.1, 100, 1000},  // Dec 24, 2024 09:00 UTC (Dec 23 23:00 Hawaii)
		{1735117200, 0.11, 110, 1000}, // Dec 25, 2024 09:00 UTC (Dec 24 23:00 Hawaii)
		{1735203600, 0.12, 120, 1000}, // Dec 26, 2024 09:00 UTC (Dec 25 23:00 Hawaii)
	}

	for i, expected := range expectedDataPoints {
		dp := timeSeries.DataPoints[i]
		s.Require().NotNil(dp.TimeRange, "data point %d should have time range", i)
		s.Equal(expected.startTimestamp, dp.TimeRange.StartTimestamp.Seconds,
			"data point %d start timestamp mismatch", i)
		s.InDelta(expected.value, dp.Value, 0.001,
			"data point %d value mismatch", i)
		s.Equal(expected.numerator, dp.ValueNumerator,
			"data point %d numerator mismatch", i)
		s.Equal(expected.denominator, dp.ValueDenominator,
			"data point %d denominator mismatch", i)
	}
}

// TestRetrieveAnnotatedTimeSeriesCompositeWithTopicChild verifies that when a COMPOSITE moment
// contains a TOPIC child (cluster), the fallback conversation-level path with has_child is used
// instead of the optimized message-level path. TOPIC children require message_group documents
// which would be excluded by speaker_role filters at the message level.
func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesCompositeWithTopicChild() {
	s.mockESServer.SubTest(s.T())

	// Enable message-level optimization flag (test should still use fallback)
	envutil.SetEnv("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "true")
	s.Require().NoError(envflag.Set("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "true"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	compositeMomentName := fmt.Sprintf("%s/moments/composite-with-topic", parent)
	topicChildMomentName := fmt.Sprintf("%s/moments/topic-child-1", parent)
	excludedKeywordMomentName := fmt.Sprintf("%s/moments/excluded-keyword-2", parent)

	// Given - Mock moment service: COMPOSITE moment with a TOPIC child
	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						AppTypes: []momentpb.Moment_AppType{
							momentpb.Moment_APP_TYPE_INSIGHTS,
						},
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											// Inner composite OR containing a TOPIC child
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: topicChildMomentName,
															},
														},
													},
												},
											},
										},
										{
											// Outer excluded keyword
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: topicChildMomentName,
						Type: momentpb.Moment_TOPIC,
					},
					{
						Name: excludedKeywordMomentName,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "automated greeting",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	// Given - Mock ListMoments for aggregate resolution (composite has data = not an aggregate)
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...any) (*momentpb.ListMomentsResponse, error) {
			return &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						AppTypes: []momentpb.Moment_AppType{
							momentpb.Moment_APP_TYPE_INSIGHTS,
						},
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: topicChildMomentName,
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
				NextPageToken: "",
			}, nil
		})

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock Elasticsearch responses and query expectations
	// The denominator uses the same msg_level golden files (denominator is unchanged)
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_numerator_es_response.json"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_composite_with_topic_child_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_composite_with_topic_child_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When - same request as message-level test but with COMPOSITE moment that has TOPIC child
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  2,
		FlexibleTrendMinimumChange: 20,
		MinTotalCount:              10,
		FilterConversationByAttribute: &analyticspb.Attribute{
			UsecaseNames: []string{fmt.Sprintf("%s/usecases/initial", parent)},
		},
		FilterMessageByAttribute: &analyticspb.Attribute{
			SpeakerRoles: []string{"visitor"},
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name: compositeMomentName,
							Type: momentpb.Moment_COMPOSITE,
						},
					},
				},
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name: compositeMomentName,
								Type: momentpb.Moment_COMPOSITE,
							},
						},
					},
				},
			},
		},
	})

	// Then - should succeed using fallback (has_child) path
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)
	s.Require().Len(resp.AnnotatedTimeSeries, 1, "Expected one annotated time series")

	timeSeries := resp.AnnotatedTimeSeries[0]
	s.Require().NotNil(timeSeries.Attribute)
	s.Require().Len(timeSeries.Attribute.MomentGroups, 1)
	s.Require().Len(timeSeries.Attribute.MomentGroups[0].Moments, 1)
	s.Equal(compositeMomentName, timeSeries.Attribute.MomentGroups[0].Moments[0].Name)

	// Verify data points exist - the exact values come from the mock ES response
	s.Require().Len(timeSeries.DataPoints, 3, "Expected 3 daily data points")

	// Verify numerator uses DocCount (fallback path), not cardinality
	expectedDataPoints := []struct {
		startTimestamp int64
		value          float32
		numerator      int32
		denominator    int32
	}{
		{1735030800, 0.1, 100, 1000},
		{1735117200, 0.11, 110, 1000},
		{1735203600, 0.12, 120, 1000},
	}

	for i, expected := range expectedDataPoints {
		dp := timeSeries.DataPoints[i]
		s.Require().NotNil(dp.TimeRange, "data point %d should have time range", i)
		s.Equal(expected.startTimestamp, dp.TimeRange.StartTimestamp.Seconds,
			"data point %d start timestamp mismatch", i)
		s.InDelta(expected.value, dp.Value, 0.001,
			"data point %d value mismatch", i)
		s.Equal(expected.numerator, dp.ValueNumerator,
			"data point %d numerator mismatch", i)
		s.Equal(expected.denominator, dp.ValueDenominator,
			"data point %d denominator mismatch", i)
	}
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesWithAggregateSubcategories() {
	// Scope gomock expectations to this test method
	s.mockESServer.SubTest(s.T())

	// Set flag in both OS env and envflag
	envutil.SetEnv("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true")
	s.Require().NoError(envflag.Set("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)

	// Define moment names with nested aggregate hierarchy
	// aggregate-parent (input) → nested-aggregate (NOT in input) → [leaf-grandchild-1, leaf-grandchild-2]
	aggregateWithChildrenName := fmt.Sprintf("%s/moments/aggregate-parent", parent)
	nestedAggregateName := fmt.Sprintf("%s/moments/nested-aggregate", parent)
	leafGrandchild1Name := fmt.Sprintf("%s/moments/leaf-grandchild-1", parent)
	leafGrandchild2Name := fmt.Sprintf("%s/moments/leaf-grandchild-2", parent)
	aggregateNoChildrenName := fmt.Sprintf("%s/moments/aggregate-empty", parent)

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock ListMoments to return all composite moments for hierarchy traversal
	// This includes the nested aggregate hierarchy: aggregate-parent → nested-aggregate → [leaf-grandchild-1, leaf-grandchild-2]
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...any) (*momentpb.ListMomentsResponse, error) {
			return &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name:                   aggregateWithChildrenName,
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "",
						MomentData:             nil, // Empty moment data = aggregate
					},
					{
						Name:                   nestedAggregateName,
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: aggregateWithChildrenName,
						MomentData:             nil, // Empty moment data = nested aggregate (should be filtered out)
					},
					{
						Name:                   leafGrandchild1Name,
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: nestedAggregateName,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   leafGrandchild2Name,
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: nestedAggregateName,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   aggregateNoChildrenName,
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: "",
						MomentData:             nil, // Empty moment data = aggregate
					},
				},
				NextPageToken: "",
			}, nil
		})

	// Given - Mock BatchGetMoments with AnyTimes to handle all calls flexibly
	// This will be called to fetch the leaf moments and their keyword children
	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			// Check which moments are being requested and return appropriate data
			keyword1Name := fmt.Sprintf("%s/moments/keyword-1", parent)
			keyword2Name := fmt.Sprintf("%s/moments/keyword-2", parent)
			response := &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{},
			}

			for _, name := range req.Names {
				switch name {
				case leafGrandchild1Name, leafGrandchild2Name:
					// Return leaf composite moment
					// Keywords must be nested inside an inner composite to be treated as "included"
					var baseMoment string
					switch name {
					case leafGrandchild1Name:
						baseMoment = keyword1Name
					case leafGrandchild2Name:
						baseMoment = keyword2Name
					}
					response.Moments = append(response.Moments, &momentpb.Moment{
						Name:     name,
						Type:     momentpb.Moment_COMPOSITE,
						AppTypes: []momentpb.Moment_AppType{momentpb.Moment_APP_TYPE_INSIGHTS},
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											// Keyword must be inside a nested composite to be treated as "included"
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: baseMoment,
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					})
				case keyword1Name:
					// Return keyword moment 1
					response.Moments = append(response.Moments, &momentpb.Moment{
						Name: name,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "keyword1",
												},
											},
										},
									},
								},
							},
						},
					})
				case keyword2Name:
					// Return keyword moment 2
					response.Moments = append(response.Moments, &momentpb.Moment{
						Name: name,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "keyword2",
												},
											},
										},
									},
								},
							},
						},
					})
				}
			}
			return response, nil
		})

	// Given - Mock Elasticsearch responses
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_aggregate_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_aggregate_numerator_es_response.json"))

	// Validate ES queries for aggregate expansion - the aggregates should be expanded to their leaf subcategories
	// The important validation is:
	// 1. Denominator query has correct base structure and empty moment aggs
	// 2. Numerator query has moment filters for the expanded leaf keywords (keyword1, keyword2)
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_aggregate_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_aggregate_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 8, 0, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 9, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  1,
		FlexibleTrendMinimumChange: 10,
		MinTotalCount:              5,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name: aggregateWithChildrenName,
								Type: momentpb.Moment_COMPOSITE,
							},
							{
								Name: aggregateNoChildrenName,
								Type: momentpb.Moment_COMPOSITE,
							},
							// Also include a leaf directly to test deduplication
							{
								Name: leafGrandchild1Name,
								Type: momentpb.Moment_COMPOSITE,
							},
						},
					},
				},
			},
		},
	})

	// Then
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)

	// The aggregate-parent should be expanded through nested-aggregate to its 2 leaf grandchildren
	// The intermediate nested-aggregate should be filtered out (it has no MomentData)
	// The aggregate-empty should return no data (filtered out, no children)
	// The directly specified leaf should be deduplicated (only appear once)
	// So we expect 2 time series: one for each unique leaf grandchild
	s.Require().Len(resp.AnnotatedTimeSeries, 2, "Expected 2 annotated time series for 2 unique leaf moments")

	// Verify the moment names in the response
	momentNames := make([]string, len(resp.AnnotatedTimeSeries))
	for i, ts := range resp.AnnotatedTimeSeries {
		s.Require().NotNil(ts.Attribute)
		s.Require().Len(ts.Attribute.MomentGroups, 1)
		s.Require().Len(ts.Attribute.MomentGroups[0].Moments, 1)
		momentNames[i] = ts.Attribute.MomentGroups[0].Moments[0].Name
	}

	// Both leaf grandchildren should be present, nested-aggregate should be filtered out
	s.Contains(momentNames, leafGrandchild1Name, "leaf-grandchild-1 should be in response")
	s.Contains(momentNames, leafGrandchild2Name, "leaf-grandchild-2 should be in response")
}

func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesMessageLevelFlagOff() {
	s.mockESServer.SubTest(s.T())

	// Explicitly disable the flag to verify fallback path
	envutil.SetEnv("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "false")
	s.Require().NoError(envflag.Set("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", "false"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)
	compositeMomentName := fmt.Sprintf("%s/moments/composite-1", parent)
	includedKeywordMomentName := fmt.Sprintf("%s/moments/included-keyword-1", parent)
	excludedKeywordMomentName := fmt.Sprintf("%s/moments/excluded-keyword-1", parent)

	// Given - Mock moment service (same as message level test)
	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: includedKeywordMomentName,
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	s.mockMomentClient.EXPECT().
		BatchGetMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.BatchGetMomentsRequest, _ ...any) (*momentpb.BatchGetMomentsResponse, error) {
			return &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: includedKeywordMomentName,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "transfer",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "my",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "prescription",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "send",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "it",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "to",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "another",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "pharmacy",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "move",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "my",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "meds",
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Expression{
													Expression: &momentpb.KeywordMomentData_KeywordExpression{
														Operator: momentpb.KeywordMomentData_SEQUENCE,
														Operands: []*momentpb.KeywordMomentData_Operand{
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "switch",
																},
															},
															{
																Operand: &momentpb.KeywordMomentData_Operand_Phrase{
																	Phrase: "pharmacies",
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Name: excludedKeywordMomentName,
						Type: momentpb.Moment_KEYWORD,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_KeywordMomentData{
								KeywordMomentData: &momentpb.KeywordMomentData{
									KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
										Operator: momentpb.KeywordMomentData_OR,
										Operands: []*momentpb.KeywordMomentData_Operand{
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Create an account in the CVS Pharmacy app or at CVS dot com",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "Did you know that you can go online to refill a mail order prescription get an order status, check medication costs, get other pharmacy benefit information at Aetna Medicare dot com",
												},
											},
											{
												Operand: &momentpb.KeywordMomentData_Operand_Phrase{
													Phrase: "To hear this again, say repeat",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}, nil
		})

	// Given - Mock ListMoments for aggregate resolution (returns the composite moment with data = not an aggregate)
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1).
		DoAndReturn(func(_ context.Context, req *momentpb.ListMomentsRequest, _ ...any) (*momentpb.ListMomentsResponse, error) {
			return &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: compositeMomentName,
						Type: momentpb.Moment_COMPOSITE,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_AND,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
												CompositeMoment: &momentpb.CompositeMomentData{
													Operator: momentpb.CompositeMomentData_OR,
													Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
														{
															Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																BaseMoment: includedKeywordMomentName,
															},
														},
													},
												},
											},
										},
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: excludedKeywordMomentName,
											},
										},
									},
								},
							},
						},
					},
				},
				NextPageToken: "",
			}, nil
		})

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock ES responses - use same responses as message level test (DocCount values)
	// Denominator response is identical
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_denominator_es_response.json"))
	// Numerator response should have DocCount (not cardinality)
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_off_numerator_es_response.json"))

	// Expect has_child queries (fallback path), not message-level queries
	// Denominator is same regardless of flag
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	// Numerator should use has_child when flag is off
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_msg_level_off_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When - identical request to message level test
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2025, 12, 24, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 3, 22, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2026, 2, 25, 10, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2026, 3, 25, 10, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  2,
		FlexibleTrendMinimumChange: 20,
		MinTotalCount:              10,
		FilterConversationByAttribute: &analyticspb.Attribute{
			UsecaseNames: []string{fmt.Sprintf("%s/usecases/initial", parent)},
		},
		FilterMessageByAttribute: &analyticspb.Attribute{
			SpeakerRoles: []string{"visitor"},
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name: compositeMomentName,
							Type: momentpb.Moment_COMPOSITE,
						},
					},
				},
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name: compositeMomentName,
								Type: momentpb.Moment_COMPOSITE,
							},
						},
					},
				},
			},
		},
	})

	// Then - should use fallback path (has_child with DocCount)
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)
	s.Require().Len(resp.AnnotatedTimeSeries, 1, "Expected one annotated time series")

	timeSeries := resp.AnnotatedTimeSeries[0]
	s.Require().NotNil(timeSeries.Attribute)
	s.Require().Len(timeSeries.Attribute.MomentGroups, 1)
	s.Require().Len(timeSeries.Attribute.MomentGroups[0].Moments, 1)
	s.Equal(compositeMomentName, timeSeries.Attribute.MomentGroups[0].Moments[0].Name)

	// Verify data points use DocCount from fallback path
	s.Require().Len(timeSeries.DataPoints, 3, "Expected 3 daily data points")

	// Using DocCount values from the mock response (100, 110, 120)
	expectedDataPoints := []struct {
		startTimestamp int64
		value          float32
		numerator      int32
		denominator    int32
	}{
		{1735030800, 0.1, 100, 1000},
		{1735117200, 0.11, 110, 1000},
		{1735203600, 0.12, 120, 1000},
	}

	for i, expected := range expectedDataPoints {
		dp := timeSeries.DataPoints[i]
		s.Require().NotNil(dp.TimeRange, "data point %d should have time range", i)
		s.Equal(expected.startTimestamp, dp.TimeRange.StartTimestamp.Seconds,
			"data point %d start timestamp mismatch", i)
		s.InDelta(expected.value, dp.Value, 0.001,
			"data point %d value mismatch", i)
		s.Equal(expected.numerator, dp.ValueNumerator,
			"data point %d numerator mismatch", i)
		s.Equal(expected.denominator, dp.ValueDenominator,
			"data point %d denominator mismatch", i)
	}
}

// TestRetrieveAnnotatedTimeSeriesTopicDiscoveryV3GroupBy verifies that grouping by a parent V3
// category yields a single series whose bucket OR-matches the whole subtree closure.
func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesTopicDiscoveryV3GroupBy() {
	s.mockESServer.SubTest(s.T())

	envutil.SetEnv("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true")
	s.Require().NoError(envflag.Set("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock the V3 category-tree fetch (the only ListMoments call)
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		Times(1).
		Return(&momentpb.ListMomentsResponse{Moments: v3Tree()}, nil)

	// Given - Mock Elasticsearch responses and query expectations
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_v3_group_by_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_v3_group_by_numerator_es_response.json"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_v3_group_by_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_v3_group_by_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When - group by the non-leaf V3 category cat-1
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 8, 0, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 9, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  1,
		FlexibleTrendMinimumChange: 10,
		MinTotalCount:              5,
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name:         v3TestPrefix + "cat-1",
								Type:         momentpb.Moment_TOPIC,
								DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
							},
						},
					},
				},
			},
		},
	})

	// Then - a single series labeled cat-1 that aggregates the whole subtree, stamped TOPIC + V3
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)
	s.Require().Len(resp.AnnotatedTimeSeries, 1, "expected a single series aggregating the cat-1 subtree")

	ts := resp.AnnotatedTimeSeries[0]
	s.Require().NotNil(ts.Attribute)
	s.Require().Len(ts.Attribute.MomentGroups, 1)
	s.Require().Len(ts.Attribute.MomentGroups[0].Moments, 1)
	moment := ts.Attribute.MomentGroups[0].Moments[0]
	s.Equal(v3TestPrefix+"cat-1", moment.Name)
	s.Equal(momentpb.Moment_TOPIC, moment.Type)
	s.Equal(momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC, moment.DetailedType)
}

// TestRetrieveAnnotatedTimeSeriesTopicDiscoveryV3Filter verifies that filtering by a parent V3
// category expands the filter to its full subtree closure in the ES query.
func (s *RetrieveAnnotatedTimeSeriesTestSuite) TestRetrieveAnnotatedTimeSeriesTopicDiscoveryV3Filter() {
	s.mockESServer.SubTest(s.T())

	envutil.SetEnv("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true")
	s.Require().NoError(envflag.Set("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "true"))

	parent := fmt.Sprintf("customers/%s/profiles/%s", s.customerID, s.profileID)

	// Given - Mock config client
	s.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
		Return(&config.ProfileConfig{}, nil)

	// Given - Mock the V3 category-tree fetch (the only ListMoments call)
	s.mockMomentClient.EXPECT().
		ListMoments(gomock.Any(), v3ListMomentsMatcher{}).
		Times(1).
		Return(&momentpb.ListMomentsResponse{Moments: v3Tree()}, nil)

	// Given - Mock Elasticsearch responses and query expectations
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_v3_filter_denominator_es_response.json"))
	s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile("RetrieveAnnotatedTimeSeries_v3_filter_numerator_es_response.json"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_v3_filter_denominator_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	s.mockESServer.EXPECT().Search(s.IndexName, s.getFile("RetrieveAnnotatedTimeSeries_v3_filter_numerator_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When - filter by the non-leaf V3 category cat-1, group by the leaf V3 category cat-2
	resp, err := s.Client.RetrieveAnnotatedTimeSeries(s.ctx, &analyticspb.RetrieveAnnotatedTimeSeriesRequest{
		Parent:       parent,
		Frequency:    analyticspb.Frequency_DAILY,
		QuantityType: analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT,
		TrainingTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 8, 0, 0, 0, 0, time.UTC)),
		},
		AnomalyTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 9, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		TrendTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 7, 1, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 10, 0, 0, 0, 0, time.UTC)),
		},
		FlexibleTrendMinimumWeeks:  1,
		FlexibleTrendMinimumChange: 10,
		MinTotalCount:              5,
		FilterConversationByAttribute: &analyticspb.Attribute{
			MomentGroups: []*analyticspb.MomentGroup{
				{
					Moments: []*momentpb.Moment{
						{
							Name:         v3TestPrefix + "cat-1",
							Type:         momentpb.Moment_TOPIC,
							DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
						},
					},
				},
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{
				AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
					AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
				},
			},
			{
				AttributeOneof: &analyticspb.AttributeStructure_MomentGroup{
					MomentGroup: &analyticspb.MomentGroup{
						Moments: []*momentpb.Moment{
							{
								Name:         v3TestPrefix + "cat-2",
								Type:         momentpb.Moment_TOPIC,
								DetailedType: momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC,
							},
						},
					},
				},
			},
		},
	})

	// Then - the leaf group-by yields a single series for cat-2
	s.Require().NoError(err, "RetrieveAnnotatedTimeSeries()")
	s.Require().NotNil(resp)
	s.Require().Len(resp.AnnotatedTimeSeries, 1, "leaf group-by category yields a single series")

	timeSeries := resp.AnnotatedTimeSeries[0]
	s.Require().NotNil(timeSeries.Attribute)
	s.Require().Len(timeSeries.Attribute.MomentGroups, 1)
	s.Require().Len(timeSeries.Attribute.MomentGroups[0].Moments, 1)
	s.Equal(v3TestPrefix+"cat-2", timeSeries.Attribute.MomentGroups[0].Moments[0].Name)
}
