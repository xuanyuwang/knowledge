package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmomentpb "github.com/cresta/go-servers/apiserver/sql-schema/gen/go/protos/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"github.com/cresta/go-servers/shared/converters/momentconverter"
	"github.com/cresta/shared-go/framework/extrawork"
)

const (
	esSortKeySeparator = "###"
	partialChatLength  = 10
)

func (a AnalyticsServiceImpl) RetrieveConversationMessages(ctx context.Context, req *analyticspb.RetrieveConversationMessagesRequest) (*analyticspb.RetrieveConversationMessagesResponse, error) {
	a.logger.Debugf(ctx, "RetrieveConversationMessages request:\n%s", req)
	shared.CountFilter(ctx, req.FilterConversationByAttribute)
	shared.CountFilter(ctx, req.FilterMessageByAttribute)
	if req.PopulateMomentMatchOnTheFly {
		a.logger.Infof(ctx, "RetrieveConversationMessages with PopulateMomentMatchOnTheFly=True")
	}

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
	recordConversationReadRecency(ctx, req.GetFilterByTimeRange().GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_conversation_messages")

	// Get resource ACL based on the caller identity.
	if req.FilterConversationByAttribute == nil {
		req.FilterConversationByAttribute = &analyticspb.Attribute{}
	}

	// As mentioned in proto definition, FilterConversationDocumentByConversationByAttribute should be used like enum,
	// it only has 2 states: nil or empty struct. Some legacy clients still provide convo filter in this field,
	// we merge it back to FilterConversationByAttribute for backward compatibility.
	if req.FilterConversationDocumentByConversationByAttribute != nil {
		proto.Merge(req.FilterConversationByAttribute, req.FilterConversationDocumentByConversationByAttribute)
		req.FilterConversationDocumentByConversationByAttribute = &analyticspb.Attribute{}
	}

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
		a.logger.Debugf(ctx, "[retrieveConversationMessages] return empty response due to ACL filtered users/groups")
		return &analyticspb.RetrieveConversationMessagesResponse{}, nil
	}

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

	// Handle aggregate moments by expanding them to their leaf descendants.
	momentNames, hasCompositeMoment := getInsightsCompositeMoments(req.FilterConversationByAttribute, req.FilterMessageByAttribute, nil)
	topicDetails := map[string]elasticsearch.TopicDetails{}
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
			// Get updated moment names after expansion
			momentNames, _ = getInsightsCompositeMoments(req.FilterConversationByAttribute, req.FilterMessageByAttribute, nil)
		}
	}
	// Fetch topic details for all composite moments (after expansion if applicable)
	if len(momentNames) > 0 {
		topicDetails, err = a.fetchTopicDetails(ctx, parent.String(), momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Convert InsightsTopics (both from MomentGroups and inline InsightsTopics field)
	if hasCompositeMoment {
		momentClient := a.momentClientRegistry.Client(ctx)
		if err := a.convertAPIInsightsTopics(ctx, req.FilterMessageByAttribute, topicDetails, *parent, "msg-filter", momentClient); err != nil {
			return nil, err
		}

		if err := a.convertAPIInsightsTopics(ctx, req.FilterConversationByAttribute, topicDetails, *parent, "convo-filter", momentClient); err != nil {
			return nil, err
		}
	}

	// Expand V3 category filters to their full subtree so a drill-down on a parent returns every
	// conversation classified anywhere beneath it.
	if _, err = a.applyTopicDiscoveryV3Expansion(
		ctx,
		*parent,
		[]*analyticspb.Attribute{req.FilterConversationByAttribute, req.FilterMessageByAttribute},
		nil,
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	tdV2ClusterCandidates := extractTDV2ClusterCandidates(req)
	tdV2Mode := tdV2ReadModeESOnly
	if len(tdV2ClusterCandidates) > 0 {
		tdV2Mode = a.evaluateTDV2ReadMode(ctx, parent.CustomerID, parent.ProfileID)
	}

	topicMomentsToMatch := map[string]interface{}{}
	if req.OnlyReturnPartialChat {
		if len(req.FilterConversationByAttribute.MomentGroups) == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "`only_return_partial_chat` is set to true, but no `momentGroups` are provided in `filter_conversation_by_attribute`")
		}
		for _, momentGroup := range req.FilterConversationByAttribute.MomentGroups {
			for _, moment := range momentGroup.Moments {
				if moment.Type == momentpb.Moment_TOPIC {
					topicMomentsToMatch[moment.Name] = nil
				}
			}
		}
		if len(topicMomentsToMatch) == 0 {
			return nil, status.Errorf(codes.InvalidArgument, "`only_return_partial_chat` is set to true, but no `topic` moment is provided in moment group of `filter_conversation_by_attribute`")
		}
	}

	if tdV2Mode == tdV2ReadModePGOnly && req.OnlyReturnPartialChat {
		// `only_return_partial_chat` matches topic moments against the ES moment groups that
		// pg_only would strip, so it always serves ES. Count it so a flat mismatch metric is not
		// mistaken for a clean pg_only rollout.
		a.skipTDV2Compare(ctx, parent.CustomerID, tdV2SkipReasonOnlyPartialChat)
	}
	if tdV2Mode == tdV2ReadModePGOnly && !req.OnlyReturnPartialChat {
		pgCount, pgErr := a.applyPGOnlyClusterFilter(ctx, parent, req, tdV2ClusterCandidates)
		if pgErr != nil {
			// fail-open: pg_only is a read-path migration, so any Postgres problem serves ES.
			a.logger.Warnf(ctx, "pg_only cluster filter failed, falling back to ES: %v", pgErr)
		} else if pgCount == 0 {
			return &analyticspb.RetrieveConversationMessagesResponse{}, nil
		}
	}

	esQueryConfig, err := a.getElasticsearchQueryConfig(ctx, profileConfig, req.ElasticsearchQueryConfig, req.FilterConversationDocumentByConversationByAttribute)
	if err != nil {
		return nil, err
	}

	pageSize := shared.ParsePageSizeAndPopulateDefaultValue(req.PageSize)
	query, momentGroup, err := a.queryESTrackHits(
		ctx,
		req,
		durationBuckets,
		topicDetails,
		pageSize,
		esQueryConfig,
	)
	if err != nil {
		return nil, err
	}
	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeTrackHit,
		pageSize, logESQuery(req.ElasticsearchQueryConfig), query, &res, esQueryID(req.ElasticsearchQueryConfig))
	if err != nil {
		return nil, err
	}
	resp, err := a.convertHitsToAPIResponse(ctx, parent, pageSize, res.Hits, res.Aggregations, momentGroup, topicMomentsToMatch)
	if err != nil {
		return nil, err
	}

	// Avoid populating conversation moment annotations when requested so. By default, these are included for backwards compatibility.
	if !req.ExcludeConversationMomentAnnotations {
		if err := a.populateConvoMomentAnnotations(ctx, profileConfig, parent, req.ElasticsearchQueryConfig, resp); err != nil {
			return nil, err
		}
	}

	if tdV2Mode == tdV2ReadModeDualRead {
		parentCopy := *parent
		extrawork.RunWithTimeout(ctx, pgCompareTimeout, func(ctx context.Context) error {
			a.compareTDV2ClusterMembership(ctx, &parentCopy, req, resp, tdV2ClusterCandidates)
			return nil
		})
	}

	return resp, err
}

