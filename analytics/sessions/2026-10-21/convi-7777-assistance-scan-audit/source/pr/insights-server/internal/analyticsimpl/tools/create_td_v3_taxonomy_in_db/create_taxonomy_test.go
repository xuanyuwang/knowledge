package main

import (
	"context"
	"fmt"
	"strings"
	"testing"

	customerpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	momentmocks "github.com/cresta/cresta-proto/v2/gen/gomock2/cresta/v1/moment"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/suite"
	"google.golang.org/protobuf/proto"
)

type CreateTaxonomySuite struct {
	suite.Suite
}

func TestCreateTaxonomySuite(t *testing.T) {
	suite.Run(t, new(CreateTaxonomySuite))
}

func (s *CreateTaxonomySuite) TestParentID() {
	s.Run("root has no parent", func() {
		s.Equal("", parentID("0"))
	})

	s.Run("level-1 parent is root", func() {
		s.Equal("0", parentID("1"))
		s.Equal("0", parentID("13"))
	})

	s.Run("level-2 parent is level-1", func() {
		s.Equal("1", parentID("1.1"))
		s.Equal("1", parentID("1.6"))
		s.Equal("13", parentID("13.3"))
	})

	s.Run("level-3 parent is level-2", func() {
		s.Equal("1.2", parentID("1.2.1"))
		s.Equal("1.2", parentID("1.2.5"))
		s.Equal("8.1", parentID("8.1.3"))
	})
}

func (s *CreateTaxonomySuite) TestBuildMoment() {
	s.Run("basic fields", func() {
		// Given
		row := csvRow{id: "1.2", name: "Payment Status", description: "Check payment progress", level: 2}

		// When
		m := buildMoment(row)

		// Then
		s.Equal("Payment Status", m.GetDisplayName())
		s.Equal("Check payment progress", m.GetDescription())
		s.Equal(momentpb.Moment_TOPIC, m.GetType())
		s.Equal(momentpb.Moment_DETAILED_TYPE_CONVERSATION_REASON_V3_TOPIC, m.GetDetailedType())
		s.Equal("td_v3_taxonomy_1.2", m.GetTaxonomy())
		s.Equal("1.2", m.GetTaxonomyReferenceId())
	})

	s.Run("conversation reason v3 payload", func() {
		// Given
		row := csvRow{id: "2.1", name: "Declines", description: "Investigate declined card transactions", level: 2}

		// When
		m := buildMoment(row)

		// Then
		cr := m.GetMomentData().GetTopicMomentData().GetConversationReasonV3()
		s.Require().NotNil(cr)
		s.Equal("Declines", cr.GetTitle())
		s.Equal("Investigate declined card transactions", cr.GetDescription())
		s.Equal(int32(2), cr.GetLevelIdx())
		s.False(cr.GetIsOther())
	})

	s.Run("is_other true when name starts with Other", func() {
		// Given
		row := csvRow{id: "1.6", name: "Other Payments", description: "Other topics", level: 2}

		// When
		m := buildMoment(row)

		// Then
		s.True(m.GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetIsOther())
	})

	s.Run("is_other false when Other is not prefix", func() {
		// Given
		row := csvRow{id: "1.2.4", name: "Timing Status", description: "Check expected arrival", level: 3}

		// When
		m := buildMoment(row)

		// Then
		s.False(m.GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetIsOther())
	})
}

func (s *CreateTaxonomySuite) TestReadCSVRows() {
	// Given/When
	rows, err := readCSVRows()

	// Then
	s.Require().NoError(err)
	s.Require().NotEmpty(rows)

	s.Run("first row is level 1", func() {
		s.Equal("1", rows[0].id)
		s.Equal("Category A", rows[0].name)
		s.Equal(int32(1), rows[0].level)
	})

	s.Run("all rows have non-empty required fields", func() {
		for i, row := range rows {
			s.NotEmpty(row.id, "row %d has empty id", i)
			s.NotEmpty(row.name, "row %d has empty name", i)
			s.NotEmpty(row.description, "row %d has empty description", i)
			s.True(row.level >= 1 && row.level <= 3, "row %d has unexpected level %d", i, row.level)
		}
	})

	s.Run("every non-level-1 row has a parent in the CSV", func() {
		idSet := map[string]bool{}
		for _, row := range rows {
			idSet[row.id] = true
		}
		for _, row := range rows {
			if row.level == 1 {
				continue
			}
			pid := parentID(row.id)
			s.True(idSet[pid], "row %q (level %d) references parent %q which does not exist in CSV", row.id, row.level, pid)
		}
	})
}

