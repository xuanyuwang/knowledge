package analyticsimpl

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
)

type Number interface {
	int | int32 | int64 | float32 | float64
}

func safeDivideFloat[T Number](a, b T) float32 {
	return float32(safeDivideFloat64(a, b))
}

func safeDivideFloat64[T Number](a, b T) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

type Result interface {
	GetAttribute() *analyticspb.Attribute
}

type Point[T Number] struct {
	X T
	Y T
}

// Represents a linear trend line with the equation y = Slope * x + Intercept
type TrendLine struct {
	Slope     float64
	Intercept float64
}

func sortResults[R Result](results []R) []R {
	sort.Slice(results, func(i, j int) bool {
		if results[i].GetAttribute() == nil || results[j].GetAttribute() == nil {
			return true
		}
		if len(results[i].GetAttribute().Users) != 0 &&
			len(results[j].GetAttribute().Users) != 0 &&
			results[i].GetAttribute().Users[0].FullName !=
				results[j].GetAttribute().Users[0].FullName {
			return results[i].GetAttribute().Users[0].FullName <
				results[j].GetAttribute().Users[0].FullName
		}
		if len(results[i].GetAttribute().Groups) != 0 &&
			len(results[j].GetAttribute().Groups) != 0 &&
			results[i].GetAttribute().Groups[0].DisplayName !=
				results[j].GetAttribute().Groups[0].DisplayName {
			return results[i].GetAttribute().Groups[0].DisplayName <
				results[j].GetAttribute().Groups[0].DisplayName
		}
		if len(results[i].GetAttribute().Intents) != 0 &&
			len(results[j].GetAttribute().Intents) != 0 {
			if results[i].GetAttribute().Intents[0].Order !=
				results[j].GetAttribute().Intents[0].Order {
				return results[i].GetAttribute().Intents[0].Order <
					results[j].GetAttribute().Intents[0].Order
			}
			if results[i].GetAttribute().Intents[0].Description !=
				results[j].GetAttribute().Intents[0].Description {
				return results[i].GetAttribute().Intents[0].Description <
					results[j].GetAttribute().Intents[0].Description
			}
		}
		if len(results[i].GetAttribute().Moments) != 0 &&
			len(results[j].GetAttribute().Moments) != 0 {
			if results[i].GetAttribute().Moments[0].DisplayOrder !=
				results[j].GetAttribute().Moments[0].DisplayOrder {
				return results[i].GetAttribute().Moments[0].DisplayOrder <
					results[j].GetAttribute().Moments[0].DisplayOrder
			}
			if results[i].GetAttribute().Moments[0].DisplayName !=
				results[j].GetAttribute().Moments[0].DisplayName {
				return results[i].GetAttribute().Moments[0].DisplayName <
					results[j].GetAttribute().Moments[0].DisplayName
			}
		}
		if len(results[i].GetAttribute().AttributePolicies) != 0 &&
			len(results[j].GetAttribute().AttributePolicies) != 0 {
			if results[i].GetAttribute().AttributePolicies[0].DisplayName !=
				results[j].GetAttribute().AttributePolicies[0].DisplayName {
				return results[i].GetAttribute().AttributePolicies[0].DisplayName <
					results[j].GetAttribute().AttributePolicies[0].DisplayName
			}
		}
		if len(results[i].GetAttribute().ConversationReasons) != 0 &&
			len(results[j].GetAttribute().ConversationReasons) != 0 &&
			results[i].GetAttribute().ConversationReasons[0].Description !=
				results[j].GetAttribute().ConversationReasons[0].Description {
			return results[i].GetAttribute().ConversationReasons[0].Description <
				results[j].GetAttribute().ConversationReasons[0].Description
		}
		if len(results[i].GetAttribute().ConversationDurationBuckets) != 0 &&
			len(results[j].GetAttribute().ConversationDurationBuckets) != 0 &&
			results[i].GetAttribute().ConversationDurationBuckets[0].Id !=
				results[j].GetAttribute().ConversationDurationBuckets[0].Id {
			return results[i].GetAttribute().ConversationDurationBuckets[0].Id <
				results[j].GetAttribute().ConversationDurationBuckets[0].Id
		}
		return true
	})
	return results
}

// Remove removes r from s
func Remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

const (
	AndString          = "AND"
	OrString           = "OR"
	SequenceString     = "SEQUENCE"
	crestaBotSourceStr = ", 7" // Used to include bot conversations in queries
)

// Parse the keyword expression as free-form string into keyword moment payload
func parseKeywordExpression(keywordExpression string) (*momentpb.KeywordMomentData_KeywordExpression, error) {
	res, err := helper(keywordExpression)
	if err != nil {
		return nil, err
	}
	return res.GetExpression(), nil
}

