package analyticsimpl

import (
	"context"
	"math"
	"sort"
	"time"

	trendandanomalypb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/trend_and_anomaly"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/utils"
	"github.com/cresta/shared-go/util/envflag"
)

var (
	enableMessageLevelAnnotatedTimeSeries = envflag.Bool("ENABLE_MESSAGE_LEVEL_ANNOTATED_TIME_SERIES", false, "enable message-level query optimization for RetrieveAnnotatedTimeSeries numerator")
)

const (
	changepointPriorScale float32 = 0.5
	intervalWidth         float32 = 0.99
	maxDataPoints         int     = 1000
	minAvgNumerator       float32 = 4.0
)

// Represents a linear trend line where the x axis is time and measures hours since StartTime.
type TemporalTrendLine struct {
	StartTime time.Time
	TrendLine utils.TrendLine
}

func (a AnalyticsServiceImpl) RetrieveAnnotatedTimeSeries(
	ctx context.Context, req *analyticspb.RetrieveAnnotatedTimeSeriesRequest,
) (resp *analyticspb.RetrieveAnnotatedTimeSeriesResponse, err error) {
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
	// TrainingTimeRange is the earliest (baseline) window this RPC reads back to.
	recordConversationReadRecency(ctx, req.GetTrainingTimeRange().GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_annotated_time_series")

	if req.FilterConversationByAttribute != nil && (len(req.FilterConversationByAttribute.Groups) > 0 || req.FilterConversationByAttribute.ExcludeDeactivatedUsers) {
		filterByAttribute, err := shared.MoveFiltersToUserFilter(
			ctx,
			a.userClientRegistry,
			a.internalUserServiceClientRegistry,
			parent.CustomerID,
			parent.ProfileID,
			req.FilterConversationByAttribute,
			false, // includeDirectGroupMembershipsOnly
			false, // listAgentOnly
		)
		if err != nil {
			return nil, err
		}
		req.FilterConversationByAttribute = filterByAttribute
	}

	if req.QuantityType != analyticspb.QuantityType_QUANTITY_TYPE_ABSOLUTE_CONVERSATION_COUNT &&
		req.QuantityType != analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT {
		return nil, status.Errorf(codes.InvalidArgument, "quantity type %v not yet implemented.", req.QuantityType)
	}

	// Validate group by attributes
	foundTimeRange := false
	var momentGroup *analyticspb.MomentGroup
	for _, groupByAttribute := range req.GroupByAttributes {
		switch groupByAttribute.AttributeOneof.(type) {
		case *analyticspb.AttributeStructure_AttributeType:
			if groupByAttribute.GetAttributeType() == analyticspb.AttributeType_ATTRIBUTE_TYPE_TIME_RANGE {
				foundTimeRange = true
			} else {
				return nil, status.Errorf(codes.InvalidArgument, "unsupported group by attribute type: %v", groupByAttribute.GetAttributeType())
			}
		case *analyticspb.AttributeStructure_MomentGroup:
			momentGroup = groupByAttribute.GetMomentGroup()
		}
	}

	if !foundTimeRange {
		return nil, status.Errorf(codes.InvalidArgument, "group_by_attributes must include ATTRIBUTE_TYPE_TIME_RANGE")
	}

	if momentGroup == nil {
		return nil, status.Errorf(codes.InvalidArgument, "group_by_attributes must include moment group, other aggregations are not yet implemented")
	}

	if len(momentGroup.Moments) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "moment group must include at least one moment")
	}

	// Validate filters
	if req.FilterConversationDocumentByConversationByAttribute != nil {
		return nil, status.Errorf(codes.InvalidArgument, "FilterConversationDocumentByConversationByAttribute is not supported")
	}
	if req.FilterConversationByAttribute != nil && req.FilterConversationByAttribute.SpeakerRoles != nil {
		return nil, status.Errorf(codes.InvalidArgument, "Category SpeakerRoles should be set in FilterMessageByAttribute, not FilterConversationByAttribute")
	}

	cacheKey := proto.Clone(req).(*analyticspb.RetrieveAnnotatedTimeSeriesRequest)
	sortAttributeUsersAndGroupsForStatsCache(cacheKey.FilterConversationByAttribute)
	var cachedResp analyticspb.RetrieveAnnotatedTimeSeriesResponse
	found, storeCache, err := a.getInsightsStatsFromCache(ctx, "RetrieveAnnotatedTimeSeries", insightsAnnotatedTimeSeriesCachingCustomerIDs, false, parent.ToPBProfileName(), cacheKey, &cachedResp)
	if found && err == nil {
		return &cachedResp, nil
	}
	defer func() { storeCache(resp, err) }()

	// Parse time intervals and truncate to extact hours.
	trainingStartTime, trainingEndTime, err := shared.ParseTimeRange(req.TrainingTimeRange, "training_time_range")
	if err != nil {
		return nil, err
	}
	trainingStartTime = trainingStartTime.Truncate(time.Hour)
	trainingEndTime = trainingEndTime.Truncate(time.Hour)
	anomalyStartTime, anomalyEndTime, err := shared.ParseTimeRange(req.AnomalyTimeRange, "anomaly_time_range")
	if err != nil {
		return nil, err
	}
	anomalyStartTime = anomalyStartTime.Truncate(time.Hour)
	anomalyEndTime = anomalyEndTime.Truncate(time.Hour)
	trendStartTime, trendEndTime, err := shared.ParseTimeRange(req.TrendTimeRange, "trend_time_range")
	if err != nil {
		return nil, err
	}
	trendStartTime = trendStartTime.Truncate(time.Hour)
	trendEndTime = trendEndTime.Truncate(time.Hour)

	frequencyDuration := time.Duration(0)
	switch req.Frequency {
	case analyticspb.Frequency_DAILY:
		frequencyDuration, _ = time.ParseDuration("24h")
	case analyticspb.Frequency_WEEKLY:
		frequencyDuration, _ = time.ParseDuration("168h")
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unsupported frequency: %v", req.Frequency)
	}
	frequencyDurationHours := int(frequencyDuration.Hours())

	// Adjust start times, so that the ranges are multiples of frequencyDuration.
	trainingHoursDiff := int(trainingEndTime.Sub(trainingStartTime).Hours()) % frequencyDurationHours
	if trainingHoursDiff != 0 {
		a.logger.Warnf(ctx, "Training time range is not a multiple of frequency, moving trainingStartTime back by %v hours.", frequencyDurationHours-trainingHoursDiff)
		trainingStartTime = trainingStartTime.Add(-time.Hour * time.Duration(frequencyDurationHours-trainingHoursDiff))
	}
	anomalyHoursDiff := int(anomalyEndTime.Sub(anomalyStartTime).Hours()) % frequencyDurationHours
	if anomalyHoursDiff != 0 {
		a.logger.Warnf(ctx, "Anomaly time range is not a multiple of frequency, moving anomalyStartTime back by %v hours.", frequencyDurationHours-anomalyHoursDiff)
		anomalyStartTime = anomalyStartTime.Add(-time.Hour * time.Duration(frequencyDurationHours-anomalyHoursDiff))
	}
	trendHoursDiff := int(trendEndTime.Sub(trendStartTime).Hours()) % frequencyDurationHours
	if trendHoursDiff != 0 {
		a.logger.Warnf(ctx, "Trend time range is not a multiple of frequency, moving trendStartTime back by %v hours.", frequencyDurationHours-trendHoursDiff)
		trendStartTime = trendStartTime.Add(-time.Hour * time.Duration(frequencyDurationHours-trendHoursDiff))
	}

	flexibleTrendMinimumWeeks := req.FlexibleTrendMinimumWeeks
	if flexibleTrendMinimumWeeks < 1 {
		flexibleTrendMinimumWeeks = 1
	}
	if trendEndTime.Sub(trendStartTime).Hours() < float64(7*24*flexibleTrendMinimumWeeks) {
		return nil, status.Errorf(codes.InvalidArgument, "trend time range must be at least %v week(s)", flexibleTrendMinimumWeeks)
	}

	// The time range to process is the shortest contiguous time range containing all three input time ranges.
	startTime := trainingStartTime
	if anomalyStartTime.Before(startTime) {
		startTime = anomalyStartTime
	}
	if trendStartTime.Before(startTime) {
		startTime = trendStartTime
	}
	endTime := trainingEndTime
	if anomalyEndTime.After(endTime) {
		endTime = anomalyEndTime
	}
	if trendEndTime.After(endTime) {
		endTime = trendEndTime
	}

	// Choose a DST-less timezone in such a way that start and end times fall exactly at time bucket boundaries.
	timeZoneID := shared.DefaultTimeZone
	switch endTime.Hour() {
	case 0:
		timeZoneID = shared.DefaultTimeZone
	case 1:
		timeZoneID = "Atlantic/Cape_Verde"
	case 2:
		timeZoneID = "Atlantic/South_Georgia"
	case 3:
		timeZoneID = "Atlantic/Stanley"
	case 4:
		timeZoneID = "America/Tortola"
	case 5:
		timeZoneID = "America/Rio_Branco"
	case 6:
		timeZoneID = "Pacific/Galapagos"
	case 7:
		timeZoneID = "US/Arizona"
	case 8:
		timeZoneID = "Pacific/Pitcairn"
	case 9:
		timeZoneID = "Pacific/Gambier"
	case 10:
		timeZoneID = "US/Hawaii"
	case 11:
		timeZoneID = "Pacific/Tongatapu"
	case 12:
		timeZoneID = "Pacific/Wallis"
	case 13:
		timeZoneID = "Pacific/Pohnpei"
	case 14:
		timeZoneID = "Pacific/Saipan"
	case 15:
		timeZoneID = "Pacific/Palau"
	case 16:
		timeZoneID = "Australia/Perth"
	case 17:
		timeZoneID = "Indian/Christmas"
	case 18:
		timeZoneID = "Indian/Chagos"
	case 19:
		timeZoneID = "Indian/Maldives"
	case 20:
		timeZoneID = "Indian/Reunion"
	case 21:
		timeZoneID = "Indian/Mayotte"
	case 22:
		timeZoneID = "Europe/Kaliningrad"
	case 23:
		timeZoneID = "Africa/Tunis"
	}

	if req.Metadata != nil {
		a.logger.Warnf(ctx, "The metadata field is now deprecated and will be ignored.")
	}
	metadata := &analyticspb.Metadata{
		TimeZoneId: timeZoneID,
	}

	// The changepoint range is the fraction of the time range where changepoints are allowed.
	// Only allow changepoints within or before the training time range.
	changepointRange := 1.0
	if trainingEndTime.Before(endTime) {
		changepointRange = trainingEndTime.Sub(startTime).Hours() / endTime.Sub(startTime).Hours()
	}

	dataPointCount := int(math.Ceil(endTime.Sub(startTime).Hours() / frequencyDuration.Hours()))
	if dataPointCount > maxDataPoints {
		return nil, status.Errorf(codes.InvalidArgument, "too many data points requested: %v (max %v).", dataPointCount, maxDataPoints)
	}

	durationBuckets := hardcodedConversationDurationBuckets

	momentNames, hasCompositeMoment := getInsightsCompositeMoments(
		req.FilterConversationByAttribute,
		req.FilterMessageByAttribute,
		req.GroupByAttributes,
	)
	var topicDetails map[string]elasticsearch.TopicDetails
	if hasCompositeMoment {
		aggregateToLeaves, _, err := a.resolveAggregateMoments(ctx, *parent, momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if len(aggregateToLeaves) > 0 {
			// Expand aggregates in FilterConversationByAttribute
			expandAggregateMomentsInAttribute(req.FilterConversationByAttribute, aggregateToLeaves)
			// Expand aggregates in FilterMessageByAttribute
			expandAggregateMomentsInAttribute(req.FilterMessageByAttribute, aggregateToLeaves)
			// Expand aggregates in GroupByAttributes (the MomentGroup)
			expandAggregateMomentsInMomentGroup(momentGroup, aggregateToLeaves)
			// Get updated moment names after expansion
			momentNames, _ = getInsightsCompositeMoments(
				req.FilterConversationByAttribute,
				req.FilterMessageByAttribute,
				req.GroupByAttributes,
			)
		}
	}
	// Fetch topic details for all composite moments (after expansion if applicable)
	if len(momentNames) > 0 {
		topicDetails, err = a.fetchTopicDetails(ctx, parent.String(), momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Expand V3 categories in filters (in place) and the group-by; the returned closures are recorded on
	// topicDetails so a grouped-by category's single series aggregates its whole subtree.
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

	timeRange := &analyticspb.TimeRange{
		StartTimestamp: timestamppb.New(startTime),
		EndTimestamp:   timestamppb.New(endTime),
	}

	esQueryConfig, err := a.getElasticsearchQueryConfig(ctx, profileConfig, nil, &analyticspb.Attribute{})
	if err != nil {
		return nil, err
	}

	// Get the total counts without any moment filters.
	// If the quantity type is relative they are used as the denominators.
	// In all cases they are used to determine the time buckets with data, so that we can
	// add zero data points.
	denominators := map[int64]int{}

	query, err := queryESTimeMomentBucketedConversationCount(
		timeRange,
		&req.Frequency,
		metadata,
		req.FilterConversationByAttribute,
		req.GroupByAttributes,
		durationBuckets,
		topicDetails,
		a.appsDB.DB(ctx),
		esQueryConfig,
		nil, // No moment filters for denominator query
	)
	if err != nil {
		return nil, err
	}

	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, query, &res, "RetrieveAnnotatedTimeSeries")
	if err != nil {
		return nil, err
	}

	for _, bucket := range res.Aggregations.TimeRanges.Buckets {
		total := bucket.DocCount
		den := 1
		if req.QuantityType == analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT {
			den = total
		}
		if total >= int(req.GetMinTotalCount()) {
			denominators[int64(bucket.Key/1000)] = den
		}
	}

	// Query ES with merged query using filters aggregation.
	moments := momentGroup.Moments
	numerators := make([]map[int64]int, len(moments))
	forecastResponses := make([]*trendandanomalypb.ComputeForecastResponse, len(moments))

	// Clone conversation filter to avoid mutation of originals (prevents Bug #2: shared pointer mutation)
	numeratorConvoFilter := &analyticspb.Attribute{}
	if req.FilterConversationByAttribute != nil {
		numeratorConvoFilter = proto.Clone(req.FilterConversationByAttribute).(*analyticspb.Attribute)
	}

	// Use optimized message-level path when:
	// 0. Feature flag is enabled
	// 1. All conversation filters are denormalized (no has_parent needed)
	// 2. No TOPIC moments (TOPIC requires DocumentTypeMessageGroup which doesn't support denormalized path)
	// 3. No COMPOSITE moments with TOPIC children (clusters/prerequisites need message_group docs
	//    which are excluded by speaker_role filters at message level)
	parsedTimeZone, err := shared.ParseTimeZone(metadata)
	if err != nil {
		return nil, err
	}
	convoDocFilter, err := parseConversationFilterParams(numeratorConvoFilter, nil, timeRange, parsedTimeZone, durationBuckets, nil)
	if err != nil {
		return nil, err
	}
	useMessageLevel := *enableMessageLevelAnnotatedTimeSeries &&
		elasticsearch.ConvoFiltersOnlyContainDenormalizedFields(convoDocFilter) &&
		!momentGroupHasTopicMoment(momentGroup) &&
		!momentGroupHasTopicChildren(momentGroup, topicDetails)

	var mergedQuery elasticsearch.Request
	if useMessageLevel {
		// Optimized: message-level docs, nested moment filters, cardinality count
		numeratorMsgFilter := &analyticspb.Attribute{}
		if req.FilterMessageByAttribute != nil {
			numeratorMsgFilter = proto.Clone(req.FilterMessageByAttribute).(*analyticspb.Attribute)
		}
		mergedQuery, err = queryESTimeMomentBucketedConversationCountMessageLevel(
			timeRange,
			&req.Frequency,
			metadata,
			numeratorConvoFilter,
			numeratorMsgFilter,
			req.GroupByAttributes,
			durationBuckets,
			topicDetails,
			a.appsDB.DB(ctx),
			esQueryConfig,
			momentGroup)
	} else {
		// Fallback: conversation-level with has_child (identical to today)
		if req.FilterMessageByAttribute != nil {
			numeratorConvoFilter.SpeakerRoles = req.FilterMessageByAttribute.SpeakerRoles
		}
		mergedQuery, err = queryESTimeMomentBucketedConversationCount(
			timeRange,
			&req.Frequency,
			metadata,
			numeratorConvoFilter,
			req.GroupByAttributes,
			durationBuckets,
			topicDetails,
			a.appsDB.DB(ctx),
			esQueryConfig,
			momentGroup)
	}
	if err != nil {
		return nil, err
	}

	mergedRes := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, mergedQuery, &mergedRes, "RetrieveAnnotatedTimeSeries")
	if err != nil {
		return nil, err
	}

	// Parse per-moment results from filters aggregation buckets
	for i := range moments {
		momentNumerators := map[int64]int{}
		for _, timeBucket := range mergedRes.Aggregations.TimeRanges.Buckets {
			timestamp := int64(timeBucket.Key / 1000)
			_, denominatorFound := denominators[timestamp]
			if denominatorFound && timeBucket.Moments.Buckets != nil && i < len(*timeBucket.Moments.Buckets) {
				// Read from different field depending on which query path was used
				if useMessageLevel {
					momentNumerators[timestamp] = int((*timeBucket.Moments.Buckets)[i].TotalConversationCount.Value)
				} else {
					momentNumerators[timestamp] = int((*timeBucket.Moments.Buckets)[i].DocCount)
				}
			}
		}

		// Fill in zeros in numerators
		for timestamp, denominator := range denominators {
			if denominator != 0 {
				if _, numeratorFound := momentNumerators[timestamp]; !numeratorFound {
					momentNumerators[timestamp] = 0
				}
			}
		}

		numerators[i] = momentNumerators
	}

	// Compute forecasts concurrently with semaphore to limit parallelism
	sem := make(chan struct{}, 5)
	errorChan := make(chan error, len(moments))
	for i := range moments {
		i := i
		// Compute average numerator for filtering
		numeratorTotal := 0
		numeratorCount := 0
		for timestamp, denominator := range denominators {
			if denominator != 0 {
				numeratorTotal += numerators[i][timestamp]
				numeratorCount++
			}
		}

		if numeratorCount > 1 && float32(numeratorTotal)/float32(numeratorCount) >= minAvgNumerator {
			sem <- struct{}{} // Acquire semaphore
			go utils.AvoidPanic(ctx, func() {
				defer func() { <-sem }() // Release semaphore
				forecastRequest := &trendandanomalypb.ComputeForecastRequest{
					TimeSeries: a.constructTimeSeries(numerators[i], denominators),
					ForecastConfig: &trendandanomalypb.ComputeForecastRequest_ForecastConfig{
						ChangepointPriorScale: changepointPriorScale,
						ChangepointRange:      float32(changepointRange),
						IntervalWidth:         intervalWidth,
					},
				}

				forecastResponse, err := a.trendAndAnomalyClientRegistry.Client(ctx).ComputeForecast(ctx, forecastRequest)
				forecastResponses[i] = forecastResponse
				if err != nil {
					errorChan <- status.Errorf(status.Code(err), "error calling trend-and-anomaly service ComputeForecast: %v", err)
					return
				}
				errorChan <- nil
			})()
		} else {
			errorChan <- nil
		}
	}
	for range moments {
		err := <-errorChan
		if err != nil {
			return nil, err
		}
	}

	response := &analyticspb.RetrieveAnnotatedTimeSeriesResponse{}
	capPredictions := req.QuantityType == analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_CONVERSATION_COUNT ||
		req.QuantityType == analyticspb.QuantityType_QUANTITY_TYPE_RELATIVE_MESSAGE_COUNT
	for index, moment := range moments {
		if forecastResponses[index] != nil && forecastResponses[index].ForecastTimeSeries != nil {
			dataPoints := forecastResponses[index].ForecastTimeSeries.DataPoints
			trendLine, bestTrendStartTime, valueChange := a.computeBestTrendLine(ctx, dataPoints, trendStartTime, trendEndTime, flexibleTrendMinimumWeeks, float64(req.FlexibleTrendMinimumChange))
			response.AnnotatedTimeSeries = append(response.AnnotatedTimeSeries, &analyticspb.AnnotatedTimeSeries{
				Attribute: &analyticspb.Attribute{
					MomentGroups: []*analyticspb.MomentGroup{
						{
							Moments: []*momentpb.Moment{
								moment,
							},
						},
					},
				},
				MaxAnomalyScore: a.computeMaxAnomalyScore(dataPoints, anomalyStartTime, anomalyEndTime),
				SlopeOfTrend:    float32(trendLine.TrendLine.Slope * frequencyDuration.Hours()),
				DataPoints:      a.constructDataPoints(dataPoints, frequencyDuration, numerators[index], denominators, trendLine, capPredictions),
				TrendInfo: &analyticspb.TrendInfo{
					TrendTimeRange: &analyticspb.TimeRange{
						StartTimestamp: timestamppb.New(bestTrendStartTime),
						EndTimestamp:   timestamppb.New(trendEndTime),
					},
					ValueChange: float32(valueChange),
				},
			})
		}
	}

	return response, nil
}