func (s *CreateTaxonomySuite) TestLevelGroupingWithRoot() {
	// Given
	rows, err := readCSVRows()
	s.Require().NoError(err)

	rootRow := csvRow{id: "0", name: "TD V3 Root", description: "Root node for TD V3 taxonomy", level: 0}
	rows = append([]csvRow{rootRow}, rows...)

	levelRows := map[int32][]csvRow{}
	for _, r := range rows {
		levelRows[r.level] = append(levelRows[r.level], r)
	}

	// Then
	s.Run("root is at level 0", func() {
		s.Require().Len(levelRows[0], 1)
		s.Equal("0", levelRows[0][0].id)
	})

	s.Run("all levels are populated", func() {
		s.NotEmpty(levelRows[1])
		s.NotEmpty(levelRows[2])
		s.NotEmpty(levelRows[3])
	})

	s.Run("level-1 ids have no dots", func() {
		for _, r := range levelRows[1] {
			s.False(strings.Contains(r.id, "."), "level-1 id %q contains a dot", r.id)
		}
	})

	s.Run("all level-1 parents point to root", func() {
		for _, r := range levelRows[1] {
			s.Equal("0", parentID(r.id))
		}
	})
}

func (s *CreateTaxonomySuite) TestValidateTree() {
	root := csvRow{id: "0", name: "Root", description: "root", level: 0}

	s.Run("valid tree passes", func() {
		// Given
		rows := []csvRow{
			root,
			{id: "1", name: "A", description: "a", level: 1},
			{id: "1.1", name: "B", description: "b", level: 2},
			{id: "2", name: "C", description: "c", level: 1},
		}

		// When/Then
		s.NoError(validateTree(rows))
	})

	s.Run("duplicate id", func() {
		// Given
		rows := []csvRow{
			root,
			{id: "1", name: "A", description: "a", level: 1},
			{id: "1", name: "B", description: "b", level: 1},
		}

		// When/Then
		err := validateTree(rows)
		s.Require().Error(err)
		s.Contains(err.Error(), "duplicate id")
	})

	s.Run("missing parent", func() {
		// Given
		rows := []csvRow{
			root,
			{id: "1.1", name: "A", description: "a", level: 2},
		}

		// When/Then
		err := validateTree(rows)
		s.Require().Error(err)
		s.Contains(err.Error(), "does not exist")
	})

	s.Run("inconsistent level with parent", func() {
		// Given
		rows := []csvRow{
			root,
			{id: "1", name: "A", description: "a", level: 1},
			{id: "1.1", name: "B", description: "b", level: 3},
		}

		// When/Then
		err := validateTree(rows)
		s.Require().Error(err)
		s.Contains(err.Error(), "expected parent level")
	})

	s.Run("no root node", func() {
		// Given
		rows := []csvRow{
			{id: "1", name: "A", description: "a", level: 1},
		}

		// When/Then
		err := validateTree(rows)
		s.Require().Error(err)
		s.Contains(err.Error(), "does not exist")
	})

	s.Run("real CSV passes validation", func() {
		// Given
		rows, err := readCSVRows()
		s.Require().NoError(err)
		rows = append([]csvRow{root}, rows...)

		// When/Then
		s.NoError(validateTree(rows))
	})
}

