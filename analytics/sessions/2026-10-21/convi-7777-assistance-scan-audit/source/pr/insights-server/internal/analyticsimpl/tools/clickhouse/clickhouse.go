/*
To run the script with command
	- `bazel run //insights-server/internal/analyticsimpl/tools/clickhouse:clickhouse` or
	- `go run insights-server/internal/analyticsimpl/tools/clickhouse/clickhouse.go`
*/

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cresta/go-servers/shared/utils"
)

func main() {
	ctx := context.Background()
	conn, err := connect()
	if err != nil {
		panic(err)
	}

	for i := 0; i < 500; i++ {
		go utils.AvoidPanic(ctx, func() {
			rows, err := conn.Query(ctx, "SELECT request.grpc.method, count() FROM logging.RequestLog_d group by 1 order by 2 desc;")
			if err != nil {
				panic(err)
			}

			total := 0
			for rows.Next() {
				var (
					method string
					count  uint64
				)
				if err := rows.Scan(
					&method,
					&count,
				); err != nil {
					panic(err)
				}
				total += int(count)
			}
			log.Printf("[%d] total count: %d", i, total)

			err = conn.Close()
			if err != nil {
				panic(err)
			}
		})()
	}

	time.Sleep(10 * time.Second)
	fmt.Println("done")
}

func connect() (driver.Conn, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{"clickhouse-request-log.chat-staging.internal.cresta.ai:9000"},
		Auth: clickhouse.Auth{
			Database: "logging",
			Username: "",
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
