package analyticsimpl

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"github.com/cresta/go-servers/insights-server/internal/shared"
	sharedes "github.com/cresta/go-servers/shared/conversation/elastic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// This file holds the moment-agnostic Elasticsearch value-retrieval helpers shared
// by RetrieveMetadataValues (conversation metadata moments) and
// RetrieveLlmAnnotationValues (LLM annotation moments). It resolves the ES value
// field for a moment, classifies the value kind, and runs the distinct-value /
// numeric min-max aggregation, returning generic metadatapb.MetadataValue results.

// Elasticsearch has limit of 65,536 on number of buckets.
// 1K should be big enough for most use cases.
const maxMetadataValueCount = 1000

// valueKind classifies how an Elasticsearch aggregation bucket value should be
// interpreted and shaped into a metadatapb.MetadataValue. It is derived from the
// moment type: conversation-metadata moments use their metadata type, while LLM
// annotation moments use their declared value type.
type valueKind int

const (
	valueKindString valueKind = iota
	valueKindNumber
	valueKindBoolean
)

func (a AnalyticsServiceImpl) getMetadataMomentValueField(payload *momentpb.ConversationMetadataMomentData) (string, error) {
	metadataValueField, metadataType := "", payload.Type
	switch metadataType {
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_COUNTRY,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_STATE,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_TYPE,
		metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_OS,
		metadatapb.ConversationMetadataEnum_TYPE_CHAT_SKILL,
		metadatapb.ConversationMetadataEnum_TYPE_AGENT_SURVEY:
		metadataValueField = elasticsearch.ConversationMetadataPayloadStringValueField
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
		if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
			metadataValueField = elasticsearch.ConversationMetadataPayloadNumberValueFieldExplicitMapping
		} else {
			metadataValueField = elasticsearch.ConversationMetadataPayloadNumberValueField
		}
	case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
		metadataValueField = elasticsearch.ConversationMetadataPayloadBooleanValueField
	default:
		return "", status.Errorf(codes.InvalidArgument, "unsupported metadata type %v", metadataType)
	}
	return metadataValueField, nil
}

// resolveMomentValueField resolves the Elasticsearch value field path to aggregate on
// for a moment, plus whether that value is numeric. It supports both conversation
// metadata moments and LLM annotation moments so the ES retrieval core can be shared.
func (a AnalyticsServiceImpl) resolveMomentValueField(moment *momentpb.Moment) (valueField string, isNumeric bool, err error) {
	switch moment.Type {
	case momentpb.Moment_CONVERSATION_METADATA:
		payload := moment.MomentData.GetConversationMetadataMomentData()
		valueField, err = a.getMetadataMomentValueField(payload)
		if err != nil {
			return "", false, err
		}
		return valueField, payload.Type == metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER, nil
	case momentpb.Moment_LLM_ANNOTATION:
		switch moment.MomentData.GetLlmAnnotationMomentData().GetValueType() {
		case momentpb.LlmAnnotationMomentData_STRING:
			return elasticsearch.LlmAnnotationPayloadStringValueField, false, nil
		case momentpb.LlmAnnotationMomentData_NUMBER:
			if a.enableUsageOfExplicitMappingMomentAnnotationPayload {
				return elasticsearch.LlmAnnotationPayloadNumberValueFieldExplicitMapping, true, nil
			}
			return elasticsearch.LlmAnnotationPayloadNumberValueField, true, nil
		case momentpb.LlmAnnotationMomentData_BOOLEAN:
			return elasticsearch.LlmAnnotationPayloadBooleanValueField, false, nil
		default:
			return "", false, status.Errorf(codes.InvalidArgument, "unsupported llm annotation value type %v", moment.MomentData.GetLlmAnnotationMomentData().GetValueType())
		}
	default:
		return "", false, status.Errorf(codes.InvalidArgument, "unsupported moment type %v", moment.Type)
	}
}

