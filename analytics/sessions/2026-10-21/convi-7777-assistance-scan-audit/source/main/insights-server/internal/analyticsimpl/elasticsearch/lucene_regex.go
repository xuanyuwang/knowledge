package elasticsearch

import (
	"fmt"
	"regexp/syntax"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lucene reserved regexp chars.
const luceneReserved = `\.+*?{}[]()^$|"@&~#<>`

// runeRange represents a range of runes from lo to hi (inclusive).
type runeRange struct {
	lo rune
	hi rune
}

// prepareLuceneRegex converts a user's RE2-like pattern into a Lucene regexp.
//
// Returns the rewritten pattern wrapped with .*(...).*  for substring matching,
// and true if conversion succeeded.
// Returns "", false for:
//   - invalid RE2 syntax
//   - zero-width anchors: ^, $, \A, line anchors
//   - word boundaries: \b, \B
//   - unknown regexp/syntax AST nodes

func prepareLuceneRegex(pattern string) (string, bool) {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return "", false
	}

	translated, ok := translateToLuceneRegex(re)
	if !ok {
		return "", false
	}

	if translated == "#" {
		return "#", true
	}

	// Wrap with .*(...).*  to match anywhere in the field (RE2's default unanchored behavior).
	// Lucene regexps must match the entire field, so this makes them behave like RE2.
	return ".*(" + translated + ").*", true
}

func translateToLuceneRegex(re *syntax.Regexp) (string, bool) {
	var buf strings.Builder
	if !emitLucene(&buf, re) {
		return "", false
	}
	return buf.String(), true
}

func emitLucene(buf *strings.Builder, re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpNoMatch:
		// Lucene empty language.
		buf.WriteByte('#')
		return true

	case syntax.OpEmptyMatch:
		// Lucene empty string.
		buf.WriteString("()")
		return true

	case syntax.OpLiteral:
		// Literal text
		fold := re.Flags&syntax.FoldCase != 0
		for _, r := range re.Rune {
			if fold {
				if !emitFoldedLiteralRune(buf, r) {
					return false
				}
			} else if !emitLuceneLiteralRune(buf, r) {
				return false
			}
		}
		return true

	case syntax.OpCharClass:
		// syntax.Parse already expands \d, \w, \s, Unicode classes, and
		// case-folded classes into rune ranges.
		ranges, ok := flatPairsToRanges(re.Rune)
		if !ok {
			return false
		}
		return emitLuceneCharClass(buf, ranges, true)

	case syntax.OpAnyChar:
		// RE2 (?s). ; Lucene . matches any single character.
		buf.WriteByte('.')
		return true

	case syntax.OpAnyCharNotNL:
		// RE2 plain "." excludes newline. Lucene has no \n escape in regexp
		// syntax, so emit a negated char class containing an actual newline rune.
		return emitNegatedClass(buf, []runeRange{{lo: '\n', hi: '\n'}})

	case syntax.OpBeginText, syntax.OpEndText, syntax.OpBeginLine, syntax.OpEndLine,
		syntax.OpWordBoundary, syntax.OpNoWordBoundary:
		// No Lucene equivalent.
		return false

	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest:
		var suffix string
		switch re.Op {
		case syntax.OpStar:
			suffix = "*"
		case syntax.OpPlus:
			suffix = "+"
		case syntax.OpQuest:
			suffix = "?"
		}
		return emitQuantified(buf, re.Sub[0], suffix)

	case syntax.OpRepeat:
		// Repetition
		var suffix string
		switch {
		case re.Max == -1:
			suffix = fmt.Sprintf("{%d,}", re.Min)
		case re.Min == re.Max:
			suffix = fmt.Sprintf("{%d}", re.Min)
		default:
			suffix = fmt.Sprintf("{%d,%d}", re.Min, re.Max)
		}
		return emitQuantified(buf, re.Sub[0], suffix)

	case syntax.OpConcat:
		// Concatenation
		for _, sub := range re.Sub {
			if needsGrouping(sub, re.Op) {
				buf.WriteByte('(')
				if !emitLucene(buf, sub) {
					return false
				}
				buf.WriteByte(')')
			} else if !emitLucene(buf, sub) {
				return false
			}
		}
		return true

	case syntax.OpAlternate:
		// Alternation
		for i, sub := range re.Sub {
			if i > 0 {
				buf.WriteByte('|')
			}
			if !emitLucene(buf, sub) {
				return false
			}
		}
		return true

	case syntax.OpCapture:
		// Named captures and normal captures both become plain groups.
		buf.WriteByte('(')
		for _, sub := range re.Sub {
			if !emitLucene(buf, sub) {
				return false
			}
		}
		buf.WriteByte(')')
		return true
	}

	return false
}

