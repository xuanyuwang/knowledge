// nolint:dupl
package analyticsimpl

import (
	"fmt"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	clickhousetesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
	sharedassert "github.com/cresta/go-servers/shared/testing/assert"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	sharedmock "github.com/cresta/go-servers/shared/testing/mock"
	envutil "github.com/cresta/shared-go/util/env"
)

type RetrieveMetadataValuesTestSuite struct {
	AnalyticsV2TestSuite
	handler *AnalyticsServiceImpl
}

func (s *RetrieveMetadataValuesTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
	s.UseMockedClickhouseConnections()
	s.Require().NoError(s.server.Injector().Invoke(func(handler *AnalyticsServiceImpl) {
		s.handler = handler
	}))
}

// This test is mainly for debugging purpose at the moment.
func (s *RetrieveMetadataValuesTestSuite) TestCases() {
	for _, tc := range []struct {
		desc                 string
		esReqPath            string
		esRespPath           string
		chReqPath            string
		chRespMetadataValues []any
		req                  *analyticspb.RetrieveMetadataValuesRequest
		want                 *analyticspb.RetrieveMetadataValuesResponse
		batchGetMomentsResp  *momentpb.BatchGetMomentsResponse
	}{
		{
			desc: "StringTypeBasic",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				FilterOnConversation: &analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_StringTypeBasic_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringTypeBasic_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "StringType_SubString",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:                           s.profileResourceName,
				MomentName:                       "customers/customer_id/profiles/profile_id/moments/1",
				MetadataValueStringValueContains: "value",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_StringType_SubString_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringType_SubString_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "StringType_EmptySubString",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:                           s.profileResourceName,
				MomentName:                       "customers/customer_id/profiles/profile_id/moments/1",
				MetadataValueStringValueContains: "",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_StringType_EmptySubString_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringType_SubString_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "t 3"},
					},
				},
			},
		},
		{
			desc: "NumberType",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_NumberType_es_request.json",
			esRespPath: "RetrieveMetadataValues_NumberType_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: 1},
					}, {
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: 2},
					},
				},
			},
		},
		{
			desc: "NumberType_SubString",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:                           s.profileResourceName,
				MomentName:                       "customers/customer_id/profiles/profile_id/moments/1",
				MetadataValueStringValueContains: "1",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_NumberType_es_request.json",
			esRespPath: "RetrieveMetadataValues_NumberType_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: 1},
					},
				},
			},
		},
		{
			desc: "BooleanType",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_BooleanType_es_request.json",
			esRespPath: "RetrieveMetadataValues_BooleanType_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: true},
					}, {
						Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: false},
					},
				},
			},
		},
		{
			desc: "ClickHouse_StringType",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				Source:     analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_StringTypeBasic_ch_request.sql",
			chRespMetadataValues: []any{"value 1", "value 2"},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "ClickHouse_StringType_SubString",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:                           s.profileResourceName,
				MomentName:                       "customers/customer_id/profiles/profile_id/moments/1",
				MetadataValueStringValueContains: "2",
				Source:                           analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_StringType_SubString_ch_request.sql",
			chRespMetadataValues: []any{"value 2"},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "ClickHouse_StringType_UsecaseFilter",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				FilterOnConversation: &analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
				Source: analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_StringType_UsecaseFilter_ch_request.sql",
			chRespMetadataValues: []any{"value 1", "value 2"},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "ClickHouse_NumberType",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				Source:     analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_NumberType_ch_request.sql",
			chRespMetadataValues: []any{float64(1), float64(2)},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(1)},
					}, {
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(2)},
					},
				},
			},
		},
		{
			desc: "ClickHouse_NumberType_SubString",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:                           s.profileResourceName,
				MomentName:                       "customers/customer_id/profiles/profile_id/moments/1",
				Source:                           analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
				MetadataValueStringValueContains: "1",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_NumberType_ch_request.sql",
			chRespMetadataValues: []any{float64(1), float64(2)},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(1)},
					},
				},
			},
		},
		{
			desc: "ClickHouse_BooleanType",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				Source:     analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_BooleanType_ch_request.sql",
			chRespMetadataValues: []any{false, true},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: false},
					}, {
						Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: true},
					},
				},
			},
		},
		{
			desc: "ES_NumberType_MinMaxOnly",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:            s.profileResourceName,
				MomentName:        "customers/customer_id/profiles/profile_id/moments/1",
				NumericMinMaxOnly: true,
				Source:            analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_ELASTICSEARCH,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_NumberType_MinMaxOnly_es_request.json",
			esRespPath: "RetrieveMetadataValues_NumberType_MinMaxOnly_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(1)},
					}, {
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(2)},
					},
				},
			},
		},
		{
			desc: "ClickHouse_NumberType_MinMaxOnly",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:            s.profileResourceName,
				MomentName:        "customers/customer_id/profiles/profile_id/moments/1",
				NumericMinMaxOnly: true,
				Source:            analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_NumberType_MinMaxOnly_ch_request.sql",
			chRespMetadataValues: []any{float64(1), float64(5)},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(1)},
					}, {
						Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(5)},
					},
				},
			},
		},
		{
			desc: "ClickHouse_NumberType_MinMaxOnly_NoData",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:            s.profileResourceName,
				MomentName:        "customers/customer_id/profiles/profile_id/moments/1",
				NumericMinMaxOnly: true,
				Source:            analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_NumberType_MinMaxOnly_ch_request.sql",
			chRespMetadataValues: []any{float64(0), float64(0)},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{},
			},
		},
		{
			desc: "ES_StringType_WithNumericMinMaxOnly",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: "customers/customer_id/profiles/profile_id/moments/1",
				FilterOnConversation: &analyticspb.RetrieveMetadataValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
				NumericMinMaxOnly: true,
				Source:            analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_ELASTICSEARCH,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			esReqPath:  "RetrieveMetadataValues_StringTypeBasic_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringTypeBasic_es_response.json",
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
		{
			desc: "ClickHouse_StringType_WithNumericMinMaxOnly",
			req: &analyticspb.RetrieveMetadataValuesRequest{
				Parent:            s.profileResourceName,
				MomentName:        "customers/customer_id/profiles/profile_id/moments/1",
				NumericMinMaxOnly: true,
				Source:            analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{
					{
						Name: "customers/customer_id/profiles/profile_id/moments/1",
						Type: momentpb.Moment_CONVERSATION_METADATA,
						MomentData: &momentpb.MomentData{
							Moment: &momentpb.MomentData_ConversationMetadataMomentData{
								ConversationMetadataMomentData: &momentpb.ConversationMetadataMomentData{
									Type: metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
								},
							},
						},
					},
				},
			},
			chReqPath:            "clickhouse_RetrieveMetadataValues_StringTypeBasic_ch_request.sql",
			chRespMetadataValues: []any{"value 1", "value 2"},
			want: &analyticspb.RetrieveMetadataValuesResponse{
				MetadataValues: []*metadatapb.MetadataValue{
					{
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"},
					}, {
						Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"},
					},
				},
			},
		},
	} {
		s.Run(tc.desc, func() {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			s.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)
			// Mock moment client.
			if tc.batchGetMomentsResp != nil {
				s.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.batchGetMomentsResp, nil)
			}
			// Mock ES server.
			if len(tc.esReqPath) > 0 && len(tc.esRespPath) > 0 {
				s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile(tc.esRespPath))
				s.mockESServer.EXPECT().Search(s.IndexName, s.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			}
			// Mock ClickHouse client.
			defer s.convoMockConn.AssertExpectationsAndReset(s.T())
			if len(tc.chRespMetadataValues) > 0 {
				isNumericType := false
				if tc.batchGetMomentsResp != nil && len(tc.batchGetMomentsResp.Moments) > 0 {
					moment := tc.batchGetMomentsResp.Moments[0]
					if moment.MomentData != nil && moment.MomentData.Moment != nil {
						if metadataData, ok := moment.MomentData.Moment.(*momentpb.MomentData_ConversationMetadataMomentData); ok && metadataData.ConversationMetadataMomentData != nil {
							isNumericType = metadataData.ConversationMetadataMomentData.Type == metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER
						}
					}
				}
				// For min/max query and numeric type, we expect exactly two values
				if tc.req.NumericMinMaxOnly && isNumericType {
					s.Require().Len(tc.chRespMetadataValues, 2, "NumericMinMaxOnly requires exactly 2 values (min and max), got %d", len(tc.chRespMetadataValues))
					mockValues := [][]any{
						{tc.chRespMetadataValues[0], tc.chRespMetadataValues[1]},
					}
					mockRows := clickhousetesting.NewMockRows(mockValues)
					s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)
				} else {
					mockValues := [][]any{}
					for _, v := range tc.chRespMetadataValues {
						mockValues = append(mockValues, []any{v, uint64(1)})
					}
					mockRows := clickhousetesting.NewMockRows(mockValues)
					s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)
				}
			}

			// When
			resp, err := s.Client.RetrieveMetadataValues(s.ctx, tc.req)

			// Then
			s.Require().NoError(err, "RetrieveMetadataValues()")

			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp,
					protocmp.Transform()); diff != "" {
					s.FailNow("RetrieveMetadataValues return unexpected response with diff", "(-want +got):\n %v", diff)
				}
			}
			if len(tc.chReqPath) > 0 {
				s.Require().Len(s.convoMockConn.Calls, 1, "expected to exactly query once")
				gotQuery := s.convoMockConn.Calls[0].Arguments.Get(1).(string)
				gotArgs := s.convoMockConn.Calls[0].Arguments.Get(2).([]any)
				fmt.Printf("got ClickHouse Query: \n%v\n", v2CombineQueryAndArgs(gotQuery, gotArgs))
				expectedCHQuery := s.getFile(tc.chReqPath)
				diff := cmp.Diff(formatChQuery(expectedCHQuery), formatChQuery(v2CombineQueryAndArgs(gotQuery, gotArgs)))
				s.Require().Empty(diff, "RetrieveMetadataValues got unexpected ClickHouse query with diff (-want +got):\n %v", diff)
			}
		})
	}
}