func momentGroupHasTopicMoment(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}
	for _, m := range mg.Moments {
		if m.Type == momentpb.Moment_TOPIC {
			return true
		}
	}
	return false
}

// momentGroupHasTopicChildren checks if any COMPOSITE moment in the group has TOPIC children
// (i.e., clusters or prerequisite moments in topicDetails). These require message_group documents
// which are incompatible with the message-level optimization when speaker_role filters are present.
func momentGroupHasTopicChildren(mg *analyticspb.MomentGroup, topicDetails map[string]elasticsearch.TopicDetails) bool {
	if mg == nil {
		return false
	}
	for _, m := range mg.Moments {
		if td, ok := topicDetails[m.Name]; ok {
			if len(td.Clusters) > 0 || len(td.PrerequrementMoments) > 0 {
				return true
			}
		}
	}
	return false
}

func queryESTimeMomentBucketedConversationCount(
	timeRange *analyticspb.TimeRange,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filterConversationByAttribute *analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	topicDetails map[string]elasticsearch.TopicDetails,
	db *gorm.DB,
	esQueryConfig elasticsearch.QueryConfig,
	momentGroup *analyticspb.MomentGroup,
) (elasticsearch.Request, error) {
	timeZone, err := shared.ParseTimeZone(metadata)
	if err != nil {
		return elasticsearch.Request{}, err
	}
	frequencyStr, err := shared.ParseFrequency(frequency)
	if err != nil {
		return elasticsearch.Request{}, err
	}
	// Gets the ES Query phrase.
	query, err := getESQuery(
		timeRange,
		metadata,
		durationBuckets,
		filterConversationByAttribute,
		nil,                      /* filterMessageByAttribute */
		&analyticspb.Attribute{}, /* Non-null filterConversationDocumentByConversationByAttribute forces conversation document type */
		groupByAttributes,
		topicDetails,
		db, esQueryConfig,
		false /*allowFilteringByConvoFieldsInMsg*/)
	if err != nil {
		return elasticsearch.Request{}, err
	}

	// Gets the ES Aggs phrase.
	aggs := elasticsearch.ReqAggs{
		TimeRanges: &elasticsearch.ReqTimeRanges{
			DateHistogram: elasticsearch.ReqDateHistogram{
				Field:            "conversation_started_at",
				TimeZone:         timeZone,
				CalendarInterval: frequencyStr,
			},
		},
	}

	// If momentGroup provided, add filters sub-aggregation for per-moment counts
	if momentGroup != nil && len(momentGroup.Moments) > 0 {
		momentFilters, err := elasticsearch.ConvertMomentGroupToFilter(momentGroup, topicDetails, esQueryConfig)
		if err != nil {
			return elasticsearch.Request{}, err
		}
		aggs.TimeRanges.Aggs = elasticsearch.ReqNestedAggCounts{
			Moments: &elasticsearch.ReqAnnotations{
				Filters: momentFilters,
			},
		}
	}

	request := elasticsearch.Request{Query: query, Aggs: &aggs}
	return request, nil
}

