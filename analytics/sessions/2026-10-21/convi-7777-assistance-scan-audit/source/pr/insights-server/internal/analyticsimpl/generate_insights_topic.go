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

// Actions performed by this API: 1) create moment template for excluded keywords 2) create moment template for included keywords
// create moment template for the topic with child keyword moments and clusters.
// nolint: gofmt
func (a AnalyticsServiceImpl) GenerateInsightsTopic(ctx context.Context, req *analyticspb.GenerateInsightsTopicRequest) (*analyticspb.GenerateInsightsTopicResponse, error) {
	if req.InsightsTopic == nil {
		return nil, status.Error(codes.InvalidArgument, "field insights_topic is not set")
	}

	if req.InsightsTopic.TopicMoment == nil {
		return nil, status.Error(codes.InvalidArgument, "field insights_topic.topic_moment is not set")
	}

	ctx = history.WithOutgoingUpdateInfo(ctx, nil)

	if err := validateMoment(ctx, req.InsightsTopic.TopicMoment); err != nil {
		return nil, fmt.Errorf("validating moment for topic {%v}\n: %w", req.InsightsTopic.TopicMoment, err)
	}

	momentClient := a.momentClientRegistry.Client(ctx)

	enablePrerequirementMoments, err := a.enablePrerequirementMoments(req.Parent)
	if err != nil {
		return nil, err
	}
	var prerequirementMoments []*momentpb.Moment
	if enablePrerequirementMoments {
		prerequirementMoments, err = a.fetchPrerequirementMomentsFromLabel(ctx, req.Parent, req.InsightsTopic.TopicMoment, momentClient)
		if err != nil {
			return nil, fmt.Errorf("fetching prerequirement moments: %w", err)
		}
	}

	// TODO: ensure idempotency (ie retriability).
	var (
		momentIncluded, momentExcluded, parentKeywordMomentIncluded, parentKeywordMomentExcluded *momentpb.Moment
		parentClusterMomentsIncluded                                                             []*momentpb.Moment
		momentNamesToDeprecate                                                                   []string
	)
	if req.InheritParentAttributes && req.InsightsTopic.TopicMoment.ParentMomentTemplateId != "" {
		resp, err := momentClient.GetMoment(ctx, &momentpb.GetMomentRequest{
			Name: req.InsightsTopic.TopicMoment.ParentMomentTemplateId,
		})
		if err != nil {
			return nil, fmt.Errorf("getting parent moment: %w", err)
		}
		parentClusterMomentsIncluded, parentKeywordMomentIncluded, parentKeywordMomentExcluded = GetIncludedAndExcludedMomentsFromParentMoment(ctx, resp.GetMoment(), momentClient, a.logger)
	}

	// Insert keyword moment for excluded keywords.
	if ex := req.InsightsTopic.ExcludedKeywordMoment; ex != nil {
		if err := validateMoment(ctx, ex); err != nil {
			return nil, fmt.Errorf("validating moment for excluded keywords {%v}\n: %w", ex, err)
		}

		// Append suffix to taxonomy to make it unique from included keyword moment
		if ex.Taxonomy != "" {
			ex.Taxonomy += ".exclude"
		}

		excludedMomentRequest := mergeKeywordMomentData(ex, parentKeywordMomentExcluded, &momentNamesToDeprecate)

		a.logger.Infof(ctx, "creating moment for excluded keywords %q", ex.Name)
		createResp, err := momentClient.CreateMoment(ctx, &momentpb.CreateMomentRequest{
			Parent: req.Parent,
			Moment: excludedMomentRequest,
		})
		if err != nil {
			return nil, fmt.Errorf("creating moment for excluded keywords: %w", err)
		}
		momentExcluded = createResp.GetMoment()
	}

	// Insert keyword moment for included keywords.
	if in := req.InsightsTopic.IncludedKeywordMoment; in != nil {
		if err := validateMoment(ctx, in); err != nil {
			return nil, fmt.Errorf("validating moment for included keywords {%v}\n: %w", in, err)
		}

		// Append suffix to taxonomy to make it unique from excluded keyword moment
		if in.Taxonomy != "" {
			in.Taxonomy += ".include"
		}

		var includedMoment *momentpb.Moment
		if parentKeywordMomentIncluded != nil && IsParentAndChildMomentDataForInheritanceCompatible(ctx, parentKeywordMomentIncluded, in, a.logger) {
			includedMoment = mergeKeywordMomentData(in, parentKeywordMomentIncluded, &momentNamesToDeprecate)
		} else {
			includedMoment = in
		}
		a.logger.Infof(ctx, "creating moment for included keywords %q", in.Name)
		createResp, err := momentClient.CreateMoment(ctx, &momentpb.CreateMomentRequest{
			Parent: req.Parent,
			Moment: includedMoment,
		})
		if err != nil {
			return nil, fmt.Errorf("creating moment for included keywords: %w", err)
		}
		momentIncluded = createResp.GetMoment()
	}
	if len(momentNamesToDeprecate) > 0 {
		_, err := momentClient.BatchDeprecateMoments(ctx, &momentpb.BatchDeprecateMomentsRequest{
			Parent: req.Parent,
			Names:  momentNamesToDeprecate,
		})
		if err != nil {
			return nil, fmt.Errorf("deprecating moments: %w", err)
		}
	}

	var mergedClusterMoments []*momentpb.Moment
	if parentClusterMomentsIncluded != nil {
		mergedClusterMoments = mergeClusterMoments(parentClusterMomentsIncluded, req.InsightsTopic.ClusterMoments)
		// Unpin parent clusters from the parent composite moment (not from the new topic being created)
		err := a.UnpinClusterMomentsFromTopicMoment(ctx, parentClusterMomentsIncluded, make([]*momentpb.Moment, 0), req.InsightsTopic.TopicMoment.ParentMomentTemplateId, momentClient)
		if err != nil {
			return nil, fmt.Errorf("unpinning parent cluster moments from topic failed: %w", err)
		}
	} else {
		mergedClusterMoments = req.InsightsTopic.ClusterMoments
	}
	createReq := &momentpb.CreateMomentRequest{
		Parent: req.Parent,
		Moment: req.InsightsTopic.TopicMoment,
	}
	createReq.Moment.MomentData = constructCompositeMomentData(prerequirementMoments, momentIncluded, momentExcluded, mergedClusterMoments)
	a.logger.Infof(ctx, "creating moment template for topic %q", req.InsightsTopic.TopicMoment.Name)
	createResp, err := momentClient.CreateMoment(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("creating moment template for topic: %w", err)
	}

	a.logger.Infof(ctx, "pinning cluster moments to topic %q", createResp.Moment.Name)
	pinnedClusterMoments, errorPinning := a.PinClusterMomentsToTopicMoment(ctx, mergedClusterMoments, createResp.Moment.Name, momentClient)
	if errorPinning != nil {
		return nil, errorPinning
	}

	resp := &analyticspb.GenerateInsightsTopicResponse{
		InsightsTopic: &analyticspb.InsightsTopic{
			TopicMoment:           createResp.Moment,
			ClusterMoments:        pinnedClusterMoments,
			IncludedKeywordMoment: momentIncluded,
			ExcludedKeywordMoment: momentExcluded,
			PrerequirementMoments: prerequirementMoments,
		},
	}
	return resp, nil
}

func (a AnalyticsServiceImpl) enablePrerequirementMoments(parent string) (bool, error) {
	profile, err := shared.ParseCustomerProfile(parent)
	if err != nil {
		return false, err
	}
	profileConfig, err := a.configClient.GetProfileConfig(profile.CustomerID, profile.ProfileID)
	if err != nil {
		return false, err
	}
	return profileConfig.GetFeatures() != nil &&
		profileConfig.GetFeatures().GetInsights() != nil &&
		profileConfig.GetFeatures().GetInsights().EnableMomentPrerequirementsUsage, nil
}
