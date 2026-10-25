package analyticsimpl

import (
	"context"
	"fmt"
	"strings"

	labelpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/cresta/go-servers/insights-server/internal/shared"
	"github.com/cresta/go-servers/shared/history"
)

func (a AnalyticsServiceImpl) ModifyInsightsTopic(ctx context.Context, req *analyticspb.ModifyInsightsTopicRequest) (*analyticspb.ModifyInsightsTopicResponse, error) {
	_, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	// TODO add a validation to ensure the all moment names in InsightsTopic are aligned
	// with parent. (for all insights management related APIs.)

	if req.InsightsTopic == nil {
		return nil, status.Error(codes.InvalidArgument, "field insights_topic is not set")
	}

	ctx = history.WithOutgoingUpdateInfo(ctx, nil)

	val, err := a.fetchInsightsTopics(ctx, req.Parent, []string{req.InsightsTopic.TopicMoment.Name}, false)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "fetchInsightsTopics(): %v", err)
	}
	topics := toAPIInsightsTopic(val)
	if len(topics) == 0 || topics[0].TopicMoment.Name != req.InsightsTopic.TopicMoment.Name {
		return nil, status.Errorf(codes.Internal, "fail to fetch topic %q", req.InsightsTopic.TopicMoment.Name)
	}

	/*
	 * Update keyword moment (both included and excluded).
	 */
	updateKeywordMoment := func(old, new *momentpb.Moment) (*momentpb.Moment, error) {
		resultMoment := old
		// nolint:gocritic
		if old != nil && new == nil {
			// Delete moment.
			_, err := a.momentClientRegistry.Client(ctx).BatchDeprecateMoments(ctx, &momentpb.BatchDeprecateMomentsRequest{
				Parent: req.Parent,
				Names:  []string{old.Name},
			})
			if err != nil {
				return nil, fmt.Errorf("deprecating moment {%v}: %w", old, err)
			}
			resultMoment = nil
		} else if old == nil && new != nil {
			// Create moment.
			if err := validateMoment(ctx, new); err != nil {
				return nil, fmt.Errorf("validating keyword moment before creating: {%v}\n: %w", new, err)
			}
			resp, err := a.momentClientRegistry.Client(ctx).CreateMoment(ctx, &momentpb.CreateMomentRequest{
				Parent: req.Parent,
				Moment: new,
			})
			if err != nil {
				return nil, fmt.Errorf("creating keyword moment: %w", err)
			}
			resultMoment = resp.Moment
		} else if old != nil && new != nil {
			// Update moment.
			if diff := cmp.Diff(old, new, protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time", "name")); diff != "" {
				if err := validateMoment(ctx, new); err != nil {
					return nil, fmt.Errorf("validating keyword moment before updating: {%v}\n: %w", new, err)
				}
				new.Name = old.Name
				resp, err := a.momentClientRegistry.Client(ctx).UpdateMoment(ctx, &momentpb.UpdateMomentRequest{
					Moment: new,
				})
				if err != nil {
					return nil, fmt.Errorf("updating keyword moment: %w", err)
				}
				resultMoment = resp.Moment
			}
		}
		return resultMoment, nil
	}

	oldTopic, newTopic := topics[0], req.InsightsTopic
	momentIncluded, err := updateKeywordMoment(oldTopic.IncludedKeywordMoment, newTopic.IncludedKeywordMoment)
	if err != nil {
		return nil, err
	}

	momentExcluded, err := updateKeywordMoment(oldTopic.ExcludedKeywordMoment, newTopic.ExcludedKeywordMoment)
	if err != nil {
		return nil, err
	}

	// Ignore any updates to prerequirement moment.
	prerequirementMoments := oldTopic.PrerequirementMoments

	// Update composite moment.
	momentTopic := newTopic.TopicMoment
	momentTopic.MomentData = constructCompositeMomentData(prerequirementMoments, momentIncluded, momentExcluded, newTopic.ClusterMoments)
	if diff := cmp.Diff(oldTopic.TopicMoment, momentTopic, protocmp.Transform(), protocmp.IgnoreFields(&momentpb.Moment{}, "create_time", "update_time", "name")); diff != "" {
		if err := validateMoment(ctx, momentTopic); err != nil {
			return nil, fmt.Errorf("validating composite moment for topic {%v}\n: %w", momentTopic, err)
		}
		resp, err := a.momentClientRegistry.Client(ctx).UpdateMoment(ctx, &momentpb.UpdateMomentRequest{
			Moment: momentTopic,
		})
		if err != nil {
			return nil, fmt.Errorf("updating composite moment for topic: %w", err)
		}
		momentTopic = resp.Moment
	}

	momentClient := a.momentClientRegistry.Client(ctx)
	errorUnpinning := a.UnpinClusterMomentsFromTopicMoment(ctx, oldTopic.ClusterMoments, newTopic.ClusterMoments, momentTopic.Name, momentClient)
	if errorUnpinning != nil {
		return nil, errorUnpinning
	}

	pinnedClusterMoments, errorPinning := a.PinClusterMomentsToTopicMoment(ctx, newTopic.ClusterMoments, momentTopic.Name, momentClient)
	if errorPinning != nil {
		return nil, errorPinning
	}

	return &analyticspb.ModifyInsightsTopicResponse{
		InsightsTopic: &analyticspb.InsightsTopic{
			TopicMoment:           momentTopic,
			IncludedKeywordMoment: momentIncluded,
			ExcludedKeywordMoment: momentExcluded,
			ClusterMoments:        pinnedClusterMoments,
		},
	}, nil
}

