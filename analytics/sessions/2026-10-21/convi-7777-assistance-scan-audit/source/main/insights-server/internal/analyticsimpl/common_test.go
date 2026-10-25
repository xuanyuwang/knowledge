package analyticsimpl

import (
	"fmt"
	"testing"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	intentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/intent"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
)

type ResultImpl struct {
	Attribute *analyticspb.Attribute
}

func (r ResultImpl) GetAttribute() *analyticspb.Attribute {
	return r.Attribute
}

// TestBuildResourceNameHelpers pins buildUserResourceName / buildGroupResourceName
// to the canonical userpb String() output. These helpers assemble the name manually
// to avoid fmt.Sprintf on a hot path, so this guards against the hand-built format
// drifting from the resource-name helpers.
func TestBuildResourceNameHelpers(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		customerID string
		id         string
	}{
		{desc: "typical ids", customerID: "cresta", id: "b5c1c6e518626d55"},
		{desc: "empty id", customerID: "cresta", id: ""},
		{desc: "empty customer id", customerID: "", id: "b5c1c6e518626d55"},
		{desc: "both empty", customerID: "", id: ""},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			wantUser := userpb.UserName{CustomerID: tc.customerID, UserID: tc.id}.String()
			if gotUser := buildUserResourceName(tc.customerID, tc.id); gotUser != wantUser {
				t.Errorf("buildUserResourceName(%q, %q) = %q, want %q", tc.customerID, tc.id, gotUser, wantUser)
			}

			wantGroup := userpb.GroupName{CustomerID: tc.customerID, GroupID: tc.id}.String()
			if gotGroup := buildGroupResourceName(tc.customerID, tc.id); gotGroup != wantGroup {
				t.Errorf("buildGroupResourceName(%q, %q) = %q, want %q", tc.customerID, tc.id, gotGroup, wantGroup)
			}
		})
	}
}

func TestSortResults(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		input []ResultImpl
		want  []ResultImpl
	}{
		{
			desc: "sort by user",
			input: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Users: []*userpb.User{{DisplayName: "name2"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Users: []*userpb.User{{DisplayName: "name1"}}},
				},
			},
			want: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Users: []*userpb.User{{DisplayName: "name1"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Users: []*userpb.User{{DisplayName: "name2"}}},
				},
			},
		},
		{
			desc: "sort by team",
			input: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Teams: []*userpb.Team{{DisplayName: "name2"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Teams: []*userpb.Team{{DisplayName: "name1"}}},
				},
			},
			want: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Teams: []*userpb.Team{{DisplayName: "name1"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Teams: []*userpb.Team{{DisplayName: "name2"}}},
				},
			},
		},
		{
			desc: "sort by intent order",
			input: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 3}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 1}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 2}}},
				},
			},
			want: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 1}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 2}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Order: 3}}},
				},
			},
		},
		{
			desc: "sort by intent description",
			input: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-3"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-1"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-2"}}},
				},
			},
			want: []ResultImpl{
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-1"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-2"}}},
				},
				{
					Attribute: &analyticspb.Attribute{Intents: []*intentpb.Intent{{Description: "desc-3"}}},
				},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			sortResults(tc.input)
			if diff := cmp.Diff(tc.want, tc.input, protocmp.Transform()); diff != "" {
				t.Fatalf("sortResults() returns wrong result with diff (-want +got):\n %v", diff)
			}
		})
	}
}

