package analyticsimpl

import (
	"context"
	"fmt"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	convopb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/util/envflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
)

var enableDynamicGroupsACL = envflag.Bool("ENABLE_DYNAMIC_GROUPS_ACL", false, "enable dynamic groups ACL")

func (a AnalyticsServiceImpl) RetrieveClosedConversations(
	ctx context.Context, req *analyticspb.RetrieveClosedConversationsRequest,
) (*analyticspb.RetrieveClosedConversationsResponse, error) {
	a.logger.Infof(ctx, "RetrieveClosedConversations request:\n%s", req)

	// Always enable standard pagination
	if req.ElasticsearchQueryConfig == nil {
		req.ElasticsearchQueryConfig = &analyticspb.ElasticsearchQueryConfig{}
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

	attr := req.FilterByAttribute
	if attr == nil {
		return nil, status.Error(codes.InvalidArgument, "`filter_by_attribute` is required.")
	}
	if len(attr.TimeRanges) == 0 {
		return nil, status.Error(codes.InvalidArgument, "`filter_by_attribute.time_ranges` is required.")
	}
	// Read-recency instrumentation for ES hot-window sizing (PII-free; see read_recency_metrics.go).
	recordConversationReadRecency(ctx, attr.TimeRanges[0].GetStartTimestamp(), parent.CustomerID, parent.ProfileID, "retrieve_closed_conversations")
	if len(attr.LanguageCodes) > 0 {
		if err = elasticsearch.ValidateLanguageCode(attr.LanguageCodes); err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}
	lenPolicies := 0
	if len(attr.AttributePolicies) > 0 {
		lenPolicies = len(attr.AttributePolicies)
	}
	if len(attr.ConversationReasons)+len(attr.InsightsTopics)+
		len(attr.Intents)+len(attr.Moments)+lenPolicies > 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"not supported filters: `conversation_reasons`, `insights_topics`, `intents`, "+
				"`moments`, `policies`")
	}

	allFilterByAttributeUsers := a.getAllFilterByAttributeUsers(ctx, parent, req.FilterByAttribute)
	shouldReturnEmptyResponse, err := shared.ApplyResourceACLToAttributeFilter(
		ctx,
		a.configClient,
		a.resourceACLHelperProvider.Get(),
		a.userClientRegistry,
		a.internalUserServiceClientRegistry,
		parent.CustomerID,
		parent.ProfileID,
		req.FilterByAttribute,
		true,
		allFilterByAttributeUsers,
		true, // returnEmptyResponseWhenNoUsersAfterMove
	)
	if err != nil {
		return nil, err
	}
	if shouldReturnEmptyResponse {
		a.logger.Debugf(ctx, "[retrievedClosedConversations] return empty response due to ACL filtered users/groups")
		return &analyticspb.RetrieveClosedConversationsResponse{}, nil
	}

	topicDetails, err := a.expandConversationFilter(ctx, *parent, req.FilterByAttribute)
	if err != nil {
		return nil, err
	}

	durationBuckets := hardcodedConversationDurationBuckets

	pageSize := shared.ParsePageSizeAndPopulateDefaultValue(req.PageSize)

	// For deep pagination.
	var searchAfter *[]string
	if pt := req.PageToken; pt != "" {
		sortKeys := strings.Split(pt, esSortKeySeparator)
		searchAfter = &sortKeys
	}

	if len(req.FilterByAttribute.Users) > elasticsearch.MaxNumberOfUserTerms {
		a.logger.Warnf(ctx, "number of users %d in the filter by attribute exceeds allowed maximum of %d, truncating to max.", len(req.FilterByAttribute.Users), elasticsearch.MaxNumberOfUserTerms)
		req.FilterByAttribute.Users = req.FilterByAttribute.Users[:elasticsearch.MaxNumberOfUserTerms]
	}

	// TODO(zhiyang.lu): remove this after debugging
	a.logger.Debugf(ctx, "[retrievedClosedConversations] finalized filterByAttribute: %+v", req.FilterByAttribute)

	query, err := a.convertRetrieveClosedConversationsRequstToESRequest(
		ctx,
		profileConfig,
		req.Metadata,
		req.FilterByAttribute,
		req.TargetFieldForTimeRange,
		durationBuckets,
		searchAfter,
		topicDetails,
		req.EnableRandomConversationOrder,
		req.SeedForRandomSort,
		req.FilterByAttribute.Users,
	)
	if err != nil {
		return nil, err
	}
	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeTrackTotalCount,
		pageSize, logESQuery(req.ElasticsearchQueryConfig), query, &res, esQueryID(req.ElasticsearchQueryConfig))
	if err != nil {
		return nil, err
	}
	return a.convertToClosedConversationsResponse(ctx, profileConfig, a.appsDB.DB(ctx), parent, req, pageSize, res.Hits)
}