// metadataValueKind classifies how an aggregation bucket value should be shaped into
// a metadatapb.MetadataValue, based on the moment type.
func (a AnalyticsServiceImpl) metadataValueKind(moment *momentpb.Moment) (valueKind, error) {
	switch moment.Type {
	case momentpb.Moment_CONVERSATION_METADATA:
		switch moment.MomentData.GetConversationMetadataMomentData().Type {
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_STRING,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_COUNTRY,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_GEO_STATE,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_TYPE,
			metadatapb.ConversationMetadataEnum_TYPE_VISITOR_DEVICE_OS,
			metadatapb.ConversationMetadataEnum_TYPE_CHAT_SKILL,
			metadatapb.ConversationMetadataEnum_TYPE_AGENT_SURVEY:
			return valueKindString, nil
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_NUMBER:
			return valueKindNumber, nil
		case metadatapb.ConversationMetadataEnum_TYPE_CUSTOM_BOOLEAN:
			return valueKindBoolean, nil
		default:
			return 0, status.Errorf(codes.InvalidArgument, "unsupported metadata type %v", moment.MomentData.GetConversationMetadataMomentData().Type)
		}
	case momentpb.Moment_LLM_ANNOTATION:
		switch moment.MomentData.GetLlmAnnotationMomentData().GetValueType() {
		case momentpb.LlmAnnotationMomentData_STRING:
			return valueKindString, nil
		case momentpb.LlmAnnotationMomentData_NUMBER:
			return valueKindNumber, nil
		case momentpb.LlmAnnotationMomentData_BOOLEAN:
			return valueKindBoolean, nil
		default:
			return 0, status.Errorf(codes.InvalidArgument, "unsupported llm annotation value type %v", moment.MomentData.GetLlmAnnotationMomentData().GetValueType())
		}
	default:
		return 0, status.Errorf(codes.InvalidArgument, "unsupported moment type %v", moment.Type)
	}
}