func (a AnalyticsServiceImpl) queryESTrackHits(
	ctx context.Context,
	req *analyticspb.RetrieveConversationMessagesRequest,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	topicDetails map[string]elasticsearch.TopicDetails,
	pageSize int,
	elasticsearchQueryConfig elasticsearch.QueryConfig,
) (elasticsearch.Request, *analyticspb.MomentGroup, error) {
	var (
		aggs        *elasticsearch.ReqAggs
		momentGroup *analyticspb.MomentGroup
	)
	if req.PopulateMomentMatchOnTheFly {
		var err error
		if req.FilterMessageByAttribute != nil && len(req.FilterMessageByAttribute.MomentGroups) > 0 {
			momentGroup = req.FilterMessageByAttribute.MomentGroups[0]
		}
		aggs, err = elasticsearch.ParseFilterByAttributeStructuresToAgg(
			momentGroup, topicDetails, pageSize, elasticsearchQueryConfig)
		if err != nil {
			return elasticsearch.Request{}, nil, err
		}
	}
	req.FilterConversationByAttribute, req.FilterMessageByAttribute = RemoveConvoLevelMomentsFromMessageFilter(req.FilterConversationByAttribute, req.FilterMessageByAttribute)
	// Gets the ES Query phrase.
	query, err := getESQueryInternal(
		req.FilterByTimeRange,
		req.Metadata,
		durationBuckets,
		req.FilterConversationByAttribute,
		req.FilterMessageByAttribute,
		req.FilterConversationDocumentByConversationByAttribute,
		nil, /*groupByAttributes*/
		topicDetails,
		a.appsDB.DB(ctx),
		elasticsearchQueryConfig,
		true, /*allowFilteringByConvoFieldsInMsg*/
		*enableDeprecatePostgresUserIDQuery)
	if err != nil {
		return elasticsearch.Request{}, nil, err
	}
	var highlight *elasticsearch.ReqHighlight
	skipHighlight := func() bool {
		return req.FilterConversationDocumentByConversationByAttribute != nil || (req.ElasticsearchQueryConfig != nil && req.ElasticsearchQueryConfig.SkipHighlightInMessageText)
	}
	if !skipHighlight() {
		highlight = &elasticsearch.DefaultHightlight
	}

	var source *elasticsearch.Source
	if req.ElasticsearchQueryConfig != nil && len(req.ElasticsearchQueryConfig.IgnoredFields) > 0 {
		source = &elasticsearch.Source{
			Excludes: &req.ElasticsearchQueryConfig.IgnoredFields,
		}
	}

	sort := elasticsearch.DefaultSort
	if req.FilterConversationDocumentByConversationByAttribute == nil {
		sort = elasticsearch.DefaultSortForMessageDoc
	}
	if req.SortOption == analyticspb.RetrieveConversationMessagesRequest_SORT_OPTION_RANDOM && req.SeedForRandomSort > 0 {
		sort = elasticsearch.SortByScore
	}

	// For deep pagination.
	var searchAfter *[]string
	if req.PageToken != "" {
		sortKeys := strings.Split(req.PageToken, esSortKeySeparator)
		searchAfter = &sortKeys

		if len(*searchAfter) != len(*sort) {
			return elasticsearch.Request{}, nil, fmt.Errorf("inconsistent length of search_after %v and sort %v", *searchAfter, *sort)
		}
	}

	var request elasticsearch.Request
	if req.SortOption == analyticspb.RetrieveConversationMessagesRequest_SORT_OPTION_RANDOM {
		field := ""
		var seed int32
		if req.SeedForRandomSort > 0 {
			seed = req.SeedForRandomSort
			switch elasticsearchQueryConfig.EsDocumentType {
			case elasticsearch.DocumentTypeConversation:
				field = "conversation_started_at"

			case elasticsearch.DocumentTypeMessage, elasticsearch.DocumentTypeMessageGroup:
				field = "created_at"
			}
		}

		request = elasticsearch.Request{
			Source:    source,
			Highlight: highlight,
			Aggs:      aggs,
			Query: &elasticsearch.OuterQuery{
				FunctionScore: &elasticsearch.FunctionScore{
					Query: query,
					RandomScore: &elasticsearch.RandomScoreFunction{
						Field: field,
						Seed:  seed,
					},
					BoostMode: "replace",
				},
			},
			Sort:        sort,
			SearchAfter: searchAfter,
		}
	} else {
		request = elasticsearch.Request{
			Source:      source,
			Highlight:   highlight,
			Aggs:        aggs,
			Query:       query,
			SearchAfter: searchAfter,
			Sort:        sort,
		}

	}
	return request, momentGroup, nil
}