func (testSuite *RetrieveMetadataValuesTestSuite) TestRejectsNonMetadataMoments() {
	for _, testCase := range []struct {
		name       string
		momentType momentpb.Moment_Type
		source     analyticspb.RetrieveMetadataValuesRequest_MetadataValuesSource
	}{
		{
			name:       "BuiltInOutcome_Elasticsearch",
			momentType: momentpb.Moment_CONVERSATION_OUTCOME,
		},
		{
			name:       "BuiltInOutcome_ClickHouse",
			momentType: momentpb.Moment_CONVERSATION_OUTCOME,
			source:     analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
		},
		{
			name:       "Sentiment",
			momentType: momentpb.Moment_SENTIMENT,
		},
		{
			name: "UnspecifiedType",
		},
	} {
		testSuite.Run(testCase.name, func() {
			// Given
			request := &analyticspb.RetrieveMetadataValuesRequest{
				Parent: testSuite.profileResourceName,
				MomentName: momentpb.MomentName{
					CustomerID: FakeCustomerID,
					ProfileID:  FakeProfileID,
					MomentID:   "1",
				}.String(),
				Source: testCase.source,
			}
			testSuite.mockConfigClient.EXPECT().GetProfileConfig(FakeCustomerID, FakeProfileID).
				Return(&config.ProfileConfig{}, nil).Times(1)
			testSuite.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), sharedmock.ProtoEq(&momentpb.BatchGetMomentsRequest{
				Parent: request.Parent,
				Names:  []string{request.MomentName},
			})).Return(&momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{{Name: request.MomentName, Type: testCase.momentType}},
			}, nil).Times(1)

			// When
			response, err := testSuite.Client.RetrieveMetadataValues(testSuite.ctx, request)

			// Then
			testSuite.Require().Error(err)
			testSuite.Equal(codes.InvalidArgument, status.Code(err))
			testSuite.Equal(fmt.Sprintf("type of moment must be Moment_CONVERSATION_METADATA, but found %v", testCase.momentType), status.Convert(err).Message())
			testSuite.Nil(response)
		})
	}
}

