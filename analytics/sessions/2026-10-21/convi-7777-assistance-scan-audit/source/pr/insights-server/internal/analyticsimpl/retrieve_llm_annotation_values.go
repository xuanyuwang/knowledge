package analyticsimpl

import (
	"context"
	"fmt"
	"sort"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// RetrieveLlmAnnotationValues returns the distinct values (or numeric min/max) of an
// LLM annotation moment, read from Elasticsearch. It mirrors RetrieveMetadataValues
// for conversation metadata moments, reusing the shared ES retrieval core. Unlike
// RetrieveMetadataValues, there is no ClickHouse source for LLM annotations.
func (a AnalyticsServiceImpl) RetrieveLlmAnnotationValues(
	ctx context.Context, req *analyticspb.RetrieveLlmAnnotationValuesRequest,
) (resp *analyticspb.RetrieveLlmAnnotationValuesResponse, err error) {
	a.logger.Infof(ctx, "RetrieveLlmAnnotationValues request:\n%s", req)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveLlmAnnotationValuesRequest)
	if cacheKey.FilterOnConversation != nil {
		sort.Strings(cacheKey.FilterOnConversation.UsecaseNames)
	}
	var cachedResp analyticspb.RetrieveLlmAnnotationValuesResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveLlmAnnotationValues", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	batchGetReq := &momentpb.BatchGetMomentsRequest{
		Parent: parent.String(),
		Names:  []string{req.MomentName},
	}
	batchgetResp, err := a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, batchGetReq)
	if err != nil {
		return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() with request %v failed: %v", batchGetReq, err)
	}

	if len(batchgetResp.Moments) != 1 {
		return nil, fmt.Errorf("expecting one moment to be returned, but found %v", len(batchgetResp.Moments))
	}
	moment := batchgetResp.Moments[0]
	if moment.Name != req.MomentName {
		return nil, fmt.Errorf("moment %v not found in response, instead found %v", req.MomentName, moment.Name)
	}
	if moment.Type != momentpb.Moment_LLM_ANNOTATION {
		return nil, status.Errorf(codes.InvalidArgument, "type of moment must be Moment_LLM_ANNOTATION, but found %v", moment.Type)
	}

	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	// Full conversations-v2 index scan with no conversation time filter -> an all-time read.
	recordConversationReadRecency(ctx, nil, parent.CustomerID, parent.ProfileID, "retrieve_llm_annotation_values")

	var filterUsecaseNames []string
	if req.FilterOnConversation != nil {
		filterUsecaseNames = req.FilterOnConversation.UsecaseNames
	}
	values, err := a.readMomentValuesFromElasticSearch(
		ctx, profileConfig, parent, moment,
		req.ValueStringValueContains, req.NumericMinMaxOnly, filterUsecaseNames,
		"RetrieveLlmAnnotationValues",
	)
	if err != nil {
		return nil, err
	}
	return &analyticspb.RetrieveLlmAnnotationValuesResponse{Values: values}, nil
}