func queryESTimeMomentBucketedConversationCountMessageLevel(
	timeRange *analyticspb.TimeRange,
	frequency *analyticspb.Frequency,
	metadata *analyticspb.Metadata,
	filterConversationByAttribute *analyticspb.Attribute,
	filterMessageByAttribute *analyticspb.Attribute,
	groupByAttributes []*analyticspb.AttributeStructure,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	topicDetails map[string]elasticsearch.TopicDetails,
	db *gorm.DB,
	esQueryConfig elasticsearch.QueryConfig,
	momentGroup *analyticspb.MomentGroup,
) (elasticsearch.Request, error) {
	timeZone, err := shared.ParseTimeZone(metadata)
	if err != nil {
		return elasticsearch.Request{}, err
	}
	frequencyStr, err := shared.ParseFrequency(frequency)
	if err != nil {
		return elasticsearch.Request{}, err
	}
	// Gets the ES Query phrase for message-level documents.
	query, err := getESQuery(
		timeRange,
		metadata,
		durationBuckets,
		filterConversationByAttribute,
		filterMessageByAttribute,
		nil, /* filterConversationDocumentByConversationByAttribute = nil -> message doc type */
		groupByAttributes,
		topicDetails,
		db, esQueryConfig,
		true /* allowFilteringByConvoFieldsInMsg = true -> move denormalized convo filters to message level */)
	if err != nil {
		return elasticsearch.Request{}, err
	}

	// Gets the ES Aggs phrase.
	aggs := elasticsearch.ReqAggs{
		TimeRanges: &elasticsearch.ReqTimeRanges{
			DateHistogram: elasticsearch.ReqDateHistogram{
				Field:            "conversation_started_at",
				TimeZone:         timeZone,
				CalendarInterval: frequencyStr,
			},
		},
	}

	// If momentGroup provided, add filters sub-aggregation with cardinality count for per-moment distinct conversations
	if momentGroup != nil && len(momentGroup.Moments) > 0 {
		// For message-level queries, update EsDocumentType to DocumentTypeMessage so ConvertMomentGroupToFilter
		// uses nested queries instead of has_child
		msgLevelConfig := esQueryConfig
		msgLevelConfig.EsDocumentType = elasticsearch.DocumentTypeMessage
		momentFilters, err := elasticsearch.ConvertMomentGroupToFilter(momentGroup, topicDetails, msgLevelConfig)
		if err != nil {
			return elasticsearch.Request{}, err
		}
		aggs.TimeRanges.Aggs = elasticsearch.ReqNestedAggCounts{
			Moments: &elasticsearch.ReqAnnotations{
				Filters: momentFilters,
				Aggs: &elasticsearch.ReqAggAnnotation{
					TotalConversationCount: elasticsearch.CreateReqTotalConversationCount(
						elasticsearch.MaxCardinalityPrecisionThreshold),
				},
			},
		}
	}

	request := elasticsearch.Request{Query: query, Aggs: &aggs}
	return request, nil
}

