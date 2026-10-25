package analyticsimpl

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/cresta/go-servers/insights-server/internal/shared"

	clickhouse "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/converter/retrieve_closed_non_empty_conversations"
	clickhouseConvoTesting "github.com/cresta/go-servers/shared/clickhouse/conversations/testing"
)

// CustomMockRows extends the existing MockRows to support ScanStruct
type CustomMockRows struct {
	*clickhouseConvoTesting.MockRows
	data   []clickhouse.ConversationMessageCountRow
	cursor int
}

func NewCustomMockRows(data []clickhouse.ConversationMessageCountRow) *CustomMockRows {
	return &CustomMockRows{
		MockRows: clickhouseConvoTesting.NewMockRows(nil),
		data:     data,
		cursor:   0,
	}
}

func (m *CustomMockRows) Next() bool {
	return m.cursor < len(m.data)
}

func (m *CustomMockRows) ScanStruct(dest interface{}) error {
	if m.cursor >= len(m.data) {
		return fmt.Errorf("no more rows")
	}

	// Use reflection to copy the data
	destVal := reflect.ValueOf(dest).Elem()
	srcVal := reflect.ValueOf(m.data[m.cursor])

	destVal.Set(srcVal)
	m.cursor++
	return nil
}

func (m *CustomMockRows) Close() error {
	return nil
}

// ErrorMockRows is a mock that always returns errors on ScanStruct
type ErrorMockRows struct {
	*clickhouseConvoTesting.MockRows
	called bool
}

func NewErrorMockRows() *ErrorMockRows {
	return &ErrorMockRows{
		MockRows: clickhouseConvoTesting.NewMockRows(nil),
		called:   false,
	}
}

func (m *ErrorMockRows) Next() bool {
	if m.called {
		return false
	}
	m.called = true
	return true
}

func (m *ErrorMockRows) ScanStruct(dest interface{}) error {
	return errors.New("scan error")
}

type ReadConversationIDsFromClickhouseTestSuite struct {
	AnalyticsV2TestSuite
	analyticsService *AnalyticsServiceImpl
}

func TestReadConversationIDsFromClickhouse(t *testing.T) {
	suite.Run(t, &ReadConversationIDsFromClickhouseTestSuite{})
}

func (s *ReadConversationIDsFromClickhouseTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
	s.UseMockedClickhouseConnections()

	// Create the analytics service implementation for accessing internal methods
	s.analyticsService = &AnalyticsServiceImpl{
		appsDB:                      s.DBTestSuite.AppsDBRegistry(),
		appEnv:                      "test",
		logger:                      logger.GetLogger(),
		clickhouseConvoRegistry:     s.ClickhouseContainerTestSuite.GetConversationTestRegistry(),
		clickhouseEventRegistry:     s.ClickhouseContainerTestSuite.GetEventsTestRegistry(),
		clickhouseAuthEventRegistry: s.ClickhouseContainerTestSuite.GetAuthEventsTestRegistry(),
	}
}

func (s *ReadConversationIDsFromClickhouseTestSuite) TestReadConversationIDsFromClickhouse_Success() {
	// Prepare test data
	testData := []clickhouse.ConversationMessageCountRow{
		{ConversationID: "conv1", ConversationStartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), ConversationEndTime: time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), MessageCount: 5},
		{ConversationID: "conv2", ConversationStartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), ConversationEndTime: time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), MessageCount: 10},
		{ConversationID: "conv3", ConversationStartTime: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), ConversationEndTime: time.Date(2023, 1, 1, 1, 0, 0, 0, time.UTC), MessageCount: 3},
	}

	// Setup mock
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())
	mockRows := NewCustomMockRows(testData)
	s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Execute
	parent := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	result, err := s.analyticsService.readConversationIDsFromClickhouse(s.ctx, "SELECT * FROM test", parent)

	// Verify
	s.Require().NoError(err)
	s.Require().Len(result, 3)
	s.Equal(testData[0].ConversationID, result[0].ConversationID)
	s.Equal(testData[0].ConversationStartTime, result[0].ConversationStartTime)
	s.Equal(testData[0].ConversationEndTime, result[0].ConversationEndTime)
	s.Equal(testData[0].MessageCount, result[0].MessageCount)
	s.Equal(testData[1].ConversationID, result[1].ConversationID)
	s.Equal(testData[1].ConversationStartTime, result[1].ConversationStartTime)
	s.Equal(testData[1].ConversationEndTime, result[1].ConversationEndTime)
	s.Equal(testData[1].MessageCount, result[1].MessageCount)
	s.Equal(testData[2].ConversationID, result[2].ConversationID)
}

func (s *ReadConversationIDsFromClickhouseTestSuite) TestReadConversationIDsFromClickhouse_EmptyResult() {
	// Setup mock with empty data
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())
	mockRows := NewCustomMockRows([]clickhouse.ConversationMessageCountRow{})
	s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Execute
	parent := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	result, err := s.analyticsService.readConversationIDsFromClickhouse(s.ctx, "SELECT * FROM test", parent)

	// Verify
	s.Require().NoError(err)
	s.Require().Len(result, 0)
	s.NotNil(result) // Should return empty map, not nil
}

func (s *ReadConversationIDsFromClickhouseTestSuite) TestReadConversationIDsFromClickhouse_ScanError() {
	// Create a mock that returns scan error
	defer s.convoMockConn.AssertExpectationsAndReset(s.T())

	// Create a mock rows that will return scan error
	mockRows := NewErrorMockRows()

	s.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)

	// Execute
	parent := &shared.CustomerProfile{
		CustomerID: s.customerID,
		ProfileID:  s.profileID,
	}
	result, err := s.analyticsService.readConversationIDsFromClickhouse(s.ctx, "SELECT * FROM test", parent)

	// Verify
	s.Require().Error(err)
	s.Contains(err.Error(), "scanning clickhouse row")
	s.Contains(err.Error(), "scan error")
	s.Nil(result)
}

// MockConversationRegistry is a mock implementation for testing connection errors
type MockConversationRegistry struct {
	shouldFail bool
}

func (m *MockConversationRegistry) GetConnection(ctx context.Context, customerID, profileID string) (driver.Conn, error) {
	if m.shouldFail {
		return nil, errors.New("connection failed")
	}
	return nil, nil
}

func (m *MockConversationRegistry) GetShardedConnection(ctx context.Context, customerID, profileID string, shardID int) (driver.Conn, error) {
	if m.shouldFail {
		return nil, errors.New("sharded connection failed")
	}
	return nil, nil
}
