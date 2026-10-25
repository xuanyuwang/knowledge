package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/shared/utils"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	milliSecondsThresholdForBatchDelete = envflag.Int("Milli_SECOND_THRESHOLD_FOR_BATCH_DELETE", 3000, "ES documents older than the specificed number of milli seconds will be delete in batch")
	revertedChanceToRunBatchDelete      = envflag.Int("REVERTED_CHANCE_TO_RUN_BATCH_DELETE", 5000, "the reverted chance for triggering batch delete, e.g. when its value is 100, the chance for triggering batch delete is 1/100")
)

// TODO(caiyi) - mock ES write API call so that we can delete this env flag.
var skipWriteESForTesting = envflag.String("SKIP_WRITE_ES_FOR_TESTING", "false", "")

type ESHandler struct {
	esClient *elasticsearch.Client
	logger   logger.Logger
	indices  []string
}

func NewESHandler(indices []string, logger logger.Logger, esClient *elasticsearch.Client) *ESHandler {
	return &ESHandler{
		esClient: esClient,
		logger:   logger,
		indices:  indices,
	}
}

// TODO(caiyi) - find a better way to handle control chars.
func cleanupMessage(message string) string {
	pattern := "[[:cntrl:]]"
	regex := regexp.MustCompile(pattern)
	message = regex.ReplaceAllString(message, " ")
	toRemove := []string{"\\u0009", "\\'", "\"", "\\"}
	for _, ss := range toRemove {
		message = strings.ReplaceAll(message, ss, " ")
	}
	return message
}

// Determine if the message matches keyword moments.
//
//	input:
//	  - keywordExpressions: moment_template_id -> keyword_expresssion
//	  - message: the plain message text to be matched.
//	output:
//	  - ids of matched keyword moments.
func (e *ESHandler) DetectKeywords(ctx context.Context, keywordMoments []*momentpb.Moment, message, languageCode string) (map[string]bool, error) {
	indexName, err := e.pickIndexFromPool()
	if err != nil {
		return nil, err
	}

	// trim new line char since ES does not recognize it.
	message = cleanupMessage(message)
	documentID, err := e.writeMessage(ctx, writeMessageOptions{
		message:      message,
		languageCode: languageCode,
		indexName:    indexName,
	})
	if err != nil {
		return nil, err
	}
	if shouldRunBatchDelete() {
		defer func() {
			go utils.AvoidPanic(ctx, func() {
				// Batch delete messages that are older than a specified time.q
				if err := e.batchDeleteByCreateTime(ctx, indexName, time.Now().UnixMilli()-int64(*milliSecondsThresholdForBatchDelete)); err != nil {
					e.logger.Errorf(ctx, "batchDeleteByCreateTime failed with error: %v", err)
				}
			})()
		}()
	}

	matchedMomentNames, err := e.searchKeywordMoments(ctx, searchKeywordMomentsOptions{
		languageCode:   languageCode,
		documentID:     documentID,
		keywordMoments: keywordMoments,
		indexName:      indexName,
	})
	if err != nil {
		return nil, err
	}
	return matchedMomentNames, nil
}

func shouldRunBatchDelete() bool {
	return rand.Intn(*revertedChanceToRunBatchDelete) == 0 // nolint:gosec
}

type writeMessageOptions struct {
	message, languageCode, indexName string
}

// Write a message text into Elasticsearch index, return the document id.
func (e *ESHandler) writeMessage(ctx context.Context, opts writeMessageOptions) (string, error) {
	if *skipWriteESForTesting == "true" {
		return "", nil
	}

	langCode := opts.languageCode
	documentBody := fmt.Sprintf(`{
			"create_time" : %d,
			"message_text" : "%s",
			"message_text_with_lang": {
				"%s": "%s",
				"language_code": "%s"
			}
		}`, time.Now().UnixMilli(), opts.message, langCode, opts.message, langCode)

	req := esapi.IndexRequest{
		Index:   opts.indexName,
		Body:    strings.NewReader(documentBody),
		Refresh: "wait_for",
	}

	resp, err := req.Do(ctx, e.esClient)
	if err != nil {
		return "", fmt.Errorf("indexing document: %s", err)
	}
	if resp.IsError() {
		return "", fmt.Errorf("insert response has error: %v", resp.String())
	}
	defer resp.Body.Close()

	var r map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", status.Errorf(codes.Internal, "error parsing the response body: %s", err)
	}

	return r["_id"].(string), nil
}

