package analyticsimpl

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	envutil "github.com/cresta/shared-go/util/env"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	_ "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
)

type RetrieveConversationOutcomeStatsTestSuite struct {
	AnalyticsV2TestSuite
}

var pbBehaviorTemplateStruct structpb.Struct

func (s *RetrieveConversationOutcomeStatsTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()

	behaviorTemplateString := `{
		"version": 2,
		"items": [
  		{
    		"displayName": "Labeled Radios",
    		"type": "labeled-radios",
    		"weight": 1,
    		"settings": {"options": [{"label": "Yes", "value": 1}, {"label": "No", "value": 0}]},
    		"identifier": "labeled-radios",
    		"auto_qa": {
      		"triggers": [
      			{
        			"type": "behavior",
        			"resource_name": "customers/customer_id/profiles/profile_id/behaviors/behavior1"
      			}
      		],
      		"detected": 1,
      		"not_detected": 0
    		}
  		}
  	]
	}
  `
	s.Require().NoError(json.Unmarshal([]byte(behaviorTemplateString), &pbBehaviorTemplateStruct))
	s.Require().NoError(s.DB().Create(&model.ScorecardTemplates{
		Customer:   "customer_id",
		Profile:    "profile_id",
		ResourceID: "template_id",
		Revision:   "r1",
		Template:   "{}",
		Title:      "title1",
	}).Error)
}

