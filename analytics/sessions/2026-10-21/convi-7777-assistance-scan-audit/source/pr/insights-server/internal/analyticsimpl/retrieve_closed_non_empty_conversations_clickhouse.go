package analyticsimpl

import (
	"context"
	"fmt"

	clickhouse "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/converter/retrieve_closed_non_empty_conversations"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
)

func (a AnalyticsServiceImpl) readConversationIDsFromClickhouse(ctx context.Context, chQuery string, parent *shared.CustomerProfile) ([]clickhouse.ConversationMessageCountRow, error) {
	// Get the clickhouse connection
	conn, err := a.getConversationsClickhouseConn(ctx, parent.CustomerID, parent.ProfileID)
	if err != nil {
		return nil, err
	}

	// Execute the query
	rows, err := clickhouseshared.QueryWithRetry(ctx, conn, chQuery)
	if err != nil {
		return nil, fmt.Errorf("querying clickhouse: %w", err)
	}
	defer rows.Close()

	// Scan the rows into ConversationMessageCountRow structs
	conversations := []clickhouse.ConversationMessageCountRow{}
	for rows.Next() {
		var row clickhouse.ConversationMessageCountRow
		if err := rows.ScanStruct(&row); err != nil {
			return nil, fmt.Errorf("scanning clickhouse row: %w", err)
		}
		conversations = append(conversations, row)
	}

	return conversations, nil
}