type searchKeywordMomentsOptions struct {
	languageCode, documentID, indexName string
	keywordMoments                      []*momentpb.Moment
}

// Search ES to determine if the message matches keyword expressions, and return the list of matched moments.
func (e *ESHandler) searchKeywordMoments(ctx context.Context, opts searchKeywordMomentsOptions) (matchedMomentIDs map[string]bool, err error) {
	query, aggs, err := queryForRuntimeKeywordMatching(ctx, opts.documentID, opts.keywordMoments, opts.languageCode)
	if err != nil {
		return nil, err
	}
	request := Request{
		Query: query,
		Aggs:  aggs,
	}

	res := Response{}
	err = Query(
		ctx, e.esClient, e.logger,
		opts.indexName,
		EsModeTrackTotalCount,
		9999, false, request, &res, "DetectKeywordMoments")
	if err != nil {
		return nil, err
	}

	if res.Aggregations.Moments == nil || res.Aggregations.Moments.Buckets == nil {
		return nil, fmt.Errorf("es Response.aggregations.Moments.Buickets is nil")
	}
	matchedMomentNames := map[string]bool{}
	buckets := *res.Aggregations.Moments.Buckets
	if len(buckets) != len(opts.keywordMoments) {
		return nil, fmt.Errorf("number of aggregation buckets %d is not equivalent to number of keyword moments %d", len(buckets), len(opts.keywordMoments))
	}
	for i := range buckets {
		if buckets[i].DocCount > 0 {
			matchedMomentNames[opts.keywordMoments[i].Name] = true
		}
	}
	return matchedMomentNames, nil
}