func (s *CreateTaxonomySuite) TestUpsertTaxonomy() {
	profileObj := customerpb.ProfileName{CustomerID: "test-customer", ProfileID: "test-profile"}

	s.Run("upserts levels in order and wires correct parent template IDs", func() {
		// Given
		ctrl := gomock.NewController(s.T())
		mockClient := momentmocks.NewMockMomentServiceClient(ctrl)

		usecaseNameList := []string{customerpb.UsecaseName{CustomerID: profileObj.CustomerID, ProfileID: profileObj.ProfileID, UsecaseID: "usecase-id"}.String()}
		rows := []csvRow{
			{id: "0", usecaseNames: usecaseNameList, name: "Dummy Root Node", description: "Root node for TD V3 taxonomy", level: 0},
			{id: "1", usecaseNames: usecaseNameList, name: "Payments", description: "Payment topics", level: 1},
			{id: "1.1", usecaseNames: usecaseNameList, name: "Payment Processing", description: "Process payments", level: 2},
			{id: "1.2", usecaseNames: usecaseNameList, name: "Payment Status", description: "Check status", level: 2},
			{id: "1.1.1", usecaseNames: usecaseNameList, name: "Card Processing", description: "Process cards", level: 3},
			{id: "2", usecaseNames: usecaseNameList, name: "Cards", description: "Card topics", level: 1},
		}

		var capturedRequests []*momentpb.BatchUpsertMomentsRequest
		mockClient.EXPECT().
			BatchUpsertMoments(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *momentpb.BatchUpsertMomentsRequest, _ ...any) (*momentpb.BatchUpsertMomentsResponse, error) {
				capturedRequests = append(capturedRequests, req)
				s.Equal(profileObj.String(), req.GetParent())

				var respMoments []*momentpb.Moment
				for _, m := range req.GetMoments() {
					respMoment := proto.Clone(m).(*momentpb.Moment)
					respMoment.Name = fmt.Sprintf("%s/moments/tmpl-%s", profileObj.String(), m.GetTaxonomyReferenceId())
					respMoments = append(respMoments, respMoment)
				}
				return &momentpb.BatchUpsertMomentsResponse{Moments: respMoments}, nil
			}).Times(4)

		var capturedUpdate *momentpb.UpdateMomentRequest
		mockClient.EXPECT().
			UpdateMoment(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *momentpb.UpdateMomentRequest, _ ...any) (*momentpb.UpdateMomentResponse, error) {
				capturedUpdate = req
				return &momentpb.UpdateMomentResponse{}, nil
			}).Times(1)

		// When
		err := upsertTaxonomy(context.Background(), mockClient, profileObj.String(), rows)

		// Then
		s.Require().NoError(err)
		s.Require().Len(capturedRequests, 4)

		// Call 0: root insert — no parent, no RootMomentTemplateId.
		rootInsertReq := capturedRequests[0]
		s.Require().Len(rootInsertReq.GetMoments(), 1)
		s.Empty(rootInsertReq.GetMoments()[0].GetParentMomentTemplateId())
		s.Empty(rootInsertReq.GetMoments()[0].GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetRootMomentTemplateId())

		// Call 1: level-1 moments — parent=tmpl-0, RootMomentTemplateId=tmpl-0.
		level1Req := capturedRequests[1]
		for _, m := range level1Req.GetMoments() {
			s.Equal("tmpl-0", m.GetParentMomentTemplateId(), "level-1 moment %q should have parent tmpl-0", m.GetDisplayName())
			s.Equal("tmpl-0", m.GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetRootMomentTemplateId())
		}

		// Call 2: level-2 moments — parent=tmpl-1, RootMomentTemplateId=tmpl-0.
		level2Req := capturedRequests[2]
		for _, m := range level2Req.GetMoments() {
			s.Equal("tmpl-1", m.GetParentMomentTemplateId(), "level-2 moment %q should have parent tmpl-1", m.GetDisplayName())
			s.Equal("tmpl-0", m.GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetRootMomentTemplateId())
		}

		// Call 3: level-3 moment "1.1.1" — parent=tmpl-1.1, RootMomentTemplateId=tmpl-0.
		level3Req := capturedRequests[3]
		s.Require().Len(level3Req.GetMoments(), 1)
		s.Equal("tmpl-1.1", level3Req.GetMoments()[0].GetParentMomentTemplateId())
		s.Equal("tmpl-0", level3Req.GetMoments()[0].GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetRootMomentTemplateId())

		// Root update via UpdateMoment — sets RootMomentTemplateId to itself and only masks "payload".
		s.Require().NotNil(capturedUpdate)
		s.Equal("tmpl-0", capturedUpdate.GetMoment().GetMomentData().GetTopicMomentData().GetConversationReasonV3().GetRootMomentTemplateId())
		s.Equal([]string{"payload"}, capturedUpdate.GetUpdateMask().GetPaths())
	})

	s.Run("api error is propagated", func() {
		// Given
		ctrl := gomock.NewController(s.T())
		mockClient := momentmocks.NewMockMomentServiceClient(ctrl)

		usecaseNameList := []string{customerpb.UsecaseName{CustomerID: profileObj.CustomerID, ProfileID: profileObj.ProfileID, UsecaseID: "usecase-id"}.String()}
		rows := []csvRow{
			{id: "0", usecaseNames: usecaseNameList, name: "Dummy Root Node", description: "Root node for TD V3 taxonomy", level: 0},
			{id: "1", usecaseNames: usecaseNameList, name: "Payments", description: "Payment topics", level: 1},
		}

		// Root insert succeeds, level-1 fails.
		mockClient.EXPECT().
			BatchUpsertMoments(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, req *momentpb.BatchUpsertMomentsRequest, _ ...any) (*momentpb.BatchUpsertMomentsResponse, error) {
				var respMoments []*momentpb.Moment
				for _, m := range req.GetMoments() {
					respMoment := proto.Clone(m).(*momentpb.Moment)
					respMoment.Name = fmt.Sprintf("%s/moments/tmpl-%s", profileObj.String(), m.GetTaxonomyReferenceId())
					respMoments = append(respMoments, respMoment)
				}
				return &momentpb.BatchUpsertMomentsResponse{Moments: respMoments}, nil
			}).Times(1)
		mockClient.EXPECT().
			BatchUpsertMoments(gomock.Any(), gomock.Any()).
			Return(nil, fmt.Errorf("server unavailable")).Times(1)

		// When
		err := upsertTaxonomy(context.Background(), mockClient, profileObj.String(), rows)

		// Then
		s.Require().Error(err)
		s.Contains(err.Error(), "server unavailable")
	})
}