// readMomentValuesFromElasticSearch retrieves the distinct values (or numeric
// min/max) of a moment from Elasticsearch. It is moment-agnostic and shared between
// RetrieveMetadataValues (conversation metadata moments) and
// RetrieveLlmAnnotationValues (LLM annotation moments).
//
// nolint:gosec
func (a AnalyticsServiceImpl) readMomentValuesFromElasticSearch(
	ctx context.Context,
	profileConfig *config.ProfileConfig,
	parent *shared.CustomerProfile,
	moment *momentpb.Moment,
	valueStringContains string,
	numericMinMaxOnly bool,
	filterUsecaseNames []string,
	queryID string,
) ([]*metadatapb.MetadataValue, error) {
	valueField, isNumeric, err := a.resolveMomentValueField(moment)
	if err != nil {
		return nil, err
	}

	kind, err := a.metadataValueKind(moment)
	if err != nil {
		return nil, err
	}

	momentName, err := momentpb.ParseMomentName(moment.Name)
	if err != nil {
		return nil, err
	}
	filters := []elasticsearch.NestedFilter{
		{
			Terms: &elasticsearch.NestedTermsFilter{
				MomentTemplateIDs: &[]string{momentName.MomentID},
			},
		},
	}
	if valueStringContains != "" && kind == valueKindString {
		wildcardValue := &elasticsearch.ValueField{
			Value:           fmt.Sprintf("*%s*", valueStringContains),
			CaseInsensitive: proto.Bool(true),
		}
		// The wildcard pre-filter must target the same payload field the aggregation
		// reads from, otherwise it ANDs out every nested doc for the moment type.
		wildcard := &elasticsearch.WildcardFilter{}
		switch moment.Type {
		case momentpb.Moment_LLM_ANNOTATION:
			wildcard.LlmAnnotationPayload = wildcardValue
		default:
			wildcard.MetadataPayload = wildcardValue
		}
		filters = append(filters, elasticsearch.NestedFilter{WildcardFilter: wildcard})
	}
	usecaseIDs := []string{}
	if len(filterUsecaseNames) > 0 {
		usecaseIDs, err = parseUsercaseNames(filterUsecaseNames)
		if err != nil {
			return nil, err
		}
	}
	// Only scan conversation documents.
	convoFilters := []elasticsearch.ConvoFilter{
		{
			Terms: &elasticsearch.TermsFilter{
				ConversationToMessageJoin: &[]string{"conversation"},
			},
		},
		// Pre-filter to conversations that actually carry an annotation for this moment
		// so the nested aggregation scans far fewer documents (lower latency/memory).
		{
			Nested: &elasticsearch.Nested{
				Path: elasticsearch.MomentAnnotationsPath,
				Query: &elasticsearch.NestedFilter{
					Terms: &elasticsearch.NestedTermsFilter{
						MomentTemplateIDs: &[]string{momentName.MomentID},
					},
				},
			},
		},
	}
	if len(usecaseIDs) > 0 {
		convoFilters = append(convoFilters, elasticsearch.ConvoFilter{
			Terms: &elasticsearch.TermsFilter{
				UsecaseIDs: &usecaseIDs,
			},
		})
	}

	var query elasticsearch.Request
	// For numeric min/max, use stats aggregation
	if numericMinMaxOnly && isNumeric {
		query = elasticsearch.Request{
			Aggs: &elasticsearch.ReqAggs{
				Moments: &elasticsearch.ReqAnnotations{
					Nested: &elasticsearch.Nested{
						Path: elasticsearch.MomentAnnotationsPath,
					},
					Aggs: &elasticsearch.ReqAggAnnotation{
						MomentAnnotations: &elasticsearch.ReqMomentAnnotations{
							Filter: elasticsearch.ReqMomentAnnotationFilter{
								Bool: elasticsearch.NestedFilterBool{
									Filter: &filters,
								},
							},
							Aggs: elasticsearch.ReqMomentAnnotationPayloadValueAggs{
								PayloadValue: elasticsearch.ReqPayloadValue{
									Terms: nil,
									Stats: &elasticsearch.StatsAggs{
										Field: proto.String(valueField),
									},
								},
							},
						},
					},
				},
			},
			Query: &elasticsearch.OuterQuery{
				Bool: &elasticsearch.ConvoOuterBool{
					ConvoDocumentFilter: &convoFilters,
				},
			},
		}
	} else {
		query = elasticsearch.Request{
			Aggs: &elasticsearch.ReqAggs{
				Moments: &elasticsearch.ReqAnnotations{
					Nested: &elasticsearch.Nested{
						Path: elasticsearch.MomentAnnotationsPath,
					},
					Aggs: &elasticsearch.ReqAggAnnotation{
						MomentAnnotations: &elasticsearch.ReqMomentAnnotations{
							Filter: elasticsearch.ReqMomentAnnotationFilter{
								Bool: elasticsearch.NestedFilterBool{
									Filter: &filters,
								},
							},
							Aggs: elasticsearch.ReqMomentAnnotationPayloadValueAggs{
								PayloadValue: elasticsearch.ReqPayloadValue{
									Terms: &elasticsearch.TermsAggs{
										Field: proto.String(valueField),
										Size:  proto.Int32(maxMetadataValueCount),
									},
								},
							},
						},
					},
				},
			},
			// Only scan conversation documents.
			Query: &elasticsearch.OuterQuery{
				Bool: &elasticsearch.ConvoOuterBool{
					ConvoDocumentFilter: &convoFilters,
				},
			},
		}
	}

	res := elasticsearch.Response{}
	err = elasticsearch.Query(
		ctx, a.esClient, a.logger,
		sharedes.GetIndexName(profileConfig.GetFeatures(), &customerpb.ProfileName{CustomerID: parent.CustomerID, ProfileID: parent.ProfileID}),
		elasticsearch.EsModeCount, 0 /*=pageSize*/, false, query, &res, queryID)
	if err != nil {
		return nil, err
	}
	if numericMinMaxOnly && isNumeric {
		moments := res.Aggregations.Moments
		if moments == nil || moments.MomentAnnotations == nil {
			return []*metadatapb.MetadataValue{}, nil
		}

		payload := moments.MomentAnnotations.PayloadValue
		if payload.Count == 0 {
			return []*metadatapb.MetadataValue{}, nil
		}

		minVal := payload.Min
		maxVal := payload.Max
		if math.IsInf(minVal, 0) || math.IsNaN(minVal) ||
			math.IsInf(maxVal, 0) || math.IsNaN(maxVal) {
			return []*metadatapb.MetadataValue{}, nil
		}

		return []*metadatapb.MetadataValue{
			{
				Value: &metadatapb.MetadataValue_NumberValue{
					NumberValue: float32(minVal),
				},
			},
			{
				Value: &metadatapb.MetadataValue_NumberValue{
					NumberValue: float32(maxVal),
				},
			},
		}, nil
	}

	// Other cases response handling
	var values []*metadatapb.MetadataValue
	if moments := res.Aggregations.Moments; moments != nil && moments.MomentAnnotations != nil {
		for _, bucket := range res.Aggregations.Moments.MomentAnnotations.PayloadValue.Buckets {
			val := metadatapb.MetadataValue{}
			switch kind {
			case valueKindString:
				value := bucket.Key.(string)
				if !strings.Contains(strings.ToLower(value), strings.ToLower(valueStringContains)) {
					continue
				}
				val.Value = &metadatapb.MetadataValue_StringValue{
					StringValue: value,
				}
			case valueKindNumber:
				numericValue := bucket.Key.(float64)
				// This is not an ideal solution. In theory we should have defined better
				// interface to filter on numberic value based metadata.
				if len(valueStringContains) > 0 && !strings.Contains(strconv.FormatFloat(numericValue, 'f', 2, 64), valueStringContains) {
					continue
				}
				val.Value = &metadatapb.MetadataValue_NumberValue{
					NumberValue: float32(numericValue),
				}
			case valueKindBoolean:
				boolVal := false
				if s, ok := bucket.Key.(bool); ok {
					boolVal = s
				} else {
					boolVal = bucket.Key.(float64) != 0
				}
				val.Value = &metadatapb.MetadataValue_BooleanValue{
					BooleanValue: boolVal,
				}
			default:
				return nil, status.Errorf(codes.InvalidArgument, "unsupported value kind %v", kind)
			}
			values = append(values, &val)
		}
	}
	return values, nil
}
