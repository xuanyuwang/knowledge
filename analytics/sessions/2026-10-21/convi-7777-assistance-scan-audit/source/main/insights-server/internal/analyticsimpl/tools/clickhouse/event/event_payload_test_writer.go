/*
To run the script with command
	- bazel run //insights-server/internal/analyticsimpl/tools/clickhouse/event:clickhouse
*/

package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"google.golang.org/protobuf/encoding/protojson"

	clickhouseeventspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/clickhouse/events"
)

type EventRow struct {
	ConversationID string    `ch:"conversation_id"`
	CustomerID     string    `ch:"customer_id"`
	EventID        string    `ch:"event_id"`
	EventPayload   string    `ch:"event_payload"`
	CreateTime     time.Time `ch:"create_time"`
}

func main() {
	conn, err := connect()
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	if err := writeEvent(ctx, conn); err != nil {
		panic(err)
	}
	fmt.Println("done")
}

func connect() (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse-events.chat-staging.internal.cresta.ai:9440"},
		Auth: clickhouse.Auth{
			Database: "cox_sales",
			Username: "admin",
			Password: "", // The pwd secret.
		},
		// #nosec G402
		TLS: &tls.Config{
			InsecureSkipVerify: true,
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

func writeEvent(ctx context.Context, conn driver.Conn) error {
	batch, err := conn.PrepareBatch(ctx, fmt.Sprintf("INSERT INTO conversation_event_d (%s)", extractCHFields(EventRow{})))
	if err != nil {
		return err
	}

	clickhousePayload := &clickhouseeventspb.ConversationEventPayload{}
	clickhousePayload.Payload = &clickhouseeventspb.ConversationEventPayload_UsedSummarization{
		UsedSummarization: &clickhouseeventspb.UsedSummarization{},
	}
	clickhousePayload.GetUsedSummarization().Data = &clickhouseeventspb.UsedSummarization_CopyAllSummarizationNotes_{
		CopyAllSummarizationNotes: &clickhouseeventspb.UsedSummarization_CopyAllSummarizationNotes{
			Count: 10,
		},
	}
	payload1, _ := protojson.Marshal(clickhousePayload)

	clickhousePayload.GetUsedSummarization().Data = &clickhouseeventspb.UsedSummarization_CopySummarizationNote_{
		// The empty proto will be skipped by clickhouse.
		CopySummarizationNote: &clickhouseeventspb.UsedSummarization_CopySummarizationNote{},
	}
	payload2, _ := protojson.Marshal(clickhousePayload)

	println(string(payload1))
	println(string(payload2))

	for _, event := range []*EventRow{
		// - 1) String-based payload.
		{
			ConversationID: "test_conversation",
			CustomerID:     "test_customer",
			EventID:        "test_event_id_1",
			EventPayload:   `{"aa": "1", "bb": {"b": "2"}}`,
			CreateTime:     time.Now(),
		},
		// - 2) Proto-to-String-based payloads.
		{
			ConversationID: "test_conversation",
			CustomerID:     "test_customer",
			EventID:        "test_event_id_2",
			EventPayload:   string(payload1),
			CreateTime:     time.Now(),
		},
		{
			ConversationID: "test_conversation",
			CustomerID:     "test_customer",
			EventID:        "test_event_id_3",
			EventPayload:   string(payload2),
			CreateTime:     time.Now(),
		},
	} {
		if err := batch.AppendStruct(event); err != nil {
			return err
		}
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