func (testSuite *RetrieveMetadataValuesTestSuite) TestPreservesBatchGetMomentsErrors() {
	for _, errorCode := range []codes.Code{codes.InvalidArgument, codes.NotFound, codes.Unavailable, codes.Internal} {
		for _, caller := range []string{"Handler", "GRPC"} {
			testSuite.Run(errorCode.String()+"_"+caller, func() {
				// Given
				request := &analyticspb.RetrieveMetadataValuesRequest{
					Parent: testSuite.profileResourceName,
					MomentName: momentpb.MomentName{
						CustomerID: FakeCustomerID,
						ProfileID:  FakeProfileID,
						MomentID:   "1",
					}.String(),
				}
				detailStatus := status.New(errorCode, "moment service details")
				errorStatus, err := status.New(errorCode, "failed to load moment").WithDetails(detailStatus.Proto())
				testSuite.Require().NoError(err)
				downstreamError := errorStatus.Err()
				expectedStatus := errorStatus.Proto()
				expectedStatus.Message = "BatchGetMoments failed: " + downstreamError.Error()
				if caller == "GRPC" {
					expectedStatus.Message += "\n" + detailStatus.Message()
				}
				requestMetadata, _ := metadata.FromOutgoingContext(testSuite.ctx)
				requestContext := metadata.NewOutgoingContext(testSuite.T().Context(), requestMetadata)
				testSuite.mockConfigClient.EXPECT().GetProfileConfig(FakeCustomerID, FakeProfileID).
					Return(&config.ProfileConfig{}, nil)
				testSuite.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), sharedmock.ProtoEq(&momentpb.BatchGetMomentsRequest{
					Parent: request.Parent,
					Names:  []string{request.MomentName},
				})).Return(nil, downstreamError)

				// When
				var response *analyticspb.RetrieveMetadataValuesResponse
				var retrieveError error
				if caller == "Handler" {
					response, retrieveError = testSuite.handler.RetrieveMetadataValues(requestContext, request)
				} else {
					response, retrieveError = testSuite.Client.RetrieveMetadataValues(requestContext, request)
				}

				// Then
				testSuite.Require().Error(retrieveError)
				if caller == "Handler" {
					testSuite.ErrorIs(retrieveError, downstreamError)
				}
				sharedassert.CmpEquals(testSuite.T(), expectedStatus, status.Convert(retrieveError).Proto())
				testSuite.Nil(response)
			})
		}
	}
}

