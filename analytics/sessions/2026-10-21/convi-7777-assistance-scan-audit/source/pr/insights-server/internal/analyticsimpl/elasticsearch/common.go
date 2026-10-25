package elasticsearch

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	logger "github.com/cresta/shared-go/framework/log"
	"google.golang.org/protobuf/proto"
)

const (
	MaxNumberOfUserTerms = 60000
)

// TODO(zhucaiyi): consolidate bool Flexible and SkipStemming into one enum KeywordType {Exact, Flexible}.
type Phrase struct {
	Text     string
	Flexible bool
	// If SkipStemming is true, it will query against `message_text` with `default` analyzer.
	SkipStemming bool
}

type TopicDetails struct {
	IncludedPhrases          []Phrase
	IncludedPhrasesMatchType MatchPharesType
	KeywordMomentConstraint  *KeywordMomentConstraint
	ExcludedPhrases          []Phrase
	ExcludedPhrasesMatchType MatchPharesType
	Clusters                 []string
	// Pre topic moment that act as top level filter.
	PrerequrementMoments []string
	// SpeakerRoles inherited from parent moment.
	SpeakerRoles                  []string
	MessageWindowAsMessageCount   *int32
	MessageWindownAsDurationInMin *int32
	// Language code supported by the topic.
	LanguageCode string
	// Regex fields for regex keyword moments.
	RegexExpression        string
	ExcludeRegexExpression string
	// V3SubtreeClosureTemplateIDs holds, for a grouped-by Topic Discovery V3 category, the template ids
	// of its full subtree closure (the category itself + every descendant); its single bucket OR-matches
	// them all so the series aggregates the whole subtree. Empty for every other moment.
	V3SubtreeClosureTemplateIDs []string
}

// MergeV3ClosuresIntoTopicDetails records each grouped-by V3 category's subtree closure onto the matching
// TopicDetails entry, allocating the map when nil and returning the (possibly new) map.
func MergeV3ClosuresIntoTopicDetails(topicDetails map[string]TopicDetails, closureTemplateIDs map[string][]string) map[string]TopicDetails {
	if len(closureTemplateIDs) == 0 {
		return topicDetails
	}
	if topicDetails == nil {
		topicDetails = make(map[string]TopicDetails, len(closureTemplateIDs))
	}
	for momentName, templateIDs := range closureTemplateIDs {
		details := topicDetails[momentName]
		details.V3SubtreeClosureTemplateIDs = templateIDs
		topicDetails[momentName] = details
	}
	return topicDetails
}

// Extra constraints that applies to messages when matching keyword moments.
type KeywordMomentConstraint struct {
	MessageForwardPostionUpperBound  *int32
	MessageBackwardPostionUpperBound *int32
}

// nolint: gosec
func ConvertKeywordMomentToPhrases(ctx context.Context, logger logger.Logger, m *momentpb.Moment) ([]Phrase, MatchPharesType, *KeywordMomentConstraint) {
	phrases, matchType := []Phrase{}, MatchTypeUnknown
	if m.GetMomentData() != nil && m.GetMomentData().GetKeywordMomentData() != nil && m.GetMomentData().GetKeywordMomentData().GetKeywordExpression() != nil {
		exp := m.GetMomentData().GetKeywordMomentData().GetKeywordExpression()
		if exp.Operator == momentpb.KeywordMomentData_AND {
			matchType = MatchAll
		}
		if exp.Operator == momentpb.KeywordMomentData_OR {
			matchType = MatchAny
		}

		for _, op := range exp.GetOperands() {
			if ph := op.GetPhrase(); ph != "" {
				phrases = append(phrases, Phrase{Text: ph, SkipStemming: true})
			}
			// Handle sequence, ie flexible phrases.
			if op.GetExpression() != nil && op.GetExpression().GetOperator() == momentpb.KeywordMomentData_SEQUENCE {
				sequencePhrases := []string{}
				for _, innerOp := range op.GetExpression().GetOperands() {
					if word := innerOp.GetPhrase(); word != "" {
						sequencePhrases = append(sequencePhrases, word)
					}
				}
				phrases = append(phrases, Phrase{Text: strings.Join(sequencePhrases, " "), Flexible: true})
			}
		}
	}

	constraint := &KeywordMomentConstraint{}
	if m.Labels != nil && m.Labels.Labels != nil {
		startRange, exists := m.Labels.Labels["start_range"]
		if exists {
			val, err := strconv.ParseInt(startRange, 10, 32)
			if err != nil {
				logger.Errorf(ctx, "Failed to parse start_range label %q for moment %v", startRange, m.Name)
			} else {
				constraint.MessageForwardPostionUpperBound = proto.Int32(int32(val))
			}
		}

		endRange, exists := m.Labels.Labels["end_range"]
		if exists {
			val, err := strconv.ParseInt(endRange, 10, 32)
			if err != nil {
				logger.Errorf(ctx, "Failed to parse end_range label %q for moment %v", endRange, m.Name)
			} else {
				constraint.MessageBackwardPostionUpperBound = proto.Int32(int32(val))
			}
		}
	}

	return phrases, matchType, constraint
}

