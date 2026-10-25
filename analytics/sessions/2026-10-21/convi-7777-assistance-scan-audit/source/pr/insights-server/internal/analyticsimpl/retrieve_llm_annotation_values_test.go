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
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/testing/cmputil"
	envutil "github.com/cresta/shared-go/util/env"
)

type RetrieveLlmAnnotationValuesTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *RetrieveLlmAnnotationValuesTestSuite) SetupSuite() {
	envutil.SetEnv("ENABLE_USAGE_OF_EXPLICIT_MAPPING_MOMENT_ANNOTATION_PAYLOAD", "true")
	s.AnalyticsV2TestSuite.SetupSuite()
}

func llmAnnotationMoment(name string, valueType momentpb.LlmAnnotationMomentData_ValueType) *momentpb.Moment {
	return &momentpb.Moment{
		Name: name,
		Type: momentpb.Moment_LLM_ANNOTATION,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_LlmAnnotationMomentData{
				LlmAnnotationMomentData: &momentpb.LlmAnnotationMomentData{
					ValueType: valueType,
				},
			},
		},
	}
}

func (s *RetrieveLlmAnnotationValuesTestSuite) TestCases() {
	const momentName = "customers/customer_id/profiles/profile_id/moments/1"
	for _, tc := range []struct {
		desc                string
		esReqPath           string
		esRespPath          string
		req                 *analyticspb.RetrieveLlmAnnotationValuesRequest
		want                *analyticspb.RetrieveLlmAnnotationValuesResponse
		batchGetMomentsResp *momentpb.BatchGetMomentsResponse
	}{
		{
			desc: "StringTypeBasic",
			req: &analyticspb.RetrieveLlmAnnotationValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: momentName,
				FilterOnConversation: &analyticspb.RetrieveLlmAnnotationValuesRequest_FilterOnConversation{
					UsecaseNames: []string{"customers/customer_id/profiles/profile_id/usecases/usecase_id_1"},
				},
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{llmAnnotationMoment(momentName, momentpb.LlmAnnotationMomentData_STRING)},
			},
			esReqPath:  "RetrieveLlmAnnotationValues_StringTypeBasic_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringTypeBasic_es_response.json",
			want: &analyticspb.RetrieveLlmAnnotationValuesResponse{
				Values: []*metadatapb.MetadataValue{
					{Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"}},
					{Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"}},
				},
			},
		},
		{
			// Regression test for the wildcard substring filter: the wildcard pre-filter
			// must target the llmAnnotationPayload field, not the conversationMetadataPayload
			// field, otherwise it ANDs out every LLM-annotation bucket and returns nothing.
			desc: "StringType_SubString",
			req: &analyticspb.RetrieveLlmAnnotationValuesRequest{
				Parent:                   s.profileResourceName,
				MomentName:               momentName,
				ValueStringValueContains: "value",
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{llmAnnotationMoment(momentName, momentpb.LlmAnnotationMomentData_STRING)},
			},
			esReqPath:  "RetrieveLlmAnnotationValues_StringType_SubString_es_request.json",
			esRespPath: "RetrieveMetadataValues_StringType_SubString_es_response.json",
			want: &analyticspb.RetrieveLlmAnnotationValuesResponse{
				Values: []*metadatapb.MetadataValue{
					{Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 1"}},
					{Value: &metadatapb.MetadataValue_StringValue{StringValue: "value 2"}},
				},
			},
		},
		{
			desc: "NumberType",
			req: &analyticspb.RetrieveLlmAnnotationValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: momentName,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{llmAnnotationMoment(momentName, momentpb.LlmAnnotationMomentData_NUMBER)},
			},
			esReqPath:  "RetrieveLlmAnnotationValues_NumberType_es_request.json",
			esRespPath: "RetrieveMetadataValues_NumberType_es_response.json",
			want: &analyticspb.RetrieveLlmAnnotationValuesResponse{
				Values: []*metadatapb.MetadataValue{
					{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: 1}},
					{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: 2}},
				},
			},
		},
		{
			desc: "NumberType_MinMaxOnly",
			req: &analyticspb.RetrieveLlmAnnotationValuesRequest{
				Parent:            s.profileResourceName,
				MomentName:        momentName,
				NumericMinMaxOnly: true,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{llmAnnotationMoment(momentName, momentpb.LlmAnnotationMomentData_NUMBER)},
			},
			esReqPath:  "RetrieveLlmAnnotationValues_NumberType_MinMaxOnly_es_request.json",
			esRespPath: "RetrieveMetadataValues_NumberType_MinMaxOnly_es_response.json",
			want: &analyticspb.RetrieveLlmAnnotationValuesResponse{
				Values: []*metadatapb.MetadataValue{
					{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(1)}},
					{Value: &metadatapb.MetadataValue_NumberValue{NumberValue: float32(2)}},
				},
			},
		},
		{
			desc: "BooleanType",
			req: &analyticspb.RetrieveLlmAnnotationValuesRequest{
				Parent:     s.profileResourceName,
				MomentName: momentName,
			},
			batchGetMomentsResp: &momentpb.BatchGetMomentsResponse{
				Moments: []*momentpb.Moment{llmAnnotationMoment(momentName, momentpb.LlmAnnotationMomentData_BOOLEAN)},
			},
			esReqPath:  "RetrieveLlmAnnotationValues_BooleanType_es_request.json",
			esRespPath: "RetrieveMetadataValues_BooleanType_es_response.json",
			want: &analyticspb.RetrieveLlmAnnotationValuesResponse{
				Values: []*metadatapb.MetadataValue{
					{Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: true}},
					{Value: &metadatapb.MetadataValue_BooleanValue{BooleanValue: false}},
				},
			},
		},
	} {
		s.Run(tc.desc, func() {
			fmt.Printf("================================== Test: %s =====================================\n", tc.desc)
			s.mockConfigClient.EXPECT().
				GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
				Return(&config.ProfileConfig{}, nil)
			if tc.batchGetMomentsResp != nil {
				s.mockMomentClient.EXPECT().BatchGetMoments(gomock.Any(), gomock.Any()).Times(1).Return(tc.batchGetMomentsResp, nil)
			}
			if len(tc.esReqPath) > 0 && len(tc.esRespPath) > 0 {
				s.mockESServer.Resps = append(s.mockESServer.Resps, s.getFile(tc.esRespPath))
				s.mockESServer.EXPECT().Search(s.IndexName, s.getFile(tc.esReqPath), cmputil.IgnoreJSONFields("fields"))
			}

			// When
			resp, err := s.Client.RetrieveLlmAnnotationValues(s.ctx, tc.req)

			// Then
			s.Require().NoError(err, "RetrieveLlmAnnotationValues()")
			if tc.want != nil {
				if diff := cmp.Diff(tc.want, resp, protocmp.Transform()); diff != "" {
					s.FailNow("RetrieveLlmAnnotationValues return unexpected response with diff", "(-want +got):\n %v", diff)
				}
			}
		})
	}
}

func TestRetrieveLlmAnnotationValues(t *testing.T) {
	suite.Run(t, &RetrieveLlmAnnotationValuesTestSuite{})
}