func TestRetrieveConversationOutcomeStats(t *testing.T) {
	r := RetrieveConversationOutcomeStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	for _, tc := range []struct {
		desc                          string
		esReqPath                     string
		esRespPath                    string
		getScorecardTemplateRequest   *coachingpb.GetScorecardTemplateRequest
		getScorecardTemplateResponse  *coachingpb.GetScorecardTemplateResponse
		listGroupsRequest             *userpb.ListGroupsRequest
		listGroupsResponse            *userpb.ListGroupsResponse
		listUsersForAnalyticsRequest  []*internaluserpb.ListUsersForAnalyticsRequest
		listUsersForAnalyticsResponse []*internaluserpb.ListUsersForAnalyticsResponse
		retrieveBehaviorsRequest      *behaviorpb.RetrieveBehaviorsRequest
		retrieveBehaviorsResponse     *behaviorpb.RetrieveBehaviorsResponse
		req                           *analyticspb.RetrieveConversationOutcomeStatsRequest
		want                          *analyticspb.RetrieveConversationOutcomeStatsResponse
		batchGetMomentResps           []*momentpb.BatchGetMomentsResponse
		listMomentsResp               *momentpb.ListMomentsResponse
	}{
		{
			desc:       "BehaviorNumericStats",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorNumericStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorNumericStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 37035,
						BehaviorAdherence: 0.07387606054544449,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 2736,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   2596,
							Mean:                    0.38761170990463745,
							ConfidenceIntervalStart: 0.37069976329803467,
							ConfidenceIntervalEnd:   0.40452367067337036,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 34299,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   28178,
							Mean:                    0.013016182773803597,
							ConfidenceIntervalStart: 0.008814854547381401,
							ConfidenceIntervalEnd:   0.01721751131117344,
						},
					},
				},
			},
		},
		{
			desc:       "BehaviorNumericOutcomeStatsExcludesUnscoredSentinel",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorNumericOutcomeStatsExcludesUnscoredSentinel_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorNumericStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
								ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
									OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER,
									OutcomeSubtype:   enumspb.OutcomeSubtype_OUTCOME_SUBTYPE_CSAT,
								},
							},
						},
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 37035,
						BehaviorAdherence: 0.07387606054544449,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 2736,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   2596,
							Mean:                    0.38761170990463745,
							ConfidenceIntervalStart: 0.37069976329803467,
							ConfidenceIntervalEnd:   0.40452367067337036,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 34299,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   28178,
							Mean:                    0.013016182773803597,
							ConfidenceIntervalStart: 0.008814854547381401,
							ConfidenceIntervalEnd:   0.01721751131117344,
						},
					},
				},
			},
		},
		{
			desc:       "BehaviorNumericStatsWithinDeclaredBins",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorNumericStatsWithinDeclaredBins_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorNumericStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
									NumericBins: []*metadata.NumericBin{
										{
											FromValue:              0,
											FromValueIsExclusive:   false,
											ToValue:                50,
											ToValueIsInclusive:     false,
											Description:            "0-50",
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
										{
											FromValue:              50,
											FromValueIsExclusive:   true,
											ToValue:                100,
											ToValueIsInclusive:     true,
											Description:            "50-100",
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
									},
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 37035,
						BehaviorAdherence: 0.07387606054544449,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 2736,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   2596,
							Mean:                    0.38761170990463745,
							ConfidenceIntervalStart: 0.37069976329803467,
							ConfidenceIntervalEnd:   0.40452367067337036,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 34299,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   28178,
							Mean:                    0.013016182773803597,
							ConfidenceIntervalStart: 0.008814854547381401,
							ConfidenceIntervalEnd:   0.01721751131117344,
						},
					},
				},
			},
		},
		{
			desc:       "BehaviorNumericBinaryStats",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorNumericBinaryStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorNumericBinaryStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
									OutcomeStatsTypes: []enumspb.OutcomeStatsTypeEnum{
										enumspb.OutcomeStatsTypeEnum_CONVERSION_RATE,
									},
									NumericBins: []*metadata.NumericBin{
										{
											FromValue:              0,
											FromValueIsExclusive:   false,
											ToValue:                0,
											ToValueIsInclusive:     true,
											Description:            "0",
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
										{
											FromValue:              0,
											FromValueIsExclusive:   true,
											ToValue:                100,
											ToValueIsInclusive:     false,
											Description:            "0-100",
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											FromValue:              100,
											FromValueIsExclusive:   false,
											ToValue:                200,
											ToValueIsInclusive:     false,
											Description:            "100-200",
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
									},
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 4243,
						BehaviorAdherence: 250. / 4243.,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 250,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   88 + 46,
							Mean:                    88. / (88. + 46.),
							ConfidenceIntervalStart: 0.5727083086967468,
							ConfidenceIntervalEnd:   0.731639564037323,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 3993,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   928 + 1325,
							Mean:                    928. / (928. + 1325.),
							ConfidenceIntervalStart: 0.3917448818683624,
							ConfidenceIntervalEnd:   0.43235793709754944,
						},
					},
				},
			},
		},
		{
			desc:       "BehaviorStringStats",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorStringStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorStringStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
									AllowedValues: []*momentpb.ConversationMetadataMomentData_AllowedConversationMetadataValue{
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "PRE_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "DIY_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "NO_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
									},
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 4243,
						BehaviorAdherence: 250. / 4243.,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 250,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   88 + 46,
							Mean:                    88. / (88. + 46.),
							ConfidenceIntervalStart: 0.5727083086967468,
							ConfidenceIntervalEnd:   0.731639564037323,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 3993,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   928 + 1325,
							Mean:                    928. / (928. + 1325.),
							ConfidenceIntervalStart: 0.3917448818683624,
							ConfidenceIntervalEnd:   0.43235793709754944,
						},
					},
				},
			},
		},
		{
			desc:       "BehaviorBoolStats",
			esReqPath:  "RetrieveConversationOutcomeStats_BehaviorBoolStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_BehaviorBoolStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
								ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
									OutcomeTypeEnum:  enumspb.OutcomeTypeEnum_SALE,
									OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_BOOLEAN,
									OutcomeValues: []*momentpb.ConversationOutcomeMomentData_OutcomeValue{
										{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: true,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: false,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
									},
								},
							},
						},
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 4243,
						BehaviorAdherence: 250. / 4243.,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 250,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   88 + 46,
							Mean:                    88. / (88. + 46.),
							ConfidenceIntervalStart: 0.5727083086967468,
							ConfidenceIntervalEnd:   0.731639564037323,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 3993,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   928 + 1325,
							Mean:                    928. / (928. + 1325.),
							ConfidenceIntervalStart: 0.3917448818683624,
							ConfidenceIntervalEnd:   0.43235793709754944,
						},
					},
				},
			},
		},
		{
			desc:       "MetadataBooleanStats",
			esReqPath:  "RetrieveConversationOutcomeStats_MetadataBooleanStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_MetadataBooleanStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN,
									AllowedValues: []*momentpb.ConversationMetadataMomentData_AllowedConversationMetadataValue{
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_BooleanValue{
													BooleanValue: true,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_BooleanValue{
													BooleanValue: false,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
									},
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_SHOULD_DO_X,
						},
						ConversationCount: 4243,
						BehaviorAdherence: 250. / 4243.,
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_DO_X,
						},
						ConversationCount: 250,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   88 + 46,
							Mean:                    88. / (88. + 46.),
							ConfidenceIntervalStart: 0.5727083086967468,
							ConfidenceIntervalEnd:   0.731639564037323,
						},
					},
					{
						Attribute: &analyticspb.Attribute{
							Moments: []*momentpb.Moment{
								{
									Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
								},
							},
							Adherence: enumspb.AdherenceType_ADHERENCE_TYPE_DID_NOT_DO_X,
						},
						ConversationCount: 3993,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count:                   928 + 1325,
							Mean:                    928. / (928. + 1325.),
							ConfidenceIntervalStart: 0.3917448818683624,
							ConfidenceIntervalEnd:   0.43235793709754944,
						},
					},
				},
			},
		},
		{
			desc:       "AgentAdherenceStats",
			esReqPath:  "RetrieveConversationOutcomeStats_AgentAdherenceStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_AgentAdherenceStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				BehaviorMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT_TIER,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AgentTierOutcomeStats: []*analyticspb.AgentTierOutcomeStats{
					{
						AgentTier:         analyticspb.AgentTier_TOP_AGENTS,
						AgentCount:        1,
						ConversationCount: 1660,
						BehaviorAdherence: 181 / 1660.,
						BehaviorMoment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						AgentTier:         analyticspb.AgentTier_AVERAGE_AGENTS,
						AgentCount:        1,
						ConversationCount: 1660,
						BehaviorAdherence: 167 / 1660.,
						BehaviorMoment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						AgentTier:         analyticspb.AgentTier_BOTTOM_AGENTS,
						AgentCount:        1,
						ConversationCount: 1000,
						BehaviorAdherence: 50 / 1000.,
						BehaviorMoment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
				},
			},
		},
		{
			desc:       "GroupNumericStats",
			esReqPath:  "RetrieveConversationOutcomeStats_GroupNumericStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_GroupNumericStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{
					{
						Name:        "customers/customer_id/groups/group1",
						DisplayName: "group 1",
						Type:        userpb.Group_TEAM,
					},
					{
						Name:        "customers/customer_id/groups/group2",
						DisplayName: "group 2",
						Type:        userpb.Group_TEAM,
					},
				},
			},
			listUsersForAnalyticsRequest: []*internaluserpb.ListUsersForAnalyticsRequest{
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group1"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group2"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
			},
			listUsersForAnalyticsResponse: []*internaluserpb.ListUsersForAnalyticsResponse{
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user1",
						},
						{
							UserId: "user2",
						},
					},
				},
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user3",
						},
						{
							UserId: "user4",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group1",
									DisplayName: "group 1",
									Type:        userpb.Group_TEAM,
								},
							},
						},
						ConversationCount: 1800,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count: 1000,
							Mean:  0.14,
						},
						AgentCount: 2,
					},
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group2",
									DisplayName: "group 2",
									Type:        userpb.Group_TEAM,
								},
							},
						},
						ConversationCount: 1000,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count: 0,
							Mean:  0,
						},
						AgentCount: 1,
					},
				},
			},
		},
		{
			desc:       "GroupNumericOutcomeStats",
			esReqPath:  "RetrieveConversationOutcomeStats_GroupNumericOutcomeStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_GroupNumericStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{
					{
						Name:        "customers/customer_id/groups/group1",
						DisplayName: "group 1",
						Type:        userpb.Group_TEAM,
					},
					{
						Name:        "customers/customer_id/groups/group2",
						DisplayName: "group 2",
						Type:        userpb.Group_TEAM,
					},
				},
			},
			listUsersForAnalyticsRequest: []*internaluserpb.ListUsersForAnalyticsRequest{
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group1"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group2"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
			},
			listUsersForAnalyticsResponse: []*internaluserpb.ListUsersForAnalyticsResponse{
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user1",
						},
						{
							UserId: "user2",
						},
					},
				},
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user3",
						},
						{
							UserId: "user4",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
								ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
									OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_NUMBER,
									OutcomeSubtype:   enumspb.OutcomeSubtype_OUTCOME_SUBTYPE_CSAT,
								},
							},
						},
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group1",
									DisplayName: "group 1",
									Type:        userpb.Group_TEAM,
								},
							},
						},
						ConversationCount: 1800,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count: 1000,
							Mean:  0.14,
						},
						AgentCount: 2,
					},
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group2",
									DisplayName: "group 2",
									Type:        userpb.Group_TEAM,
								},
							},
						},
						ConversationCount: 1000,
						OutcomeStats: &analyticspb.OutcomeStats{
							Count: 0,
							Mean:  0,
						},
						AgentCount: 1,
					},
				},
			},
		},
		{
			desc:       "GroupStringStats",
			esReqPath:  "RetrieveConversationOutcomeStats_GroupStringStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_GroupStringStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadata.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
									AllowedValues: []*momentpb.ConversationMetadataMomentData_AllowedConversationMetadataValue{
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "PRE_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "DIY_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &metadata.MetadataValue{
												Value: &metadata.MetadataValue_StringValue{
													StringValue: "NO_SALE",
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
									},
								},
							},
						},
						Type:         momentpb.Moment_CONVERSATION_METADATA,
						DetailedType: momentpb.Moment_DETAILED_TYPE_METADATA_OUTCOME,
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
		},
		{
			desc:       "GroupBoolStats",
			esReqPath:  "RetrieveConversationOutcomeStats_GroupBoolStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_GroupBoolStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				OutcomeMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/outcome",
						Type: momentpb.Moment_CONVERSATION_OUTCOME,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
								ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
									OutcomeTypeEnum:  enumspb.OutcomeTypeEnum_SALE,
									OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_BOOLEAN,
									OutcomeValues: []*momentpb.ConversationOutcomeMomentData_OutcomeValue{
										{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: true,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
										},
										{
											Value: &momentpb.ConversationOutcomeValue{
												Value: &momentpb.ConversationOutcomeValue_BooleanValue{
													BooleanValue: false,
												},
											},
											BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
										},
									},
								},
							},
						},
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
		},
		{
			desc:       "GroupAdherenceStats",
			esReqPath:  "RetrieveConversationOutcomeStats_AgentAdherenceStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_AgentAdherenceStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{
					{
						Name:        "customers/customer_id/groups/group1",
						DisplayName: "group 1",
						Type:        userpb.Group_TEAM,
					},
					{
						Name:        "customers/customer_id/groups/group2",
						DisplayName: "group 2",
						Type:        userpb.Group_TEAM,
					},
				},
			},
			listUsersForAnalyticsRequest: []*internaluserpb.ListUsersForAnalyticsRequest{
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group1"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group2"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
			},
			listUsersForAnalyticsResponse: []*internaluserpb.ListUsersForAnalyticsResponse{
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user1",
						},
						{
							UserId: "user2",
						},
					},
				},
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user3",
						},
						{
							UserId: "user4",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				BehaviorMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group1",
									DisplayName: "group 1",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.10481927543878555, // (181 + 161) / (1660. * 2)
						ConversationCount: 1660 * 2,
						AgentCount:        2,
					},
					{
						Attribute: &analyticspb.Attribute{
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group2",
									DisplayName: "group 2",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.05,
						ConversationCount: 1000,
						AgentCount:        1,
					},
				},
			},
		},
		{
			desc:       "GroupWeeklyAdherenceStats",
			esReqPath:  "RetrieveConversationOutcomeStats_AgentWeeklyAdherenceStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_AgentWeeklyAdherenceStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			listGroupsRequest: &userpb.ListGroupsRequest{
				Parent: "customers/customer_id",
				Filter: &userpb.ListGroupsRequest_ListGroupsFilter{
					CustomerProfiles: []string{"customers/customer_id/profiles/profile_id"},
					GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
				},
				PageSize:                        200,
				IncludeGroupMemberships:         false,
				IncludeIndirectGroupMemberships: false,
			},
			listGroupsResponse: &userpb.ListGroupsResponse{
				Groups: []*userpb.Group{
					{
						Name:        "customers/customer_id/groups/group1",
						DisplayName: "group 1",
						Type:        userpb.Group_TEAM,
					},
					{
						Name:        "customers/customer_id/groups/group2",
						DisplayName: "group 2",
						Type:        userpb.Group_TEAM,
					},
				},
			},
			listUsersForAnalyticsRequest: []*internaluserpb.ListUsersForAnalyticsRequest{
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group1"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
				{
					Parent:                          "customers/customer_id",
					ProfileIds:                      []string{"profile_id"},
					GroupIds:                        []string{"group2"},
					IncludeIndirectGroupMemberships: true,
					IncludeInactiveUsers:            true,
				},
			},
			listUsersForAnalyticsResponse: []*internaluserpb.ListUsersForAnalyticsResponse{
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user1",
						},
						{
							UserId: "user2",
						},
					},
				},
				{
					Users: []*internaluserpb.LiteUser{
						{
							UserId: "user3",
						},
						{
							UserId: "user4",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				Frequency:              analyticspb.Frequency_WEEKLY,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				BehaviorMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654387200,
									},
								},
							},
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group1",
									DisplayName: "group 1",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.1,
						ConversationCount: 300,
						AgentCount:        2,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654387200,
									},
								},
							},
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group2",
									DisplayName: "group 2",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.2,
						ConversationCount: 50,
						AgentCount:        1,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654992000,
									},
								},
							},
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group1",
									DisplayName: "group 1",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.25,
						ConversationCount: 200,
						AgentCount:        2,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654992000,
									},
								},
							},
							Groups: []*userpb.Group{
								{
									Name:        "customers/customer_id/groups/group2",
									DisplayName: "group 2",
									Type:        userpb.Group_TEAM,
								},
							},
							MomentGroups: []*analyticspb.MomentGroup{
								{
									Moments: []*momentpb.Moment{
										{
											Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
										},
									},
								},
							},
						},
						BehaviorAdherence: 0.5,
						ConversationCount: 100,
						AgentCount:        2,
					},
				},
			},
		},
		{
			desc:       "WeeklyAdherenceStats",
			esReqPath:  "RetrieveConversationOutcomeStats_WeeklyAdherenceStats_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_WeeklyAdherenceStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				Frequency:              analyticspb.Frequency_WEEKLY,
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				BehaviorMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
			},
			want: &analyticspb.RetrieveConversationOutcomeStatsResponse{
				AttributeOutcomeStats: []*analyticspb.AttributeOutcomeStats{
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654387200,
									},
								},
							},
						},
						BehaviorAdherence: 0.1,
						ConversationCount: 200,
					},
					{
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: &timestamppb.Timestamp{
										Seconds: 1654992000,
									},
								},
							},
						},
						BehaviorAdherence: 0.2,
						ConversationCount: 100,
					},
				},
			},
		},
		{
			desc:       "AggregateSubcategoryExpansion",
			esReqPath:  "RetrieveConversationOutcomeStats_AggregateSubcategoryExpansion_es_request.json",
			esRespPath: "RetrieveConversationOutcomeStats_WeeklyAdherenceStats_es_response.json",
			getScorecardTemplateRequest: &coachingpb.GetScorecardTemplateRequest{
				Name: "customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1",
			},
			getScorecardTemplateResponse: &coachingpb.GetScorecardTemplateResponse{
				ScorecardTemplate: &coachingpb.ScorecardTemplate{
					Title:    "scorecard template",
					Template: &pbBehaviorTemplateStruct,
				},
			},
			retrieveBehaviorsRequest: &behaviorpb.RetrieveBehaviorsRequest{
				Parent: r.profileResourceName,
			},
			retrieveBehaviorsResponse: &behaviorpb.RetrieveBehaviorsResponse{
				Behaviors: []*behaviorpb.Behavior{
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior1",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
						},
					},
					{
						Name: "customers/customer_id/profiles/profile_id/behaviors/behavior2",
						Moment: &momentpb.Moment{
							Name: "customers/customer_id/profiles/profile_id/moments/behavior2",
						},
					},
				},
			},
			req: &analyticspb.RetrieveConversationOutcomeStatsRequest{
				Parent:                 r.profileResourceName,
				ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
				FilterByTimeRange: &analyticspb.TimeRange{
					StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
					EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
				},
				FilterConversationByAttribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
								{
									Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
									Type: momentpb.Moment_COMPOSITE,
								},
							},
						},
					},
				},
				BehaviorMoments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/behavior1",
					},
				},
				GroupByAttributes: []*analyticspb.AttributeStructure{
					{
						AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
							AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE,
						},
					},
				},
				Frequency: analyticspb.Frequency_WEEKLY,
			},
			batchGetMomentResps: []*momentpb.BatchGetMomentsResponse{
				{
					Moments: []*momentpb.Moment{
						{
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
																},
															},
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-1",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
																},
															},
														},
													},
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
													CompositeMoment: &momentpb.CompositeMomentData{
														Operator: momentpb.CompositeMomentData_NOT,
														Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
															{
																Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
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
							Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
							Type:                   momentpb.Moment_COMPOSITE,
							ParentMomentTemplateId: "aggregate-subcategory-parent-id-2",
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
																	BaseMoment: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
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
				},
				{
					Moments: []*momentpb.Moment{
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-topic-cluster-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_TOPIC,
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/excluded-keyword-keyword-subcategory-1-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "excluded-keyword-keyword-subcategory-1-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-1", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-1",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: fmt.Sprintf("customers/%s/profiles/%s/moments/included-keyword-keyword-subcategory-2-2", r.customerID, r.profileID),
							Type: momentpb.Moment_KEYWORD,
							MomentData: &momentpb.MomentData{
								Moment: &momentpb.MomentData_KeywordMomentData{
									KeywordMomentData: &momentpb.KeywordMomentData{
										KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_OR,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "included-keyword-keyword-subcategory-2-2",
													},
												},
											},
										},
									},
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-1",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
						{
							Name: momentpb.MomentName{
								CustomerID: r.customerID,
								ProfileID:  r.profileID,
								MomentID:   "aggregate-subcategory-parent-id-2",
							}.String(),
							Type: momentpb.Moment_COMPOSITE,
							Labels: &labelspb.Labels{
								Labels: map[string]string{
									"message_filters": "agent",
								},
							},
						},
					},
				},
			},
			listMomentsResp: &momentpb.ListMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
					},
					{
						Name: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						Type: momentpb.Moment_COMPOSITE,
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-1-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-1", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-1", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
					{
						Name:                   fmt.Sprintf("customers/%s/profiles/%s/moments/leaf-subcategory-2-2", r.customerID, r.profileID),
						Type:                   momentpb.Moment_COMPOSITE,
						ParentMomentTemplateId: fmt.Sprintf("customers/%s/profiles/%s/moments/aggregate-subcategory-2", r.customerID, r.profileID),
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_CompositeMomentData{
								CompositeMomentData: &momentpb.CompositeMomentData{},
							},
						},
					},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			r.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)
			r.mockConfigClient.EXPECT().
				Get(gomock.Any()).AnyTimes().
				Return(&config.CustomerConfig{}, nil)
			if tc.getScorecardTemplateRequest != nil && tc.getScorecardTemplateResponse != nil {
				r.mockCoachingClient.EXPECT().GetScorecardTemplate(gomock.Any(), tc.getScorecardTemplateRequest).Times(1).Return(tc.getScorecardTemplateResponse, nil)
			}
			if tc.listGroupsRequest != nil && tc.listGroupsResponse != nil {
				r.mockUserClient.EXPECT().ListGroups(gomock.Any(), tc.listGroupsRequest).Times(1).Return(tc.listGroupsResponse, nil)
			}
			if len(tc.listUsersForAnalyticsRequest) > 0 && len(tc.listUsersForAnalyticsResponse) > 0 {
				for i := range tc.listUsersForAnalyticsRequest {
					r.mockInternalUserClient.EXPECT().ListUsersForAnalytics(gomock.Any(), tc.listUsersForAnalyticsRequest[i]).Times(1).Return(tc.listUsersForAnalyticsResponse[i], nil)
				}
			}
			if tc.retrieveBehaviorsRequest != nil && tc.retrieveBehaviorsResponse != nil {
				r.mockBehaviorClient.EXPECT().RetrieveBehaviors(gomock.Any(), tc.retrieveBehaviorsRequest).Times(1).Return(tc.retrieveBehaviorsResponse, nil)
			}
			if len(tc.batchGetMomentResps) > 0 {
				calls := make([]*gomock.Call, 0, len(tc.batchGetMomentResps)+1)
				if tc.listMomentsResp != nil {
					calls = append(calls, r.mockMomentClient.EXPECT().ListMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.listMomentsResp, nil))
				}
				for _, resp := range tc.batchGetMomentResps {
					calls = append(calls, r.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(resp, nil))
				}
				gomock.InOrder(calls...)
			}
			if tc.esRespPath != "" {
				r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))
			}
			r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			resp, err := r.Client.RetrieveConversationOutcomeStats(r.ctx, tc.req)
			if err != nil {
				t.Fatalf("Got code %v in error: %v", err, status.Code(err))
			}
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					t.Fatalf("RetrieveConversationOutcomeStats return unexpected response with diff (-want +got):\n %v", diff)
				}
			}
		})
	}
}

