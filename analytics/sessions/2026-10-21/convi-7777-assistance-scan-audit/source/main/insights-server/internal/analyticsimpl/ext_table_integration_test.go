package analyticsimpl

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/ext"
	"github.com/stretchr/testify/suite"

	clickhouseshared "github.com/cresta/go-servers/shared/clickhouse/shared"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
)

const (
	testCustomerID = "test_customer"
	testProfileID  = "test_profile"
)

type ExtTableIntegrationTestSuite struct {
	suite.Suite
	chtesting.ClickhouseContainerTestSuite

	testSeqNum int
}

func (s *ExtTableIntegrationTestSuite) SetupSuite() {
	s.ClickhouseContainerTestSuite.SetupSuite(s.T())
}

func (s *ExtTableIntegrationTestSuite) TearDownSuite() {
	s.ClickhouseContainerTestSuite.TearDownSuite(s.T())
}

// seedConversations inserts rows with prefixed user IDs to isolate test data.
// Returns the prefixed user IDs actually inserted.
func (s *ExtTableIntegrationTestSuite) seedConversations(agentUserIDs []string) []string {
	s.testSeqNum++
	prefix := fmt.Sprintf("t%d_", s.testSeqNum)

	ctx := context.Background()
	conn := s.GetConversationConnection(testCustomerID, testProfileID)

	batch, err := clickhouseshared.PrepareBatchWithRetry(ctx, conn,
		fmt.Sprintf("INSERT INTO conversation_d (%s)", clickhouseshared.ExtractCHFields(clickhouseshared.ConversationRow{})))
	s.Require().NoError(err)

	now := time.Now()
	prefixed := make([]string, len(agentUserIDs))
	for i, userID := range agentUserIDs {
		prefixed[i] = prefix + userID
		row := &clickhouseshared.ConversationRow{
			AgentUserID:           prefixed[i],
			ConversationID:        fmt.Sprintf("%sconv_%d", prefix, i),
			ConversationStartTime: now.Add(-time.Duration(i) * time.Hour),
			ConversationEndTime:   now.Add(-time.Duration(i)*time.Hour + 30*time.Minute),
			CustomerID:            testCustomerID,
			ProfileID:             testProfileID,
			CreateTime:            now,
			UpdateTime:            now,
		}
		s.Require().NoError(batch.AppendStruct(row))
	}
	s.Require().NoError(clickhouseshared.SendBatchWithRetry(ctx, batch))
	return prefixed
}

func (s *ExtTableIntegrationTestSuite) TestBuildUsersExtTable() {
	type testCase struct {
		name      string
		input     []string
		wantNil   bool
		wantError bool
	}

	tests := []testCase{
		{
			name:    "nil input returns nil table",
			input:   nil,
			wantNil: true,
		},
		{
			name:    "empty slice returns nil table",
			input:   []string{},
			wantNil: true,
		},
		{
			name:  "single ID",
			input: []string{"user1"},
		},
		{
			name:  "multiple IDs",
			input: []string{"user1", "user2", "user3"},
		},
		{
			name:  "large list",
			input: makeUserIDs(500),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			table, err := buildUsersExtTable(tc.input, extTableAgentFilter, extTableAgentFilterColumn)
			if tc.wantError {
				s.Require().Error(err)
				return
			}
			s.Require().NoError(err)
			if tc.wantNil {
				s.Nil(table)
			} else {
				s.NotNil(table)
			}
		})
	}
}

func (s *ExtTableIntegrationTestSuite) TestAttachExtTablesToContext() {
	type testCase struct {
		name        string
		tables      []*ext.Table
		wantSameCtx bool
	}

	nonNilTable, err := ext.NewTable("test_table", ext.Column("id", "String"))
	s.Require().NoError(err)

	tests := []testCase{
		{
			name:        "no tables",
			tables:      nil,
			wantSameCtx: true,
		},
		{
			name:        "all nil tables",
			tables:      []*ext.Table{nil, nil},
			wantSameCtx: true,
		},
		{
			name:        "one non-nil table",
			tables:      []*ext.Table{nonNilTable},
			wantSameCtx: false,
		},
		{
			name:        "mix of nil and non-nil",
			tables:      []*ext.Table{nil, nonNilTable, nil},
			wantSameCtx: false,
		},
	}

	ctx := context.Background()
	for _, tc := range tests {
		s.Run(tc.name, func() {
			result := attachExtTablesToContext(ctx, tc.tables...)
			if tc.wantSameCtx {
				s.Equal(ctx, result)
			} else {
				s.NotEqual(ctx, result)
			}
		})
	}
}