func esQueryID(esQueryConfig *analyticspb.ElasticsearchQueryConfig) string {
	if esQueryConfig == nil {
		return ""
	}
	return esQueryConfig.QueryId
}

func logESQuery(esQueryConfig *analyticspb.ElasticsearchQueryConfig) bool {
	return esQueryConfig.GetLogQuery()
}

func (a AnalyticsServiceImpl) convertRetrieveClosedConversationsRequstToESRequest(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	metadata *analyticspb.Metadata,
	filter *analyticspb.Attribute,
	targetFieldForTimeRangeFilter analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	searchAfter *[]string,
	topicDetails map[string]elasticsearch.TopicDetails,
	randomize bool,
	seedForRandomSort int32,
	users []*userpb.User,
) (elasticsearch.Request, error) {
	query, timeZone, err := a.buildClosedConversationsQuery(ctx, profileConfig, metadata, filter, targetFieldForTimeRangeFilter, durationBuckets, topicDetails, users)
	if err != nil {
		return elasticsearch.Request{}, err
	}

	var request elasticsearch.Request
	if randomize {
		var (
			fieldRandomScore string
			seedRandomScore  int32
			esSort           *[]elasticsearch.Sort
		)
		if seedForRandomSort > 0 {
			fieldRandomScore = "conversation_started_at"
			seedRandomScore = seedForRandomSort
			esSort = elasticsearch.SortByScore
		}
		if searchAfter != nil {
			if len(*searchAfter) != len(*esSort) {
				return elasticsearch.Request{}, status.Errorf(codes.InvalidArgument, "searchAfter %+v from page_token should same number of elements as sortKeys", searchAfter)
			}
		}
		request = elasticsearch.Request{
			Source: &elasticsearch.Source{
				Includes: &[]string{"conversation_id", "moment_annotations", "message_count"},
			},
			Query: &elasticsearch.OuterQuery{
				FunctionScore: &elasticsearch.FunctionScore{
					Query: &query,
					RandomScore: &elasticsearch.RandomScoreFunction{
						Field: fieldRandomScore,
						Seed:  seedRandomScore,
					},
					BoostMode: "replace",
				},
			},
			Sort:        esSort,
			SearchAfter: searchAfter,
		}
	} else {
		esSort := elasticsearch.SecureSortForConversation
		if searchAfter != nil {
			if len(*searchAfter) != len(*esSort) {
				return elasticsearch.Request{}, status.Errorf(codes.InvalidArgument, "searchAfter %+v from page_token should same number of elements as sortKeys", searchAfter)
			}
		}
		request = elasticsearch.Request{
			Source: &elasticsearch.Source{
				Includes: &[]string{"conversation_id", "moment_annotations", "message_count"},
			},
			Query:       &query,
			SearchAfter: searchAfter,
			Sort:        esSort,
		}
	}

	if len(filter.TruncatedTimeRanges) > 0 {
		request.RuntimeMappings = elasticsearch.BuildRuntimeMappingsForTruncatedTimeRangeFilters(timeZone, targetFieldForTimeRangeFilter)
	}

	return request, nil
}

