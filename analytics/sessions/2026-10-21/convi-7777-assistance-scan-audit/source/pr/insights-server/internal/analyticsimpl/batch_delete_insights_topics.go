package analyticsimpl

import (
	"context"
	"fmt"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/history"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a AnalyticsServiceImpl) BatchDeleteInsightsTopics(ctx context.Context, req *analyticspb.BatchDeleteInsightsTopicsRequest) (*analyticspb.BatchDeleteInsightsTopicsResponse, error) {
	_, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	if len(req.Names) == 0 && len(req.FolderNames) == 0 {
		return nil, status.Error(codes.InvalidArgument, "both category and folder names are not set")
	}

	ctx = history.WithOutgoingUpdateInfo(ctx, nil)

	val, err := a.fetchInsightsTopics(ctx, req.Parent, req.Names, false)
	if err != nil {
		return nil, err
	}
	topics := toAPIInsightsTopic(val)

	names := []string{}
	for _, t := range topics {
		names = append(names, t.TopicMoment.Name)
		if in := t.IncludedKeywordMoment; in != nil {
			names = append(names, in.Name)
		}
		if ex := t.ExcludedKeywordMoment; ex != nil {
			names = append(names, ex.Name)
		}
	}
	names = append(names, req.FolderNames...)
	momentClient := a.momentClientRegistry.Client(ctx)
	batchDeprecateResp, err := momentClient.BatchDeprecateMoments(ctx, &momentpb.BatchDeprecateMomentsRequest{
		Parent: req.Parent,
		Names:  names,
	})
	if err != nil {
		return nil, fmt.Errorf("BatchDeprecateMoments(): %w", err)
	}

	// Defensive check: ensure all requested moments are deprecated.
	undeprecated := map[string]bool{}
	for _, n := range names {
		undeprecated[n] = true
	}

	momentMap := map[string]*momentpb.Moment{}
	for _, m := range batchDeprecateResp.Moments {
		momentMap[m.Name] = m
		if m.TaxonomyState == momentpb.Moment_STATE_DEPRECATED {
			delete(undeprecated, m.Name)
		}
	}
	if len(undeprecated) > 0 {
		return nil, fmt.Errorf("BatchDeprecateMoments(names=%q) do not deprecate moments %v successfully", names, undeprecated)
	}

	for _, t := range topics {
		t.TopicMoment = momentMap[t.TopicMoment.Name]
		if in := t.IncludedKeywordMoment; in != nil {
			t.IncludedKeywordMoment = momentMap[in.Name]
		}
		if ex := t.ExcludedKeywordMoment; ex != nil {
			t.ExcludedKeywordMoment = momentMap[ex.Name]
		}
		errorUnpinning := a.UnpinClusterMomentsFromTopicMoment(ctx, t.ClusterMoments, make([]*momentpb.Moment, 0), t.TopicMoment.Name, momentClient)
		if errorUnpinning != nil {
			return nil, errorUnpinning
		}
	}

	return &analyticspb.BatchDeleteInsightsTopicsResponse{InsightsTopics: topics}, nil
}
