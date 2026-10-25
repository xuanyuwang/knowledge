package analyticsimpl

import (
	"fmt"
	"testing"

	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/testing/shared/grpc/clientregistry"
	logger "github.com/cresta/shared-go/framework/log"
	"github.com/stretchr/testify/assert"
)

type CommonMomentTraversalTestSuite struct {
	AnalyticsV2TestSuite
}

func (s *CommonMomentTraversalTestSuite) SetupSuite() {
	s.AnalyticsV2TestSuite.SetupSuite()
}

func TestTraverseMomentTemplateHierarchy(t *testing.T) {
	r := CommonMomentTraversalTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
	r.SetupSuite()

	// Create AnalyticsServiceImpl instance for testing with moment client registry
	mockMomentRegistry := clientregistry.NewTestRegistry[momentpb.MomentServiceClient](r.mockMomentClient)
	analyticsService := &AnalyticsServiceImpl{
		momentClientRegistry: mockMomentRegistry,
		logger:               logger.NoOpsLogger,
	}

	// Create moment template data as moments for mocking
	allMoments := []*momentpb.Moment{
		{
			Name:     "customers/customer_id/profiles/profile_id/moments/root-template",
			Type:     momentpb.Moment_COMPOSITE,
			Usecases: []string{"usecase-1"},
		},
		{
			Name:                   "customers/customer_id/profiles/profile_id/moments/child-1",
			Type:                   momentpb.Moment_COMPOSITE,
			Usecases:               []string{"usecase-1"},
			ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/root-template",
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_AND,
						Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
							{
								Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
									CompositeMoment: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_NOT,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: "customers/customer_id/profiles/profile_id/moments/excluded_keyword_moment_1",
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
		},
		{
			Name:                   "customers/customer_id/profiles/profile_id/moments/child-2",
			Type:                   momentpb.Moment_COMPOSITE,
			Usecases:               []string{"usecase-1"},
			ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/root-template",
		},
		{
			Name:                   "customers/customer_id/profiles/profile_id/moments/grandchild-1",
			Type:                   momentpb.Moment_COMPOSITE,
			Usecases:               []string{"usecase-1"},
			ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/child-2",
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_AND,
						Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
							{
								Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
									CompositeMoment: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: "customers/customer_id/profiles/profile_id/moments/included_keyword_moment_1",
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: "customers/customer_id/profiles/profile_id/moments/included_keyword_moment_2",
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
		},
		{
			Name:                   "customers/customer_id/profiles/profile_id/moments/grandchild-2",
			Type:                   momentpb.Moment_COMPOSITE,
			Usecases:               []string{"usecase-1"},
			ParentMomentTemplateId: "customers/customer_id/profiles/profile_id/moments/child-2",
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_AND,
						Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
							{
								Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
									CompositeMoment: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_AND,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: "customers/customer_id/profiles/profile_id/moments/included_keyword_moment_3",
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: "customers/customer_id/profiles/profile_id/moments/moment_cluster_1",
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
		},
	}

	for _, tc := range []struct {
		desc                string
		momentTemplateID    string
		listMomentsResp     *momentpb.ListMomentsResponse
		expectedDescendants []string
	}{
		{
			desc:             "SimpleHierarchy",
			momentTemplateID: "customers/customer_id/profiles/profile_id/moments/root-template",
			listMomentsResp:  &momentpb.ListMomentsResponse{Moments: allMoments},
			// Should return root and all its descendants
			expectedDescendants: []string{
				"customers/customer_id/profiles/profile_id/moments/root-template",
				"customers/customer_id/profiles/profile_id/moments/child-1",
				"customers/customer_id/profiles/profile_id/moments/child-2",
				"customers/customer_id/profiles/profile_id/moments/grandchild-1",
				"customers/customer_id/profiles/profile_id/moments/grandchild-2",
			},
		},
		{
			desc:             "FromMiddleNode",
			momentTemplateID: "customers/customer_id/profiles/profile_id/moments/child-2",
			listMomentsResp:  &momentpb.ListMomentsResponse{Moments: allMoments},
			// Should return child-2 and its descendants
			expectedDescendants: []string{
				"customers/customer_id/profiles/profile_id/moments/child-2",
				"customers/customer_id/profiles/profile_id/moments/grandchild-1",
				"customers/customer_id/profiles/profile_id/moments/grandchild-2",
			},
		},
		{
			desc:             "LeafNode",
			momentTemplateID: "customers/customer_id/profiles/profile_id/moments/grandchild-1",
			listMomentsResp:  &momentpb.ListMomentsResponse{Moments: allMoments},
			// Should return only itself (no children)
			expectedDescendants: []string{
				"customers/customer_id/profiles/profile_id/moments/grandchild-1",
			},
		},
		{
			desc:                "NoMomentTemplatesFound",
			momentTemplateID:    "customers/customer_id/profiles/profile_id/moments/root-template",
			listMomentsResp:     &momentpb.ListMomentsResponse{Moments: []*momentpb.Moment{}},
			expectedDescendants: []string{},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			descendants, err := analyticsService.TraverseMomentTemplateHierarchy(
				r.ctx,
				tc.momentTemplateID,
				tc.listMomentsResp.Moments,
			)

			r.Require().NoError(err)

			fmt.Printf("descendants: %v\n", descendants)

			assert.ElementsMatch(t, tc.expectedDescendants, descendants)
		})
	}
}
