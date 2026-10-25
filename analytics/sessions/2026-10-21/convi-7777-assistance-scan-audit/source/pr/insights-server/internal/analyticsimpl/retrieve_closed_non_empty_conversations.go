package analyticsimpl

import (
	"context"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	converter "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/converter/retrieve_closed_non_empty_conversations"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/postgres"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (a AnalyticsServiceImpl) RetrieveClosedNonEmptyConversations(
	ctx context.Context, req *analyticspb.RetrieveClosedNonEmptyConversationsRequest,
) (*analyticspb.RetrieveClosedNonEmptyConversationsResponse, error) {
	a.logger.Infof(ctx, "RetrieveClosedNonEmptyConversations request:\n%s", req)

	// Parse the parent to get customerID and profileID
	parent, err := shared.ParseCustomerProfile(req.Parent)
	if err != nil {
		return nil, err
	}

	// Retrieve user ID for requester
	authEntity, err := a.authEntityProvider.GetAuthEntity(ctx, parent.CustomerID)
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error during authentication: %v", err)
	}
	requesterUserID, err := authEntity.GetClaimUserID()
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "error during authentication: %v", err)
	}

	// Convert the request to a clickhouse query
	chQuery, pageSize, offset := converter.ConvertRetrieveClosedNonEmptyConversationsRequestToClickhouseQuery(req, []string{requesterUserID})
	if chQuery == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request: could not generate query")
	}
	a.logger.Infof(ctx, "RetrieveClosedNonEmptyConversations clickhouse query:\n%s", chQuery)

	// Read the conversation IDs from the clickhouse
	conversations, err := a.readConversationIDsFromClickhouse(ctx, chQuery, parent)
	if err != nil {
		return nil, err
	}

	// Read the conversations from the database
	convos, err := postgres.ReadDBConversations(a.appsDB.DB(ctx), a.converter, parent, extractConversationIDs(conversations))
	if err != nil {
		return nil, err
	}

	a.logger.Infof(ctx, "RetrieveClosedNonEmptyConversations found %d conversations", len(convos))

	return converter.ConvertClickhouseResponseToRetrieveClosedNonEmptyConversationsResponse(conversations, parent, pageSize, offset)
}

// extractConversationIDs extracts the keys (conversation IDs) from the map
func extractConversationIDs(conversations []converter.ConversationMessageCountRow) []string {
	keys := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		keys = append(keys, conversation.ConversationID)
	}
	return keys
}