func TestParseKeywordExpression(t *testing.T) {
	for _, tc := range []struct {
		desc              string
		keywordExpression string
		want              *momentpb.KeywordMomentData_KeywordExpression
		fail              bool
	}{
		{
			desc:              "simple OR",
			keywordExpression: `OR("a", "b")`,
			want: &momentpb.KeywordMomentData_KeywordExpression{
				Operator: momentpb.KeywordMomentData_OR,
				Operands: []*momentpb.KeywordMomentData_Operand{
					{
						Operand: &momentpb.KeywordMomentData_Operand_Phrase{
							Phrase: "a",
						},
					},
					{
						Operand: &momentpb.KeywordMomentData_Operand_Phrase{
							Phrase: "b",
						},
					},
				},
			},
		},
		{
			desc:              "simple AND",
			keywordExpression: `AND("a", "b")`,
			want: &momentpb.KeywordMomentData_KeywordExpression{
				Operator: momentpb.KeywordMomentData_AND,
				Operands: []*momentpb.KeywordMomentData_Operand{
					{
						Operand: &momentpb.KeywordMomentData_Operand_Phrase{
							Phrase: "a",
						},
					},
					{
						Operand: &momentpb.KeywordMomentData_Operand_Phrase{
							Phrase: "b",
						},
					},
				},
			},
		},
		{
			desc:              "And_Squence",
			keywordExpression: `AND(SEQUENCE("a", "b"))`,
			want: &momentpb.KeywordMomentData_KeywordExpression{
				Operator: momentpb.KeywordMomentData_AND,
				Operands: []*momentpb.KeywordMomentData_Operand{
					{
						Operand: &momentpb.KeywordMomentData_Operand_Expression{
							Expression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_SEQUENCE,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "a",
										},
									},
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "b",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			desc:              "3-layer keyword",
			keywordExpression: `AND( SEQUENCE("talk to", OR("wife","husband")), OR("today","tomorrow"))`,
			want: &momentpb.KeywordMomentData_KeywordExpression{
				Operator: momentpb.KeywordMomentData_AND,
				Operands: []*momentpb.KeywordMomentData_Operand{
					{
						Operand: &momentpb.KeywordMomentData_Operand_Expression{
							Expression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_SEQUENCE,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "talk to",
										},
									},
									{
										Operand: &momentpb.KeywordMomentData_Operand_Expression{
											Expression: &momentpb.KeywordMomentData_KeywordExpression{
												Operator: momentpb.KeywordMomentData_OR,
												Operands: []*momentpb.KeywordMomentData_Operand{
													{
														Operand: &momentpb.KeywordMomentData_Operand_Phrase{
															Phrase: "wife",
														},
													},
													{
														Operand: &momentpb.KeywordMomentData_Operand_Phrase{
															Phrase: "husband",
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
					{
						Operand: &momentpb.KeywordMomentData_Operand_Expression{
							Expression: &momentpb.KeywordMomentData_KeywordExpression{
								Operator: momentpb.KeywordMomentData_OR,
								Operands: []*momentpb.KeywordMomentData_Operand{
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "today",
										},
									},
									{
										Operand: &momentpb.KeywordMomentData_Operand_Phrase{
											Phrase: "tomorrow",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	} {
		fmt.Printf("************** %s ************\n", tc.desc)
		got, err := parseKeywordExpression(tc.keywordExpression)
		if tc.fail != (err != nil) {
			t.Fatalf("got error %v, but want error %v", err, tc.fail)
		}
		fmt.Printf("got is: %v\n", got.String())

		diff := cmp.Diff(tc.want, got, protocmp.Transform())
		if diff != "" {
			t.Fatalf("response mismatch with diff (-want, +got): %v", diff)
		}

	}
}

func TestSplitToChildExpresion(t *testing.T) {
	for _, tc := range []struct {
		expression string
		want       []string
		fail       bool
	}{
		{
			expression: `("a")`,
			want:       []string{`"a"`},
		},
		{
			expression: `("a", "b")`,
			want:       []string{`"a"`, `"b"`},
		},
		{
			expression: `(SEQUENCE("a", "b "),"a")`,
			want:       []string{`SEQUENCE("a", "b ")`, `"a"`},
		},
		{
			expression: `("a",SEQUENCE("a", "b "))`,
			want:       []string{`"a"`, `SEQUENCE("a", "b ")`},
		},
		{
			expression: `("a", SEQUENCE("a", OR("a","b")))`,
			want:       []string{`"a"`, `SEQUENCE("a", OR("a","b"))`},
		},
		{
			expression: `("a", AND("a","b"), OR("a","b"), SEQUENCE("a",b),)`,
			want:       []string{`"a"`, `AND("a","b")`, `OR("a","b")`, `SEQUENCE("a",b)`},
		},
		{
			expression: `("a"`,
			fail:       true,
		},
	} {
		got, err := splitToChildExpresion(tc.expression)
		if err != nil != tc.fail {
			t.Fatalf("splitToChildExpresion(%s): got error %v, but expect error %v", tc.expression, err, tc.fail)
		}
		if tc.fail {
			continue
		}
		if diff := cmp.Diff(got, tc.want); diff != "" {
			t.Errorf("splitToChildExpresion(%s) has unexpected results: \ngot %q \nwant %q", tc.expression, got, tc.want)
		}
	}
}
