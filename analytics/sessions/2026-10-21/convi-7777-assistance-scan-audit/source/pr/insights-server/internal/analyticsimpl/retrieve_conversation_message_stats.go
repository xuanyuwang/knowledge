package analyticsimpl

import (
	"context"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
)

// nolint:gosec
func (a AnalyticsServiceImpl) RetrieveConversationMessageStats(
	ctx context.Context, req *analyticspb.RetrieveConversationMessageStatsRequest,
) (resp *analyticspb.RetrieveConversationMessageStatsResponse, err error) {
	a.logger.Debugf(ctx, "RetrieveConversationMessageStats request:\n%s", req)

	if req.FilterConversationByAttribute == nil {
		req.FilterConversationByAttribute = &analyticspb.Attribute{}
	}
	// As mentioned in proto definition, FilterConversationDocumentByConversationByAttribute should be used like enum,
	// it only has 2 states:nil or empty struct. Some legacy clients still provide convo filter in this field,
	// we merge it back to FilterConversationByAttribute for backward compatibility.
	if req.FilterConversationDocumentByConversationByAttribute != nil {
		proto.Merge(req.FilterConversationByAttribute, req.FilterConversationDocumentByConversationByAttribute)
		req.FilterConversationDocumentByConversationByAttribute = &analyticspb.Attribute{}
	}

	shared.CountFilter(ctx, req.FilterConversationByAttribute)
	shared.CountFilter(ctx, req.FilterMessageByAttribute)

	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	recordConversationReadRecency(ctx, req.GetFilterByTimeRange().GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_conversation_message_stats")

	// Get resource ACL based on the caller identity.
	allFilterByAttributeUsers := a.getAllFilterByAttributeUsers(ctx, parent, req.FilterConversationByAttribute)
	shouldReturnEmptyResponse, err := shared.ApplyResourceACLToAttributeFilter(
		ctx,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		parent.CustomerID,
		parent.ProfileID,
		req.FilterConversationByAttribute,
		true,
		allFilterByAttributeUsers,
		false, // returnEmptyResponseWhenNoUsersAfterMove
	)
	if err != nil {
		return nil, err
	}
	if shouldReturnEmptyResponse {
		a.logger.Debugf(ctx, "[RetrieveConversationMessageStats] return empty response due to ACL filtered users/groups")
		return &analyticspb.RetrieveConversationMessageStatsResponse{}, nil
	}

	noopStore := func(proto.Message, error) {}
	storeCache := noopStore
	var cachedResp analyticspb.RetrieveConversationMessageStatsResponse
	if conversationMessageStatsCachingEligible(parent.ToPBProfileName()) && groupByMomentsAllTopicForCache(req.GroupByAttributes) {
		cacheKey := proto.Clone(req).(*analyticspb.RetrieveConversationMessageStatsRequest)
		sortRetrieveConversationMessageStatsRequestForCache(cacheKey)
		var found bool
		var cacheErr error
		found, storeCache, cacheErr = a.getInsightsStatsFromCache(ctx, "RetrieveConversationMessageStats", insightsConversationMessageStatsCachingCustomerIDs, *insightsConversationMessageStatsCachingAllCustomers, parent.ToPBProfileName(), cacheKey, &cachedResp)
		if found && cacheErr == nil {
			return &cachedResp, nil
		}
	}
	defer func() { storeCache(resp, err) }()

	if req.FilterConversationByAttribute != nil && len(req.FilterConversationByAttribute.LanguageCodes) > 0 {
		if err = elasticsearch.ValidateLanguageCode(req.FilterConversationByAttribute.LanguageCodes); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	if req.FilterMessageByAttribute != nil && len(req.FilterMessageByAttribute.LanguageCodes) > 0 {
		if err = elasticsearch.ValidateLanguageCode(req.FilterMessageByAttribute.LanguageCodes); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	err = shared.UpdateMomentGroupsWithAllBehaviors(req.FilterConversationByAttribute)
	if err != nil {
		return nil, err
	}

	durationBuckets := hardcodedConversationDurationBuckets

	momentNames, hasCompositeMoment := getInsightsCompositeMoments(req.FilterConversationByAttribute, req.FilterMessageByAttribute, req.GroupByAttributes)
	var topicDetails map[string]elasticsearch.TopicDetails
	if hasCompositeMoment {
		aggregateToLeaves, _, err := a.resolveAggregateMoments(ctx, *parent, momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if len(aggregateToLeaves) > 0 {
			expandAggregateMomentsInAttribute(req.FilterConversationByAttribute, aggregateToLeaves)
			expandAggregateMomentsInAttribute(req.FilterMessageByAttribute, aggregateToLeaves)
			momentNames, _ = getInsightsCompositeMoments(req.FilterConversationByAttribute, req.FilterMessageByAttribute, req.GroupByAttributes)
		}
		topicDetails, err = a.fetchTopicDetails(ctx, parent.String(), momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		momentClient := a.momentClientRegistry.Client(ctx)
		if err := a.convertAPIInsightsTopics(ctx, req.FilterMessageByAttribute, topicDetails, *parent, "msg-filter", momentClient); err != nil {
			return nil, err
		}

		if err := a.convertAPIInsightsTopics(ctx, req.FilterConversationByAttribute, topicDetails, *parent, "convo-filter", momentClient); err != nil {
			return nil, err
		}
	}

	// Expand V3 categories in filters (in place) and the group-by; the returned closures are recorded on
	// topicDetails so a grouped-by category's single bucket aggregates its whole subtree.
	v3GroupByClosureTemplateIDs, err := a.applyTopicDiscoveryV3Expansion(
		ctx,
		*parent,
		[]*analyticspb.Attribute{req.FilterConversationByAttribute, req.FilterMessageByAttribute},
		req.GroupByAttributes,
	)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	topicDetails = elasticsearch.MergeV3ClosuresIntoTopicDetails(topicDetails, v3GroupByClosureTemplateIDs)

	intervalSpec, err := elasticsearch.ParseInterval(&req.Frequency, req.CustomBucketInterval)
	if err != nil {
		return nil, err
	}
	if err := intervalSpec.ValidateBucketCount(req.FilterByTimeRange); err != nil {
		return nil, err
	}
	timeZone, err := shared.ParseTimeZone(req.Metadata)
	if err != nil {
		return nil, err
	}
	esQueryConfig, err := a.getElasticsearchQueryConfig(ctx, profileConfig, req.ElasticsearchQueryConfig, req.FilterConversationDocumentByConversationByAttribute)
	if err != nil {
		return nil, err
	}

	query, momentGroup, err := queryESCount(
		req.FilterByTimeRange,
		intervalSpec, timeZone,
		req.Metadata,
		req.FilterConversationByAttribute,
		req.FilterMessageByAttribute,
		req.FilterConversationDocumentByConversationByAttribute,
		req.GroupByAttributes,
		durationBuckets,
		topicDetails,
		a.appsDB.DB(ctx),
		esQueryConfig,
	)
	if err != nil {
		return nil, err
	}

	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, logESQuery(req.ElasticsearchQueryConfig), query, &res, esQueryID(req.ElasticsearchQueryConfig))
	if err != nil {
		return nil, err
	}

	resp = a.convertAggregationToAPIResponse(res.Aggregations, req.GroupByAttributes, momentGroup)
	return resp, nil
}

func queryESCount(
	timeRange *analyticspb.TimeRange,
	intervalSpec elasticsearch.IntervalSpec,
	timeZone string,
	metadata *analyticspb.Metadata,
	filterConversationByAttribute, filterMessageByAttribute, filterConversationDocumentByConversationByAttribute *analyticspb.Attribute,
	groupByAttributeStructures []*analyticspb.AttributeStructure,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	topicDetails map[string]elasticsearch.TopicDetails,
	db *gorm.DB,
	esQueryConfig elasticsearch.QueryConfig,
) (elasticsearch.Request, *analyticspb.MomentGroup, error) {
	filterConversationByAttribute, filterMessageByAttribute = RemoveConvoLevelMomentsFromMessageFilter(filterConversationByAttribute, filterMessageByAttribute)
	// Gets the ES Query phrase.
	query, err := getESQuery(
		timeRange,
		metadata,
		durationBuckets,
		filterConversationByAttribute,
		filterMessageByAttribute,
		filterConversationDocumentByConversationByAttribute,
		groupByAttributeStructures,
		topicDetails,
		db,
		esQueryConfig,
		false /*allowFilteringByConvoFieldsInMsg*/)
	if err != nil {
		return elasticsearch.Request{}, nil, err
	}
	// Gets the ES Aggs phrase.
	startDayOfWeekBias := shared.ParseStartDayOfWeek(metadata, intervalSpec.IsCalendarWeek())
	aggs, momentGroup, err := elasticsearch.ParseGroupByAttributeStructuresToAgg(
		groupByAttributeStructures, topicDetails, timeZone, intervalSpec, startDayOfWeekBias, esQueryConfig)
	if err != nil {
		return elasticsearch.Request{}, momentGroup, err
	}

	request := elasticsearch.Request{Query: query, Aggs: &aggs}
	return request, momentGroup, nil
}

func (a AnalyticsServiceImpl) convertAggregationToAPIResponse(
	aggregations elasticsearch.ResAggregations, attributeStructures []*analyticspb.AttributeStructure,
	momentGroup *analyticspb.MomentGroup,
) *analyticspb.RetrieveConversationMessageStatsResponse {
	conversationMessageStatsList := []*analyticspb.ConversationMessageStats{}

	isConversationMetadata := elasticsearch.IsConversationMetadataMomentGroup(momentGroup)
	isConversationOutcome := elasticsearch.IsConversationOutcomeMomentGroup(momentGroup)
	if len(attributeStructures) == 1 {
		switch attributeStructures[0].AttributeOneof.(type) {
		case *analyticspb.AttributeStructure_AttributeType:
			if attributeType := attributeStructures[0].GetAttributeType(); attributeType == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE {
				for _, bucket := range aggregations.TimeRanges.Buckets {
					conversationMessageStats := analyticspb.ConversationMessageStats{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        int32(bucket.TotalConversationCount.Value),
							TotalConversationMessageCount: int32(bucket.TotalConversationMessageCount.Value),
							AverageHandleTime: &durationpb.Duration{
								Seconds: int64(safeDivideFloat(bucket.TotalConversationHandleTimeSecsSum.Value, bucket.TotalConversationCount.Value)),
							},
						},
						Attribute: &analyticspb.Attribute{
							TimeRanges: []*analyticspb.TimeRange{
								{
									StartTimestamp: shared.ConvertTimeToAPITimestamp(time.Unix(int64(bucket.Key/1000), 0)),
								},
							},
						},
					}
					conversationMessageStatsList = append(conversationMessageStatsList, &conversationMessageStats)
				}
			} else if attributeType == analyticspb.AttributeType_ATTRIBUTE_TYPE_LANGUAGE_CODE {
				for _, bucket := range *aggregations.Moments.Buckets {
					conversationMessageStats := analyticspb.ConversationMessageStats{
						AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
							TotalConversationCount:        int32(bucket.TotalConversationCount.Value),
							TotalConversationMessageCount: int32(bucket.TotalConversationMessageCount.Value),
							AverageHandleTime: &durationpb.Duration{
								Seconds: int64(safeDivideFloat(bucket.TotalConversationHandleTimeSecsSum.Value, bucket.TotalConversationCount.Value)),
							},
						},
						Attribute: &analyticspb.Attribute{
							LanguageCodes: []string{
								bucket.BucketKey,
							},
						},
					}
					conversationMessageStatsList = append(conversationMessageStatsList, &conversationMessageStats)
				}
			}

		default:
			for index, bucket := range *aggregations.Moments.Buckets {
				if bucket.TotalConversationCount.Value == 0 {
					continue
				}
				conversationMessageStats := analyticspb.ConversationMessageStats{
					AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
						TotalConversationCount:        int32(bucket.TotalConversationCount.Value),
						TotalConversationMessageCount: int32(bucket.TotalConversationMessageCount.Value),
						AverageHandleTime: &durationpb.Duration{
							Seconds: int64(safeDivideFloat(bucket.TotalConversationHandleTimeSecsSum.Value, bucket.TotalConversationCount.Value)),
						},
					},
				}
				switch {
				case isConversationMetadata:
					conversationMessageStats.Attribute = &analyticspb.Attribute{
						MomentGroups: []*analyticspb.MomentGroup{
							{
								Moments: momentGroup.Moments,
								MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
									momentGroup.MetadataValueAttributes[index],
								},
							},
						},
					}
				case isConversationOutcome:
					conversationMessageStats.Attribute = &analyticspb.Attribute{
						MomentGroups: []*analyticspb.MomentGroup{
							{
								Moments: momentGroup.Moments,
								OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
									momentGroup.OutcomeValueAttributes[index],
								},
							},
						},
					}
				default:
					conversationMessageStats.Attribute = &analyticspb.Attribute{
						MomentGroups: []*analyticspb.MomentGroup{
							{
								Moments: []*momentpb.Moment{
									momentGroup.Moments[index],
								},
							},
						},
					}
				}
				conversationMessageStatsList = append(conversationMessageStatsList, &conversationMessageStats)
			}
		}
	}
	if len(attributeStructures) == 2 {
		for _, bucket := range aggregations.TimeRanges.Buckets {
			for index, subBucket := range *bucket.Moments.Buckets {
				if int(subBucket.TotalConversationCount.Value) == 0 {
					continue
				}
				conversationMessageStats := analyticspb.ConversationMessageStats{
					AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
						TotalConversationCount:        int32(subBucket.TotalConversationCount.Value),
						TotalConversationMessageCount: int32(subBucket.TotalConversationMessageCount.Value),
						AverageHandleTime: &durationpb.Duration{
							Seconds: int64(safeDivideFloat(bucket.TotalConversationHandleTimeSecsSum.Value, bucket.TotalConversationCount.Value)),
						},
					},
					Attribute: &analyticspb.Attribute{
						TimeRanges: []*analyticspb.TimeRange{
							{
								StartTimestamp: shared.ConvertTimeToAPITimestamp(time.Unix(int64(bucket.Key/1000), 0)),
							},
						},
					},
				}
				switch {
				case isConversationMetadata:
					conversationMessageStats.Attribute.MomentGroups = []*analyticspb.MomentGroup{
						{
							Moments: momentGroup.Moments,
							MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{
								momentGroup.MetadataValueAttributes[index],
							},
						},
					}
				case isConversationOutcome:
					conversationMessageStats.Attribute.MomentGroups = []*analyticspb.MomentGroup{
						{
							Moments: momentGroup.Moments,
							OutcomeValueAttributes: []*analyticspb.OutcomeValueAttribute{
								momentGroup.OutcomeValueAttributes[index],
							},
						},
					}
				case momentGroup != nil && index < len(momentGroup.Moments):
					conversationMessageStats.Attribute.MomentGroups = []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{momentGroup.Moments[index]},
						},
					}
				}
				conversationMessageStatsList = append(conversationMessageStatsList, &conversationMessageStats)
			}
		}
	}
	return &analyticspb.RetrieveConversationMessageStatsResponse{
		AggregatedConversationMessageStats: &analyticspb.AggregatedConversationMessageStats{
			TotalConversationCount:        int32(aggregations.TotalConversationCount.Value),
			TotalConversationMessageCount: int32(aggregations.TotalConversationMessageCount.Value),
			AverageHandleTime: &durationpb.Duration{
				Seconds: int64(safeDivideFloat(aggregations.TotalConversationHandleTimeSecsSum.Value, aggregations.TotalConversationCount.Value)),
			},
		},
		ConversationMessageStats: conversationMessageStatsList,
	}
}
