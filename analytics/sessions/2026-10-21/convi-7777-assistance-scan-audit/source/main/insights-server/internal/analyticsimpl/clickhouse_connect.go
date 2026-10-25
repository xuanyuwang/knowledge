package analyticsimpl

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Connect to conversation clickhouse cluster
func (a AnalyticsServiceImpl) getConversationsClickhouseConn(ctx context.Context, customerID, profileID string) (driver.Conn, error) {
	return a.clickhouseConvoRegistry.GetConnection(ctx, customerID, profileID)
}

// Connect to events clickhouse cluster
func (a AnalyticsServiceImpl) getEventsClickhouseConn(ctx context.Context, customerID, profileID string) (driver.Conn, error) {
	return a.clickhouseEventRegistry.GetConnection(ctx, customerID, profileID)
}