// emitQuantified writes a quantified expression (e.g., a*, foo+, [abc]{2,5}).
// Adds grouping parens if needed for precedence (e.g., (abc)+ not abc+).
func emitQuantified(buf *strings.Builder, sub *syntax.Regexp, suffix string) bool {
	// Check if sub-expression needs parens for correct precedence.
	if needsGrouping(sub, syntax.OpRepeat) {
		buf.WriteByte('(')
		if !emitLucene(buf, sub) {
			return false
		}
		buf.WriteByte(')')
	} else if !emitLucene(buf, sub) {
		return false
	}

	// Append the quantifier suffix (*, +, ?, {n,m})
	buf.WriteString(suffix)
	return true
}

func needsGrouping(sub *syntax.Regexp, parent syntax.Op) bool {
	switch parent {
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		return (sub.Op == syntax.OpLiteral && len(sub.Rune) > 1) ||
			sub.Op == syntax.OpConcat ||
			sub.Op == syntax.OpAlternate

	case syntax.OpConcat:
		return sub.Op == syntax.OpAlternate
	}

	return false
}

func emitFoldedLiteralRune(buf *strings.Builder, r rune) bool {
	folded := simpleFoldRanges(r)
	if len(folded) == 1 && folded[0].lo == folded[0].hi {
		return emitLuceneLiteralRune(buf, r)
	}

	// RE2/Go uses simple Unicode case folding. Emit the fold set explicitly.
	return emitLuceneCharClass(buf, folded, false)
}

// simpleFoldRanges returns the case-folded equivalents of r as runeRanges.
func simpleFoldRanges(r rune) []runeRange {
	seen := map[rune]struct{}{r: {}}

	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		seen[f] = struct{}{}
	}

	runes := make([]rune, 0, len(seen))
	for f := range seen {
		runes = append(runes, f)
	}

	sort.Slice(runes, func(i, j int) bool {
		return runes[i] < runes[j]
	})

	// Convert sorted runes to ranges by merging consecutive runes.
	ranges := make([]runeRange, 0, len(runes))
	lo, hi := runes[0], runes[0]

	for _, rn := range runes[1:] {
		if rn == hi+1 {
			hi = rn
			continue
		}
		ranges = append(ranges, runeRange{lo: lo, hi: hi})
		lo, hi = rn, rn
	}

	ranges = append(ranges, runeRange{lo: lo, hi: hi})
	return ranges
}

// flatPairsToRanges converts a flat array [lo1, hi1, lo2, hi2, ...] to []runeRange.
// This is only used at the boundary when receiving flat pairs from regexp/syntax.
func flatPairsToRanges(pairs []rune) ([]runeRange, bool) {
	if len(pairs)%2 != 0 {
		return nil, false
	}

	ranges := make([]runeRange, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		lo, hi := pairs[i], pairs[i+1]
		if lo > hi || !validRegexpRune(lo) || !validRegexpRune(hi) {
			return nil, false
		}
		ranges = append(ranges, runeRange{lo: lo, hi: hi})
	}
	return ranges, true
}

func emitLuceneLiteralRune(buf *strings.Builder, r rune) bool {
	if !validRegexpRune(r) {
		return false
	}

	if r <= 0x7f && strings.ContainsRune(luceneReserved, r) {
		buf.WriteByte('\\')
	}

	buf.WriteRune(r)
	return true
}

// emitLuceneCharClass emits a Lucene atom for a character set.
// It may emit:
//   - "#" for the empty set
//   - "." for all Unicode runes
//   - a single literal for a one-character positive class
//   - [...] or [^...] otherwise
func emitLuceneCharClass(buf *strings.Builder, ranges []runeRange, allowSingleLiteral bool) bool {
	ranges = normalizeRanges(ranges)

	if len(ranges) == 0 {
		buf.WriteByte('#')
		return true
	}

	if isAllRunes(ranges) {
		buf.WriteByte('.')
		return true
	}

	if coversBothEnds(ranges) {
		holes := complementRanges(ranges)
		if len(holes) == 0 {
			buf.WriteByte('.')
			return true
		}
		return emitNegatedClass(buf, holes)
	}

	if allowSingleLiteral && len(ranges) == 1 && ranges[0].lo == ranges[0].hi {
		return emitLuceneLiteralRune(buf, ranges[0].lo)
	}

	buf.WriteByte('[')
	emitCharClassContent(buf, ranges)
	buf.WriteByte(']')
	return true
}