func (a AnalyticsServiceImpl) getConvoMomentAnnotations(ctx context.Context, profileConfig *config.ProfileConfig, customerProfile *shared.CustomerProfile, esQueryConfig *analyticspb.ElasticsearchQueryConfig, convoIDs []string) (map[string][]*analyticspb.MomentResult, error) {
	query, err := elasticsearch.ParseFilterByAttributeToQuery(
		&elasticsearch.ConversationFilterParams{
			ConversationIDs: convoIDs,
		},
		nil,
		elasticsearch.DocumentTypeConversation,
		nil, elasticsearch.QueryConfig{})
	if err != nil {
		return nil, err
	}

	request := elasticsearch.Request{
		Query: &query,
		Sort:  elasticsearch.DefaultSort,
	}

	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: customerProfile.CustomerID, ProfileID: customerProfile.ProfileID}),
		elasticsearch.EsModeTrackHit,
		9999, false, request, &res, esQueryID(esQueryConfig))
	if err != nil {
		return nil, err
	}

	convoIDTOMomentAnnotations := map[string][]*analyticspb.MomentResult{}
	for _, hit := range res.Hits.Hits {
		// Fill from precomputed moment_annotation
		for _, momentAnnotation := range hit.Source.MomentAnnotations {
			convoID := hit.Source.ConversationID
			momentResult, err := a.toMomentResult(customerProfile, momentAnnotation, nil)
			if err != nil {
				return nil, err
			}
			convoIDTOMomentAnnotations[convoID] = append(convoIDTOMomentAnnotations[convoID], momentResult)
		}
	}
	return convoIDTOMomentAnnotations, nil
}

