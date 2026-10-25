package elasticsearch

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestPrepareLuceneRegex(t *testing.T) {
	tests := []struct {
		desc           string
		pattern        string
		wantTranslated string
		wantOK         bool
	}{
		// Shorthand character class expansion
		{
			desc:           "\\d expands to character class",
			pattern:        `\d{5}`,
			wantTranslated: `.*([0-9]{5}).*`,
			wantOK:         true,
		},
		{
			desc:           "\\D expands to negated character class",
			pattern:        `\D+`,
			wantTranslated: `.*([^0-9]+).*`,
			wantOK:         true,
		},
		{
			desc:           "\\w expands to character class",
			pattern:        `\w+`,
			wantTranslated: `.*([0-9A-Z_a-z]+).*`,
			wantOK:         true,
		},
		{
			desc:           "\\W expands to negated character class",
			pattern:        `\W`,
			wantTranslated: `.*([^0-9A-Z_a-z]).*`,
			wantOK:         true,
		},
		{
			desc:           "\\s expands to whitespace character class",
			pattern:        `\s*hello\s*`,
			wantTranslated: ".*([\t-\n\f-\r ]*hello[\t-\n\f-\r ]*).*",
			wantOK:         true,
		},
		{
			desc:           "\\d inside character class expands inline",
			pattern:        `[\d-]`,
			wantTranslated: `.*([\-0-9]).*`,
			wantOK:         true,
		},
		{
			desc:           "\\w inside character class expands inline",
			pattern:        `[\w.]`,
			wantTranslated: `.*([\.0-9A-Z_a-z]).*`,
			wantOK:         true,
		},
		{
			desc:           "email regex character classes with literal hyphen and period",
			pattern:        `[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`,
			wantTranslated: `.*([%\+\--\.0-9A-Z_a-z]+\@[\--\.0-9A-Za-z]+\.[A-Za-z]{2,}).*`,
			wantOK:         true,
		},
		// Pathological character class tests - hyphen handling
		{
			desc:           "hyphen at start of character class is literal",
			pattern:        `[-abc]`,
			wantTranslated: `.*([\-a-c]).*`,
			wantOK:         true,
		},
		{
			desc:           "hyphen at end of character class is literal",
			pattern:        `[abc-]`,
			wantTranslated: `.*([\-a-c]).*`,
			wantOK:         true,
		},
		{
			desc:           "hyphen alone in character class becomes literal",
			pattern:        `[-]`,
			wantTranslated: `.*(-).*`,
			wantOK:         true,
		},
		{
			desc:           "multiple hyphens mixed with ranges",
			pattern:        `[a-z-0-9]`,
			wantTranslated: `.*([\-0-9a-z]).*`,
			wantOK:         true,
		},
		{
			desc:           "hyphen creates range with adjacent chars",
			pattern:        `[./%]`,
			wantTranslated: `.*([%\.-/]).*`,
			wantOK:         true,
		},
		{
			desc:           "consecutive punctuation becomes escaped range",
			pattern:        `[+,-.]`,
			wantTranslated: `.*([\+-\.]).*`,
			wantOK:         true,
		},
		// Special characters in character classes
		{
			desc:           "closing bracket needs escaping",
			pattern:        `[ab\]cd]`,
			wantTranslated: `.*([a-d\]]).*`,
			wantOK:         true,
		},
		{
			desc:           "caret at start needs escaping in negation",
			pattern:        `[^abc]`,
			wantTranslated: `.*([^a-c]).*`,
			wantOK:         true,
		},
		{
			desc:           "caret not at start is literal",
			pattern:        `[a^bc]`,
			wantTranslated: `.*([\^a-c]).*`,
			wantOK:         true,
		},
		{
			desc:           "backslash in character class",
			pattern:        `[a\\b]`,
			wantTranslated: `.*([a-b\\]).*`,
			wantOK:         true,
		},
		// Range edge cases
		{
			desc:           "single character in brackets becomes literal",
			pattern:        `[a]`,
			wantTranslated: `.*(a).*`,
			wantOK:         true,
		},
		{
			desc:           "non-consecutive characters don't become range",
			pattern:        `[ace]`,
			wantTranslated: `.*([ace]).*`,
			wantOK:         true,
		},
		{
			desc:           "all consecutive chars become single range",
			pattern:        `[abcdef]`,
			wantTranslated: `.*([a-f]).*`,
			wantOK:         true,
		},
		{
			desc:           "overlapping ranges are merged",
			pattern:        `[a-dm-p]`,
			wantTranslated: `.*([a-dm-p]).*`,
			wantOK:         true,
		},
		{
			desc:           "range spanning special chars",
			pattern:        `[!-/]`,
			wantTranslated: `.*([!-/]).*`,
			wantOK:         true,
		},
		{
			desc:           "range from hyphen to digit",
			pattern:        `[--0]`,
			wantTranslated: `.*([\--0]).*`,
			wantOK:         true,
		},
		// Negated character classes
		{
			desc:           "negated class with hyphen",
			pattern:        `[^a-z-]`,
			wantTranslated: `.*([^\-a-z]).*`,
			wantOK:         true,
		},
		{
			desc:           "negated class with range",
			pattern:        `[^a-z]`,
			wantTranslated: `.*([^a-z]).*`,
			wantOK:         true,
		},
		// Empty and minimal classes
		{
			desc:           "two adjacent chars",
			pattern:        `[ab]`,
			wantTranslated: `.*([a-b]).*`,
			wantOK:         true,
		},
		{
			desc:           "two consecutive chars become range",
			pattern:        `[12]`,
			wantTranslated: `.*([1-2]).*`,
			wantOK:         true,
		},
		// Mixed alphanumeric and special chars
		{
			desc:           "URL character class with compressed ASCII ranges",
			pattern:        `[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=-]`,
			wantTranslated: `.*([!\#-\$\&-;=\?-\[_a-z\~\]]).*`,
			wantOK:         true,
			// Note: This test verifies aggressive ASCII range compression.
			// The output contains ranges like \?-\[ which includes @, A-Z, [
		},
		{
			desc:           "password special chars with compressed ASCII ranges",
			pattern:        `[A-Za-z0-9!@#$%^&*()_+\-=\[\]{}|;:'",.<>?/\\]`,
			wantTranslated: `.*([!-_a-\}]).*`,
			wantOK:         true,
			// Note: This test verifies aggressive ASCII range compression.
			// The output !-_ covers many punctuation and uppercase letters.
		},
		// Character classes with escaped sequences
		{
			desc:           "escaped hyphen in middle",
			pattern:        `[a\-z]`,
			wantTranslated: `.*([\-az]).*`,
			wantOK:         true,
		},
		{
			desc:           "escaped period",
			pattern:        `[a\.z]`,
			wantTranslated: `.*([\.az]).*`,
			wantOK:         true,
		},
		{
			desc:           "escaped plus",
			pattern:        `[a\+z]`,
			wantTranslated: `.*([\+az]).*`,
			wantOK:         true,
		},
		// Ranges at boundaries
		{
			desc:           "range from space to exclamation",
			pattern:        `[ -!]`,
			wantTranslated: `.*([ -!]).*`,
			wantOK:         true,
		},
		{
			desc:           "range including all digits and punctuation",
			pattern:        `[0-@]`,
			wantTranslated: `.*([0-\@]).*`,
			wantOK:         true,
		},
		{
			desc:           "range from Z to underscore",
			pattern:        `[Z-_]`,
			wantTranslated: `.*([Z-_]).*`,
			wantOK:         true,
		},
		{
			desc:           "escaped backslash followed by d is literal",
			pattern:        `\\d`,
			wantTranslated: `.*(\\d).*`,
			wantOK:         true,
		},
		{
			desc:           "escaped backslash in middle is literal",
			pattern:        `foo\\bar`,
			wantTranslated: `.*(foo\\bar).*`,
			wantOK:         true,
		},
		// Group syntax translation
		{
			desc:           "non-capturing group is dropped when not needed",
			pattern:        `(?:hello|world)`,
			wantTranslated: `.*(hello|world).*`,
			wantOK:         true,
		},
		{
			desc:           "Python named group strips name",
			pattern:        `(?P<name>foo)`,
			wantTranslated: `.*((foo)).*`,
			wantOK:         true,
		},
		{
			desc:           "PCRE named group strips name",
			pattern:        `(?<name>foo)`,
			wantTranslated: `.*((foo)).*`,
			wantOK:         true,
		},
		{
			desc:           "plain capturing group unchanged",
			pattern:        `(hello|world)`,
			wantTranslated: `.*((hello|world)).*`,
			wantOK:         true,
		},
		// Non-greedy quantifier translation
		{
			desc:           "non-greedy star with dot becomes greedy star",
			pattern:        `foo.*?bar`,
			wantTranslated: ".*(foo[^\n]*bar).*",
			wantOK:         true,
		},
		{
			desc:           "non-greedy plus with dot becomes greedy plus",
			pattern:        `foo.+?bar`,
			wantTranslated: ".*(foo[^\n]+bar).*",
			wantOK:         true,
		},
		{
			desc:           "non-greedy optional becomes greedy optional",
			pattern:        `colou??r`,
			wantTranslated: `.*(colou?r).*`,
			wantOK:         true,
		},
		{
			desc:           "non-greedy repeat becomes greedy repeat",
			pattern:        `a{2,5}?b`,
			wantTranslated: `.*(a{2,5}b).*`,
			wantOK:         true,
		},
		// Pass-through patterns
		{
			desc:           "simple literal passes through unchanged",
			pattern:        `hello world`,
			wantTranslated: `.*(hello world).*`,
			wantOK:         true,
		},
		{
			desc:           "alternation passes through unchanged",
			pattern:        `hello|world`,
			wantTranslated: `.*(hello|world).*`,
			wantOK:         true,
		},
		{
			desc:           "empty alternative becomes empty string branch",
			pattern:        `a|`,
			wantTranslated: `.*(a|()).*`,
			wantOK:         true,
		},
		// Incompatible features — preview disabled
		{
			desc:    "word boundary \\b disables preview",
			pattern: `\bfoo\b`,
			wantOK:  false,
		},
		{
			desc:    "negative word boundary \\B disables preview",
			pattern: `foo\Bbar`,
			wantOK:  false,
		},
		{
			desc:    "start anchor ^ disables preview",
			pattern: `^foo`,
			wantOK:  false,
		},
		{
			desc:    "end anchor $ disables preview",
			pattern: `foo$`,
			wantOK:  false,
		},
		// Features now translated instead of rejected
		{
			desc:           "case-insensitive flag expands into fold classes",
			pattern:        `(?i)abc`,
			wantTranslated: `.*([Aa][Bb][Cc]).*`,
			wantOK:         true,
		},
		{
			desc:           "case-insensitive single char in class",
			pattern:        `(?i)[a]`,
			wantTranslated: `.*([Aa]).*`,
			wantOK:         true,
		},
		{
			desc:           "case-insensitive range in class",
			pattern:        `(?i)[a-c]`,
			wantTranslated: `.*([A-Ca-c]).*`,
			wantOK:         true,
		},
		{
			desc:           "case-insensitive non-ASCII in class",
			pattern:        `(?i)[é]`,
			wantTranslated: `.*([Éé]).*`,
			wantOK:         true,
		},
		// Newline handling verification
		{
			desc:           "plain dot excludes newline - actual newline byte in output",
			pattern:        `.`,
			wantTranslated: ".*([^\n]).*",
			wantOK:         true,
		},
		{
			desc:           "dotall dot matches newline",
			pattern:        `(?s).`,
			wantTranslated: `.*(.).*`,
			wantOK:         true,
		},
		{
			desc:           "plain dot in longer pattern excludes newline",
			pattern:        `foo.bar`,
			wantTranslated: ".*(foo[^\n]bar).*",
			wantOK:         true,
		},
		{
			desc:           "dotall dot in longer pattern includes newline",
			pattern:        `(?s)foo.bar`,
			wantTranslated: `.*(foo.bar).*`,
			wantOK:         true,
		},
		{
			desc:           "empty match becomes Lucene empty string",
			pattern:        `(?:)`,
			wantTranslated: `.*(()).*`,
			wantOK:         true,
		},
		{
			desc:           "no-match character class becomes Lucene empty language",
			pattern:        `[^\s\S]`,
			wantTranslated: `#`,
			wantOK:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// When
			gotTranslated, gotOK := prepareLuceneRegex(tc.pattern)

			// Then
			require.Equal(t, tc.wantOK, gotOK)
			if tc.wantOK {
				require.Equal(t, tc.wantTranslated, gotTranslated)
			}
		})
	}
}

func TestConvertTopicDetailToTopicBool_IncompatibleRegex(t *testing.T) {
	tests := []struct {
		desc              string
		regexExpression   string
		excludeExpression string
	}{
		{
			desc:            "start anchor disables preview",
			regexExpression: `^foo`,
		},
		{
			desc:            "end anchor disables preview",
			regexExpression: `foo$`,
		},
		{
			desc:            "word boundary disables preview",
			regexExpression: `\bfoo\b`,
		},
		{
			desc:              "incompatible exclude expression disables preview",
			regexExpression:   `foo|bar`,
			excludeExpression: `^baz`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			topic := TopicDetails{
				RegexExpression:        tc.regexExpression,
				ExcludeRegexExpression: tc.excludeExpression,
			}

			// When
			_, err := ConvertTopicDetailToTopicBool(topic, nil)

			// Then
			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			require.Equal(t, codes.InvalidArgument, st.Code())
			require.Contains(t, st.Message(), "preview unavailable")
		})
	}
}
