package analyticsimpl

import (
	"context"
	"fmt"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (a AnalyticsServiceImpl) RetrieveMetadataValues(
	ctx context.Context, req *analyticspb.RetrieveMetadataValuesRequest,
) (resp *analyticspb.RetrieveMetadataValuesResponse, err error) {
	a.logger.Infof(ctx, "RetrieveMetadataValues request:\n%s", req)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveMetadataValuesRequest)
	sortRetrieveMetadataValuesRequestForStatsCache(cacheKey)
	var cachedResp analyticspb.RetrieveMetadataValuesResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveMetadataValues", insightsStatsAPICachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
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
		return nil, fmt.Errorf("BatchGetMoments failed: %w", err)
	}

	moments := batchgetResp.GetMoments()
	if len(moments) != 1 {
		return nil, status.Errorf(codes.Internal, "BatchGetMoments returned %d moments; expected exactly one", len(moments))
	}
	moment := moments[0]
	if moment.GetName() != req.MomentName {
		return nil, status.Errorf(codes.Internal, "BatchGetMoments returned moment %q; expected %q", moment.GetName(), req.MomentName)
	}
	if moment.GetType() != momentpb.Moment_CONVERSATION_METADATA {
		return nil, status.Errorf(codes.InvalidArgument, "type of moment must be Moment_CONVERSATION_METADATA, but found %v", moment.GetType())
	}
	metadataData := moment.GetMomentData().GetConversationMetadataMomentData()
	if metadataData == nil {
		return nil, status.Errorf(codes.Internal, "BatchGetMoments returned metadata moment %q without conversation metadata payload", moment.GetName())
	}

	// Read metadata values from different source.
	if req.Source == analyticspb.RetrieveMetadataValuesRequest_METADATA_VALUES_SOURCE_CLICKHOUSE {
		momentName, err := momentpb.ParseMomentName(moment.Name)
		if err != nil {
			return nil, err
		}
		metadataType := metadataData.Type
		return a.readMetadataValuesFromClickhouse(ctx, parent, req.FilterOnConversation, metadataType, momentName.MomentID, req.MetadataValueStringValueContains, req.NumericMinMaxOnly)
	}

	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	// This ES path (non-ClickHouse) scans the full conversations-v2 index with no conversation
	// time filter -> an all-time read, the deepest lookback case.
	recordConversationReadRecency(ctx, nil, parent.CustomerID, parent.ProfileID, "retrieve_metadata_values")

	var filterUsecaseNames []string
	if req.FilterOnConversation != nil {
		filterUsecaseNames = req.FilterOnConversation.UsecaseNames
	}
	values, err := a.readMomentValuesFromElasticSearch(
		ctx, profileConfig, parent, moment,
		req.MetadataValueStringValueContains, req.NumericMinMaxOnly, filterUsecaseNames,
		"RetrieveMetadataValues",
	)
	if err != nil {
		return nil, err
	}
	return &analyticspb.RetrieveMetadataValuesResponse{MetadataValues: values}, nil
}
