package analyticsimpl

import (
	"context"
	"testing"

	nonpubconfig "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/config"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/featureflag"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	"github.com/cresta/shared-go/fn"
	logger "github.com/cresta/shared-go/framework/log"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
)

// IsMessageOrConvoEntityDurationSuite unit-tests the flag-gated classification of
// attributeValueTypeConversationDurationRange in isMessageOrConvoEntity (INSI-4329).
// Unlike the handle_time classification (unconditional dual since PR #30858), duration
// only becomes dual when the rollout flag is enabled, so the test exercises both flag
// states plus a handle_time regression guard.
type IsMessageOrConvoEntityDurationSuite struct {
	suite.Suite
	ctx context.Context
}

func TestIsMessageOrConvoEntity_ConversationDurationRange(t *testing.T) {
	suite.Run(t, new(IsMessageOrConvoEntityDurationSuite))
}

func (s *IsMessageOrConvoEntityDurationSuite) SetupTest() {
	s.ctx = context.Background()
}

// durationRangeValueSet builds a ChartValueSet carrying a single ConversationDurationRange
// attribute value, the input shape isMessageOrConvoEntity classifies for duration filters.
func (s *IsMessageOrConvoEntityDurationSuite) durationRangeValueSet() *dashboardpb.ChartValueSet {
	return &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{
				AttributeValues: []*dashboardpb.ChartAttributeValue{
					{
						Value: &dashboardpb.ChartAttributeValue_ConversationDurationRange{
							ConversationDurationRange: &dashboardpb.ChartDurationRange{
								LowerBound: &dashboardpb.ChartDurationRange_Bound{
									Value:       &durationpb.Duration{Seconds: 123},
									IsExclusive: true,
								},
								UpperBound: &dashboardpb.ChartDurationRange_Bound{
									Value: &durationpb.Duration{Seconds: 456},
								},
							},
						},
					},
				},
			},
		},
	}
}