// Converts a maps of timestamps to numerators and denominators into a trendandanomalypb.TimeSeries.
func (a AnalyticsServiceImpl) constructTimeSeries(numeratorsByTimestamp map[int64]int, denominatorsByTimestamp map[int64]int) *trendandanomalypb.TimeSeries {
	dataPoints := make([]*trendandanomalypb.TimeSeriesDataPoint, 0, len(numeratorsByTimestamp))
	for timestamp, numerator := range numeratorsByTimestamp {
		value := float64(numerator)
		if value != 0 {
			denominator, denominatorFound := denominatorsByTimestamp[timestamp]
			if denominatorFound && denominator != 0 {
				value /= float64(denominator)
			}
		}
		dataPoints = append(dataPoints, &trendandanomalypb.TimeSeriesDataPoint{
			Time:  shared.ConvertTimeToAPITimestamp(time.Unix(timestamp, 0)),
			Value: structpb.NewNumberValue(value),
		})
	}
	sort.Slice(dataPoints, func(i, j int) bool { return dataPoints[i].Time.AsTime().Before(dataPoints[j].Time.AsTime()) })

	return &trendandanomalypb.TimeSeries{
		DataPoints: dataPoints,
	}
}

// Anomaly score is 0.0 if the value falls within the confidence interval.
// Otherwise it is the distance to the interval as a fraction of the interval width.
func (a AnalyticsServiceImpl) computeAnomalyScore(forecastDataPoint *trendandanomalypb.TimeSeriesDataPoint) float32 {
	lower := forecastDataPoint.ConfidenceIntervalLower.GetNumberValue()
	upper := forecastDataPoint.ConfidenceIntervalUpper.GetNumberValue()
	switch value := forecastDataPoint.Value.GetNumberValue(); {
	case value > upper:
		return float32((value - upper) / (upper - lower))
	case value < lower:
		return float32((lower - value) / (upper - lower))
	default:
		return 0.0
	}
}

