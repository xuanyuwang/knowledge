package testing

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/labels"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	"github.com/cresta/go-servers/insights-server/internal/analyticsimpl/elasticsearch"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type FakeMomentServer struct {
	momentpb.MomentServiceClient
	momentMap             map[string]*momentpb.Moment
	prerequirementMoments []*momentpb.Moment
	TopicNames            []string
	TopicDetails          map[string]elasticsearch.TopicDetails
	DeprecatedMoments     []string           // Track deprecated moment names
	UpdatedMoments        []*momentpb.Moment // Track all moments that were updated
}

// Pre-insert some topics (i.e., subcatetories).
func (f *FakeMomentServer) InsertTopics() {
	if f.momentMap == nil {
		f.momentMap = map[string]*momentpb.Moment{}
		f.TopicDetails = map[string]elasticsearch.TopicDetails{}
	}
	for i := 1; i <= 10; i++ {
		includedPhrases := []elasticsearch.Phrase{
			{Text: fmt.Sprintf("included keyword 1 for topic %d", i), SkipStemming: true},
			{Text: fmt.Sprintf("included keyword 2 for topic %d", i), SkipStemming: true},
			{Text: "flexible phrase", Flexible: true},
		}
		startRange, endRange := 5, 10
		in := &momentpb.Moment{
			Name: fmt.Sprintf("incldued-keyword-moment-name-%d", i),
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: includedPhrases[0].Text,
									},
								},
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: includedPhrases[1].Text,
									},
								},
								{
									Operand: &momentpb.KeywordMomentData_Operand_Expression{
										Expression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_SEQUENCE,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "flexible",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "phrase",
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
			Labels: &labels.Labels{
				Labels: map[string]string{
					"prev_pattern": "blah blah",
					"start_range":  strconv.Itoa(startRange),
					"end_range":    strconv.Itoa(endRange),
					"other_label":  "foo bar",
				},
			},
		}

		excludedPhrases := []elasticsearch.Phrase{
			{Text: fmt.Sprintf("excluded keyword 1 for topic %d", i), SkipStemming: true},
			{Text: fmt.Sprintf("excluded keyword 2 for topic %d", i), SkipStemming: true},
			{Text: "sequence words", Flexible: true}}
		ex := &momentpb.Moment{
			Name: fmt.Sprintf("excluded-keyword-moment-name-%d", i),
			Type: momentpb.Moment_KEYWORD,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_KeywordMomentData{
					KeywordMomentData: &momentpb.KeywordMomentData{
						KeywordExpression: &momentpb.KeywordMomentData_KeywordExpression{
							Operator: momentpb.KeywordMomentData_OR,
							Operands: []*momentpb.KeywordMomentData_Operand{
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: excludedPhrases[0].Text,
									},
								},
								{
									Operand: &momentpb.KeywordMomentData_Operand_Phrase{
										Phrase: excludedPhrases[1].Text,
									},
								},
								{
									Operand: &momentpb.KeywordMomentData_Operand_Expression{
										Expression: &momentpb.KeywordMomentData_KeywordExpression{
											Operator: momentpb.KeywordMomentData_SEQUENCE,
											Operands: []*momentpb.KeywordMomentData_Operand{
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "sequence",
													},
												},
												{
													Operand: &momentpb.KeywordMomentData_Operand_Phrase{
														Phrase: "words",
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

		clusters := []*momentpb.Moment{
			{
				Name: fmt.Sprintf("cluster-moment-name-1-for-topic-%d", i),
				Type: momentpb.Moment_TOPIC,
			},
			{
				Name: fmt.Sprintf("cluster-moment-name-2-for-topic-%d", i),
				Type: momentpb.Moment_TOPIC,
			},
		}

		comp := &momentpb.Moment{
			Name: fmt.Sprintf("composite-moment-name-%d", i),
			Type: momentpb.Moment_COMPOSITE,
			MomentData: &momentpb.MomentData{
				Moment: &momentpb.MomentData_CompositeMomentData{
					CompositeMomentData: &momentpb.CompositeMomentData{
						Operator: momentpb.CompositeMomentData_AND,
						Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
							{
								Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
									BaseMoment: ex.Name,
								},
							},
							{
								Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_CompositeMoment{
									CompositeMoment: &momentpb.CompositeMomentData{
										Operator: momentpb.CompositeMomentData_OR,
										Operands: []*momentpb.CompositeMomentData_CompositeMomentOperand{
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: in.Name,
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: clusters[0].Name,
												},
											},
											{
												Operand: &momentpb.CompositeMomentData_CompositeMomentOperand_BaseMoment{
													BaseMoment: clusters[1].Name,
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

		f.momentMap[in.Name] = in
		f.momentMap[ex.Name] = ex
		for _, c := range clusters {
			f.momentMap[c.Name] = c
		}
		f.momentMap[comp.Name] = comp
		f.TopicNames = append(f.TopicNames, comp.Name)

		topicDetail := elasticsearch.TopicDetails{
			IncludedPhrases:          includedPhrases,
			IncludedPhrasesMatchType: elasticsearch.MatchAny,
			ExcludedPhrases:          excludedPhrases,
			ExcludedPhrasesMatchType: elasticsearch.MatchAny,
			KeywordMomentConstraint: &elasticsearch.KeywordMomentConstraint{
				MessageForwardPostionUpperBound:  proto.Int32(int32(startRange)),
				MessageBackwardPostionUpperBound: proto.Int32(int32(endRange)),
			},
		}
		for _, cl := range clusters {
			topicDetail.Clusters = append(topicDetail.Clusters, cl.Name)
		}
		f.TopicDetails[comp.Name] = topicDetail
	}
}

func (f *FakeMomentServer) GetMoment(ctx context.Context, req *momentpb.GetMomentRequest, opts ...grpc.CallOption) (*momentpb.GetMomentResponse, error) {
	m, exists := f.momentMap[req.Name]
	if !exists {
		return nil, fmt.Errorf("moment %q not found", req.Name)
	}
	return &momentpb.GetMomentResponse{
		Moment: m,
	}, nil
}

func (f *FakeMomentServer) BatchGetMoments(ctx context.Context, req *momentpb.BatchGetMomentsRequest, opts ...grpc.CallOption) (*momentpb.BatchGetMomentsResponse, error) {
	ret := []*momentpb.Moment{}
	for _, name := range req.Names {
		m, exists := f.momentMap[name]
		if !exists {
			return nil, fmt.Errorf("moment %q not found", name)
		}
		ret = append(ret, m)

	}
	return &momentpb.BatchGetMomentsResponse{
		Moments: ret,
	}, nil
}

func (f *FakeMomentServer) CreateMoment(ctx context.Context, req *momentpb.CreateMomentRequest, opts ...grpc.CallOption) (*momentpb.CreateMomentResponse, error) {
	if f.momentMap == nil {
		f.momentMap = map[string]*momentpb.Moment{}
	}
	if req == nil || req.Moment == nil || req.Parent == "" {
		return nil, fmt.Errorf("field moment, and parent must be set in request, found %v", req)
	}
	toInsert := proto.Clone(req.Moment).(*momentpb.Moment)
	if toInsert.Name == "" {
		name := fmt.Sprintf("%s/moments/%d", req.Parent, len(f.momentMap))
		toInsert.Name = name
	}
	toInsert.CreateTime = timestamppb.Now()
	toInsert.UpdateTime = timestamppb.Now()
	f.momentMap[toInsert.Name] = toInsert
	return &momentpb.CreateMomentResponse{Moment: toInsert}, nil
}

func (f *FakeMomentServer) UpdateMoment(ctx context.Context, req *momentpb.UpdateMomentRequest, opts ...grpc.CallOption) (*momentpb.UpdateMomentResponse, error) {
	if f.momentMap == nil {
		f.momentMap = map[string]*momentpb.Moment{}
	}
	if req == nil || req.Moment == nil {
		return nil, fmt.Errorf("field moment must be set in request, found %v", req)
	}
	if _, exist := f.momentMap[req.Moment.Name]; !exist {
		return nil, fmt.Errorf("moment %q not found", req.Moment.Name)
	}
	f.momentMap[req.Moment.Name] = req.Moment
	// Track updated moments
	f.UpdatedMoments = append(f.UpdatedMoments, proto.Clone(req.Moment).(*momentpb.Moment))
	return &momentpb.UpdateMomentResponse{Moment: req.Moment}, nil
}

func (f *FakeMomentServer) BatchDeprecateMoments(ctx context.Context, req *momentpb.BatchDeprecateMomentsRequest, opts ...grpc.CallOption) (*momentpb.BatchDeprecateMomentsResponse, error) {
	if f.momentMap == nil {
		f.momentMap = map[string]*momentpb.Moment{}
	}
	if req == nil || len(req.Names) == 0 || req.Parent == "" {
		return nil, fmt.Errorf("field moment, parent must be set in request, found %v", req)
	}
	moments := []*momentpb.Moment{}
	for _, name := range req.Names {
		if _, exist := f.momentMap[name]; !exist {
			return nil, fmt.Errorf("moment %v not found", name)
		}
		f.momentMap[name].TaxonomyState = momentpb.Moment_STATE_DEPRECATED
		moments = append(moments, f.momentMap[name])
		// Track deprecated moments
		f.DeprecatedMoments = append(f.DeprecatedMoments, name)
	}

	return &momentpb.BatchDeprecateMomentsResponse{Moments: moments}, nil
}

func (f *FakeMomentServer) ListMoments(ctx context.Context, req *momentpb.ListMomentsRequest, opts ...grpc.CallOption) (*momentpb.ListMomentsResponse, error) {
	return &momentpb.ListMomentsResponse{Moments: f.prerequirementMoments}, nil
}

func (f *FakeMomentServer) PopulatePrerequirementMoments(ctx context.Context, moments []*momentpb.Moment) {
	f.prerequirementMoments = moments
}
