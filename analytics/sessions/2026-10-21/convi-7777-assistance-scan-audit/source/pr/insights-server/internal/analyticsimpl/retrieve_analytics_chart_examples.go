package analyticsimpl

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	dashboardpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/dashboarding"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
)

func (a AnalyticsServiceImpl) RetrieveAnalyticsChartExamples(ctx context.Context, req *analyticspb.RetrieveAnalyticsChartExamplesRequest) (*analyticspb.RetrieveAnalyticsChartExamplesResponse, error) {
	if req == nil || req.AnalyticsChart == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty request for RetrieveAnalyticsChartExamples not allowed")
	}
	if req.AnalyticsChart.DataConfig == nil {
		return nil, status.Errorf(codes.InvalidArgument, "empty chart_data_config not allowed")
	}
	queryEngineType, err := determineQueryEngine(req.AnalyticsChart)
	if err != nil {
		return nil, err
	}
	if queryEngineType != QueryEngineES {
		return nil, status.Errorf(codes.Internal, "only ES query engine is supported, but found %v", queryEngineType)
	}

	// Parse the ES filter.
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}
	// Reading feature config only once.
	profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	chartDataConfig := req.AnalyticsChart.DataConfig

	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	// Reuse the chart time-range resolver; a nil/absent range means an all-time (full-index) read.
	if exampleTimeRange, timeRangeErr := a.getChartTimeRangeFilter(ctx, chartDataConfig, req.Metadata); timeRangeErr == nil && exampleTimeRange != nil {
		recordConversationReadRecency(ctx, exampleTimeRange.from, parent.CustomerID, parent.ProfileID, "retrieve_analytics_chart_examples")
	} else {
		recordConversationReadRecency(ctx, nil, parent.CustomerID, parent.ProfileID, "retrieve_analytics_chart_examples")
	}

	if len(req.GroupByValuesForAllKeys) > 0 && len(req.GroupByValues) > 0 {
		return nil, status.Errorf(codes.InvalidArgument, "only one of group_by_values_for_all_keys or group_by_values should be set")
	}

	addIntervalToTime := func(t time.Time, interval dashboardpb.ChartGroupBy_Time_Interval) (time.Time, error) {
		switch interval {
		case dashboardpb.ChartGroupBy_Time_DAY:
			return t.AddDate(0, 0, 1), nil
		case dashboardpb.ChartGroupBy_Time_WEEK:
			return t.AddDate(0, 0, 7), nil
		case dashboardpb.ChartGroupBy_Time_MONTH:
			return t.AddDate(0, 1, 0), nil
		case dashboardpb.ChartGroupBy_Time_HOUR:
			return t.Add(time.Hour), nil
		default:
			return t, fmt.Errorf("unknown time interval: %v", interval)
		}
	}

	filtersFromGroupByValue := []*dashboardpb.ChartDataFilter{}

	// convert group_by_values_for_all_keys to filters
	for _, groupbyValuesPerKey := range req.GroupByValuesForAllKeys {
		attributeValues := []*dashboardpb.ChartAttributeValue{}
		for _, groupByValue := range groupbyValuesPerKey.GroupByValues {
			attributeValues = append(attributeValues, groupByValue.Value)
		}
		if len(attributeValues) == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "group_by_values for a single group_by key cannot be empty")
		}

		if !hasSameConcreteType(attributeValues) {
			return nil, status.Errorf(codes.InvalidArgument, "group_by_values for a single group_by key must have the same type")
		}

		if _, ok := attributeValues[0].Value.(*dashboardpb.ChartAttributeValue_Timestamp); ok {
			// Merge timestamps into single time range.
			timestamps := []*timestamppb.Timestamp{}
			for _, attrValue := range attributeValues {
				timestamps = append(timestamps, attrValue.GetTimestamp())
			}
			// Sort timestamps to form continuous time ranges.
			sort.Slice(timestamps, func(i, j int) bool {
				return timestamps[i].AsTime().Before(timestamps[j].AsTime())
			})
			toTime, err := addIntervalToTime(timestamps[len(timestamps)-1].AsTime(), getGetGroupByTimeInterval(req))
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "error adding interval to time: %v", err)
			}
			timeRange := &dashboardpb.ChartValueRange{
				FromValue: &dashboardpb.ChartAttributeValue{
					Value: &dashboardpb.ChartAttributeValue_Timestamp{
						Timestamp: timestamps[0],
					},
				},
				ToValue: &dashboardpb.ChartAttributeValue{
					Value: &dashboardpb.ChartAttributeValue_Timestamp{
						Timestamp: timestamppb.New(toTime),
					},
				},
			}

			filtersFromGroupByValue = append(filtersFromGroupByValue, &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_RangeValues{
						RangeValues: &dashboardpb.ChartValueSet_Range{
							RangeValues: []*dashboardpb.ChartValueRange{
								timeRange,
							},
						},
					},
				},
			})
		} else {
			filtersFromGroupByValue = append(filtersFromGroupByValue, &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: attributeValues,
						},
					},
				},
			})
		}
	}

	// convert group_by_values to filters
	for _, groupByValue := range req.GroupByValues {
		switch chartAttributeValue := groupByValue.Value; chartAttributeValue.Value.(type) {
		case *dashboardpb.ChartAttributeValue_Timestamp:
			startTime := chartAttributeValue.GetTimestamp().AsTime()
			endTime, err := addIntervalToTime(startTime, getGetGroupByTimeInterval(req))
			if err != nil {
				return nil, status.Errorf(codes.InvalidArgument, "error adding interval to time: %v", err)
			}
			timeRange := &dashboardpb.ChartValueRange{
				FromValue: chartAttributeValue,
				ToValue: &dashboardpb.ChartAttributeValue{
					Value: &dashboardpb.ChartAttributeValue_Timestamp{
						Timestamp: timestamppb.New(endTime),
					},
				},
			}
			filtersFromGroupByValue = append(filtersFromGroupByValue, &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_RangeValues{
						RangeValues: &dashboardpb.ChartValueSet_Range{
							RangeValues: []*dashboardpb.ChartValueRange{
								timeRange,
							},
						},
					},
				},
			})
		default:
			filtersFromGroupByValue = append(filtersFromGroupByValue, &dashboardpb.ChartDataFilter{
				Operator: dashboardpb.ChartDataFilter_IN,
				ValueSet: &dashboardpb.ChartValueSet{
					Values: &dashboardpb.ChartValueSet_SingleValues{
						SingleValues: &dashboardpb.ChartValueSet_Single{
							AttributeValues: []*dashboardpb.ChartAttributeValue{chartAttributeValue},
						},
					},
				},
			})
		}
	}

	// Save a copy of the original filters before adding synthetic filters from metrics/groupBys.
	// We'll need this later to identify message-level entities in the original filters only.
	originalFilters := chartDataConfig.Filters

	// Add the group_by filters to the main chart filters.
	chartDataConfig.Filters = append(chartDataConfig.Filters, filtersFromGroupByValue...)

	groupByFromMetric, filterFromMetric, err := extractGroupByAndFilterFromMetric(chartDataConfig.Metrics[0])
	if err != nil {
		return nil, err
	}
	if filterFromMetric != nil {
		chartDataConfig.Filters = append(chartDataConfig.Filters, filterFromMetric)
	}

	// Collect the attribute values that make up the chart metric entity. A group_by filter is
	// "related to the metric" (and keeps its current occurrence semantics, i.e. is excluded from
	// convoFilterFromMsgEntity below) only when every value it carries also exists in the metric.
	// When the metric has a single entity it becomes filterFromMetric; when it has multiple
	// entities it becomes groupByFromMetric.
	metricValueSet := groupByFromMetric.GetByProvidedValue().GetValueSet()
	if metricValueSet == nil {
		metricValueSet = filterFromMetric.GetValueSet()
	}
	metricAttributeValues := metricValueSet.GetSingleValues().GetAttributeValues()

	// isRelatedToMetric reports whether every attribute value in the filter also exists in the
	// chart metric's value set.
	isRelatedToMetric := func(values []*dashboardpb.ChartAttributeValue) bool {
		if len(values) == 0 || len(metricAttributeValues) == 0 {
			return false
		}
		for _, value := range values {
			foundInMetric := false
			for _, metricValue := range metricAttributeValues {
				if proto.Equal(value, metricValue) {
					foundInMetric = true
					break
				}
			}
			if !foundInMetric {
				return false
			}
		}
		return true
	}

	// Prefetch topic details and aggregate expansions to avoid repeated fetching inside inner loops.
	// This must happen AFTER all filters/groupBys from metrics are added to chartDataConfig.
	fetchedTopics, aggregateExpansions, err := a.fetchTopicDetailsForChart(ctx, req.Parent, chartDataConfig.GroupBy, chartDataConfig.Filters)
	if err != nil {
		return nil, err
	}
	v3ClusterClosures, err := a.buildTopicDiscoveryV3ClusterClosures(ctx, *parent, chartDataConfig)
	if err != nil {
		return nil, err
	}
	_, documentType, _, _, err := a.convertChartMetricToESMetric(ctx, parent, profileConfig, chartDataConfig.Metrics[0], chartDataConfig.GroupBy, elasticsearch.MaxCardinalityPrecisionThreshold, fetchedTopics, aggregateExpansions, v3ClusterClosures)
	if err != nil {
		return nil, err
	}

	// Populate convoFilterFromMsgEntity if needed.
	// This identifies message-level entities in the original filters, plus any group_by filters
	// that are NOT related to the chart metric. Filters related to the metric define the
	// occurrence itself and keep their current semantics (i.e. they are excluded here).
	convoFilterFromMsgEntity := make(map[*dashboardpb.ChartDataFilter]any)
	if req.RequestOccurrence && documentType == elasticsearch.DocumentTypeConversation {
		addMsgEntityFilter := func(f *dashboardpb.ChartDataFilter) {
			for _, val := range f.ValueSet.GetSingleValues().GetAttributeValues() {
				if occurrenceDocumentType(val) == elasticsearch.DocumentTypeMessage {
					convoFilterFromMsgEntity[f] = nil
					break
				}
			}
		}

		for _, f := range originalFilters {
			addMsgEntityFilter(f)
		}

		// Treat non-metric group_by filters like the original filters; skip metric-related ones.
		for _, f := range filtersFromGroupByValue {
			if isRelatedToMetric(f.ValueSet.GetSingleValues().GetAttributeValues()) {
				continue
			}
			addMsgEntityFilter(f)
		}
	}

	// Modify documentType to Message based on request_occurrence.
	if req.RequestOccurrence {
		documentType = elasticsearch.DocumentTypeMessage
		// Ensure there exists message-level Cresta entity, otherwise there is
		// no point returning occurrences (aka messages).
		exists := false
		for _, f := range chartDataConfig.Filters {
			if exists {
				break
			}
			if f.Operator == dashboardpb.ChartDataFilter_IN && f.ValueSet.GetSingleValues() != nil {
				for _, val := range f.ValueSet.GetSingleValues().AttributeValues {
					docType := occurrenceDocumentType(val)
					if docType == elasticsearch.DocumentTypeMessage {
						exists = true
						break
					}
				}
			}
		}
		if !exists {
			return nil, status.Errorf(codes.InvalidArgument, "occurrences not applicable when there is no message-level Cresta entity in filter (including filter from group_by_values)")
		}
	}

	esQuery, err := a.convertChartToESQuery(ctx, *parent, profileConfig, documentType, chartDataConfig, req.ConvoFilter, req.Metadata, fetchedTopics, convoFilterFromMsgEntity, aggregateExpansions, v3ClusterClosures, true /*=agentFilterAlwaysOnConversation*/)
	if err != nil {
		return nil, err
	}

	// Parse the page token from API req.
	var searchAfter *[]string
	sort := elasticsearch.DefaultSort
	if req.PageToken != "" {
		sortKeys := strings.Split(req.PageToken, esSortKeySeparator)
		searchAfter = &sortKeys
	}
	if documentType == elasticsearch.DocumentTypeMessage {
		sort = elasticsearch.DefaultSortForMessageDoc
	}
	if searchAfter != nil && len(*searchAfter) != len(*sort) {
		return nil, fmt.Errorf("inconsistent length of search_after %v and sort %v", *searchAfter, *sort)
	}

	// Parse the page size.
	pageSize := elasticsearch.DefaultPageSize
	if req.PageSize > 0 {
		pageSize = int(req.PageSize)
	}

	// Parse the source with ignored fields.
	source := &elasticsearch.Source{
		Excludes: &elasticsearch.DefaultIgnoredFields,
	}

	// Build ES request.
	var esRequest ChartESRequest
	if req.GetSortOptions().GetSortBy() == analyticspb.RetrieveAnalyticsChartExamplesRequest_SortOptions_SORT_BY_RANDOM {
		esRequest = ChartESRequest{
			Query: &ChartESQuery{
				FunctionScore: &elasticsearch.FunctionScore{
					Query:       &esQuery,
					RandomScore: &elasticsearch.RandomScoreFunction{},
					BoostMode:   "replace",
				},
			},
			Source: source,
		}
	} else {
		esRequest = ChartESRequest{
			Query:       &esQuery,
			SearchAfter: searchAfter,
			Sort:        sort,
			Source:      source,
		}
	}

	if documentType == elasticsearch.DocumentTypeMessage {
		esRequest.Highlight = &elasticsearch.DefaultHightlight
		// Add aggregation to count conversations when querying messages.
		esRequest.Aggs = &ChartESAggregation{
			MetricsConversationCount: elasticsearch.CreateReqTotalConversationCount(elasticsearch.MaxCardinalityPrecisionThreshold),
		}
	}
	if chartFiltersHaveAdvancedTimeRule(chartDataConfig.Filters) {
		timeZone := req.Metadata.GetTimeZoneId()
		if timeZone == "" {
			timeZone = shared.DefaultTimeZone
		}
		esRequest.RuntimeMappings = elasticsearch.BuildRuntimeMappingsForTruncatedTimeRangeFilters(
			timeZone, analyticspb.RetrieveClosedConversationsRequest_TARGET_FIELD_FOR_TIME_RANGE_UNSPECIFIED,
		)
	}
	a.logger.Infof(ctx, "RetrieveAnalyticsChartExamples ES request: %v", esRequest)

	// Query ES.
	esResponse := ChartESResponse{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount,
		pageSize, true, /*=logESQuery*/
		esRequest, &esResponse, fmt.Sprintf("RetrieveAnalyticsChartExamples for %v", req.AnalyticsChart.Name))
	if err != nil {
		return nil, err
	}
	a.logger.Infof(ctx, "RetrieveAnalyticsChartExamples ES response: %v", esResponse)

	return a.convertToChartExamplesResponse(parent, documentType, pageSize, esResponse.Hits, esResponse.Aggregations)
}