func (testSuite *RetrieveMetadataValuesTestSuite) TestRejectsUnexpectedMomentResponses() {
	// Given
	momentName := momentpb.MomentName{CustomerID: FakeCustomerID, ProfileID: FakeProfileID, MomentID: "1"}.String()
	otherMomentName := momentpb.MomentName{CustomerID: FakeCustomerID, ProfileID: FakeProfileID, MomentID: "2"}.String()
	missingPayloadMessage := fmt.Sprintf("BatchGetMoments returned metadata moment %q without conversation metadata payload", momentName)
	for _, testCase := range []struct {
		name        string
		moments     []*momentpb.Moment
		source      analyticspb.RetrieveMetadataValuesRequest_MetadataValuesSource
		wantMessage string
	}{
		{
			name:        "NoMoments",
			wantMessage: "BatchGetMoments returned 0 moments; expected exactly one",
		},
		{
			name:        "MultipleMoments",
			moments:     []*momentpb.Moment{{Name: momentName}, {Name: otherMomentName}},
			wantMessage: "BatchGetMoments returned 2 moments; expected exactly one",
		},
		{
			name:        "UnexpectedMomentName",
			moments:     []*momentpb.Moment{{Name: otherMomentName}},
			wantMessage: fmt.Sprintf("BatchGetMoments returned moment %q; expected %q", otherMomentName, momentName),
		},
		{
			name:        "MissingMetadataPayload_Elasticsearch",
			moments:     []*momentpb.Moment{{Name: momentName, Type: momentpb.Moment_CONVERSATION_METADATA}},
			wantMessage: missingPayloadMessage,
		},
		{
			name:        "MissingMetadataPayload_ClickHouse",
			moments:     []*momentpb.Moment{{Name: momentName, Type: momentpb.Moment_CONVERSATION_METADATA}},
			source:      analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE,
			wantMessage: missingPayloadMessage,
		},
		{
			name: "EmptyMetadataPayload",
			moments: []*momentpb.Moment{{
				Name: momentName, Type: momentpb.Moment_CONVERSATION_METADATA, MomentData: &momentpb.MomentData{},
			}},
			wantMessage: missingPayloadMessage,
		},
		{
			name: "WrongMetadataPayload",
			moments: []*momentpb.Moment{{
				Name: momentName, Type: momentpb.Moment_CONVERSATION_METADATA,
				MomentData: &momentpb.MomentData{
					Moment: &momentpb.MomentData_ConversationOutcomeMomentData{
						ConversationOutcomeMomentData: &momentpb.ConversationOutcomeMomentData{},
					},
				},
			}},
			wantMessage: missingPayloadMessage,
		},
	} {
		for _, caller := range []string{"Handler", "GRPC"} {
			testSuite.Run(testCase.name+"_"+caller, func() {
				// Given
				request := &analyticspb.RetrieveMetadataValuesRequest{
					Parent: testSuite.profileResourceName, MomentName: momentName, Source: testCase.source,
				}
				requestMetadata, _ := metadata.FromOutgoingContext(testSuite.ctx)
				requestContext := metadata.NewOutgoingContext(testSuite.T().Context(), requestMetadata)
				testSuite.mockConfigClient.EXPECT().GetProfileConfig(FakeCustomerID, FakeProfileID).
					Return(&config.ProfileConfig{}, nil)
				testSuite.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), sharedmock.ProtoEq(&momentpb.BatchGetMomentsRequest{
					Parent: request.Parent,
					Names:  []string{request.MomentName},
				})).Return(&momentpb.BatchGetMomentsResponse{Moments: testCase.moments}, nil)

				// When
				var response *analyticspb.RetrieveMetadataValuesResponse
				var retrieveError error
				if caller == "Handler" {
					response, retrieveError = testSuite.handler.RetrieveMetadataValues(requestContext, request)
				} else {
					response, retrieveError = testSuite.Client.RetrieveMetadataValues(requestContext, request)
				}

				// Then
				testSuite.Require().Error(retrieveError)
				testSuite.Equal(codes.Internal, status.Code(retrieveError))
				if caller == "Handler" {
					sharedassert.CmpEquals(testSuite.T(), status.New(codes.Internal, testCase.wantMessage).Proto(), status.Convert(retrieveError).Proto())
				}
				testSuite.Nil(response)
			})
		}
	}
}

func TestRetrieveMetadataValues(t *testing.T) {
	suite.Run(t, &RetrieveMetadataValuesTestSuite{})
}