func (a AnalyticsServiceImpl) populateConvoMomentAnnotations(ctx context.Context, profileConfig *config.ProfileConfig, customerProfile *shared.CustomerProfile, apiESQueryConfig *analyticspb.ElasticsearchQueryConfig, resp *analyticspb.RetrieveConversationMessagesResponse) error {
	if apiESQueryConfig != nil {
		for _, field := range apiESQueryConfig.IgnoredFields {
			if field == "moment_annotations" {
				return nil
			}
		}
	}
	convoIDs := []string{}
	for _, c := range resp.ConversationMessageResults {
		convoName, err := c.ConversationResult.Conversation.ParseResourceName()
		if err != nil {
			return err
		}
		convoIDs = append(convoIDs, convoName.GetConversationID())

	}
	if len(convoIDs) == 0 {
		return nil
	}
	convoAnnotations, err := a.getConvoMomentAnnotations(ctx, profileConfig, customerProfile, apiESQueryConfig, convoIDs)
	if err != nil {
		return err
	}
	for _, c := range resp.ConversationMessageResults {
		convoName, err := c.ConversationResult.Conversation.ParseResourceName()
		if err != nil {
			return err
		}
		c.ConversationResult.MomentResults = convoAnnotations[convoName.GetConversationID()]
	}
	return nil
}

func (a *AnalyticsServiceImpl) toMomentResult(customerProfile *shared.CustomerProfile, momentAnnotation elasticsearch.ResMomentAnnotation, nameToMoment map[string]*momentpb.Moment) (*analyticspb.MomentResult, error) {
	momentType, err := momentconverter.DBToServiceMomentType(dbmomentpb.Moment_MomentType(momentAnnotation.Type))
	if err != nil {
		return nil, err
	}
	moment := &momentpb.Moment{
		Name: momentpb.MomentName{
			CustomerID: customerProfile.CustomerID,
			ProfileID:  customerProfile.ProfileID,
			MomentID:   momentAnnotation.MomentTemplateID,
		}.String(),
		DisplayName: momentAnnotation.DisplayName,
		Taxonomy:    momentAnnotation.Taxonomy,
		Type:        momentType,
	}

	if val, exists := nameToMoment[moment.Name]; exists {
		moment.DetailedType = val.DetailedType
	}

	momentResult := &analyticspb.MomentResult{
		Moment: moment,
	}

	if momentAnnotation.Payload != nil {
		momentResult.MomentAnnotation = &momentpb.MomentAnnotation{
			Type:         moment.Type,
			DetailedType: moment.DetailedType,
			Payload: &momentpb.MomentAnnotation_TopicPayload{
				TopicPayload: &momentpb.TopicPayload{
					ConversationReason: momentAnnotation.Payload.TopicPayload.ConversationReason,
					ExtractedQuestion:  momentAnnotation.Payload.TopicPayload.ExtractedQuestion,
				},
			},
		}
	}
	return momentResult, nil
}