// Returns the maximum anomaly score within the anomaly time range.
func (a AnalyticsServiceImpl) computeMaxAnomalyScore(
	forecastDataPoints []*trendandanomalypb.TimeSeriesDataPoint, anomalyStartTime time.Time, anomalyEndTime time.Time,
) float32 {
	maxAnomalyScore := 0.0
	for _, forecastDataPoint := range forecastDataPoints {
		if forecastDataPoint.Time.AsTime().Before(anomalyStartTime) || !forecastDataPoint.Time.AsTime().Before(anomalyEndTime) {
			continue
		}
		maxAnomalyScore = math.Max(float64(a.computeAnomalyScore(forecastDataPoint)), maxAnomalyScore)
	}
	return float32(maxAnomalyScore)
}

// Computes the best trend line within the trend time range subject to constraints.
// Returns the trend line, the trend start time and the change in value during the trend time period.
func (a AnalyticsServiceImpl) computeBestTrendLine(
	ctx context.Context, forecastDataPoints []*trendandanomalypb.TimeSeriesDataPoint, trendStartTime time.Time, trendEndTime time.Time, flexibleTrendMinimumWeeks int32, flexibleTrendMinimumChange float64,
) (TemporalTrendLine, time.Time, float64) {
	week := time.Duration(24*7) * time.Hour
	bestTrendStartTime := trendEndTime.Add(-week * time.Duration(flexibleTrendMinimumWeeks))
	bestTrendLine, bestValueChange := a.computeTrendLine(ctx, forecastDataPoints, bestTrendStartTime, trendEndTime)
	if bestValueChange == 0.0 || math.Abs(bestValueChange) < flexibleTrendMinimumChange*float64(flexibleTrendMinimumWeeks) {
		// Do not extend the trend if the shortest time period already doesn't meet the minimum change requirement
		return bestTrendLine, bestTrendStartTime, bestValueChange
	}
	for startTime := bestTrendStartTime.Add(-week); !startTime.Before(trendStartTime); startTime = startTime.Add(-week) {
		trendLine, valueChange := a.computeTrendLine(ctx, forecastDataPoints, startTime, trendEndTime)
		if math.Abs(valueChange) < math.Abs(bestValueChange)+flexibleTrendMinimumChange || valueChange*bestValueChange < 0.0 {
			break
		}
		bestTrendStartTime = startTime
		bestTrendLine = trendLine
		bestValueChange = valueChange
	}
	return bestTrendLine, bestTrendStartTime, bestValueChange
}

