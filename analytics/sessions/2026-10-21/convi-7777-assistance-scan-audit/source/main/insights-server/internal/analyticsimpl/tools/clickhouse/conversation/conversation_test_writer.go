/*
To run the script with command
	- `bazel run //insights-server/internal/analyticsimpl/tools/clickhouse/conversation:clickhouse` or
	- `go run insights-server/internal/analyticsimpl/tools/clickhouse/conversation/clickhouse.go`
*/

package main

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ConversationRow struct {
	AgentUserID           string    `ch:"agent_user_id"`
	ConversationEndTime   time.Time `ch:"conversation_end_time"`
	ConversationID        string    `ch:"conversation_id"`
	ConversationSource    int8      `ch:"conversation_source"`
	ConversationStartTime time.Time `ch:"conversation_start_time"`
	CreateTime            time.Time `ch:"create_time"`
	CustomerID            string    `ch:"customer_id"`
	LanguageCode          string    `ch:"language_code"`
	PlatformAccountID     string    `ch:"platform_account_id"`
	PlatformChatID        string    `ch:"platform_chat_id"`
	PlatformID            string    `ch:"platform_id"`
	ProfileID             string    `ch:"profile_id"`
	RaiseHandState        int8      `ch:"raise_hand_state"`
	UpdateTime            time.Time `ch:"update_time"`
	UsecaseID             string    `ch:"usecase_id"`
}

type ConversationSubRow struct {
	ConversationID string    `ch:"conversation_id"`
	CreateTime     time.Time `ch:"create_time"`
	CustomerID     string    `ch:"customer_id"`
	ProfileID      string    `ch:"profile_id"`
	UpdateTime     time.Time `ch:"update_time"`
}

func main() {
	conn, err := connect()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		log.Printf("The batch %d", i)
		if err := writeConversation(ctx, conn, i); err != nil {
			panic(err)
		}
	}
	fmt.Println("done")
}

func connect() (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse-conversation.us-west-2-staging.internal.cresta.ai:9000"},
		// Addr: []string{
		// 	"localhost:9000",
		// 	"localhost:9001",
		// },
		Auth: clickhouse.Auth{
			Database: "oportun_us_west_2",
			Username: "admin",
			Password: "",
		},
		ClientInfo: clickhouse.ClientInfo{
			Products: []struct {
				Name    string
				Version string
			}{
				{Name: "an-example-go-client", Version: "0.1"},
			},
		},
	})

	if err != nil {
		return nil, err
	}

	if err := conn.Ping(context.Background()); err != nil {
		if exception, ok := err.(*clickhouse.Exception); ok {
			fmt.Printf("Exception [%d] %s \n%s\n", exception.Code, exception.Message, exception.StackTrace)
		}
		return nil, err
	}
	v, _ := conn.ServerVersion()
	fmt.Println(v.String())
	return conn, nil
}

func writeConversation(ctx context.Context, conn driver.Conn, i int) error {
	batch, err := conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO conversation_d (%s)", extractCHFields(ConversationSubRow{})))
	if err != nil {
		return err
	}

	err = batch.AppendStruct(&ConversationSubRow{
		ConversationID: fmt.Sprintf("conversation_%d", i),
		CreateTime:     time.Now(),
		CustomerID:     "test_customer",
		ProfileID:      "test_profile",
		UpdateTime:     time.Now(),
	})
	if err != nil {
		return err
	}
	return batch.Send()
}

func extractCHFields(data interface{}) string {
	var chFields []string

	val := reflect.ValueOf(data)
	typ := reflect.TypeOf(data)

	for i := 0; i < val.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("ch")
		if tag != "" {
			chFields = append(chFields, tag)
		}
	}

	return strings.Join(chFields, ", ")
}
