package analyticsimpl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
)

type partialConversationsRequest struct {
	Query       *partialConversationsQuery `json:"query,omitempty"`
	SearchAfter *[]string                  `json:"search_after,omitempty"`
	Sort        *[]elasticsearch.Sort      `json:"sort,omitempty"`
}

type partialConversationsQuery struct {
	Bool partialConversationsOuterBool `json:"bool"`
}

type partialConversationsOuterBool struct {
	Should             []partialConversationsShould        `json:"should"`
	Filter             []partialConversationsDocTypeFilter `json:"filter"`
	MustNot            *partialConversationsDocTypeFilter  `json:"must_not,omitempty"`
	MinimumShouldMatch int32                               `json:"minimum_should_match,omitempty"`
}

type partialConversationsShould struct {
	Bool partialConversationsInnerBool `json:"bool"`
}

type partialConversationsDocTypeFilter struct {
	Terms elasticsearch.TermsFilter `json:"terms"`
}

type partialConversationsInnerBool struct {
	Filter []partialConversationsInnerFilter `json:"filter"`
}

type partialConversationsInnerFilter struct {
	Terms *elasticsearch.TermsFilter `json:"terms,omitempty"`
	Range *elasticsearch.RangeFilter `json:"range,omitempty"`
}

func (a AnalyticsServiceImpl) RetrievePartialConversations(
	ctx context.Context, req *analyticspb.RetrievePartialConversationsRequest,
) (*analyticspb.RetrievePartialConversationsResponse, error) {
	a.logger.Debugf(ctx, "RetrievePartialConversations request:\n%s", req)
	if err := validateInput(req); err != nil {
		return nil, err
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
	// This reads specific message windows/lists with no conversation time filter -> all-time reach.
	recordConversationReadRecency(ctx, nil, parent.CustomerID, parent.ProfileID, "retrieve_partial_conversations")

	pageSize := shared.ParsePageSizeAndPopulateDefaultValue(req.PageSize)

	aclUsers, aclGroups, err := shared.ApplyResourceACL(ctx, a.configClient, parent.CustomerID, a.resourceACLHelperProvider.Get(), []*userpb.User{}, []*userpb.Group{}, false)
	if err != nil {
		return nil, err
	}

	resourceACLUsersAndGroups := &analyticspb.Attribute{
		Users:  aclUsers,
		Groups: aclGroups,
	}
	if len(aclGroups) > 0 {
		filterByAttribute, err := shared.MoveFiltersToUserFilter(ctx, a.userClientRegistry, a.internalUserServiceClientRegistry, parent.CustomerID, parent.ProfileID, resourceACLUsersAndGroups, false, true)
		if err != nil {
			return nil, err
		}

		// If empty users after group and deactivated user filter, we will return empty response.
		// This should be rare (should only happen for empty groups).
		if len(filterByAttribute.Users) == 0 {
			return &analyticspb.RetrievePartialConversationsResponse{}, nil
		}
	}

	// We need to do intersection between
	// - 1) returned ACL users; and
	// - 2) expanded users from ACL filtered group filter.
	allFilterByAttributeUsers := a.getAllFilterByAttributeUsers(ctx, parent, resourceACLUsersAndGroups)
	if len(aclUsers) > 0 {
		if len(allFilterByAttributeUsers) > 0 {
			resourceACLUsersAndGroups.Users = shared.FilterUsers(allFilterByAttributeUsers, aclUsers)
			// If no overlap, we will return empty response.
			// This should not happen in production.
			if len(resourceACLUsersAndGroups.Users) == 0 {
				return &analyticspb.RetrievePartialConversationsResponse{}, nil
			}
		} else {
			// If no user filter, only apply ACL user filter.
			resourceACLUsersAndGroups.Users = aclUsers
		}
	}

	// Sort users by name to have deterministic order on agent-user-id filter.
	sort.Slice(resourceACLUsersAndGroups.Users, func(i, j int) bool {
		return resourceACLUsersAndGroups.Users[i].Name < resourceACLUsersAndGroups.Users[j].Name
	})

	if len(resourceACLUsersAndGroups.Users) > elasticsearch.MaxNumberOfUserTerms {
		a.logger.Warnf(ctx, "number of users %d in the filter by attribute exceeds allowed maximum of %d, truncating to max.", len(resourceACLUsersAndGroups.Users), elasticsearch.MaxNumberOfUserTerms)
		resourceACLUsersAndGroups.Users = resourceACLUsersAndGroups.Users[:elasticsearch.MaxNumberOfUserTerms]
	}

	request, err := a.queryESPartialConversationMessages(parent, req.MessageWindows, req.MessageLists, req.PageToken, resourceACLUsersAndGroups.Users)
	if err != nil {
		return nil, err
	}

	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeTrackHit, pageSize, false /*=logESQuery*/, request, &res, "RetrievePartialConversations",
	)
	if err != nil {
		return nil, err
	}

	resp, err := a.convertHitsToPartialConversationsResponse(parent, res.Hits, pageSize)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (a AnalyticsServiceImpl) queryESPartialConversationMessages(
	customerProfile *shared.CustomerProfile,
	messageWindows []*analyticspb.MessageWindow,
	messageLists []*analyticspb.MessageList,
	pageToken string,
	users []*userpb.User,
) (partialConversationsRequest, error) {
	parentQuery := partialConversationsQuery{
		Bool: partialConversationsOuterBool{
			Should: []partialConversationsShould{},
			Filter: []partialConversationsDocTypeFilter{
				{
					Terms: elasticsearch.TermsFilter{
						ConversationToMessageJoin: &[]string{string(elasticsearch.DocumentTypeMessage)},
					},
				},
			},
			MustNot: &partialConversationsDocTypeFilter{
				Terms: elasticsearch.TermsFilter{
					TranslationInfoTranslated: &[]bool{
						true,
					},
				},
			},
			MinimumShouldMatch: 1,
		},
	}

	if len(messageWindows) > 0 {
		if err := appendMessageWindowRanges(&parentQuery, customerProfile, messageWindows); err != nil {
			return partialConversationsRequest{}, err
		}
	}

	if len(messageLists) > 0 {
		if err := appendMessageListIds(&parentQuery, customerProfile, messageLists); err != nil {
			return partialConversationsRequest{}, err
		}
	}

	if len(users) > 0 {
		if err := appendACLUsers(&parentQuery, users); err != nil {
			return partialConversationsRequest{}, err
		}
	}

	// For deep pagination.
	var searchAfter *[]string
	if pageToken != "" {
		sortKeys := strings.Split(pageToken, esSortKeySeparator)
		searchAfter = &sortKeys
	}

	request := partialConversationsRequest{
		Query:       &parentQuery,
		SearchAfter: searchAfter,
		Sort:        elasticsearch.DefaultSortForMessageDoc,
	}
	return request, nil
}

func (a AnalyticsServiceImpl) convertHitsToPartialConversationsResponse(
	customerProfile *shared.CustomerProfile,
	hits elasticsearch.ResHits,
	pageSize int,
) (*analyticspb.RetrievePartialConversationsResponse, error) {
	conversationMessages := make([]*conversationpb.ConversationMessage, 0, len(hits.Hits))

	for _, hit := range hits.Hits {
		messageName := conversationpb.ConversationMessageName{
			CustomerID:     customerProfile.CustomerID,
			ProfileID:      customerProfile.ProfileID,
			ConversationID: hit.Source.ConversationID,
			MessageID:      hit.Source.ConversationMessageID,
		}.String()
		messageText := elasticsearch.MarkHighlightMessageText(hit)
		speakerRole, err := shared.ConvertESToAPISpeakerRole(hit.Source.SpeakerRole)
		if err != nil {
			return nil, err
		}
		messageCreatedAt, err := messageCreateTimeFromESSource(hit)
		if err != nil {
			return nil, err
		}
		sequencePosition := proto.Int32(int32(hit.Source.MessageForwardPosition))

		conversationMessage := &conversationpb.ConversationMessage{
			Name:             messageName,
			Text:             messageText,
			Speaker:          speakerRole,
			CreateTime:       messageCreatedAt,
			SequencePosition: sequencePosition,
		}
		conversationMessages = append(conversationMessages, conversationMessage)
	}

	nextPageToken := ""
	if hits := hits.Hits; len(hits) > 0 && len(hits) >= pageSize {
		ss := []string{}
		for _, val := range hits[len(hits)-1].Sort {
			ss = append(ss, fmt.Sprintf("%v", val))
		}
		nextPageToken = strings.Join(ss, esSortKeySeparator)
	}

	return &analyticspb.RetrievePartialConversationsResponse{
		ConversationMessages: conversationMessages,
		NextPageToken:        nextPageToken,
	}, nil
}

func createMessagePositionFilter(messageWindow *analyticspb.MessageWindow) *partialConversationsInnerFilter {
	messagePositionRange := messageWindow.MessagePositionRange
	if messagePositionRange == nil {
		return nil
	}

	lowerBound := messagePositionRange.LowerBound
	upperBound := messagePositionRange.UpperBound
	direction := messageWindow.MessagePositionDirection
	// Don't add filter if both bounds are null. Retrieve entire conversation instead.
	if lowerBound == nil && upperBound == nil {
		return nil
	}

	messagePositionFilter := &partialConversationsInnerFilter{
		Range: &elasticsearch.RangeFilter{},
	}
	positionRange := &elasticsearch.IntRange{}
	if lowerBound != nil {
		value := lowerBound.Value
		if lowerBound.IsExclusive {
			value += 1
		}
		positionRange.GTE = &value
	}
	if upperBound != nil {
		value := upperBound.Value
		if upperBound.IsExclusive {
			value -= 1
		}
		positionRange.LTE = &value
	}

	if direction == analyticspb.MessageWindow_BACKWARD {
		messagePositionFilter.Range.BackwardPostionRange = positionRange
	} else {
		messagePositionFilter.Range.FowardPositionRange = positionRange
	}

	return messagePositionFilter
}

func validateInput(req *analyticspb.RetrievePartialConversationsRequest) error {
	if len(req.MessageWindows) == 0 && len(req.MessageLists) == 0 {
		return status.Errorf(codes.InvalidArgument, "at least one message window or message list must be specified")
	} else if len(req.MessageWindows) > 0 && len(req.MessageLists) > 0 {
		return status.Errorf(codes.InvalidArgument, "either a message window or a message list must be specified, both are not supported")
	}

	return nil
}

func appendMessageWindowRanges(parentQuery *partialConversationsQuery, customerProfile *shared.CustomerProfile, messageWindows []*analyticspb.MessageWindow) error {
	for _, messageWindow := range messageWindows {
		conversationName, err := conversationpb.ParseConversationName(messageWindow.Conversation)
		if err != nil {
			return err
		}
		if err := validateConversationMatchesCustomerProfile(conversationName, customerProfile); err != nil {
			return err
		}

		childQuery := partialConversationsShould{
			Bool: partialConversationsInnerBool{
				Filter: []partialConversationsInnerFilter{
					{
						Terms: &elasticsearch.TermsFilter{
							ConversationIDs: &[]string{conversationName.ConversationID},
						},
					},
				},
			},
		}

		if messagePositionFilter := createMessagePositionFilter(messageWindow); messagePositionFilter != nil {
			childQuery.Bool.Filter = append(childQuery.Bool.Filter, *messagePositionFilter)
		}
		parentQuery.Bool.Should = append(parentQuery.Bool.Should, childQuery)
	}
	return nil
}

func appendMessageListIds(parentQuery *partialConversationsQuery, customerProfile *shared.CustomerProfile, messageLists []*analyticspb.MessageList) error {
	for _, messageList := range messageLists {
		conversationName, err := conversationpb.ParseConversationName(messageList.Conversation)
		if err != nil {
			return err
		}
		if err := validateConversationMatchesCustomerProfile(conversationName, customerProfile); err != nil {
			return err
		}

		childQuery := partialConversationsShould{
			Bool: partialConversationsInnerBool{
				Filter: []partialConversationsInnerFilter{
					{
						Terms: &elasticsearch.TermsFilter{
							ConversationIDs: &[]string{conversationName.ConversationID},
						},
					},
				},
			},
		}
		if len(messageList.MessageIds) > 0 {
			childQuery.Bool.Filter = append(childQuery.Bool.Filter, partialConversationsInnerFilter{
				Terms: &elasticsearch.TermsFilter{
					ConversationMessageIDs: &messageList.MessageIds,
				},
			})
		}
		parentQuery.Bool.Should = append(parentQuery.Bool.Should, childQuery)
	}
	return nil
}

func appendACLUsers(parentQuery *partialConversationsQuery, users []*userpb.User) error {
	userIDs, err := shared.ConvertUsersToUserIDs(users)
	if err != nil {
		return err
	}
	childQueryFilter := partialConversationsDocTypeFilter{
		Terms: elasticsearch.TermsFilter{
			AgentUserIDs: &userIDs,
		},
	}
	parentQuery.Bool.Filter = append(parentQuery.Bool.Filter, childQueryFilter)
	return nil
}

func validateConversationMatchesCustomerProfile(conversationName *conversationpb.ConversationName, customerProfile *shared.CustomerProfile) error {
	if conversationName.CustomerID != customerProfile.CustomerID ||
		conversationName.ProfileID != customerProfile.ProfileID {
		return status.Errorf(
			codes.InvalidArgument,
			"conversation '%s' does not match the parent's customer or profile ID",
			conversationName.String(),
		)
	}
	return nil
}