// TestBehaviorScopedMomentName verifies that the helper produces a syntactically valid
// moment resource name with the behavior UUID substituted for the moment UUID.
func TestBehaviorScopedMomentName(t *testing.T) {
	got := behaviorScopedMomentName("my-customer", "my-profile", "my-behavior-uuid")
	want := "customers/my-customer/profiles/my-profile/moments/my-behavior-uuid"
	if got != want {
		t.Errorf("behaviorScopedMomentName: got %q, want %q", got, want)
	}
}

// TestGetOutcomeNumberValueBounds verifies the bins envelope keeps shared endpoints
// reachable: when several numeric_bins sit at the same from/to value, inclusivity wins
// over exclusivity so the merged range does not drop scores at that endpoint.
func TestGetOutcomeNumberValueBounds(t *testing.T) {
	testCases := []struct {
		desc string
		bins []*metadata.NumericBin
		want numberValueBounds
	}{
		{
			desc: "SharedEndpointsStayReachable",
			bins: []*metadata.NumericBin{
				{FromValue: 0, FromValueIsExclusive: true, ToValue: 10, ToValueIsInclusive: false},
				{FromValue: 0, FromValueIsExclusive: false, ToValue: 10, ToValueIsInclusive: true},
			},
			want: numberValueBounds{from: 0, fromIsExclusive: false, to: 10, toIsInclusive: true},
		},
		{
			desc: "DistinctEndpointsTakeWidestBinFlags",
			bins: []*metadata.NumericBin{
				{FromValue: 0, FromValueIsExclusive: false, ToValue: 50, ToValueIsInclusive: false},
				{FromValue: 50, FromValueIsExclusive: true, ToValue: 100, ToValueIsInclusive: true},
			},
			want: numberValueBounds{from: 0, fromIsExclusive: false, to: 100, toIsInclusive: true},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Given: a metadata outcome moment declaring the bins.
			outcomeMoment := &momentpb.Moment{
				Type: momentpb.Moment_CONVERSATION_METADATA,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_ConversationMetadataMomentData{
						ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
							Type:        metadata.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
							NumericBins: tc.bins,
						},
					},
				},
			}

			// When: the stats bounds are derived.
			got := getOutcomeNumberValueBounds(outcomeMoment)

			// Then: the envelope matches the union of the bins.
			if got == nil {
				t.Fatalf("getOutcomeNumberValueBounds: got nil, want %+v", tc.want)
			}
			if *got != tc.want {
				t.Errorf("getOutcomeNumberValueBounds: got %+v, want %+v", *got, tc.want)
			}
		})
	}
}