// Computes a linear trend line within the trend time range.
// Returns the trend line and the change in value during the trend time period.
func (a AnalyticsServiceImpl) computeTrendLine(
	ctx context.Context, forecastDataPoints []*trendandanomalypb.TimeSeriesDataPoint, trendStartTime time.Time, trendEndTime time.Time,
) (TemporalTrendLine, float64) {
	emptyTrendLine := TemporalTrendLine{
		StartTime: time.Unix(0, 0),
		TrendLine: utils.TrendLine{
			Slope:     0.0,
			Intercept: 0.0,
		},
	}
	if len(forecastDataPoints) == 0 {
		return emptyTrendLine, 0.0
	}
	startIndex := sort.Search(len(forecastDataPoints), func(i int) bool { return !forecastDataPoints[i].Time.AsTime().Before(trendStartTime) })
	endIndex := sort.Search(len(forecastDataPoints), func(i int) bool { return !forecastDataPoints[i].Time.AsTime().Before(trendEndTime) })
	if endIndex-startIndex < 2 {
		// Need at least two data points for computing trend.
		a.logger.Warnf(ctx, "Attempting to compute trend in a time range with fewer than 2 data points")
		return emptyTrendLine, 0.0
	}
	startTime := forecastDataPoints[startIndex].Time.AsTime()
	points := make([]utils.Point[float64], endIndex-startIndex)
	for index, forecastDataPoint := range forecastDataPoints[startIndex:endIndex] {
		points[index] = utils.Point[float64]{
			X: forecastDataPoint.Time.AsTime().Sub(startTime).Hours(),
			Y: forecastDataPoint.Value.GetNumberValue(),
		}
	}
	trendLine, err := utils.ComputeTrendLine(points)
	if err != nil {
		// Shouldn't happen as long as time buckets have different timestamps.
		a.logger.Warnf(ctx, "Unexpected lack of spacing between data points in the time series")
		return emptyTrendLine, 0.0
	}
	return TemporalTrendLine{
		StartTime: startTime,
		TrendLine: *trendLine,
	}, trendLine.Slope * trendEndTime.Sub(trendStartTime).Hours()
}

