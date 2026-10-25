package analyticsimpl

import (
	"context"
	"fmt"
	"strings"

	aicommonpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/ai_service/common"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	enumspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/enums"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/cresta/shared-go/util/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
)

const NumOfTokensToTruncate = 5

var (
	fanoutToES       = envflag.Int("FAN_OUT_TO_ES", 1, "the number of times calling es to insert+search")
	esServingIndices = envflag.String("ES_SERVING_INDEX", "multi-tenant-live-messages-v0-1,multi-tenant-live-messages-v0-2,multi-tenant-live-messages-v0-3,multi-tenant-live-messages-v0-4,multi-tenant-live-messages-v0-5,multi-tenant-live-messages-v0-6,multi-tenant-live-messages-v0-7,multi-tenant-live-messages-v0-8,multi-tenant-live-messages-v0-9,multi-tenant-live-messages-v0-10", "the comma separate serving ES indices")
)

func (a AnalyticsServiceImpl) DetectKeywordMoments(ctx context.Context, req *analyticspb.DetectKeywordMomentsRequest) (*analyticspb.DetectKeywordMomentsResponse, error) {
	if req == nil || req.Chat == nil || req.Chat.AiServiceConfig == nil || req.Chat.AiServiceConfig.GetKeywordConfig() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid request: req.chat.ai_service_config.keyword_config is empty")
	}
	languageCode := ""
	if chatState := req.Chat.ChatState; chatState != nil {
		languageCode = chatState.LanguageCode
	}

	if languageCode == "" {
		return nil, status.Errorf(codes.InvalidArgument, "empty chat.chat_stat.language_code is invalid")
	}

	message := getLastMessage(req.Chat.Messages)

	keywordMoments := []*momentpb.Moment{}

	momentNamesWithMatchedMessageType := map[string]bool{}
	for _, def := range req.Chat.AiServiceConfig.GetKeywordConfig().Definitions {
		name := fmt.Sprintf("customers/customer-id-placeholder/profiles/profile-id-placeholder/moments/%s", def.MomentTemplateId)
		m := momentpb.Moment{
			Name: name,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: convertKeywordMomentData(def.KeywrodMomentData),
				},
			},
		}
		keywordMoments = append(keywordMoments, &m)
		if messageTypeMatch(message, def) {
			momentNamesWithMatchedMessageType[name] = true
		}

	}

	esHandler := elasticsearch.NewESHandler(a.getServingESIndicies(), a.logger, a.esClient)

	resultChan := make(chan map[string]bool, *fanoutToES)
	eg := errgroup.New(errgroup.WithLimit(*fanoutToES))
	for i := 0; i < *fanoutToES; i++ {
		eg.Go(func() error {
			mm, err := esHandler.DetectKeywords(ctx, keywordMoments, message.Text, languageCode)
			if err != nil {
				return err
			}
			select {
			case resultChan <- mm:
				return nil
			default:
				return fmt.Errorf("resultChan is full or blocked")
			}
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	close(resultChan)

	var matchedMomentNames map[string]bool
	for mm := range resultChan {
		// NOTE(dhoske): I don't understand why this overrides the whole map.
		matchedMomentNames = mm
	}

	if matchedMomentNames != nil {
		return convertAPIResponse(req, message, keywordMoments, matchedMomentNames, momentNamesWithMatchedMessageType)
	}
	return nil, nil
}

// Copy the implementation from https://github.com/cresta/python-ai-services/blob/8b1af6bee086013f62e85371b219ea1c87aa51ac/keyword/src/utils.py#L123
func messageTypeMatch(msg *aicommonpb.AiConversationMessage, def *aicommonpb.KeywordConfig_Definition) bool {
	if len(def.MessageTypes) == 0 {
		return true
	}
	switch msg.SpeakerRole {
	case enumspb.SpeakerRole_AGENT:
		for _, momentMsgType := range def.MessageTypes {
			if momentMsgType == aicommonpb.KeywordConfig_Definition_MESSAGE_TYPE_AGENT {
				return true
			}
		}
	case enumspb.SpeakerRole_VISITOR:
		for _, momentMsgType := range def.MessageTypes {
			if momentMsgType == aicommonpb.KeywordConfig_Definition_MESSAGE_TYPE_VISITOR {
				return true
			}
		}
	case enumspb.SpeakerRole_SYSTEM_MESSAGE:
		for _, momentMsgType := range def.MessageTypes {
			if momentMsgType == aicommonpb.KeywordConfig_Definition_MESSAGE_TYPE_SYSTEM {
				return true
			}
		}
	}
	return false
}

func (a *AnalyticsServiceImpl) getServingESIndicies() []string {
	ss := *esServingIndices
	indices := []string{}
	for _, s := range strings.Split(ss, ",") {
		indices = append(indices, strings.Trim(s, " "))
	}
	return indices
}

func truncateMessageText(text string) string {
	strs := strings.Split(text, " ")
	if len(strs) == 0 {
		return ""
	}
	if len(strs) <= NumOfTokensToTruncate {
		return strs[0]
	}
	return strings.Join(strs[:len(strs)-NumOfTokensToTruncate], " ")
}

func getLastMessage(messages []*aicommonpb.AiConversationMessage) *aicommonpb.AiConversationMessage {
	message := messages[0]
	if message.MessageState == aicommonpb.AiConversationMessage_INCOMPLETE {
		message.Text = truncateMessageText(message.Text)
	}
	return message
}

func convertAPIResponse(request *analyticspb.DetectKeywordMomentsRequest, message *aicommonpb.AiConversationMessage, keywordMoments []*momentpb.Moment, matchedMomentNames, momentNamesWithMatchedMessageType map[string]bool) (*analyticspb.DetectKeywordMomentsResponse, error) {
	conversationName := request.Chat.Name
	momentAnnotations := []*aicommonpb.MomentAnnotation{}
	metadata := conversationMetadata(message.Name, conversationName)
	for i := range keywordMoments {
		momentName, err := momentpb.ParseMomentName(keywordMoments[i].Name)
		if err != nil {
			return nil, err
		}

		keywordPayload := &aicommonpb.KeywordPayload{
			MomentTemplateId: momentName.MomentID,
		}
		if matchedMomentNames[momentName.String()] && momentNamesWithMatchedMessageType[momentName.String()] {
			// TODO: try to populate start_pos, end_pos and matched_token.
			keywordPayload.MatchedMoments = append(keywordPayload.MatchedMoments, &aicommonpb.KeywordPayload_MatchedMoment{})
		}
		momentAnnotations = append(momentAnnotations, &aicommonpb.MomentAnnotation{
			Metadata: metadata,
			Type:     aicommonpb.MomentAnnotation_KEYWORD,
			Payload: &aicommonpb.MomentAnnotation_KeywordPayload{
				KeywordPayload: keywordPayload,
			},
		})
	}
	return &analyticspb.DetectKeywordMomentsResponse{
		Result: &aicommonpb.AiConversationPredictResult{
			Name:          conversationName,
			OutputMoments: momentAnnotations,
		},
	}, nil
}

func conversationMetadata(messageName, conversationName string) *aicommonpb.GeneralMetadata {
	return &aicommonpb.GeneralMetadata{
		ConversationName: conversationName,
		MessageName:      messageName,
		ServiceSource:    "AnalyticsService",
	}
}

func convertKeywordMomentData(input *aicommonpb.KeywordMomentData) *momentpb.KeywordMomentData {
	if input == nil {
		return nil
	}
	output := &momentpb.KeywordMomentData{
		KeywordExpression: convertKeywordExpression(input.KeywordExpression),
	}
	return output
}

func convertKeywordExpression(input *aicommonpb.KeywordMomentData_KeywordExpression) *momentpb.KeywordMomentData_KeywordExpression {
	if input == nil {
		return nil
	}
	output := &momentpb.KeywordMomentData_KeywordExpression{}
	output.Operator = convertKeywordMomentOperator(input.Operator)
	output.Operands = convertKeywordOperands(input.Operands)
	return output
}

func convertKeywordMomentOperator(input aicommonpb.KeywordMomentData_Operator) momentpb.KeywordMomentData_Operator {
	return momentpb.KeywordMomentData_Operator(input)
}

func convertKeywordOperands(input []*aicommonpb.KeywordMomentData_Operand) []*momentpb.KeywordMomentData_Operand {
	if len(input) == 0 {
		return nil
	}
	output := []*momentpb.KeywordMomentData_Operand{}
	for _, op := range input {
		if op == nil {
			continue
		}
		convertedOp := &momentpb.KeywordMomentData_Operand{}
		switch op.Operand.(type) {
		case *aicommonpb.KeywordMomentData_Operand_Expression:
			convertedOp.Operand = &momentpb.KeywordMomentData_Operand_Expression{
				Expression: convertKeywordExpression(op.GetExpression()),
			}
		case *aicommonpb.KeywordMomentData_Operand_Phrase:
			convertedOp.Operand = &momentpb.KeywordMomentData_Operand_Phrase{
				Phrase: op.GetPhrase(),
			}
		}
		output = append(output, convertedOp)
	}
	return output
}