func (s *ExtTableIntegrationTestSuite) TestExtTableQuery() {
	type testCase struct {
		name          string
		seedUsers     []string
		filterIndexes []int // indexes into the prefixed seedUsers to filter
		wantCount     int
		checkExactIDs bool
	}

	tests := []testCase{
		{
			name:          "filter subset of users",
			seedUsers:     []string{"agent_1", "agent_2", "agent_3", "agent_4", "agent_5"},
			filterIndexes: []int{0, 2, 4},
			wantCount:     3,
			checkExactIDs: true,
		},
		{
			name:          "filter single user",
			seedUsers:     []string{"agent_a", "agent_b", "agent_c"},
			filterIndexes: []int{1},
			wantCount:     1,
			checkExactIDs: true,
		},
		{
			name:          "filter all users",
			seedUsers:     []string{"agent_x", "agent_y"},
			filterIndexes: []int{0, 1},
			wantCount:     2,
			checkExactIDs: true,
		},
		{
			name:          "filter no matching users returns empty",
			seedUsers:     []string{"agent_1", "agent_2"},
			filterIndexes: nil, // we'll use non-existent IDs
			wantCount:     0,
		},
		{
			name:          "10k users ext table",
			seedUsers:     makeUserIDs(10000),
			filterIndexes: makeEvenIndexes(10000),
			wantCount:     5000,
		},
	}

	ctx := context.Background()
	conn := s.GetConversationConnection(testCustomerID, testProfileID)
	extQuery := fmt.Sprintf(
		"SELECT agent_user_id FROM conversation_d WHERE agent_user_id IN (SELECT %s FROM %s) ORDER BY agent_user_id",
		extTableAgentFilterColumn, extTableAgentFilter,
	)

	for _, tc := range tests {
		s.Run(tc.name, func() {
			prefixed := s.seedConversations(tc.seedUsers)

			var filterUsers []string
			if tc.filterIndexes != nil {
				filterUsers = make([]string, len(tc.filterIndexes))
				for i, idx := range tc.filterIndexes {
					filterUsers[i] = prefixed[idx]
				}
			} else {
				filterUsers = []string{"nonexistent_1", "nonexistent_2"}
			}

			extTable, err := buildUsersExtTable(filterUsers, extTableAgentFilter, extTableAgentFilterColumn)
			s.Require().NoError(err)
			extCtx := attachExtTablesToContext(ctx, extTable)

			rows, err := clickhouseshared.QueryWithRetry(extCtx, conn, extQuery)
			s.Require().NoError(err)
			defer rows.Close()

			var results []string
			for rows.Next() {
				var id string
				s.Require().NoError(rows.Scan(&id))
				results = append(results, id)
			}
			s.Require().NoError(rows.Err())

			s.Len(results, tc.wantCount)
			if tc.checkExactIDs {
				s.ElementsMatch(filterUsers, results)
			}
		})
	}
}

func (s *ExtTableIntegrationTestSuite) TestExtTableMatchesInClause() {
	type testCase struct {
		name          string
		filterIndexes []int
	}

	tests := []testCase{
		{
			name:          "subset filter",
			filterIndexes: []int{0, 2, 4},
		},
		{
			name:          "single filter",
			filterIndexes: []int{1},
		},
		{
			name:          "all users",
			filterIndexes: []int{0, 1, 2, 3, 4},
		},
	}

	prefixed := s.seedConversations([]string{"agent_1", "agent_2", "agent_3", "agent_4", "agent_5"})

	ctx := context.Background()
	conn := s.GetConversationConnection(testCustomerID, testProfileID)

	for _, tc := range tests {
		s.Run(tc.name, func() {
			filterUsers := make([]string, len(tc.filterIndexes))
			for i, idx := range tc.filterIndexes {
				filterUsers[i] = prefixed[idx]
			}

			// Query using IN clause
			placeholders := ""
			inArgs := make([]any, len(filterUsers))
			for i, u := range filterUsers {
				if i > 0 {
					placeholders += ", "
				}
				placeholders += "?"
				inArgs[i] = u
			}
			inQuery := fmt.Sprintf(
				"SELECT agent_user_id FROM conversation_d WHERE agent_user_id IN (%s) ORDER BY agent_user_id", //nolint:sqlinjection
				placeholders,
			)
			rowsIn, err := clickhouseshared.QueryWithRetry(ctx, conn, inQuery, inArgs...)
			s.Require().NoError(err)
			defer rowsIn.Close()

			var inResults []string
			for rowsIn.Next() {
				var id string
				s.Require().NoError(rowsIn.Scan(&id))
				inResults = append(inResults, id)
			}
			s.Require().NoError(rowsIn.Err())

			// Query using ext table
			extTable, err := buildUsersExtTable(filterUsers, extTableAgentFilter, extTableAgentFilterColumn)
			s.Require().NoError(err)
			extCtx := attachExtTablesToContext(ctx, extTable)

			extQuery := fmt.Sprintf(
				"SELECT agent_user_id FROM conversation_d WHERE agent_user_id IN (SELECT %s FROM %s) ORDER BY agent_user_id",
				extTableAgentFilterColumn, extTableAgentFilter,
			)
			rowsExt, err := clickhouseshared.QueryWithRetry(extCtx, conn, extQuery)
			s.Require().NoError(err)
			defer rowsExt.Close()

			var extResults []string
			for rowsExt.Next() {
				var id string
				s.Require().NoError(rowsExt.Scan(&id))
				extResults = append(extResults, id)
			}
			s.Require().NoError(rowsExt.Err())

			s.Equal(inResults, extResults)
		})
	}
}