// handleTimeRangeValueSet builds a ChartValueSet carrying a single ConversationHandleTimeRange
// attribute value, used as an unconditional-dual regression guard.
func (s *IsMessageOrConvoEntityDurationSuite) handleTimeRangeValueSet() *dashboardpb.ChartValueSet {
	return &dashboardpb.ChartValueSet{
		Values: &dashboardpb.ChartValueSet_SingleValues{
			SingleValues: &dashboardpb.ChartValueSet_Single{
				AttributeValues: []*dashboardpb.ChartAttributeValue{
					{
						Value: &dashboardpb.ChartAttributeValue_ConversationHandleTimeRange{
							ConversationHandleTimeRange: &dashboardpb.ChartFloatRange{
								LowerBound: &dashboardpb.ChartFloatRange_Bound{
									Value:       10,
									IsExclusive: true,
								},
								UpperBound: &dashboardpb.ChartFloatRange_Bound{
									Value: 20,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *IsMessageOrConvoEntityDurationSuite) TestClassification() {
	for _, tc := range []struct {
		desc               string
		valueSet           *dashboardpb.ChartValueSet
		featureFlagBools   map[string]bool
		wantIsMessageLevel bool
		wantIsConvoLevel   bool
	}{
		{
			desc:               "duration_filter_flag_off_stays_conversation_level_only",
			valueSet:           s.durationRangeValueSet(),
			featureFlagBools:   nil,
			wantIsMessageLevel: false,
			wantIsConvoLevel:   true,
		},
		{
			desc:               "duration_filter_flag_on_becomes_dual_message_and_conversation_level",
			valueSet:           s.durationRangeValueSet(),
			featureFlagBools:   map[string]bool{"enable_reading_denormalized_conversation_duration_from_message_doc": true},
			wantIsMessageLevel: true,
			wantIsConvoLevel:   true,
		},
		{
			desc:               "handle_time_filter_unconditional_dual_regardless_of_duration_flag",
			valueSet:           s.handleTimeRangeValueSet(),
			featureFlagBools:   nil,
			wantIsMessageLevel: true,
			wantIsConvoLevel:   true,
		},
	} {
		s.Run(tc.desc, func() {
			// Given
			a := AnalyticsServiceImpl{
				featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{Bools: tc.featureFlagBools}),
				logger:            logger.GetLogger(),
			}

			// When
			isMessageLevel, isConversationLevel, err := a.isMessageOrConvoEntity(s.ctx, &config.ProfileConfig{}, tc.valueSet)

			// Then
			s.Require().NoError(err)
			s.Require().Equal(tc.wantIsMessageLevel, isMessageLevel, "isMessageLevel mismatch")
			s.Require().Equal(tc.wantIsConvoLevel, isConversationLevel, "isConversationLevel mismatch")
		})
	}
}

// TestEvaluateAnalyticsChart_ConversationDurationRangeFilter_FlagOn verifies the INSI-4329
// read-side change end-to-end: with the rollout flag enabled, a conversation-count chart that
// groups by subcategory and filters on conversation duration uses the simplified message-doc
// query path (filtering on the denormalized conversation_duration_secs) instead of the
// conversation-doc + join path. With the flag off (default), duration stays conversation-level
// and the conversation-doc + join path is used; that regression path is already covered by the
// pre-existing EvaluateAnalyticsChart_basic_convo_duration_as_filter golden.
func TestEvaluateAnalyticsChart_ConversationDurationRangeFilter_FlagOn(t *testing.T) {
	// Given
	r := EvaluateAnalyticsChartCommonTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	envutil.SetEnv("ENABLE_SIMPLIFIED_SUBCATEGORY_GROUP_BY_IN_EVALUATE_ANALYTICS_CHART", "true")
	t.Cleanup(func() {
		envutil.SetEnv("ENABLE_SIMPLIFIED_SUBCATEGORY_GROUP_BY_IN_EVALUATE_ANALYTICS_CHART", "false")
	})
	r.featureFlagStubClientConfig = featureflag.StubClientConfig{
		Bools: map[string]bool{
			"enable_reading_denormalized_conversation_duration_from_message_doc": true,
		},
	}
	r.SetupSuite()

	resourceACL := &auth.ResourceACL{
		IsRootAccess:    true,
		ManagedUserIDs:  []string{"agent-id-1", "agent-id-2", "agent-id-3", "agent-id-4"},
		ManagedGroupIDs: []string{"4", "5", "6"},
	}

	// subcategory-1 is a composite moment (NOT excluded_keyword_moment_1) AND (OR(moment_1_keyword_included_1, moment_1_keyword_included_2)).
	getCompositeMomentResp := &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{
			{
				Name: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
				Type: momentpb.Moment_COMPOSITE,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_CompositeMomentData{
						CompositeMomentData: &momentpb.CompositeMomentData{
							Operator: momentpb.CompositeMomentData_AND,
							Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
								{
									Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
										CompositeMoment: &momentpb.CompositeMomentData{
											Operator: momentpb.CompositeMomentData_NOT,
											Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
												{
													Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
														BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
													},
												},
											},
										},
									},
								},
								{
									Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
										CompositeMoment: &momentpb.CompositeMomentData{
											Operator: momentpb.CompositeMomentData_OR,
											Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
												{
													Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
														BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
													},
												},
												{
													Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
														BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
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
	}
	getChildMomentResp := &momentpb.BatchGetMomentsResponse{
		Moments: []*momentpb.Moment{
			{
				Name: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
				Type: momentpb.Moment_KEYWORD,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_KeywordMomentData{
						KeywordMomentData: &momentpb.KeywordMomentData{
							KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_OR,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "moment_1_keyword_excluded_1",
										},
									},
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "moment_1_keyword_excluded_2",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_1",
				Type: momentpb.Moment_KEYWORD,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_KeywordMomentData{
						KeywordMomentData: &momentpb.KeywordMomentData{
							KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_OR,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "moment_1_keyword_included_1",
										},
									},
								},
							},
						},
					},
				},
			},
			{
				Name: "customers/customer_id/profiles/profile_id/moments/moment_1_keyword_included_2",
				Type: momentpb.Moment_KEYWORD,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_KeywordMomentData{
						KeywordMomentData: &momentpb.KeywordMomentData{
							KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_OR,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "moment_1_keyword_included_2",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// req: conversation-count metric, subcategory-1 groupBy, duration filter, time groupBy.
	// With the flag on, duration is dual-classified so the duration filter does not set
	// hasConversationAttributeFiltersOrGroupBys and the message-doc no-join path is used.
	req := &analyticspb.EvaluateAnalyticsChartRequest{
		Parent: r.profileResourceName,
		AnalyticsChart: &dashboardpb.AnalyticsChart{
			Name:        "",
			DisplayName: "display_name",
			Description: "",
			DataConfig: &dashboardpb.ChartDataConfig{
				Metrics: []*dashboardpb.ChartMetric{
					{
						Metric: &dashboardpb.ChartMetric_ForConversation_{
							ForConversation: &dashboardpb.ChartMetric_ForConversation{
								StatsType: &dashboardpb.StatsTypeForConversation{
									StatsType: &dashboardpb.StatsTypeForConversation_CommonStatsType{
										CommonStatsType: dashboardpb.CommonStatsType_COMMON_STATS_TYPE_CONVERSATION_COUNT,
									},
								},
							},
						},
					},
				},
				Filters: []*dashboardpb.ChartDataFilter{
					{
						Operator: dashboardpb.ChartDataFilter_IN,
						ValueSet: &dashboardpb.ChartValueSet{
							Values: &dashboardpb.ChartValueSet_SingleValues{
								SingleValues: &dashboardpb.ChartValueSet_Single{
									AttributeValues: []*dashboardpb.ChartAttributeValue{
										{
											Value: &dashboardpb.ChartAttributeValue_ConversationDurationRange{
												ConversationDurationRange: &dashboardpb.ChartDurationRange{
													LowerBound: &dashboardpb.ChartDurationRange_Bound{
														Value:       &durationpb.Duration{Seconds: 123},
														IsExclusive: true,
													},
													UpperBound: &dashboardpb.ChartDurationRange_Bound{
														Value: &durationpb.Duration{Seconds: 456},
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
				GroupBy: []*dashboardpb.ChartGroupBy{
					{
						GroupByMode: &dashboardpb.ChartGroupBy_ByProvidedValue{
							ByProvidedValue: &dashboardpb.ChartGroupBy_ProvidedValue{
								ValueSet: &dashboardpb.ChartValueSet{
									Values: &dashboardpb.ChartValueSet_SingleValues{
										SingleValues: &dashboardpb.ChartValueSet_Single{
											AttributeValues: []*dashboardpb.ChartAttributeValue{
												{
													Value: &dashboardpb.ChartAttributeValue_SubcategoryValue{
														SubcategoryValue: &dashboardpb.SubcategoryForChart{
															MomentName: "customers/customer_id/profiles/profile_id/moments/subcategory-1",
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
						GroupByMode: &dashboardpb.ChartGroupBy_ByTime{
							ByTime: &dashboardpb.ChartGroupBy_Time{
								Interval: dashboardpb.ChartGroupBy_Time_DAY,
							},
						},
					},
				},
			},
			RenderingConfig: &dashboardpb.RenderingConfig{
				ChartDisplayType: dashboardpb.ChartDisplayType_BAR_CHART,
			},
			Audience:      &dashboardpb.DashboardAudience{},
			Accessibility: dashboardpb.DashboardAccessibility_PERSONAL,
		},
	}

	// When
	gomock.InOrder(
		r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(getCompositeMomentResp, nil),
		r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(getChildMomentResp, nil),
	)
	cConfig := nonpubconfig.CustomerConfig{
		Customer: "customers/customer_id",
		ProfileConfigs: []*nonpubconfig.ProfileConfig{
			{
				Name: "customers/customer_id/profiles/profile_id",
			},
		},
		AuthConfig: &nonpubconfig.CustomerAuthConfig{
			EnableAdvancedDataAccessControl: true,
		},
	}
	testCustomerConfig := fn.Must(config.NewCustomerConfig(&cConfig))
	r.mockConfigClient.EXPECT().
		GetProfileConfig(gomock.Any(), gomock.Any()).AnyTimes().
		Return(testCustomerConfig.ListProfiles()[0], nil)
	r.mockConfigClient.EXPECT().
		Get(gomock.Any()).Times(1).
		Return(testCustomerConfig, nil)
	r.setupACLHelper(gomock.Any(), 1, resourceACL)
	r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile("EvaluateAnalyticsChart_subcategory_as_groupby_and_denormalized_metadata_as_filters_simplified_es_response.json"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("EvaluateAnalyticsChart_convo_duration_filter_simplified_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	resp, err := r.Client.EvaluateAnalyticsChart(r.ctx, req)

	// Then
	if err != nil {
		t.Fatalf("Got code %v in error: %v", err, status.Code(err))
	}
	// resp is not asserted: this test verifies the ES request shape (the simplified message-doc
	// path) via the golden file above, not the parsed response values.
	_ = resp
}

// FilterFromConversationDurationRangeSuite unit-tests the flag-gated DocumentTypeMessage branch
// of filterFromConversationDurationRange (INSI-4329). A message-doc query is used inherently for
// message-level metric charts regardless of the flag, so the branch must fall back to has_parent
// when the flag is off (pre-backfill) and only use the denormalized conversation_duration_secs
// field when the flag is on — otherwise message-level charts filtered by duration go empty.
type FilterFromConversationDurationRangeSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFilterFromConversationDurationRange(t *testing.T) {
	suite.Run(t, new(FilterFromConversationDurationRangeSuite))
}

func (s *FilterFromConversationDurationRangeSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FilterFromConversationDurationRangeSuite) durationRange() *dashboardpb.ChartDurationRange {
	return &dashboardpb.ChartDurationRange{
		LowerBound: &dashboardpb.ChartDurationRange_Bound{
			Value:       &durationpb.Duration{Seconds: 123},
			IsExclusive: true,
		},
		UpperBound: &dashboardpb.ChartDurationRange_Bound{
			Value: &durationpb.Duration{Seconds: 456},
		},
	}
}

func (s *FilterFromConversationDurationRangeSuite) TestBranch() {
	for _, tc := range []struct {
		desc              string
		documentType      elasticsearch.DocumentType
		featureFlagBools  map[string]bool
		wantHasParent     bool
		wantDenormalized  bool
		wantConvoDocRange bool
	}{
		{
			desc:             "message_doc_flag_off_uses_has_parent",
			documentType:     elasticsearch.DocumentTypeMessage,
			featureFlagBools: nil,
			wantHasParent:    true,
		},
		{
			desc:             "message_doc_flag_on_uses_denormalized_field",
			documentType:     elasticsearch.DocumentTypeMessage,
			featureFlagBools: map[string]bool{"enable_reading_denormalized_conversation_duration_from_message_doc": true},
			wantDenormalized: true,
		},
		{
			desc:              "conversation_doc_uses_conversation_field_flag_off",
			documentType:      elasticsearch.DocumentTypeConversation,
			featureFlagBools:  nil,
			wantConvoDocRange: true,
		},
		{
			desc:              "conversation_doc_uses_conversation_field_flag_on",
			documentType:      elasticsearch.DocumentTypeConversation,
			featureFlagBools:  map[string]bool{"enable_reading_denormalized_conversation_duration_from_message_doc": true},
			wantConvoDocRange: true,
		},
	} {
		s.Run(tc.desc, func() {
			// Given
			a := AnalyticsServiceImpl{
				featureFlagClient: featureflag.NewStubClient(featureflag.StubClientConfig{Bools: tc.featureFlagBools}),
				logger:            logger.GetLogger(),
			}

			// When
			ret, err := a.filterFromConversationDurationRange(s.ctx, s.durationRange(), tc.documentType)

			// Then
			s.Require().NoError(err)
			switch tc.documentType {
			case elasticsearch.DocumentTypeMessage:
				msgFilter, ok := ret.(elasticsearch.MsgFilter)
				s.Require().True(ok, "expected MsgFilter for message doc, got %T", ret)
				s.Require().Equal(tc.wantHasParent, msgFilter.HasParent != nil, "HasParent presence mismatch")
				s.Require().Equal(tc.wantDenormalized, msgFilter.Range != nil && msgFilter.Range.ConversationDurationSec != nil, "denormalized field presence mismatch")
			case elasticsearch.DocumentTypeConversation:
				convoFilter, ok := ret.(elasticsearch.ConvoFilter)
				s.Require().True(ok, "expected ConvoFilter for conversation doc, got %T", ret)
				s.Require().Equal(tc.wantConvoDocRange, convoFilter.Range != nil && convoFilter.Range.ConversationDurationSec != nil, "conversation-doc range presence mismatch")
			}
		})
	}
}