func ExtractRegexFromKeywordMoment(m *momentpb.Moment) (expression, excludeExpression string) {
	regexConfig := m.GetMomentData().GetKeywordMomentData().GetRegexConfig()
	if regexConfig == nil {
		return "", ""
	}
	return regexConfig.GetExpression(), regexConfig.GetExcludeExpression()
}

func IsConversationMetadataMomentGroup(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}
	for _, moment := range mg.Moments {
		if moment.Type == momentpb.Moment_CONVERSATION_METADATA {
			return true
		}
	}
	for _, moment := range mg.ExcludedMoments {
		if moment.Type == momentpb.Moment_CONVERSATION_METADATA {
			return true
		}
	}
	return false
}

func IsConversationOutcomeMomentGroup(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}
	for _, moment := range mg.Moments {
		if moment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
			return true
		}
	}
	for _, moment := range mg.ExcludedMoments {
		if moment.Type == momentpb.Moment_CONVERSATION_OUTCOME {
			return true
		}
	}
	return false
}

func IsConversationReasonMomentGroup(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}

	if mg.MomentType == momentpb.Moment_CONVERSATION_REASON {
		return true
	}
	for _, moment := range mg.Moments {
		if moment.Type == momentpb.Moment_CONVERSATION_REASON {
			return true
		}
	}
	for _, moment := range mg.ExcludedMoments {
		if moment.Type == momentpb.Moment_CONVERSATION_REASON {
			return true
		}
	}
	return false
}

func isConversationReasonMoment(momentType momentpb.Moment_Type, moments []*momentpb.Moment) bool {
	if momentType == momentpb.Moment_CONVERSATION_REASON {
		return true
	}
	for _, m := range moments {
		if m.Type == momentpb.Moment_CONVERSATION_REASON {
			return true
		}
	}

	return false
}

// IsTopicDiscoveryV3Moment reports whether the moment is a Topic Discovery V3 category. These are
// Moment_TOPIC templates carrying the V3 detailed type.
func IsTopicDiscoveryV3Moment(m *momentpb.Moment) bool {
	return m != nil && m.GetDetailedType() == momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC
}

// ContainsTopicDiscoveryV3Moment reports whether any moment in the list is a Topic Discovery V3
// category.
func ContainsTopicDiscoveryV3Moment(moments []*momentpb.Moment) bool {
	for _, m := range moments {
		if IsTopicDiscoveryV3Moment(m) {
			return true
		}
	}
	return false
}

func IsVoicemailMomentGroup(mg *analyticspb.MomentGroup) bool {
	if mg == nil {
		return false
	}
	for _, moment := range mg.Moments {
		if moment.Type == momentpb.Moment_VOICE_MAIL {
			return true
		}
	}
	for _, moment := range mg.ExcludedMoments {
		if moment.Type == momentpb.Moment_VOICE_MAIL {
			return true
		}
	}
	return false
}

