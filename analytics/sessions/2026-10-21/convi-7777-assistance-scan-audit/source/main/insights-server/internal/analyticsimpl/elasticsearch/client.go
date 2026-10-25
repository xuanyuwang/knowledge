package elasticsearch

import (
	"bytes"
	"context"
	"encoding/json"

	logger "github.com/cresta/shared-go/framework/log"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Mode int

const (
	UnspecifiedESMode Mode = iota
	EsModeCount
	EsModeTrackHit
	// Used by RetrieveClosedConversations to get a total convo count.
	EsModeTrackTotalCount
)

const maxTotalHintCount = 1<<31 - 1

func unsetJSONFieldRecursively(ctx context.Context, logger logger.Logger, queryID string, v any, targetField string) {
	switch x := v.(type) {
	case map[string]any:
		// delete at current level
		if userIDs, found := x[targetField]; found {
			// Log the (potentially large) user id list as structured zap fields rather than
			// interpolating it into the message. Dynamic dd.trace_id/dd.span_id fields are
			// written before explicit zap fields, so even if the log pipeline truncates this
			// oversized line the trace IDs survive and the entry stays attached to its trace.
			logger.InfoZap(ctx, "ES query agent user id list for query_id "+queryID,
				zap.String("query_id", queryID),
				zap.Any("agent_user_id_list", userIDs),
			)
			x[targetField] = nil
		}
		// recurse into remaining fields
		for _, child := range x {
			unsetJSONFieldRecursively(ctx, logger, queryID, child, targetField)
		}

	case []any:
		for _, child := range x {
			unsetJSONFieldRecursively(ctx, logger, queryID, child, targetField)
		}
	}
}

// Useful util method for debugging. Emits compact (non-indented) JSON so the resulting log
// line stays as small and single-line as possible, reducing the chance the log pipeline
// truncates it.
func JSONOfStruct(ctx context.Context, logger logger.Logger, queryID string, obj any) (string, error) {
	byteArr, err := json.Marshal(obj)
	if err != nil {
		return "", err
	}

	// Unset field agent_user_id.keyword with null values to avoid truncation in logs due to
	// big list of agent user ids.
	var structObj map[string]interface{}
	if err := json.Unmarshal(byteArr, &structObj); err != nil {
		return "", err
	}
	unsetJSONFieldRecursively(ctx, logger, queryID, structObj, "agent_user_id.keyword")

	byteArr, err = json.Marshal(structObj)
	if err != nil {
		return "", err
	}
	return string(byteArr), nil
}

// Parameter "query" can be any go struct that can be marshalled to a valid ES query in json format.
func Query(
	ctx context.Context,
	esClient *elasticsearch.Client,
	logger logger.Logger,
	indexName string,
	mode Mode, pageSize int, logESQuery bool, query interface{}, response interface{}, queryID string,
) error {
	// Build the request body.
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		logger.Errorf(ctx, "Fail to encode query: %s", err)
	}
	if logESQuery {
		formattedJSON, _ := JSONOfStruct(ctx, logger, queryID, query)
		// Pass the (potentially very large) ES query as a structured zap field instead of
		// interpolating it into the message. Dynamic dd.trace_id/dd.span_id fields are emitted
		// before explicit zap fields, so if the log pipeline truncates this oversized line the
		// trace IDs are preserved and the log stays attached to the request trace.
		logger.InfoZap(ctx, "Sent ES query for query_id "+queryID,
			zap.String("query_id", queryID),
			zap.String("index_name", indexName),
			zap.String("es_query", formattedJSON),
		)
	}

	// Perform the search request.
	var trackTotalCount interface{}
	switch mode {
	case EsModeTrackTotalCount:
		trackTotalCount = maxTotalHintCount
	case EsModeCount:
		trackTotalCount = true
	default:
		trackTotalCount = false
	}
	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(indexName),
		esClient.Search.WithBody(&buf),
		esClient.Search.WithTrackTotalHits(trackTotalCount),
		esClient.Search.WithSize(pageSize),
		esClient.Search.WithPretty(),
	)
	if err != nil {
		formattedJSON, marshalErr := json.Marshal(query)
		if marshalErr != nil {
			// fail-open: marshaling query is best-effort for logging only.
			formattedJSON = []byte(`"<marshal_failed>"`)
		}
		// Structured fields (see logESQuery branch above) keep dd.trace_id/dd.span_id ahead of
		// the large es_query payload so truncation cannot detach this error from its trace.
		logger.ErrorZap(ctx, "fail to get ES response",
			zap.Error(err),
			zap.String("index_name", indexName),
			zap.String("es_query", string(formattedJSON)),
		)
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		var e map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&e); err != nil {
			logger.Errorf(ctx, "Fail to parse the response body: %s", err)
			// Print the response status and response.
			logger.Infof(ctx, "response: %s", res.String())
			return err
		}
		// Print the response status and error information.
		logger.Errorf(ctx, "[%s]: %s.",
			res.Status(), e,
		)
		return status.Errorf(codes.Internal, "elastic search query failed: %v", e)
	}

	var r map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&r); err != nil {
		return status.Errorf(codes.Internal, "error parsing the response body: %s", err)
	}

	if err := mapstructure.Decode(r, response); err != nil {
		return err
	}

	return nil
}