// PinClusterMomentsToTopicMoment updates the 'pinned_to' label of the cluster moment to contain the resource name
// of the topic moment
func (a AnalyticsServiceImpl) PinClusterMomentsToTopicMoment(ctx context.Context, clusterMoments []*momentpb.Moment, pinnedToMomentResourceName string, momentClient momentpb.MomentServiceClient) ([]*momentpb.Moment, error) {
	updatedClusterMoments := make([]*momentpb.Moment, 0)
	for _, clusterMoment := range clusterMoments {
		if clusterMoment.Labels != nil && clusterMoment.Labels.Labels != nil {
			if strings.Contains(clusterMoment.Labels.Labels["pinned_to"], pinnedToMomentResourceName) {
				// Don't update if the cluster is pinned to that moment just make sure that the moment is in return value
				updatedClusterMoments = append(updatedClusterMoments, clusterMoment)
				continue
			}
			pinnedToMomentResourceNames := strings.Split(clusterMoment.Labels.Labels["pinned_to"], ",")
			pinnedToMomentResourceNames = append(pinnedToMomentResourceNames, pinnedToMomentResourceName)
			// Make sure to remove the empty string from the list
			pinnedToMomentResourceNames = Remove(pinnedToMomentResourceNames, "")
			clusterMoment.Labels.Labels["pinned_to"] = strings.Join(pinnedToMomentResourceNames, ",")
		} else if clusterMoment.Labels == nil {
			labels := map[string]string{
				"pinned_to": pinnedToMomentResourceName,
			}
			clusterMoment.Labels = &labelpb.Labels{
				Labels: labels,
			}
		}

		updateMomentResponse, err := momentClient.UpdateMoment(ctx, &momentpb.UpdateMomentRequest{
			Moment: clusterMoment,
		})

		updatedClusterMoments = append(updatedClusterMoments, updateMomentResponse.Moment)

		if err != nil {
			return nil, fmt.Errorf("pinning cluster moment for topic: error: %w", err)
		}
	}
	return updatedClusterMoments, nil
}

// UnpinClusterMomentsFromTopicMoment updates the 'pinned_to' label and removes the resource name of the topic moment
// It unpins all cluster moments from topicMomentResourceName that are inside oldClusterMoments but not inside newClusterMoments
func (a AnalyticsServiceImpl) UnpinClusterMomentsFromTopicMoment(ctx context.Context, oldClusterMoments []*momentpb.Moment, newClusterMoments []*momentpb.Moment, topicMomentResourceName string, momentClient momentpb.MomentServiceClient) error {
	newClusterMomentResourceNames := map[string]bool{}

	for _, newClusterMoment := range newClusterMoments {
		newClusterMomentResourceNames[newClusterMoment.Name] = true
	}

	unpinnedClusterMoments := make([]*momentpb.Moment, 0)
	for _, oldClusterMoment := range oldClusterMoments {
		_, isPresent := newClusterMomentResourceNames[oldClusterMoment.Name]
		if !isPresent {
			unpinnedClusterMoments = append(unpinnedClusterMoments, oldClusterMoment)
		}
	}

	for _, unpinnedClusterMoment := range unpinnedClusterMoments {
		pinnedToMomentResourceNames := strings.Split(unpinnedClusterMoment.Labels.Labels["pinned_to"], ",")
		pinnedToMomentResourceNames = Remove(pinnedToMomentResourceNames, topicMomentResourceName)
		unpinnedClusterMoment.Labels.Labels["pinned_to"] = strings.Join(pinnedToMomentResourceNames, ",")

		_, err := momentClient.UpdateMoment(ctx, &momentpb.UpdateMomentRequest{
			Moment: unpinnedClusterMoment,
		})
		if err != nil {
			return fmt.Errorf("unpinning cluster moment for topic: error: %w", err)
		}
	}
	return nil
}