func helper(keywordExpression string) (*momentpb.KeywordMomentData_Operand, error) {
	keywordExpression = strings.Trim(keywordExpression, " ")
	if len(keywordExpression) == 0 {
		return nil, nil
	}
	handleSubExp := func(subExp string) ([]*momentpb.KeywordMomentData_Operand, error) {
		oprands := []*momentpb.KeywordMomentData_Operand{}
		expressions, err := splitToChildExpresion(subExp)
		if err != nil {
			return nil, err
		}

		for _, exp := range expressions {
			res, err := helper(exp)
			if err != nil {
				return nil, err
			}
			oprands = append(oprands, res)
		}
		return oprands, nil
	}

	var (
		op      momentpb.KeywordMomentData_Operator
		oprands []*momentpb.KeywordMomentData_Operand
		err     error
	)

	switch {
	case keywordExpression[0] == '"':
		// This is already a keyword.
		return &momentpb.KeywordMomentData_Operand{
			Operand: &momentpb.KeywordMomentData_Operand_Phrase{
				Phrase: strings.Trim(keywordExpression, "\""),
			},
		}, nil
	case strings.HasPrefix(keywordExpression, AndString):
		op = momentpb.KeywordMomentData_AND
		oprands, err = handleSubExp(keywordExpression[len(AndString):])
	case strings.HasPrefix(keywordExpression, OrString):
		op = momentpb.KeywordMomentData_OR
		oprands, err = handleSubExp(keywordExpression[len(OrString):])
	case strings.HasPrefix(keywordExpression, SequenceString):
		op = momentpb.KeywordMomentData_SEQUENCE
		oprands, err = handleSubExp(keywordExpression[len(SequenceString):])
	default:
		return nil, fmt.Errorf("invalid keyword expression")
	}
	if err != nil {
		return nil, err
	}

	return &momentpb.KeywordMomentData_Operand{
		Operand: &momentpb.KeywordMomentData_Operand_Expression{
			Expression: &momentpb.KeywordMomentData_KeywordExpression{
				Operator: op,
				Operands: oprands,
			},
		},
	}, nil
}

// in the format of "(...)"
func splitToChildExpresion(exp string) ([]string, error) {
	counter := 0
	res := []string{}

	exp = strings.Trim(exp, " ")
	if exp[0] != '(' || exp[len(exp)-1] != ')' {
		return nil, fmt.Errorf("invalid keyword expression: %v", exp)
	}
	// Remove the pair of brackets at the beginning and the end.
	exp = strings.Trim(exp[1:len(exp)-1], " ")
	idx := 0
	for idx < len(exp) {
		counter++
		if counter > 100 {
			break
		}
		// Ignore empty space between oprands.
		if exp[idx] == ' ' {
			idx++
		}

		switch {
		case exp[idx] == '"':
			currIdx := idx + 1
			for currIdx < len(exp) {
				if exp[currIdx] == '"' {
					res = append(res, exp[idx:currIdx+1])
					// Move currIdx to next ',' or the end.
					for currIdx < len(exp) {
						if exp[currIdx] == ',' {
							break
						}
						currIdx++
					}
					break
				}
				currIdx++
			}
			idx = currIdx + 1
		case strings.HasPrefix(exp[idx:], AndString) || strings.HasPrefix(exp[idx:], OrString) || strings.HasPrefix(exp[idx:], SequenceString):
			// assuming '(' and ')' are not part of a keyword.
			currIdx := idx
			visitedOpenBracket := false
			openBracketCount := 0
			for {
				if visitedOpenBracket && openBracketCount == 0 {
					res = append(res, exp[idx:currIdx])
					// Move currIdx to next ',' or the end.
					for currIdx < len(exp) {
						if exp[currIdx] == ',' {
							break
						}
						currIdx++
					}
					break
				}

				if currIdx >= len(exp) {
					break
				}
				if exp[currIdx] == '(' {
					visitedOpenBracket = true
					openBracketCount++
				}
				if exp[currIdx] == ')' {
					openBracketCount--
				}
				currIdx++
			}
			idx = currIdx + 1
		default:
			return nil, fmt.Errorf("invalid keyword expression: %v", exp)

		}
	}
	return res, nil
}

func safeCastToFloat32(v any) (float32, bool) {
	switch n := v.(type) {
	case float32:
		return n, true
	case float64:
		return float32(n), true
	case int, int8, int16, int32, int64:
		return float32(reflect.ValueOf(n).Int()), true
	case uint, uint8, uint16, uint32, uint64:
		return float32(reflect.ValueOf(n).Uint()), true
	default:
		return 0, false
	}
}

// buildResourceName assembles a two-segment resource name
// ("customers/{customerID}/{collection}/{id}") using a strings.Builder.
//
// This exists purely as an efficiency optimization: it produces the exact same
// output as the userpb resource-name String() helpers but avoids the fmt.Sprintf
// they use, which showed up as a top allocator/CPU cost in profiles when these
// run once per user (and per group membership) over the full profile. Prefer the
// buildUserResourceName / buildGroupResourceName wrappers below on hot paths that
// build resource names in a loop; the canonical userpb helpers remain fine for
// one-off constructions. TestBuildResourceNameHelpers pins these to the userpb
// output so the hand-built format cannot drift.
func buildResourceName(customerID, collection, id string) string {
	const prefix = "customers/"
	var b strings.Builder
	b.Grow(len(prefix) + len(customerID) + 1 + len(collection) + 1 + len(id))
	b.WriteString(prefix)
	b.WriteString(customerID)
	b.WriteByte('/')
	b.WriteString(collection)
	b.WriteByte('/')
	b.WriteString(id)
	return b.String()
}

// buildUserResourceName mirrors userpb.UserName{...}.String().
func buildUserResourceName(customerID, userID string) string {
	return buildResourceName(customerID, "users", userID)
}

// buildGroupResourceName mirrors userpb.GroupName{...}.String().
func buildGroupResourceName(customerID, groupID string) string {
	return buildResourceName(customerID, "groups", groupID)
}
