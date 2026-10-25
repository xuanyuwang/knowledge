package analyticsimpl

import (
	"context"
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/require"

	"github.com/cresta/go-servers/shared/clickhouse/registry"
	chtesting "github.com/cresta/go-servers/shared/clickhouse/testing"
)

//go:embed testdata/clickhouse_*_request.sql
var recordedSQLGoldenFiles embed.FS

type sqlGoldenConversationRegistry struct {
	registry.ConversationsClickhouseRegistry
	recording  *chtesting.RecordingConnection
	customerID string
	profileID  string
}

func (r *sqlGoldenConversationRegistry) GetConnection(_ context.Context, customerID, profileID string) (driver.Conn, error) {
	if customerID != r.customerID || profileID != r.profileID {
		return nil, fmt.Errorf("SQL golden connection requested for %s/%s, want %s/%s", customerID, profileID, r.customerID, r.profileID)
	}
	return r.recording, nil
}

func recordConversationSQL(server *AnalyticsServiceImpl, connection driver.Conn, customerID, profileID string) *chtesting.RecordingConnection {
	recording := chtesting.NewRecordingConnection(connection)
	server.clickhouseConvoRegistry = &sqlGoldenConversationRegistry{
		ConversationsClickhouseRegistry: server.clickhouseConvoRegistry,
		recording:                       recording,
		customerID:                      customerID,
		profileID:                       profileID,
	}
	return recording
}

func assertRecordedSQLGolden(t *testing.T, recording *chtesting.RecordingConnection, filename string, expectedArgs []any) {
	t.Helper()
	queries := recording.Queries()
	require.Len(t, queries, 1)
	query := queries[0]
	// IN-list membership is unordered; bind parameter positions remain ordered.
	require.Empty(t, cmp.Diff(expectedArgs, query.Args, cmpopts.SortSlices(func(a, b string) bool { return a < b })), "bind arguments differ")
	golden, err := recordedSQLGoldenFiles.ReadFile("testdata/" + filename)
	require.NoError(t, err)
	rendered := renderRecordedSQL(query.SQL, query.Args)
	t.Logf("Recorded SQL for %s:\n%s\nEnd recorded SQL", filename, rendered)
	require.Empty(t, cmp.Diff(normalizeRecordedSQL(string(golden)), normalizeRecordedSQL(rendered)), "SQL golden differs: %s", filename)
}

func normalizeRecordedSQL(s string) string {
	reg := regexp.MustCompile("[\n\t ]+")
	return reg.ReplaceAllString(s, "")
}

// Only used in unit testing to compare query strings.
func renderRecordedSQL(query string, args []any) string {
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			// Enclose string values in single quotes
			query = strings.Replace(query, "?", fmt.Sprintf("'%v'", strings.ReplaceAll(v, "'", "''")), 1)
		case []string:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("'%v'", strings.ReplaceAll(sliceV, "'", "''")))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		case []int:
			// Replace the first occurrence of "?" with the comma-split elements in the slice
			values := []string{}
			for _, sliceV := range v {
				values = append(values, fmt.Sprintf("%v", sliceV))
			}
			sort.Slice(values, func(i, j int) bool {
				return values[i] < values[j]
			})
			query = strings.Replace(query, "?", strings.Join(values, ","), 1)
		default:
			// For simple types, replace the first occurrence of "?" with the string representation
			query = strings.Replace(query, "?", fmt.Sprintf("%v", v), 1)
		}
	}
	return query
}