func IsInsightsCompositeMoment(m *momentpb.Moment) bool {
	if m == nil || m.Type != momentpb.Moment_COMPOSITE {
		return false
	}
	// return true if app_types field NOT set or APP_TYPE_INSIGHTS is found in app_types field.
	if len(m.AppTypes) == 0 {
		return true
	}
	for _, t := range m.AppTypes {
		if t == momentpb.Moment_APP_TYPE_INSIGHTS {
			return true
		}
	}
	return false
}

// Validates whether the language codes are supported.
func ValidateLanguageCode(languageCodes []string) error {
	var invalidLanguageCodes []string
	for _, languageCode := range languageCodes {
		if _, exists := langCodeToMsgTextField[languageCode]; !exists {
			invalidLanguageCodes = append(invalidLanguageCodes, languageCode)
		}
	}
	if len(invalidLanguageCodes) > 0 {
		return fmt.Errorf(
			"found invalid language codes [%s] among [%s], the supported languages are [%s]",
			strings.Join(invalidLanguageCodes, ","),
			strings.Join(languageCodes, ","),
			strings.Join(listSupportedLangCodes(), ","),
		)
	}
	return nil
}

func MarkHighlightMessageText(hit ResHit) string {
	messageText := hit.Source.MessageText
	// Default
	highlight := ""
	if v := hit.Highlight.MessageText; len(v) > 0 {
		highlight = v[0]
	}
	// Whitespace analyzer.
	if v := hit.Highlight.MessageTextWithWhitespaceAanalyzer; len(v) > 0 {
		highlight = v[0]
	}
	// English with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerEnglish; len(v) > 0 {
		highlight = v[0]
	}
	// Spanish with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerSpanish; len(v) > 0 {
		highlight = v[0]
	}
	// French with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerFrench; len(v) > 0 {
		highlight = v[0]
	}
	// Italian with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerItalian; len(v) > 0 {
		highlight = v[0]
	}
	// German with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerGerman; len(v) > 0 {
		highlight = v[0]
	}
	// Dutch with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerDutch; len(v) > 0 {
		highlight = v[0]
	}
	// Portuguese with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerPortuguese; len(v) > 0 {
		highlight = v[0]
	}
	// Russian with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerRussian; len(v) > 0 {
		highlight = v[0]
	}
	// Thai with custom analyzer.
	if v := hit.Highlight.MessageTextWithCustomAnalyzerThai; len(v) > 0 {
		highlight = v[0]
	}
	return combineHighlightMessageText(messageText, highlight)
}

// combineHighlightMessageText combines the message text with the highlight message text.
// It preserves marked words from both sources and merges consecutive marked words.
func combineHighlightMessageText(messageText string, highlightMessageText string) string {
	if highlightMessageText == "" {
		return messageText
	}

	// Split both texts into words to identify marked positions
	highlightWords := strings.Split(highlightMessageText, " ")
	messageWords := strings.Split(messageText, " ")

	// Build result by combining marks from both sources
	var resultWords []string
	for i := 0; i < len(highlightWords); i++ {
		switch {
		case strings.Contains(highlightWords[i], "<mark>") || strings.Contains(highlightWords[i], "</mark>"):
			resultWords = append(resultWords, highlightWords[i])
		case strings.Contains(messageWords[i], "<mark>") || strings.Contains(messageWords[i], "</mark>"):
			resultWords = append(resultWords, messageWords[i])
		default:
			resultWords = append(resultWords, messageWords[i])
		}
	}

	combined := strings.Join(resultWords, " ")

	// Use regex to merge consecutive marked words
	// Pattern: <mark>word</mark> <mark>word</mark> -> <mark>word word</mark>
	re := regexp.MustCompile(`</mark>\s+<mark>`)
	merged := re.ReplaceAllString(combined, " ")

	return merged
}