// buildClosedConversationsQuery returns the conversation document query for filter, plus the time zone it was built in.
// filter must already be expanded with expandConversationFilter.
func (a AnalyticsServiceImpl) buildClosedConversationsQuery(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	metadata *analyticspb.Metadata,
	filter *analyticspb.Attribute,
	targetFieldForTimeRangeFilter analyticspb.RetrieveClosedConversationsRequest_TargetFieldForTimeRange,
	durationBuckets []*analyticspb.ConversationDurationBucket,
	topicDetails map[string]elasticsearch.TopicDetails,
	users []*userpb.User,
) (query elasticsearch.OuterQuery, timeZone string, err error) {
	timeZone, err = shared.ParseTimeZone(metadata)
	if err != nil {
		return elasticsearch.OuterQuery{}, "", err
	}

	var userIDs []string
	if *enableDeprecatePostgresUserIDQuery {
		userIDs, err = fn.MapE(users, func(user *userpb.User) (string, error) {
			parsedUserName, err := userpb.ParseUserName(user.Name)
			if err != nil {
				return "", status.Errorf(codes.InvalidArgument, "invalid user name %s", user.Name)
			}
			return parsedUserName.UserID, nil
		})
		if err != nil {
			return elasticsearch.OuterQuery{}, "", err
		}
	} else {
		userIDs, err = postgres.GetUserIDsInDB(a.appsDB.DB(ctx), filter)
		if err != nil {
			return elasticsearch.OuterQuery{}, "", err
		}
	}

	if len(userIDs) > elasticsearch.MaxNumberOfUserTerms {
		a.logger.Warnf(ctx, "number of users %d in the db query exceeds allowed maximum of %d, truncating to max.", len(userIDs), elasticsearch.MaxNumberOfUserTerms)
		userIDs = userIDs[:elasticsearch.MaxNumberOfUserTerms]
	}

	// To support the deprecated `actions` field during migration period.
	if len(filter.Actions) != 0 {
		ag := analyticspb.ActionGroup{}
		for _, a := range filter.Actions {
			ag.Actions = append(ag.Actions, &analyticspb.ActionGroup_ActionRecord{Action: a})
		}
		filter.ActionGroups = append(filter.ActionGroups, &ag)
	}

	convoDocFilter, err := parseConversationFilterParams(
		filter, nil, filter.TimeRanges[0], timeZone, durationBuckets, userIDs)
	if err != nil {
		return elasticsearch.OuterQuery{}, "", err
	}
	convoDocFilter.TargetFieldForTimeRangeFilter = targetFieldForTimeRangeFilter

	esQueryConfig, err := a.getElasticsearchQueryConfig(ctx, profileConfig, nil, nil)
	if err != nil {
		return elasticsearch.OuterQuery{}, "", err
	}
	if hasConversationIDFilter(filter) {
		convoDocFilter = unsetOtherFiltersWhenFilterByConversationID(convoDocFilter)
	}
	convoDocFilter.WorkflowAutomationConditions = filter.GetWorkflowAutomationConditions()

	query, err = elasticsearch.ParseFilterByAttributeToQuery(
		convoDocFilter,
		nil,
		elasticsearch.DocumentTypeConversation,
		topicDetails, esQueryConfig)
	if err != nil {
		return elasticsearch.OuterQuery{}, "", err
	}
	return query, timeZone, nil
}