func hasSameConcreteType(chartAttributeValues []*dashboardpb.ChartAttributeValue) bool {
	if len(chartAttributeValues) == 0 {
		return true
	}

	firstType := reflect.TypeOf(chartAttributeValues[0].GetValue())
	for _, attrValue := range chartAttributeValues {
		if reflect.TypeOf(attrValue.GetValue()) != firstType {
			return false
		}
	}
	return true
}

func getGetGroupByTimeInterval(req *analyticspb.RetrieveAnalyticsChartExamplesRequest) dashboardpb.ChartGroupBy_Time_Interval {
	ret := dashboardpb.ChartGroupBy_Time_INTERVAL_UNSPECIFIED
	for _, groupBy := range req.AnalyticsChart.DataConfig.GroupBy {
		groupByTime, ok := groupBy.GroupByMode.(*dashboardpb.ChartGroupBy_ByTime)
		if ok {
			ret = groupByTime.ByTime.Interval
			break
		}
	}
	return ret
}

func (a AnalyticsServiceImpl) convertToChartExamplesResponse(
	customerProfile *shared.CustomerProfile,
	documentType elasticsearch.DocumentType,
	pageSize int, hits elasticsearch.ResHits,
	aggregations *ESRespSingleBucket,
) (*analyticspb.RetrieveAnalyticsChartExamplesResponse, error) {
	nextPageToken := ""
	if hits := hits.Hits; len(hits) > 0 && len(hits) == pageSize {
		ss := []string{}
		for _, val := range hits[len(hits)-1].Sort {
			ss = append(ss, fmt.Sprintf("%v", val))
		}
		nextPageToken = strings.Join(ss, esSortKeySeparator)
	}

	// Parse the conversation example from ES response hits.
	if documentType == elasticsearch.DocumentTypeConversation {
		convos := []*convopb.Conversation{}
		for _, hit := range hits.Hits {
			convo := &convopb.Conversation{
				Name: convopb.ConversationName{
					CustomerID:     customerProfile.CustomerID,
					ProfileID:      customerProfile.ProfileID,
					ConversationID: hit.Source.ConversationID,
				}.String(),
				ConversationCorrelationId: hit.Source.ConversationCorrelationID,
			}
			if platformConvoID := hit.Source.PlatformConversationID; platformConvoID != "" {
				platformInfo := &convopb.Conversation_PlatformInfo{
					PlatformConversationId: platformConvoID,
				}
				if platformAccountID := hit.Source.PlatformAccountID; platformAccountID != "" {
					platformInfo.PlatformAccountId = platformAccountID
				}
				convo.PlatformInfo = platformInfo
			}
			if val := hit.Source.ConversationStartedAt; val != "" {
				startedAt, err := shared.ConvertStringToAPITimestamp(val)
				if err != nil {
					return nil, err
				}
				convo.StartTime = startedAt
			}
			if val := hit.Source.ConversationEndedAt; val != "" {
				endedAt, err := shared.ConvertStringToAPITimestamp(val)
				if err != nil {
					return nil, err
				}
				convo.EndTime = endedAt
			}
			if name := hit.Source.AgentName; name != "" {
				convo.AgentInfo = &convopb.ConversationMessage_SpeakerInfo{
					DisplayName: name,
				}
			}
			convos = append(convos, convo)
		}
		return &analyticspb.RetrieveAnalyticsChartExamplesResponse{
			ChartExamples: &dashboardpb.AnalyticsChartExamples{
				Examples: &dashboardpb.AnalyticsChartExamples_Conversation{
					Conversation: &dashboardpb.ConversationExamples{
						ConversationItems: convos,
					},
				},
			},
			NextPageToken:          nextPageToken,
			TotalExampleCount:      int32(hits.Total.Value),
			TotalConversationCount: int32(hits.Total.Value),
		}, nil
	}

	// Parse the message example from ES response hits.
	if documentType == elasticsearch.DocumentTypeMessage {
		msgs := []*convopb.ConversationMessage{}
		for _, hit := range hits.Hits {
			msgName := convopb.ConversationMessageName{
				CustomerID:     customerProfile.CustomerID,
				ProfileID:      customerProfile.ProfileID,
				ConversationID: hit.Source.ConversationID,
				MessageID:      hit.Source.ConversationMessageID,
			}.String()
			speakerRole, err := shared.ConvertESToAPISpeakerRole(hit.Source.SpeakerRole)
			if err != nil {
				return nil, err
			}
			msgCreatedAt, err := messageCreateTimeFromESSource(hit)
			if err != nil {
				return nil, err
			}
			msg := &convopb.ConversationMessage{
				Name:       msgName,
				Text:       elasticsearch.MarkHighlightMessageText(hit),
				Speaker:    speakerRole,
				CreateTime: msgCreatedAt,
			}
			if name := hit.Source.AgentName; name != "" {
				msg.SpeakerInfo = &convopb.ConversationMessage_SpeakerInfo{
					DisplayName: name,
				}
			}
			msgs = append(msgs, msg)
		}

		// Extract conversation count from aggregations
		totalConversationCount := int32(0)
		if aggregations != nil && aggregations.MetricsConversationCount != nil {
			if value, ok := aggregations.MetricsConversationCount.numericValue(); ok {
				totalConversationCount = int32(value)
			}
		}

		return &analyticspb.RetrieveAnalyticsChartExamplesResponse{
			ChartExamples: &dashboardpb.AnalyticsChartExamples{
				Examples: &dashboardpb.AnalyticsChartExamples_Message{
					Message: &dashboardpb.MessageExamples{
						MessageItems: msgs,
					},
				},
			},
			NextPageToken:          nextPageToken,
			TotalExampleCount:      int32(hits.Total.Value),
			TotalConversationCount: totalConversationCount,
		}, nil
	}

	return nil, fmt.Errorf("unexpected document type: %v", documentType)
}
