package analyticsimpl

import (
	"context"
	"fmt"
	"testing"

	labelspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	testingfake "github.com/cresta/go-servers/insights-server/internal/analyticsimpl/testing"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeKeywordMomentWithPhrases(count int) *momentpb.Moment {
	operands := make([]*momentpb.KeywordMomentData_Operand, count)
	for i := range operands {
		operands[i] = &momentpb.KeywordMomentData_Operand{
			Operand: &momentpb.KeywordMomentData_Operand_Phrase{Phrase: fmt.Sprintf("phrase-%d", i)},
		}
	}
	return &momentpb.Moment{
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_KeywordMomentData{
				KeywordMomentData: &momentpb.KeywordMomentData{
					KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
						Operator: momentpb.KeywordMomentData_OR,
						Operands: operands,
					},
				},
			},
		},
	}
}

func TestValidateKeywordPhraseCount(t *testing.T) {
	t.Run("under limit passes", func(t *testing.T) {
		// Given
		original := *maxKeywordPhraseCount
		*maxKeywordPhraseCount = 10
		t.Cleanup(func() { *maxKeywordPhraseCount = original })

		moment := makeKeywordMomentWithPhrases(5)

		// When
		err := validateKeywordPhraseCount(moment)

		// Then
		require.NoError(t, err)
	})

	t.Run("at exactly the limit passes", func(t *testing.T) {
		// Given
		original := *maxKeywordPhraseCount
		*maxKeywordPhraseCount = 10
		t.Cleanup(func() { *maxKeywordPhraseCount = original })

		moment := makeKeywordMomentWithPhrases(10)

		// When
		err := validateKeywordPhraseCount(moment)

		// Then
		require.NoError(t, err)
	})

	t.Run("exceeding limit returns InvalidArgument", func(t *testing.T) {
		// Given
		original := *maxKeywordPhraseCount
		*maxKeywordPhraseCount = 3
		t.Cleanup(func() { *maxKeywordPhraseCount = original })

		moment := makeKeywordMomentWithPhrases(4)

		// When
		err := validateKeywordPhraseCount(moment)

		// Then
		require.Error(t, err)
		s, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, s.Code())
		require.Contains(t, s.Message(), "4")
		require.Contains(t, s.Message(), "3")
	})

	t.Run("limit disabled allows any count", func(t *testing.T) {
		// Given
		original := *maxKeywordPhraseCount
		*maxKeywordPhraseCount = 0
		t.Cleanup(func() { *maxKeywordPhraseCount = original })

		moment := makeKeywordMomentWithPhrases(5000)

		// When
		err := validateKeywordPhraseCount(moment)

		// Then
		require.NoError(t, err)
	})

	t.Run("nil keyword expression passes", func(t *testing.T) {
		// Given
		moment := &momentpb.Moment{
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{},
				},
			},
		}

		// When
		err := validateKeywordPhraseCount(moment)

		// Then
		require.NoError(t, err)
	})
}

func TestMomentHasSpeakerRoleFilterLabel(t *testing.T) {
	t.Run("present non-empty label returns true", func(t *testing.T) {
		m := &momentpb.Moment{Labels: &labelspb.Labels{Labels: map[string]string{speakerRoleFilterLabelKey: "VISITOR"}}}
		require.True(t, momentHasSpeakerRoleFilterLabel(m))
	})

	t.Run("whitespace-only label returns false", func(t *testing.T) {
		m := &momentpb.Moment{Labels: &labelspb.Labels{Labels: map[string]string{speakerRoleFilterLabelKey: "  "}}}
		require.False(t, momentHasSpeakerRoleFilterLabel(m))
	})

	t.Run("missing label returns false", func(t *testing.T) {
		m := &momentpb.Moment{Labels: &labelspb.Labels{Labels: map[string]string{}}}
		require.False(t, momentHasSpeakerRoleFilterLabel(m))
	})

	t.Run("nil moment returns false", func(t *testing.T) {
		require.False(t, momentHasSpeakerRoleFilterLabel(nil))
	})
}