// expandConversationFilter rewrites the moment and category filters in attr in place into the leaf filters the
// Elasticsearch query builder understands, and returns the topic details the builder needs for them.
func (a AnalyticsServiceImpl) expandConversationFilter(
	ctx context.Context, profile shared.CustomerProfile, attr *analyticspb.Attribute,
) (map[string]elasticsearch.TopicDetails, error) {
	err := shared.UpdateMomentGroupsWithAllBehaviors(attr)
	if err != nil {
		return nil, err
	}

	momentNames, hasCompositeMoment := getInsightsCompositeMoments(attr, nil, nil)
	var topicDetails map[string]elasticsearch.TopicDetails
	if hasCompositeMoment {
		aggregateToLeaves, _, err := a.resolveAggregateMoments(ctx, profile, momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		if len(aggregateToLeaves) > 0 {
			expandAggregateMomentsInAttribute(attr, aggregateToLeaves)
			momentNames, _ = getInsightsCompositeMoments(attr, nil, nil)
		}
		topicDetails, err = a.fetchTopicDetails(ctx, profile.String(), momentNames)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	// Expand V3 category filters to their full subtree so a drill-down on a parent returns every
	// conversation classified anywhere beneath it.
	if _, err = a.applyTopicDiscoveryV3Expansion(
		ctx,
		profile,
		[]*analyticspb.Attribute{attr},
		nil, // no group-by when only filtering conversations
	); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return topicDetails, nil
}

// Return true when attribute has filter on specific conversations.
func hasConversationIDFilter(attribute *analyticspb.Attribute) bool {
	if attribute != nil && (len(attribute.Conversations) > 0 || len(attribute.ConversationIdentifiers) > 0) {
		return true
	}
	return false
}

func unsetOtherFiltersWhenFilterByConversationID(convoDocFilter *elasticsearch.ConversationFilterParams) *elasticsearch.ConversationFilterParams {
	return &elasticsearch.ConversationFilterParams{
		ConversationIDs:         convoDocFilter.ConversationIDs,
		PlatformConversationIDs: convoDocFilter.PlatformConversationIDs,
		// Keep usecase ids filter because there exists identical platform_conversation_id in difference usecases.
		UsecaseIDs: convoDocFilter.UsecaseIDs,
		// Keep agent id to ensure ACL is not vialoted.
		AgentUserIDs: convoDocFilter.AgentUserIDs,
	}
}

func (a AnalyticsServiceImpl) convertToClosedConversationsResponse(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	db *gorm.DB,
	customerProfile *shared.CustomerProfile,
	req *analyticspb.RetrieveClosedConversationsRequest,
	pageSize int, hits elasticsearch.ResHits,
) (*analyticspb.RetrieveClosedConversationsResponse, error) {
	metadataMomentGroups := getMetadataMomentGroups(req.FilterByAttribute)
	enabled := false
	if len(metadataMomentGroups) > 0 {
		var err error
		enabled, err = a.matchConversationsWithLatestMetadataMomentAnnotation(profileConfig)
		if err != nil {
			return nil, err
		}
		if enabled {
			a.logger.Infof(ctx, "enabling matching convo with latest metadata value")
		}
	}

	hitsByConvoID := map[string]elasticsearch.ResHit{}
	nextPageToken := ""
	if hits := hits.Hits; len(hits) > 0 && len(hits) == pageSize {
		ss := []string{}
		for _, val := range hits[len(hits)-1].Sort {
			ss = append(ss, fmt.Sprintf("%v", val))
		}
		nextPageToken = strings.Join(ss, esSortKeySeparator)
	}
	for _, hit := range hits.Hits {
		// Do not further filter conversations with latest metdata value when conversation filter is present.
		if enabled && !hasConversationIDFilter(req.FilterByAttribute) {
			matched, err := matchMetadataMoments(hit.Source.MomentAnnotations, metadataMomentGroups)
			if err != nil {
				return nil, err
			}
			if !matched {
				a.logger.Infof(ctx, "Conversation %q is skipped due to latest metatdata value not match", hit.Source.ConversationID)
				continue
			}
		}
		hitsByConvoID[hit.Source.ConversationID] = hit
	}
	convoIDs := fn.MapKeyToSlice(hitsByConvoID)
	if req.OnlyPopulateConversationName {
		convos := []*convopb.Conversation{}
		for _, id := range convoIDs {
			convos = append(convos, &convopb.Conversation{
				Name: convopb.ConversationName{
					CustomerID:     customerProfile.CustomerID,
					ProfileID:      customerProfile.ProfileID,
					ConversationID: id,
				}.String(),
			})
		}
		return &analyticspb.RetrieveClosedConversationsResponse{
			Conversations:          convos,
			TotalConversationCount: int32(hits.Total.Value),
			NextPageToken:          nextPageToken,
		}, nil
	}
	convos, err := postgres.ReadDBConversations(db, a.converter, customerProfile, convoIDs)
	if err != nil {
		return nil, err
	}
	for _, convo := range convos {
		conversationName, err := convopb.ParseConversationName(convo.Name)
		if err != nil {
			return nil, err
		}
		convoHit, ok := hitsByConvoID[conversationName.ConversationID]
		if !ok {
			continue
		}
		convo.ExtendedInfo = &convopb.Conversation_ExtendedInfo{
			MessageCount: int32(convoHit.Source.MessageCount),
		}
	}
	return &analyticspb.RetrieveClosedConversationsResponse{
		Conversations:          convos,
		TotalConversationCount: int32(hits.Total.Value),
		NextPageToken:          nextPageToken,
	}, nil
}

func matchMetadataMoments(esMomentAnnotations []elasticsearch.ResMomentAnnotation, metadataMomentGroups []*analyticspb.MomentGroup) (bool, error) {
	matched := func(apiMetadataValues []*analyticspb.MetadataValueAttribute, esMetadataValues []elasticsearch.ResMetadataValue) bool {
		// When metadata_values is not provided, we only match by metadata moment template id.
		if len(apiMetadataValues) == 0 {
			return true
		}
		if len(esMetadataValues) == 0 {
			return true
		}

		for _, apiVal := range apiMetadataValues {
			// For numeric metadata.
			if bin := apiVal.NumericBin; bin != nil {
				fromVal, toVal := bin.FromValue, bin.ToValue
				for _, esMetadataValue := range esMetadataValues {
					esNumber := esMetadataValue.NumberValue

					fromMatched, toMatched := false, false
					if bin.FromValueIsExclusive {
						fromMatched = esNumber > fromVal
					} else {
						fromMatched = esNumber >= fromVal
					}
					if bin.ToValueIsInclusive {
						toMatched = esNumber <= toVal
					} else {
						toMatched = esNumber < toVal
					}

					if fromMatched && toMatched {
						return true
					}
				}
			}

			// For categorical metadata.
			if apiVal := apiVal.MetadataValue; apiVal != nil {
				for _, esMetadataValue := range esMetadataValues {
					switch val := apiVal.Value.(type) {
					case *metadata.MetadataValue_StringValue:
						if val.StringValue == esMetadataValue.StringValue {
							return true
						}
					case *metadata.MetadataValue_NumberValue:
						if val.NumberValue == esMetadataValue.NumberValue {
							return true
						}
					case *metadata.MetadataValue_BooleanValue:
						if val.BooleanValue == esMetadataValue.BooleanValue {
							return true
						}
					}
				}
			}
		}
		return false
	}

	momentIDToLatestMetadataValues := map[string][]elasticsearch.ResMetadataValue{}
	for _, ma := range esMomentAnnotations {
		momentIDToLatestMetadataValues[ma.MomentTemplateID] = ma.Payload.ConversationMetadataPayload.Values
	}

	// Every metadata moment filter has to be matched.
	for _, mg := range metadataMomentGroups {
		// only evaluate if there are moments to match in the moment groups
		if len(mg.Moments) > 0 {
			name, err := momentpb.ParseMomentName(mg.Moments[0].Name)
			if err != nil {
				return false, err
			}

			if name.MomentID != "*" {
				if !matched(mg.MetadataValueAttributes, momentIDToLatestMetadataValues[name.MomentID]) {
					return false, nil
				}
			} else {
				// Handle wildcard moment.
				foundMatched := false
				for _, v := range momentIDToLatestMetadataValues {
					if matched(mg.MetadataValueAttributes, v) {
						foundMatched = true
					}
				}
				if !foundMatched {
					return false, nil
				}
			}
		}
	}
	return true, nil
}

var includeEmptyConversations = envflag.String("INCLUDE_EMPTY_CONVERSATIONS_IN_ELASTICSEARCH", "false", "control whether to include empty conversations in ES query")

func (a AnalyticsServiceImpl) includeEmptyConversations(ctx context.Context, profileConfig *config.ProfileConfig) (bool, error) {
	if *includeEmptyConversations == TrueString {
		return true, nil
	}
	return profileConfig.GetFeatures().GetInsights().GetIncludeEmptyConversationInElasticsearchQuery(), nil
}

func (a AnalyticsServiceImpl) getElasticsearchQueryConfig(ctx context.Context, profileConfig *config.ProfileConfig, esQueryConfig *analyticspb.ElasticsearchQueryConfig, filterConversationDocumentByConversationByAttribute *analyticspb.Attribute) (elasticsearch.QueryConfig, error) {
	ret := elasticsearch.QueryConfig{}
	if esQueryConfig != nil {
		if esQueryConfig.SkipConversationMessageCount {
			ret.SkipConversationMessageCount = true
		}
	}
	ret.EsDocumentType = elasticsearch.DocumentTypeMessage
	if filterConversationDocumentByConversationByAttribute != nil {
		ret.EsDocumentType = elasticsearch.DocumentTypeConversation
	}

	includeEmptyConvo, err := a.includeEmptyConversations(ctx, profileConfig)
	if err != nil {
		return elasticsearch.QueryConfig{}, err
	}
	ret.IncludeEmptyConversations = includeEmptyConvo
	ret.EnableNumHoldsField = profileConfig.GetFeatures().GetInsights().GetEnableNumHoldsFieldForHoldCountFilter()

	return ret, nil
}

func (a AnalyticsServiceImpl) matchConversationsWithLatestMetadataMomentAnnotation(profileConfig *config.ProfileConfig) (bool, error) {
	return profileConfig.GetFeatures().GetInsights().GetMatchConversationsWithLatestMomentAnnotation(), nil
}

func (a AnalyticsServiceImpl) getAllFilterByAttributeUsers(ctx context.Context, parent *shared.CustomerProfile, filterByAttribute *analyticspb.Attribute) []*userpb.User {
	var users []*userpb.User
	if len(filterByAttribute.Users) > 0 {
		users = append(users, filterByAttribute.Users...)
	} else {
		users = []*userpb.User{}
	}
	if *enableDynamicGroupsACL {
		userClient, userClientErr := a.userClientRegistry.CreateClient(parent.CustomerID, parent.ProfileID)
		if userClientErr != nil {
			a.logger.Warnf(ctx, "failed to create user client and could not expand users from dynamic groups: %v", userClientErr)
		} else {
			expandedUsersList, listUsersErr := shared.ListUsersFromDynamicGroups(ctx, userClient, parent.CustomerID, parent.ProfileID, filterByAttribute.DynamicGroups)
			if listUsersErr != nil {
				a.logger.Warnf(ctx, "failed to expand users from dynamic groups: %v", listUsersErr)
			}
			if len(expandedUsersList) > 0 {
				users = append(users, expandedUsersList...)
			}
		}
	}
	return users
}
