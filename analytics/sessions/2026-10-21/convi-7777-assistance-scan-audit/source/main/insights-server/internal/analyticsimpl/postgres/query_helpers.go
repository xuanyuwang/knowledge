package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"

	"github.com/cresta/go-servers/insights-server/internal/shared"
)

const (
	LeftJoinAppUserOnAppChatsTableQuery string = "LEFT JOIN (" +
		"SELECT app.users.id AS id, " +
		"app.users.user_id AS user_id, " +
		"app.users.full_name AS user_full_name " +
		"FROM app.users) all_users ON all_users.id = app.chats.latest_agent_id "
)

// ParseArgs returns sql.Named values "frequency", "time_zone", and "first_day_of_week_bias".
func ParseArgs(frequency *analyticspb.Frequency, metadata *analyticspb.Metadata) ([]interface{}, error) {
	var args []interface{}
	var isWeekly bool
	freq, err := shared.ParseFrequency(frequency)
	if err != nil {
		return args, err
	}
	if freq == "week" {
		isWeekly = true
	}
	args = append(args, sql.Named("frequency", freq))

	timeZone, err := shared.ParseTimeZone(metadata)
	if err != nil {
		return args, err
	}
	args = append(args, sql.Named("time_zone", flipOffset(timeZone)))

	interval := shared.ParseStartDayOfWeek(metadata, isWeekly)
	args = append(args, sql.Named("first_day_of_week_bias", interval))

	return args, nil
}

func flipOffset(offset string) string {
	switch {
	case strings.HasPrefix(offset, "-"):
		return fmt.Sprintf("+%s", offset[1:])
	case strings.HasPrefix(offset, "+"):
		return fmt.Sprintf("-%s", offset[1:])
	default:
		return offset
	}
}
