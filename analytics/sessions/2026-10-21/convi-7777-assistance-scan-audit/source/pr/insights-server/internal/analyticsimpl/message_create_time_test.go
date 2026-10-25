package analyticsimpl

import (
	"testing"
	"time"

	"github.com/cresta/shared-go/util/envflag"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
)

func TestMessageCreateTimeFromESSource(t *testing.T) {
	reset := func() {
		t.Helper()
		require.NoError(t, envflag.Set("INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME", "true"))
	}
	reset()
	t.Cleanup(reset)

	t.Run("uses platform_timestamp when enabled and set", func(t *testing.T) {
		require.NoError(t, envflag.Set("INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME", "true"))
		t.Cleanup(reset)

		hit := elasticsearch.ResHit{
			Source: elasticsearch.ResSource{
				CreatedAt:         "2024-01-02T12:00:00Z",
				PlatformTimestamp: "2024-01-01T10:30:45.123Z",
			},
		}
		got, err := messageCreateTimeFromESSource(hit)
		require.NoError(t, err)
		require.Equal(t, timestamppb.New(time.Date(2024, 1, 1, 10, 30, 45, 123000000, time.UTC)), got)
	})

	t.Run("falls back to created_at when platform_timestamp empty", func(t *testing.T) {
		require.NoError(t, envflag.Set("INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME", "true"))
		t.Cleanup(reset)

		hit := elasticsearch.ResHit{
			Source: elasticsearch.ResSource{
				CreatedAt: "2024-03-04T08:09:10.111Z",
			},
		}
		got, err := messageCreateTimeFromESSource(hit)
		require.NoError(t, err)
		require.Equal(t, timestamppb.New(time.Date(2024, 3, 4, 8, 9, 10, 111000000, time.UTC)), got)
	})

	t.Run("uses created_at when disabled even if platform_timestamp set", func(t *testing.T) {
		require.NoError(t, envflag.Set("INSIGHTS_USE_PLATFORM_TIMESTAMP_FOR_MESSAGE_CREATE_TIME", "false"))
		t.Cleanup(reset)

		hit := elasticsearch.ResHit{
			Source: elasticsearch.ResSource{
				CreatedAt:         "2024-05-06T01:02:03Z",
				PlatformTimestamp: "2024-12-31T23:59:59Z",
			},
		}
		got, err := messageCreateTimeFromESSource(hit)
		require.NoError(t, err)
		require.Equal(t, timestamppb.New(time.Date(2024, 5, 6, 1, 2, 3, 0, time.UTC)), got)
	})
}