func (e *ESHandler) batchDeleteByCreateTime(ctx context.Context, indexName string, createTimeUnixMilli int64) error {
	queryBody := fmt.Sprintf(`{
		"query" : {
			"range": {
				"create_time": {
				  "lte": %d
				}
			}
		}
	}`, createTimeUnixMilli)

	docCount, err := e.getDocCount(ctx, indexName, queryBody)
	if err != nil {
		return fmt.Errorf("getDocCount return error: %v", err)
	}

	if docCount < int32(*revertedChanceToRunBatchDelete) {
		e.logger.Infof(ctx, "Batch delete is skipped because the number of documents to delete is %d, which is less than threshold %d", docCount, *revertedChanceToRunBatchDelete)
		return nil
	}

	e.logger.Infof(ctx, "batchDeleteByCreateTime(index=%s, createTimeUnixMilli=%d)", indexName, createTimeUnixMilli)
	resp, err := e.esClient.DeleteByQuery([]string{indexName}, strings.NewReader(queryBody))
	if err != nil {
		return fmt.Errorf("batch delete documents failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return fmt.Errorf("batch delete documents response has error: %s", resp.String())
	}
	return nil
}

func (e *ESHandler) getDocCount(ctx context.Context, indexName string, queryBody string) (int32, error) {
	countRequest := esapi.CountRequest{
		Index: []string{indexName},
		Body:  strings.NewReader(queryBody),
	}

	resp, err := countRequest.Do(context.Background(), e.esClient)
	if err != nil {
		return 0, fmt.Errorf("executing the count request: %s", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return 0, fmt.Errorf("getDocCount response has error: %s", resp.String())
	}

	var r map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return 0, fmt.Errorf("error parsing the response body: %s", err)
	}
	count := int32(r["count"].(float64))
	return count, nil
}

// Randomly pick up an index from from the pool of indices.
func (e *ESHandler) pickIndexFromPool() (string, error) {
	indexCount := len(e.indices)
	if indexCount == 0 {
		return "", status.Errorf(codes.Internal, "index pool for live message detection is empty")
	}
	return e.indices[rand.Intn(indexCount)], nil // nolint:gosec
}

func queryForRuntimeKeywordMatching(ctx context.Context, documentID string, keywordMoments []*momentpb.Moment, languageCode string) (*OuterQuery, *ReqAggs, error) {
	outerQuery := &OuterQuery{
		Bool: MsgOuterBool{
			Filter: &[]MsgFilter{
				{
					Terms: &TermsFilter{
						DocumentIDs: &[]string{documentID},
					},
				},
			},
		},
	}

	filters := []ReqAnnotationSubFilter{}
	for _, m := range keywordMoments {
		filter, err := convertKeywordMomentToKeywordMomentBool(m, languageCode)
		if err != nil {
			return nil, nil, fmt.Errorf("convertKeywordMomentToKeywordMomentBool() for moment %s: %v", m.Name, err)
		}
		f := ReqAnnotationSubFilter{
			Bool: filter,
		}
		filters = append(filters, f)
	}

	aggs := &ReqAggs{Moments: &ReqAnnotations{
		Filters: &ReqAnnotationFilters{
			Filters: filters,
		},
	}}
	return outerQuery, aggs, nil
}

func convertKeywordMomentHelper(expression *momentpb.KeywordMomentData_KeywordExpression, languageCode string) (*KeywordMomentBool, error) {
	if expression.Operator == momentpb.KeywordMomentData_SEQUENCE {
		return nil, fmt.Errorf("unexpected sequence operator in keyword expression: %+v", expression)
	}
	filters := []KeywordMomentBoolFilter{}
	for _, operrand := range expression.Operands {
		switch operrand.Operand.(type) {
		case *momentpb.KeywordMomentData_Operand_Phrase:
			f := KeywordMomentBoolFilter{MultiMatch: &MultiMatchField{
				Query:  operrand.GetPhrase(),
				Type:   phraseType,
				Fields: []string{defaultMsgTextField},
			}}
			filters = append(filters, f)
		// Language specific analyzer is only applied for SEQUENCE oprator (aka flexible matching).
		case *momentpb.KeywordMomentData_Operand_Expression:
			if operrand.GetExpression().Operator == momentpb.KeywordMomentData_SEQUENCE {
				msgTextField, exists := langCodeToMsgTextField[languageCode]
				if !exists {
					return nil, status.Errorf(codes.InvalidArgument, "message text field for language code %q does not exist", languageCode)
				}
				words := []string{}
				for _, op := range operrand.GetExpression().Operands {
					// Only allow Phrase operands in SEQUENCE Operator
					phrase, isInstance := op.Operand.(*momentpb.KeywordMomentData_Operand_Phrase)
					if !isInstance {
						return nil, fmt.Errorf("only phrase operand is allowed in sequence operator, but found %v", expression)
					}
					words = append(words, phrase.Phrase)
				}
				filters = append(filters, KeywordMomentBoolFilter{MultiMatch: &MultiMatchField{
					Query:  strings.Join(words, " "),
					Type:   phraseType,
					Fields: []string{msgTextField},
					Slop:   proto.Int32(maxSlopForFlexiblePhrase),
				}})
			} else {
				b, err := convertKeywordMomentHelper(operrand.GetExpression(), languageCode)
				if err != nil {
					return nil, err
				}
				filters = append(filters, KeywordMomentBoolFilter{
					Bool: b,
				})
			}
		}
	}

	res := KeywordMomentBool{}
	switch expression.Operator {
	case momentpb.KeywordMomentData_AND:
		res.Must = &filters
	case momentpb.KeywordMomentData_OR:
		res.Should = &filters
		res.MinimumShouldMatch = 1
	case momentpb.KeywordMomentData_EXCLUDE:
		res.MustNot = &filters
	default:
		return nil, fmt.Errorf("unexpected keyword operator %v", expression.Operator)

	}
	return &res, nil
}

func convertKeywordMomentToKeywordMomentBool(m *momentpb.Moment, languageCode string) (*KeywordMomentBool, error) {
	if m == nil || m.GetMomentData() == nil || m.GetMomentData().GetKeywordMomentData() == nil || m.GetMomentData().GetKeywordMomentData().GetKeywordExpression() == nil {
		return nil, fmt.Errorf("moment.moment_data.keyword_moment_data.keyword_expression is empty")
	}

	exp := m.GetMomentData().GetKeywordMomentData().KeywordExpression

	switch exp.Operator {
	case momentpb.KeywordMomentData_AND:
	case momentpb.KeywordMomentData_OR:
	default:
		return nil, fmt.Errorf("%v is not allowed as toper layer operator", exp.Operator)
	}

	return convertKeywordMomentHelper(exp, languageCode)
}