// TestGetMomentIDBehaviorMappingIndexesByBehaviorID verifies that getMomentIDBehaviorMapping
// indexes each behavior by BOTH its moment ID (for forward ES queries) and its behavior ID
// (for drill-down requests that echo back the behavior-scoped synthetic moment name).
func TestGetMomentIDBehaviorMappingIndexesByBehaviorID(t *testing.T) {
	moment := &momentpb.Moment{Name: "customers/c/profiles/p/moments/moment-uuid"}
	behavior := &behaviorpb.Behavior{Name: "customers/c/profiles/p/behaviors/behavior-uuid"}

	// Given: one behavior referencing a moment with different IDs.
	// When: getMomentIDBehaviorMapping is called.
	mapping, err := getMomentIDBehaviorMapping(map[*momentpb.Moment]*behaviorpb.Behavior{moment: behavior})
	if err != nil {
		t.Fatal(err)
	}

	// Then: the result should be indexed by both moment ID and behavior ID.
	if got := mapping["moment-uuid"]; got != behavior {
		t.Errorf("expected behavior at moment-uuid key, got %v", got)
	}
	if got := mapping["behavior-uuid"]; got != behavior {
		t.Errorf("expected behavior at behavior-uuid key (drill-down support), got %v", got)
	}
	if len(mapping) != 2 {
		t.Errorf("expected 2 entries in mapping (moment ID + behavior ID), got %d", len(mapping))
	}
}