func (a AnalyticsServiceImpl) convertHitsToAPIResponse(
	ctx context.Context,
	customerProfile *shared.CustomerProfile,
	pageSize int,
	hits elasticsearch.ResHits,
	aggregations elasticsearch.ResAggregations,
	momentGroup *analyticspb.MomentGroup,
	topicMomentsToMatch map[string]interface{},
) (*analyticspb.RetrieveConversationMessagesResponse, error) {
	convoMsgMomentMap := map[string]map[string][]*momentpb.Moment{}
	if momentGroup != nil && aggregations.Moments != nil {
		for index, bucket := range *aggregations.Moments.Buckets {
			if bucket.TotalConversationCount.Value == 0 {
				continue
			}
			for _, hit := range bucket.AnnotationHit.Hits.Hits {
				msg := hit.Source
				convoName := conversationpb.ConversationName{
					CustomerID:     customerProfile.CustomerID,
					ProfileID:      customerProfile.ProfileID,
					ConversationID: msg.ConversationID,
				}.String()
				msgName := conversationpb.ConversationMessageName{
					CustomerID:     customerProfile.CustomerID,
					ProfileID:      customerProfile.ProfileID,
					ConversationID: msg.ConversationID,
					MessageID:      msg.ConversationMessageID,
				}.String()
				if _, exists := convoMsgMomentMap[convoName]; !exists {
					convoMsgMomentMap[convoName] = make(map[string][]*momentpb.Moment)
				}
				convoMsgMomentMap[convoName][msgName] = append(
					convoMsgMomentMap[convoName][msgName], momentGroup.Moments[index])
			}
		}
	}

	messageGroupIDs := map[string]int{} // only contains message groups with 1 message
	messageIDs := map[string]int{}

	uniqueMsgKeyFunc := func(convoID, msgID string) string {
		return fmt.Sprintf("%s_%s", convoID, msgID)
	}

	for i, hit := range hits.Hits {
		if hit.Source.ConversationMessageID != "" {
			// ConversationMessageID is not unique across conversations.
			uniqueMsgKey := uniqueMsgKeyFunc(hit.Source.ConversationID, hit.Source.ConversationMessageID)
			docType, err := hit.Source.ReadConversationToMessageJoin()
			if err != nil {
				return nil, err
			}
			if docType == string(elasticsearch.DocumentTypeMessageGroup) && len(hit.Source.MessageIDs) == 1 {
				messageGroupIDs[uniqueMsgKey] = i
			} else if docType == string(elasticsearch.DocumentTypeMessage) {
				messageIDs[uniqueMsgKey] = i
			}
		}
	}

	topicMomentsFetched := map[string]*momentpb.Moment{}
	if len(topicMomentsToMatch) > 0 {
		momentNames := []string{}
		for name := range topicMomentsToMatch {
			momentNames = append(momentNames, name)
		}
		batchGetReq := &momentpb.BatchGetMomentsRequest{Parent: customerProfile.String(), Names: momentNames}

		batchGetResp, err := a.momentClientRegistry.Client(ctx).BatchGetMoments(ctx, batchGetReq)
		if err != nil {
			return nil, status.Errorf(status.Code(err), "calling BatchGetMoments() for topic moments with request %v: %v", batchGetReq, err)
		}
		for _, m := range batchGetResp.Moments {
			topicMomentsFetched[m.Name] = m
		}
	}

	// skip messages that have the corresponding message group with 1 message
	// message groups with more than 1 message are not skipped
	messagesToSkip := map[string]interface{}{}
	for uniqueMsgKey, msgGroupIndex := range messageGroupIDs {
		if msgIndex, exists := messageIDs[uniqueMsgKey]; exists {
			messagesToSkip[uniqueMsgKey] = nil
			// copy the message forward position from the message to the message group
			hits.Hits[msgGroupIndex].Source.MessageForwardPosition = hits.Hits[msgIndex].Source.MessageForwardPosition
		}
	}

	type MessageResultWithPosition struct {
		MessageResult *analyticspb.ConversationMessageResult_MessageResult
		Position      int
	}

	conversationMessageResultsMap := map[string]*analyticspb.ConversationMessageResult{}
	messageResultWithPositionsMap := map[string][]MessageResultWithPosition{}
	for _, hit := range hits.Hits {
		docType, err := hit.Source.ReadConversationToMessageJoin()
		if err != nil {
			return nil, err
		}
		// Skip the message if there are duplicates.
		if docType == string(elasticsearch.DocumentTypeMessage) {
			uniqueMsgKey := uniqueMsgKeyFunc(hit.Source.ConversationID, hit.Source.ConversationMessageID)
			if _, exists := messagesToSkip[uniqueMsgKey]; exists {
				continue
			}
		}

		var convoStartedAt, convoEndedAt *timestamppb.Timestamp
		if val := hit.Source.ConversationStartedAt; val != "" {
			convoStartedAt, err = shared.ConvertStringToAPITimestamp(val)
			if err != nil {
				return nil, err
			}
		}
		if val := hit.Source.ConversationEndedAt; val != "" {
			convoEndedAt, err = shared.ConvertStringToAPITimestamp(val)
			if err != nil {
				return nil, err
			}
		}

		convoName := conversationpb.ConversationName{
			CustomerID:     customerProfile.CustomerID,
			ProfileID:      customerProfile.ProfileID,
			ConversationID: hit.Source.ConversationID,
		}.String()
		if _, exists := conversationMessageResultsMap[convoName]; !exists {
			convoMsgResult := &analyticspb.ConversationMessageResult{
				ConversationResult: &analyticspb.ConversationMessageResult_ConversationResult{
					Conversation: &conversationpb.Conversation{
						Name:         convoName,
						LegacyChatId: int32(hit.Source.ChatID),
						StartTime:    convoStartedAt,
						EndTime:      convoEndedAt,
						PlatformInfo: &conversationpb.Conversation_PlatformInfo{
							PlatformConversationId: hit.Source.PlatformConversationID,
							PlatformAccountId:      hit.Source.PlatformAccountID,
						},
						ConversationCorrelationId: hit.Source.ConversationCorrelationID,
					},
				},
			}
			if platformConvoID := hit.Source.PlatformConversationID; platformConvoID != "" {
				platformInfo := &conversationpb.Conversation_PlatformInfo{
					PlatformConversationId: platformConvoID,
				}
				if platformAccountID := hit.Source.PlatformAccountID; platformAccountID != "" {
					platformInfo.PlatformAccountId = platformAccountID
				}
				convoMsgResult.ConversationResult.Conversation.PlatformInfo = platformInfo
			}
			conversationMessageResultsMap[convoName] = convoMsgResult
		}
		if docType == string(elasticsearch.DocumentTypeConversation) {
			if name := hit.Source.AgentName; name != "" {
				conversationMessageResultsMap[convoName].ConversationResult.Conversation.AgentInfo = &conversationpb.ConversationMessage_SpeakerInfo{
					DisplayName: name,
				}
			}
		} else {
			speakerRole, err := shared.ConvertESToAPISpeakerRole(hit.Source.SpeakerRole)
			if err != nil {
				return nil, err
			}
			msgCreatedTime, err := messageCreateTimeFromESSource(hit)
			if err != nil {
				return nil, err
			}
			messageText := elasticsearch.MarkHighlightMessageText(hit)
			msgName := conversationpb.ConversationMessageName{
				CustomerID:     customerProfile.CustomerID,
				ProfileID:      customerProfile.ProfileID,
				ConversationID: hit.Source.ConversationID,
				MessageID:      hit.Source.ConversationMessageID,
			}.String()
			msgResult := &analyticspb.ConversationMessageResult_MessageResult{
				ConversationMessage: &conversationpb.ConversationMessage{
					Name:             msgName,
					Text:             messageText,
					Speaker:          speakerRole,
					CreateTime:       msgCreatedTime,
					SequencePosition: proto.Int32(int32(hit.Source.MessageForwardPosition)),
				},
			}
			if name := hit.Source.AgentName; name != "" {
				msgResult.ConversationMessage.SpeakerInfo = &conversationpb.ConversationMessage_SpeakerInfo{
					DisplayName: name,
				}
			}

			momentResults := []*analyticspb.MomentResult{}

			// Do not populate precomputed moment annotations for moments matched on the fly.
			momentIDsMatchedOnTheFly := map[string]interface{}{}
			if momentGroup != nil {
				for _, m := range momentGroup.Moments {
					momentName, err := momentpb.ParseMomentName(m.Name)
					if err != nil {
						return nil, err
					}
					momentIDsMatchedOnTheFly[momentName.MomentID] = nil
				}
			}
			for _, momentAnnotation := range hit.Source.MomentAnnotations {
				if _, exists := momentIDsMatchedOnTheFly[momentAnnotation.MomentTemplateID]; exists {
					continue
				}
				momentResult, err := a.toMomentResult(customerProfile, momentAnnotation, topicMomentsFetched)
				if err != nil {
					return nil, err
				}
				momentResults = append(momentResults, momentResult)
			}

			// Fill from on-the-fly moment_annotation
			if msgMomentMap, exist := convoMsgMomentMap[convoName]; exist {
				if momentsOnTheFly, exist := msgMomentMap[msgName]; exist {
					for _, moment := range momentsOnTheFly {
						momentResults = append(momentResults, &analyticspb.MomentResult{
							Moment: moment,
						})
					}
				}
			}
			msgResult.MomentResults = momentResults
			messageResultWithPositionsMap[convoName] = append(messageResultWithPositionsMap[convoName], MessageResultWithPosition{
				MessageResult: msgResult,
				Position:      hit.Source.MessageForwardPosition,
			})
		}
	}

	// sort the messages by message forward position
	for _, messageResultWithPositions := range messageResultWithPositionsMap {
		sort.Slice(messageResultWithPositions, func(i, j int) bool {
			return messageResultWithPositions[i].Position < messageResultWithPositions[j].Position
		})
	}

	for convoName, conversationMessageResult := range conversationMessageResultsMap {
		messageResultWithPositions := messageResultWithPositionsMap[convoName]
		for _, messageResultWithPosition := range messageResultWithPositions {
			conversationMessageResult.MessageResults = append(conversationMessageResult.MessageResults, messageResultWithPosition.MessageResult)
		}
	}

	partialChatStartIndices := map[string][]int{}
	if len(topicMomentsToMatch) > 0 {
		for convoName, messageResultWithPositions := range messageResultWithPositionsMap {
			for i, messageResultWithPosition := range messageResultWithPositions {
				momentResults := messageResultWithPosition.MessageResult.MomentResults
				for _, momentResult := range momentResults {
					if _, exists := topicMomentsToMatch[momentResult.Moment.Name]; exists {
						partialChatStartIndices[convoName] = append(partialChatStartIndices[convoName], i)
						break
					}
				}
			}
		}
		for convoName, conversationMessageResult := range conversationMessageResultsMap {
			partialChatPointers := []*analyticspb.ConversationMessageResult_PartialChatPointer{}
			for _, idx := range partialChatStartIndices[convoName] {
				partialChatPointers = append(partialChatPointers, &analyticspb.ConversationMessageResult_PartialChatPointer{
					StartIdx: int32(idx),
					EndIdx:   int32(min(idx+partialChatLength, len(conversationMessageResult.MessageResults))),
				})
			}
			conversationMessageResult.PartialChatPointers = partialChatPointers
		}
	}

	nextPageToken := ""
	if hits := hits.Hits; len(hits) > 0 && len(hits) >= pageSize {
		ss := []string{}
		for _, val := range hits[len(hits)-1].Sort {
			ss = append(ss, fmt.Sprintf("%v", val))
		}
		nextPageToken = strings.Join(ss, esSortKeySeparator)
	}

	l := []*analyticspb.ConversationMessageResult{}
	for _, r := range conversationMessageResultsMap {
		l = append(l, r)
	}
	return &analyticspb.RetrieveConversationMessagesResponse{
		ConversationMessageResults: l,
		NextPageToken:              nextPageToken,
	}, nil
}