// Converts trendandanomalypb.TimeSeriesDataPoints to analyticspb.AnnotatedTimeSeriesDataPoints.
func (a AnalyticsServiceImpl) constructDataPoints(
	forecastDataPoints []*trendandanomalypb.TimeSeriesDataPoint, frequencyDuration time.Duration, numeratorsByTimestamp map[int64]int, denominatorsByTimestamp map[int64]int, trendLine TemporalTrendLine, capPredictions bool,
) []*analyticspb.AnnotatedTimeSeriesDataPoint {
	dataPoints := make([]*analyticspb.AnnotatedTimeSeriesDataPoint, len(forecastDataPoints))
	for index, forecastDataPoint := range forecastDataPoints {
		timestamp := forecastDataPoint.Time.AsTime().Unix()
		numerator, numeratorFound := numeratorsByTimestamp[timestamp]
		if !numeratorFound {
			numerator = int(forecastDataPoint.Value.GetNumberValue())
		}
		denominator, denominatorFound := denominatorsByTimestamp[timestamp]
		if !denominatorFound {
			denominator = 1
		}
		predictedValue := forecastDataPoint.ForecastedValue.GetNumberValue()
		predictedRangeLower := forecastDataPoint.ConfidenceIntervalLower.GetNumberValue()
		predictedRangeUpper := forecastDataPoint.ConfidenceIntervalUpper.GetNumberValue()
		if capPredictions {
			predictedValue = math.Min(1.0, math.Max(0.0, predictedValue))
			predictedRangeLower = math.Min(1.0, math.Max(0.0, predictedRangeLower))
			predictedRangeUpper = math.Min(1.0, math.Max(0.0, predictedRangeUpper))
		}
		dataPoints[index] = &analyticspb.AnnotatedTimeSeriesDataPoint{
			TimeRange: &analyticspb.TimeRange{
				StartTimestamp: forecastDataPoint.Time,
				EndTimestamp:   timestamppb.New(forecastDataPoint.Time.AsTime().Add(frequencyDuration)),
			},
			Value:               float32(forecastDataPoint.Value.GetNumberValue()),
			ValueNumerator:      int32(numerator),
			ValueDenominator:    int32(denominator),
			PredictedValue:      float32(predictedValue),
			PredictedRangeLower: float32(predictedRangeLower),
			PredictedRangeUpper: float32(predictedRangeUpper),
			AnomalyScore:        a.computeAnomalyScore(forecastDataPoint),
			TrendLineValue:      float32(forecastDataPoint.Time.AsTime().Sub(trendLine.StartTime).Hours()*trendLine.TrendLine.Slope + trendLine.TrendLine.Intercept),
		}
	}
	return dataPoints
}
