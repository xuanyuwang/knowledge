package elasticsearch

import (
	"testing"

	"github.com/cresta/go-servers/shared/consts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertTopicDetailToTopicBool_LanguageCodeFieldSelection(t *testing.T) {
	t.Run("topic with specific lang_code and no request lang filter uses only that language analyzer", func(t *testing.T) {
		// Given
		topicDetails := TopicDetails{
			IncludedPhrases: []Phrase{{Text: "hola"}},
			LanguageCode:    "es-ES",
		}
		requestLangCodes := []string{}

		// When
		topicBool, err := ConvertTopicDetailToTopicBool(topicDetails, requestLangCodes)

		// Then
		require.NoError(t, err)
		require.NotNil(t, topicBool)
		require.NotNil(t, topicBool.Should)

		multiMatchFields := extractMultiMatchFields(t, topicBool)
		assert.Equal(t, []string{"message_text_with_custom_analyzer.spanish"}, multiMatchFields,
			"expected only the Spanish analyzer, but got all analyzers — topic lang_code is not being used when request has no language filter")
	})

	t.Run("topic with all-languages lang_code uses all analyzers", func(t *testing.T) {
		// Given
		topicDetails := TopicDetails{
			IncludedPhrases: []Phrase{{Text: "hello"}},
			LanguageCode:    consts.AllLanguages,
		}
		requestLangCodes := []string{}

		// When
		topicBool, err := ConvertTopicDetailToTopicBool(topicDetails, requestLangCodes)

		// Then
		require.NoError(t, err)
		require.NotNil(t, topicBool)
		require.NotNil(t, topicBool.Should)

		multiMatchFields := extractMultiMatchFields(t, topicBool)
		assert.Greater(t, len(multiMatchFields), 1,
			"expected all analyzers for all-languages topic")
	})

	t.Run("topic with specific lang_code and matching request lang filter uses topic language", func(t *testing.T) {
		// Given
		topicDetails := TopicDetails{
			IncludedPhrases: []Phrase{{Text: "hola"}},
			LanguageCode:    "es-ES",
		}
		requestLangCodes := []string{"es-ES"}

		// When
		topicBool, err := ConvertTopicDetailToTopicBool(topicDetails, requestLangCodes)

		// Then
		require.NoError(t, err)
		require.NotNil(t, topicBool)
		require.NotNil(t, topicBool.Should)

		multiMatchFields := extractMultiMatchFields(t, topicBool)
		assert.Equal(t, []string{"message_text_with_custom_analyzer.spanish"}, multiMatchFields)
	})

	t.Run("topic with specific lang_code and different request lang filter includes both languages", func(t *testing.T) {
		// Given
		topicDetails := TopicDetails{
			IncludedPhrases: []Phrase{{Text: "hello"}},
			LanguageCode:    "es-ES",
		}
		requestLangCodes := []string{"en-US"}

		// When
		topicBool, err := ConvertTopicDetailToTopicBool(topicDetails, requestLangCodes)

		// Then
		require.NoError(t, err)
		require.NotNil(t, topicBool)
		require.NotNil(t, topicBool.Should)

		multiMatchFields := extractMultiMatchFields(t, topicBool)
		assert.Contains(t, multiMatchFields, "message_text_with_custom_analyzer.english")
		assert.Contains(t, multiMatchFields, "message_text_with_custom_analyzer.spanish")
		assert.Len(t, multiMatchFields, 2)
	})
}

func extractMultiMatchFields(t *testing.T, topicBool *TopicBool) []string {
	t.Helper()
	require.NotNil(t, topicBool.Should)
	for _, s := range *topicBool.Should {
		if kw := s.KeywordsToInclude; kw != nil {
			var filters []KeywordMomentBoolFilter
			if kw.Must != nil {
				filters = *kw.Must
			} else if kw.Should != nil {
				filters = *kw.Should
			}
			for _, f := range filters {
				if f.MultiMatch != nil {
					return f.MultiMatch.Fields
				}
			}
		}
	}
	t.Fatal("no multi_match found in TopicBool")
	return nil
}