// TestRetrieveConversationOutcomeStats_MultipleBehaviorsSameMoment verifies that when two
// behaviors in a scorecard template share the same underlying moment, the API returns
// distinct stats entries for each behavior (6 entries = 2 behaviors x 3 adherence types),
// and each entry uses a behavior-scoped moment name unique to that behavior.
func TestRetrieveConversationOutcomeStats_MultipleBehaviorsSameMoment(t *testing.T) {
	r := RetrieveConversationOutcomeStatsTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	// SetupSuite sets the env flag, initializes gRPC mocks, and seeds the scorecard template DB record.
	r.SetupSuite()

	// Given: a scorecard template with two behavior triggers (behaviorA and behaviorB).
	var twoItemTemplateStruct structpb.Struct
	r.Require().NoError(json.Unmarshal([]byte(`{
		"version": 2,
		"items": [
			{
				"displayName": "BehaviorA",
				"type": "labeled-radios",
				"weight": 1,
				"settings": {"options": [{"label": "Yes", "value": 1}, {"label": "No", "value": 0}]},
				"identifier": "A",
				"auto_qa": {
					"triggers": [{"type": "behavior", "resource_name": "customers/customer_id/profiles/profile_id/behaviors/behaviorA"}],
					"detected": 1,
					"not_detected": 0
				}
			},
			{
				"displayName": "BehaviorB",
				"type": "labeled-radios",
				"weight": 1,
				"settings": {"options": [{"label": "Yes", "value": 1}, {"label": "No", "value": 0}]},
				"identifier": "B",
				"auto_qa": {
					"triggers": [{"type": "behavior", "resource_name": "customers/customer_id/profiles/profile_id/behaviors/behaviorB"}],
					"detected": 1,
					"not_detected": 0
				}
			}
		]
	}`), &twoItemTemplateStruct))

	// Both behaviors reference the same underlying moment (shared_moment), which is the
	// scenario that triggered the original deduplication bug.
	retrieveBehaviorsResp := &behaviorpb.RetrieveBehaviorsResponse{
		Behaviors: []*behaviorpb.Behavior{
			{
				Name:        "customers/customer_id/profiles/profile_id/behaviors/behaviorA",
				DisplayName: "Behavior A",
				Moment: &momentpb.Moment{
					Name: "customers/customer_id/profiles/profile_id/moments/shared_moment",
				},
			},
			{
				Name:        "customers/customer_id/profiles/profile_id/behaviors/behaviorB",
				DisplayName: "Behavior B",
				Moment: &momentpb.Moment{
					Name: "customers/customer_id/profiles/profile_id/moments/shared_moment",
				},
			},
		},
	}

	req := &analyticspb.RetrieveConversationOutcomeStatsRequest{
		Parent:                 r.profileResourceName,
		ScorecardTemplateNames: []string{"customers/customer_id/profiles/profile_id/scorecardTemplates/template_id@r1"},
		FilterByTimeRange: &analyticspb.TimeRange{
			StartTimestamp: timestamppb.New(time.Date(2022, 4, 13, 0, 0, 0, 0, time.UTC)),
			EndTimestamp:   timestamppb.New(time.Date(2022, 7, 13, 0, 0, 0, 0, time.UTC)),
		},
		OutcomeMoments: []*momentpb.Moment{
			{
				Name: "customers/customer_id/profiles/profile_id/moments/outcome",
				Type: momentpb.Moment_CONVERSATION_OUTCOME,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
						ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{
							OutcomeTypeEnum:  enumspb.OutcomeTypeEnum_SALE,
							OutcomeValueType: momentpb.ConversationOutcomeMomentData_OUTCOME_VALUE_TYPE_BOOLEAN,
							OutcomeValues: []*momentpb.ConversationOutcomeMomentData_OutcomeValue{
								{
									Value:                  &momentpb.ConversationOutcomeValue{Value: &momentpb.ConversationOutcomeValue_BooleanValue{BooleanValue: true}},
									BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_POSITIVE,
								},
								{
									Value:                  &momentpb.ConversationOutcomeValue{Value: &momentpb.ConversationOutcomeValue_BooleanValue{BooleanValue: false}},
									BinaryOutcomeValueEnum: enumspb.BinaryOutcomeValueEnum_NEGATIVE,
								},
							},
						},
					},
				},
			},
		},
		GroupByAttributes: []*analyticspb.AttributeStructure{
			{AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
				AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT,
			}},
			{AttributeOneof: &analyticspb.AttributeStructure_AttributeType{
				AttributeType: analyticspb.AttributeType_ATTRIBUTE_TYPE_BEHAVIOR_MOMENT_ADHERENCE_TYPE,
			}},
		},
	}

	r.mockConfigClient.EXPECT().GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).Return(&config.ProfileConfig{}, nil)
	r.mockConfigClient.EXPECT().Get(gomock.Any()).AnyTimes().Return(&config.CustomerConfig{}, nil)
	r.mockCoachingClient.EXPECT().GetScorecardTemplate(gomock.Any(), gomock.Any()).Times(1).Return(
		&coachingpb.GetScorecardTemplateResponse{
			ScorecardTemplate: &coachingpb.ScorecardTemplate{
				Title:    "multi-behavior",
				Template: &twoItemTemplateStruct,
			},
		}, nil)
	r.mockBehaviorClient.EXPECT().RetrieveBehaviors(gomock.Any(), gomock.Any()).Times(1).Return(retrieveBehaviorsResp, nil)

	// Each behavior fires one ES query with its own behavior_id filter (even though both
	// behaviors share shared_moment, the per-behavior mapping ensures the correct behavior_id
	// is used for each query). Reuse the existing bool-stats response for both queries.
	// The testify mock matches expectations by argument value regardless of call order,
	// so registering both expectations works correctly even with concurrent goroutines.
	esResp := r.getFile("RetrieveConversationOutcomeStats_BehaviorBoolStats_es_response.json")
	r.mockESServer.Resps = append(r.mockESServer.Resps, esResp, esResp)
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("RetrieveConversationOutcomeStats_MultiBehaviorSameMoment_behaviorA_es_request.json"), cmputil.IgnoreJSONFields("fields"))
	r.mockESServer.EXPECT().Search(r.IndexName, r.getFile("RetrieveConversationOutcomeStats_MultiBehaviorSameMoment_behaviorB_es_request.json"), cmputil.IgnoreJSONFields("fields"))

	// When: the API is called.
	resp, err := r.Client.RetrieveConversationOutcomeStats(r.ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v (code %v)", err, status.Code(err))
	}

	// Then: each behavior produces 3 entries (SDX, DDX, DNDX) = 6 total.
	// Before the fix this returned only 3 entries because both behaviors collapsed to one.
	if got := len(resp.AttributeOutcomeStats); got != 6 {
		t.Errorf("expected 6 stats (3 per behavior), got %d", got)
	}

	// Verify that both behavior-scoped moment names appear in the response.
	// The response uses the behavior UUID as the moment ID to keep entries distinct
	// even when both behaviors share the same underlying moment.
	momentNameSet := make(map[string]bool)
	for _, stat := range resp.AttributeOutcomeStats {
		if len(stat.Attribute.GetMoments()) > 0 {
			momentNameSet[stat.Attribute.Moments[0].Name] = true
		}
	}
	wantMomentNames := []string{
		"customers/customer_id/profiles/profile_id/moments/behaviorA",
		"customers/customer_id/profiles/profile_id/moments/behaviorB",
	}
	for _, name := range wantMomentNames {
		if !momentNameSet[name] {
			t.Errorf("expected moment name %q in response, got names: %v", name, momentNameSet)
		}
	}
}