func (s *ExtTableIntegrationTestSuite) TestExtTableEmptyFilter() {
	s.seedConversations([]string{"agent_a", "agent_b"})

	ctx := context.Background()

	table, err := buildUsersExtTable([]string{}, extTableAgentFilter, extTableAgentFilterColumn)
	s.Require().NoError(err)
	s.Nil(table)

	extCtx := attachExtTablesToContext(ctx, table)
	s.Equal(ctx, extCtx)
}

// helpers

func makeUserIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("user_%03d", i)
	}
	return ids
}

func makeEvenIndexes(total int) []int {
	indexes := make([]int, total/2)
	for i := range indexes {
		indexes[i] = i * 2
	}
	return indexes
}

func (s *ExtTableIntegrationTestSuite) TestExtTableVsInClausePerformance() {
	if os.Getenv("RUN_PERF_TESTS") == "" {
		s.T().Skip("Skipping performance test; set RUN_PERF_TESTS=1 to run")
	}
	// Seed a large dataset once, then benchmark both approaches at different filter sizes.
	allUsers := makeUserIDs(10000)
	prefixed := s.seedConversations(allUsers)

	ctx := context.Background()
	conn := s.GetConversationConnection(testCustomerID, testProfileID)

	type testCase struct {
		name       string
		filterSize int
	}

	tests := []testCase{
		{name: "5 users", filterSize: 5},
		{name: "50 users", filterSize: 50},
		{name: "500 users", filterSize: 500},
		{name: "2000 users", filterSize: 2000},
		{name: "5000 users", filterSize: 5000},
		{name: "10000 users", filterSize: 10000},
	}

	for _, tc := range tests {
		filterUsers := prefixed[:tc.filterSize]

		// Warmup + measure: ext table approach
		s.Run(tc.name, func() {
			const iterations = 5

			// ext table approach
			var extDurations []time.Duration
			for i := 0; i < iterations; i++ {
				extTable, err := buildUsersExtTable(filterUsers, extTableAgentFilter, extTableAgentFilterColumn)
				s.Require().NoError(err)
				extCtx := attachExtTablesToContext(ctx, extTable)

				extQuery := fmt.Sprintf(
					"SELECT count() FROM conversation_d WHERE agent_user_id IN (SELECT %s FROM %s)",
					extTableAgentFilterColumn, extTableAgentFilter,
				)
				start := time.Now()
				rows, err := clickhouseshared.QueryWithRetry(extCtx, conn, extQuery)
				s.Require().NoError(err)
				s.Require().True(rows.Next())
				var count uint64
				s.Require().NoError(rows.Scan(&count))
				rows.Close()
				extDurations = append(extDurations, time.Since(start))
				s.Equal(uint64(tc.filterSize), count)
			}

			// IN clause approach
			var inDurations []time.Duration
			for i := 0; i < iterations; i++ {
				placeholders := make([]string, len(filterUsers))
				inArgs := make([]any, len(filterUsers))
				for j, u := range filterUsers {
					placeholders[j] = "?"
					inArgs[j] = u
				}
				inQuery := fmt.Sprintf(
					"SELECT count() FROM conversation_d WHERE agent_user_id IN (%s)", //nolint:sqlinjection
					strings.Join(placeholders, ", "),
				)
				start := time.Now()
				rows, err := clickhouseshared.QueryWithRetry(ctx, conn, inQuery, inArgs...)
				s.Require().NoError(err)
				s.Require().True(rows.Next())
				var count uint64
				s.Require().NoError(rows.Scan(&count))
				rows.Close()
				inDurations = append(inDurations, time.Since(start))
				s.Equal(uint64(tc.filterSize), count)
			}

			extAvg := avgDuration(extDurations)
			inAvg := avgDuration(inDurations)
			s.T().Logf("[%s] ext table avg: %v, IN clause avg: %v, ratio (ext/in): %.2f",
				tc.name, extAvg, inAvg, float64(extAvg)/float64(inAvg))
		})
	}
}

func avgDuration(durations []time.Duration) time.Duration {
	if len(durations) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range durations {
		total += d
	}
	return total / time.Duration(len(durations))
}

func TestExtTableIntegration(t *testing.T) {
	suite.Run(t, &ExtTableIntegrationTestSuite{})
}