func emitNegatedClass(buf *strings.Builder, ranges []runeRange) bool {
	ranges = normalizeRanges(ranges)

	if len(ranges) == 0 {
		buf.WriteByte('.')
		return true
	}

	if isAllRunes(ranges) {
		buf.WriteByte('#')
		return true
	}

	buf.WriteString("[^")
	emitCharClassContent(buf, ranges)
	buf.WriteByte(']')
	return true
}

// normalizeRanges sorts and merges overlapping/adjacent rune ranges.
func normalizeRanges(ranges []runeRange) []runeRange {
	if len(ranges) == 0 {
		return nil
	}

	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].lo == ranges[j].lo {
			return ranges[i].hi < ranges[j].hi
		}
		return ranges[i].lo < ranges[j].lo
	})

	out := make([]runeRange, 0, len(ranges))

	for _, r := range ranges {
		if len(out) == 0 {
			out = append(out, r)
			continue
		}

		last := &out[len(out)-1]
		if r.lo <= last.hi+1 {
			if r.hi > last.hi {
				last.hi = r.hi
			}
			continue
		}

		out = append(out, r)
	}

	return out
}

func validRegexpRune(r rune) bool {
	return r >= 0 && r <= unicode.MaxRune && utf8.ValidRune(r)
}

func isAllRunes(ranges []runeRange) bool {
	return len(ranges) == 1 && ranges[0].lo == 0 && ranges[0].hi == unicode.MaxRune
}

func coversBothEnds(ranges []runeRange) bool {
	return len(ranges) >= 2 && ranges[0].lo == 0 && ranges[len(ranges)-1].hi == unicode.MaxRune
}

// complementRanges computes the complement of rune ranges.
func complementRanges(ranges []runeRange) []runeRange {
	var out []runeRange
	next := rune(0)

	for _, r := range ranges {
		if next < r.lo {
			out = append(out, runeRange{lo: next, hi: r.lo - 1})
		}

		if r.hi == unicode.MaxRune {
			return out
		}

		next = r.hi + 1
	}

	if next <= unicode.MaxRune {
		out = append(out, runeRange{lo: next, hi: unicode.MaxRune})
	}

	return out
}

func emitCharClassContent(buf *strings.Builder, ranges []runeRange) {
	ranges = moveClassSpecialSingletonsToEnd(ranges)

	for _, r := range ranges {
		emitCharClassRange(buf, r.lo, r.hi)
	}
}

// moveClassSpecialSingletonsToEnd reorders ranges to move singleton ] and \ to
// the end for readability.
func moveClassSpecialSingletonsToEnd(ranges []runeRange) []runeRange {
	if len(ranges) <= 1 {
		return ranges
	}

	normal := make([]runeRange, 0, len(ranges))
	special := make([]runeRange, 0, 2)

	for _, r := range ranges {
		if r.lo == r.hi && (r.lo == ']' || r.lo == '\\') {
			special = append(special, r)
		} else {
			normal = append(normal, r)
		}
	}

	return append(normal, special...)
}

// emitCharClassRange writes a character range (or single character) inside a
// character class [...]. If lo == hi, writes a single character. Otherwise,
// writes a range with hyphen: lo-hi.
func emitCharClassRange(buf *strings.Builder, lo, hi rune) {
	if lo == hi {
		emitCharClassRune(buf, lo)
		return
	}

	emitCharClassRune(buf, lo)
	buf.WriteByte('-')
	emitCharClassRune(buf, hi)
}

// emitCharClassRune writes a single rune inside a character class [...].
// Escapes the rune with backslash if it's a Lucene reserved character or hyphen.
// Note: Newline (0x0A) is not escaped here. It's written as a literal byte.
// When the Lucene pattern is later serialized to JSON, the JSON encoder will
// escape it as "\n" in the JSON string, but Lucene receives the actual newline.
func emitCharClassRune(buf *strings.Builder, r rune) {
	if r <= 0x7f && (r == '-' || strings.ContainsRune(luceneReserved, r)) {
		buf.WriteByte('\\')
	}

	buf.WriteRune(r)
}