func TestResolveMomentNameFromTemplateID(t *testing.T) {
	t.Run("bare id is qualified with the profile parent", func(t *testing.T) {
		got, err := resolveMomentNameFromTemplateID("customers/customer_id/profiles/profile_id", "folder-1")
		require.NoError(t, err)
		require.Equal(t, "customers/customer_id/profiles/profile_id/moments/folder-1", got)
	})

	t.Run("fully-qualified resource name is returned as-is", func(t *testing.T) {
		full := "customers/customer_id/profiles/profile_id/moments/root-1"
		got, err := resolveMomentNameFromTemplateID("customers/customer_id/profiles/profile_id", full)
		require.NoError(t, err)
		require.Equal(t, full, got)
	})
}

// TestFetchInsightsTopics_InheritsSpeakerRoleFromRootNotImmediateParent verifies the walk
// skips a label-less CMv2 folder and inherits the speaker-role filter from the root (INSI-4175).
func TestFetchInsightsTopics_InheritsSpeakerRoleFromRootNotImmediateParent(t *testing.T) {
	const (
		customerID = "customer_id"
		profileID  = "profile_id"
		parent     = "customers/" + customerID + "/profiles/" + profileID
	)
	compositeName := parent + "/moments/composite-1"
	folderID := "folder-1" // bare ParentMomentTemplateId
	rootID := "root-1"     // bare ParentMomentTemplateId
	folderName := parent + "/moments/" + folderID
	rootName := parent + "/moments/" + rootID
	keywordName := parent + "/moments/keyword-1"

	fakeMomentClient := &testingfake.FakeMomentServer{}
	registry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	s := &AnalyticsServiceImpl{momentClientRegistry: registry, logger: logger.GetLogger()}

	// Root category carries the speaker-role filter label (VOC / visitor-only).
	mustCreate := func(m *momentpb.Moment) {
		t.Helper()
		_, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: m})
		require.NoError(t, err)
	}

	mustCreate(&momentpb.Moment{
		Name: rootName,
		Labels: &labelspb.Labels{Labels: map[string]string{
			speakerRoleFilterLabelKey: "VISITOR",
		}},
	})
	// Folder is an intermediate CMv2 node with NO filter labels; its parent is the root.
	mustCreate(&momentpb.Moment{
		Name:                   folderName,
		ParentMomentTemplateId: rootID,
	})
	mustCreate(&momentpb.Moment{
		Name: keywordName,
		Type: momentpb.Moment_KEYWORD,
	})
	// Composite subcategory's immediate parent is the folder, NOT the root.
	mustCreate(&momentpb.Moment{
		Name:                   compositeName,
		Type:                   momentpb.Moment_COMPOSITE,
		ParentMomentTemplateId: folderID,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_CompositeMomentData{
				CompositeMomentData: &momentpb.CompositeMomentData{
					Operator: momentpb.CompositeMomentData_AND,
					Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
						{
							Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
								CompositeMoment: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: keywordName,
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
	})

	topics, err := s.fetchInsightsTopics(context.Background(), parent, []string{compositeName}, true)
	require.NoError(t, err)
	require.Len(t, topics, 1)

	// The inherited filter must come from the root category, not the folder.
	require.NotNil(t, topics[0].parentMoment, "parentMoment should be resolved by walking past the folder")
	require.Equal(t, rootName, topics[0].parentMoment.Name, "speaker-role filter should be inherited from the root, not the immediate folder parent")
	require.Equal(t, []string{"VISITOR"}, getSpeakerRoleFilterFromLabel(topics[0].parentMoment))
}

// TestFetchInsightsTopics_ImmediateParentHasLabelStopsAtParent verifies the walk stops at
// the immediate parent when it already carries the label (pre-CMv2 flat tree shape).
func TestFetchInsightsTopics_ImmediateParentHasLabelStopsAtParent(t *testing.T) {
	const (
		customerID = "customer_id"
		profileID  = "profile_id"
		parent     = "customers/" + customerID + "/profiles/" + profileID
	)
	compositeName := parent + "/moments/composite-2"
	rootID := "root-2"
	rootName := parent + "/moments/" + rootID
	keywordName := parent + "/moments/keyword-2"

	fakeMomentClient := &testingfake.FakeMomentServer{}
	registry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	s := &AnalyticsServiceImpl{momentClientRegistry: registry, logger: logger.GetLogger()}

	mustCreate := func(m *momentpb.Moment) {
		t.Helper()
		_, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: m})
		require.NoError(t, err)
	}

	// The immediate parent carries the label directly (no folder in between).
	mustCreate(&momentpb.Moment{
		Name: rootName,
		Labels: &labelspb.Labels{Labels: map[string]string{
			speakerRoleFilterLabelKey: "AGENT",
		}},
	})
	mustCreate(&momentpb.Moment{
		Name: keywordName,
		Type: momentpb.Moment_KEYWORD,
	})
	mustCreate(&momentpb.Moment{
		Name:                   compositeName,
		Type:                   momentpb.Moment_COMPOSITE,
		ParentMomentTemplateId: rootID,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_CompositeMomentData{
				CompositeMomentData: &momentpb.CompositeMomentData{
					Operator: momentpb.CompositeMomentData_AND,
					Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
						{
							Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
								CompositeMoment: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: keywordName,
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
	})

	topics, err := s.fetchInsightsTopics(context.Background(), parent, []string{compositeName}, true)
	require.NoError(t, err)
	require.Len(t, topics, 1)
	require.NotNil(t, topics[0].parentMoment)
	require.Equal(t, rootName, topics[0].parentMoment.Name)
	require.Equal(t, []string{"AGENT"}, getSpeakerRoleFilterFromLabel(topics[0].parentMoment))
}

// TestFetchInsightsTopics_NoAncestorHasLabelYieldsNoFilter verifies the walk exhausts the
// chain gracefully (no error, no filter) when no ancestor carries the label.
func TestFetchInsightsTopics_NoAncestorHasLabelYieldsNoFilter(t *testing.T) {
	const (
		customerID = "customer_id"
		profileID  = "profile_id"
		parent     = "customers/" + customerID + "/profiles/" + profileID
	)
	compositeName := parent + "/moments/composite-3"
	folderID := "folder-3"
	rootID := "root-3"
	folderName := parent + "/moments/" + folderID
	rootName := parent + "/moments/" + rootID
	keywordName := parent + "/moments/keyword-3"

	fakeMomentClient := &testingfake.FakeMomentServer{}
	registry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](fakeMomentClient)
	s := &AnalyticsServiceImpl{momentClientRegistry: registry, logger: logger.GetLogger()}

	mustCreate := func(m *momentpb.Moment) {
		t.Helper()
		_, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: m})
		require.NoError(t, err)
	}

	// Root has no filter label; folder has no label and no filter anywhere up the chain.
	mustCreate(&momentpb.Moment{Name: rootName})
	mustCreate(&momentpb.Moment{Name: folderName, ParentMomentTemplateId: rootID})
	mustCreate(&momentpb.Moment{Name: keywordName, Type: momentpb.Moment_KEYWORD})
	mustCreate(&momentpb.Moment{
		Name:                   compositeName,
		Type:                   momentpb.Moment_COMPOSITE,
		ParentMomentTemplateId: folderID,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_CompositeMomentData{
				CompositeMomentData: &momentpb.CompositeMomentData{
					Operator: momentpb.CompositeMomentData_AND,
					Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
						{
							Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
								CompositeMoment: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{
											Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
												BaseMoment: keywordName,
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
	})

	topics, err := s.fetchInsightsTopics(context.Background(), parent, []string{compositeName}, true)
	require.NoError(t, err)
	require.Len(t, topics, 1)
	// No ancestor carries the label, so no speaker-role filter is inherited.
	require.False(t, momentHasSpeakerRoleFilterLabel(topics[0].parentMoment))
	require.Nil(t, getSpeakerRoleFilterFromLabel(topics[0].parentMoment))
}

// ancestorDroppingMomentClient wraps FakeMomentServer, silently omitting any requested
// moment whose name is in dropNames from BatchGetMoments responses (still returning success).
// This simulates a moment service that returns success without a requested ancestor.
type ancestorDroppingMomentClient struct {
	*testingfake.FakeMomentServer
	dropNames map[string]bool
}

func (d *ancestorDroppingMomentClient) BatchGetMoments(ctx context.Context, req *momentpb.BatchGetMomentsRequest, opts ...grpc.CallOption) (*momentpb.BatchGetMomentsResponse, error) {
	filtered := make([]string, 0, len(req.Names))
	for _, n := range req.Names {
		if d.dropNames[n] {
			continue
		}
		filtered = append(filtered, n)
	}
	req.Names = filtered
	return d.FakeMomentServer.BatchGetMoments(ctx, req, opts...)
}

// TestFetchInsightsTopics_MissingAncestorErrors verifies that when the moment service
// omits a requested ancestor, fetchInsightsTopics errors rather than silently returning a
// topic with incomplete inherited filters.
func TestFetchInsightsTopics_MissingAncestorErrors(t *testing.T) {
	const (
		customerID = "customer_id"
		profileID  = "profile_id"
		parent     = "customers/" + customerID + "/profiles/" + profileID
	)
	compositeName := parent + "/moments/composite-4"
	folderID := "folder-4"
	rootID := "root-4"
	folderName := parent + "/moments/" + folderID
	rootName := parent + "/moments/" + rootID
	keywordName := parent + "/moments/keyword-4"

	fake := &testingfake.FakeMomentServer{}
	registry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](&ancestorDroppingMomentClient{
		FakeMomentServer: fake,
		dropNames:        map[string]bool{rootName: true}, // service omits the root ancestor
	})
	s := &AnalyticsServiceImpl{momentClientRegistry: registry, logger: logger.GetLogger()}

	mustCreate := func(m *momentpb.Moment) {
		t.Helper()
		_, err := s.momentClientRegistry.Client(context.Background()).CreateMoment(context.Background(), &momentpb.CreateMomentRequest{Parent: parent, Moment: m})
		require.NoError(t, err)
	}

	mustCreate(&momentpb.Moment{Name: rootName, Labels: &labelspb.Labels{Labels: map[string]string{speakerRoleFilterLabelKey: "VISITOR"}}})
	mustCreate(&momentpb.Moment{Name: folderName, ParentMomentTemplateId: rootID})
	mustCreate(&momentpb.Moment{Name: keywordName, Type: momentpb.Moment_KEYWORD})
	mustCreate(&momentpb.Moment{
		Name:                   compositeName,
		Type:                   momentpb.Moment_COMPOSITE,
		ParentMomentTemplateId: folderID,
		MomentData: &momentpb.MomentData{
			Moment: &momentpb.MomentData_CompositeMomentData{
				CompositeMomentData: &momentpb.CompositeMomentData{
					Operator: momentpb.CompositeMomentData_AND,
					Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
						{
							Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
								CompositeMoment: &momentpb.CompositeMomentData{
									Operator: momentpb.CompositeMomentData_OR,
									Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
										{Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{BaseMoment: keywordName}},
									},
								},
							},
						},
					},
				},
			},
		},
	})

	_, err := s.fetchInsightsTopics(context.Background(), parent, []string{compositeName}, true)
	require.Error(t, err, "missing ancestor should surface an error, not silently degrade")
	require.Contains(t, err.Error(), "ancestor")
}
