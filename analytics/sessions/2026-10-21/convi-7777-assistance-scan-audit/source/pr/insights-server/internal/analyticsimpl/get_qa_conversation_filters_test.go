package analyticsimpl

import (
	"database/sql"
	"testing"
	"time"

	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	coachingpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/coaching"
	metadatapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/metadata"
	momentpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/moment"
	qapb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/qa"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/timestamppb"

	dbmodel "github.com/cresta/go-servers/apiserver/sql-schema/gen/model"
	"github.com/cresta/go-servers/shared/qa"

	"github.com/google/go-cmp/cmp"
)

func TestGetQAConversationFilters(t *testing.T) {
	// Given
	momentName := momentpb.MomentName{CustomerID: "customer", ProfileID: "profile", MomentID: "template-moment"}.String()
	templateMomentGroup := &analyticspb.MomentGroup{
		Moments: []*momentpb.Moment{{Name: momentName}},
		MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
			MetadataValue: &metadatapb.MetadataValue{
				Value: &metadatapb.MetadataValue_StringValue{StringValue: "gold"},
			},
		}},
	}
	configJSON, err := protojson.Marshal(&qapb.QAScoreConfig{
		ConversationFilters: &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{templateMomentGroup}},
	})
	require.NoError(t, err)

	templateWithCondition := &dbmodel.ScorecardTemplates{
		Customer:      "customer",
		Profile:       "profile",
		ResourceID:    "template-with-condition",
		Revision:      "rev1",
		Title:         "with-condition",
		QaScoreConfig: sql.NullString{Valid: true, String: string(configJSON)},
	}
	templateWithoutCondition := &dbmodel.ScorecardTemplates{
		Customer:   "customer",
		Profile:    "profile",
		ResourceID: "template-without-condition",
		Revision:   "rev1",
		Title:      "without-condition",
	}
	period := &qa.EvaluationPeriod{
		ConversationEvaluationPeriod: &coachingpb.ConversationEvaluationPeriod{
			StartTime: timestamppb.New(time.Unix(1000, 0)),
			EndTime:   timestamppb.New(time.Unix(2000, 0)),
		},
	}
	// Passing filtered users keeps the audience path out of this test.
	users := []*userpb.User{{Name: "customers/customer/profiles/profile/users/agent1"}}

	t.Run("appends template condition when no task filter attribute (progress path)", func(t *testing.T) {
		// When
		got, err := GetQAConversationFilters(period, users, templateWithCondition, nil)

		// Then
		require.NoError(t, err)
		require.Len(t, got.MomentGroups, 1)
		if diff := cmp.Diff(templateMomentGroup, got.MomentGroups[0], protocmp.Transform()); diff != "" {
			t.Errorf("unexpected template moment group (-want +got):\n%s", diff)
		}
	})

	t.Run("ANDs template condition onto the task's own filter attribute (suggest path)", func(t *testing.T) {
		// Given
		taskMomentGroup := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: momentpb.MomentName{CustomerID: "customer", ProfileID: "profile", MomentID: "task-moment"}.String()}},
		}
		filterAttribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{taskMomentGroup}}

		// When
		got, err := GetQAConversationFilters(period, users, templateWithCondition, filterAttribute)

		// Then
		require.NoError(t, err)
		require.Len(t, got.MomentGroups, 2)
		if diff := cmp.Diff(
			[]*analyticspb.MomentGroup{taskMomentGroup, templateMomentGroup},
			got.MomentGroups,
			protocmp.Transform(),
		); diff != "" {
			t.Errorf("unexpected moment groups (-want +got):\n%s", diff)
		}
	})

	t.Run("template wins over a task filter on the same metadata field", func(t *testing.T) {
		// Given
		disjointTaskGroup := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: momentpb.MomentName{CustomerID: "customer", ProfileID: "profile", MomentID: "task-moment"}.String()}},
		}
		// Same field (MomentID) as the template's condition but a different value —
		// the template's version must win and this task group must be dropped.
		overlappingTaskGroup := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: momentName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{
					Value: &metadatapb.MetadataValue_StringValue{StringValue: "silver"},
				},
			}},
		}
		filterAttribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{disjointTaskGroup, overlappingTaskGroup}}

		// When
		got, err := GetQAConversationFilters(period, users, templateWithCondition, filterAttribute)

		// Then
		require.NoError(t, err)
		if diff := cmp.Diff(
			[]*analyticspb.MomentGroup{disjointTaskGroup, templateMomentGroup},
			got.MomentGroups,
			protocmp.Transform(),
		); diff != "" {
			t.Errorf("unexpected moment groups (-want +got):\n%s", diff)
		}
	})

	t.Run("template wins even when it only negates the shared field", func(t *testing.T) {
		// Given
		templateNegatingGroup := &analyticspb.MomentGroup{
			ExcludedMoments: []*momentpb.Moment{{Name: momentName}},
		}
		negatingConfigJSON, err := protojson.Marshal(&qapb.QAScoreConfig{
			ConversationFilters: &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{templateNegatingGroup}},
		})
		require.NoError(t, err)
		templateNegating := &dbmodel.ScorecardTemplates{
			Customer:      "customer",
			Profile:       "profile",
			ResourceID:    "template-negating",
			Revision:      "rev1",
			Title:         "negating",
			QaScoreConfig: sql.NullString{Valid: true, String: string(negatingConfigJSON)},
		}
		taskGroupOnSameField := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: momentName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{
					Value: &metadatapb.MetadataValue_StringValue{StringValue: "silver"},
				},
			}},
		}
		filterAttribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{taskGroupOnSameField}}

		// When
		got, err := GetQAConversationFilters(period, users, templateNegating, filterAttribute)

		// Then
		require.NoError(t, err)
		if diff := cmp.Diff(
			[]*analyticspb.MomentGroup{templateNegatingGroup},
			got.MomentGroups,
			protocmp.Transform(),
		); diff != "" {
			t.Errorf("unexpected moment groups (-want +got):\n%s", diff)
		}
	})

	t.Run("wildcard groups are never treated as the same field", func(t *testing.T) {
		// Given
		wildcardName := momentpb.MomentName{CustomerID: "customer", ProfileID: "profile", MomentID: "*"}.String()
		templateWildcardGroup := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: wildcardName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "gold"}},
			}},
		}
		wildcardConfigJSON, err := protojson.Marshal(&qapb.QAScoreConfig{
			ConversationFilters: &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{templateWildcardGroup}},
		})
		require.NoError(t, err)
		templateWildcard := &dbmodel.ScorecardTemplates{
			Customer: "customer", Profile: "profile", ResourceID: "template-wildcard", Revision: "rev1", Title: "wildcard",
			QaScoreConfig: sql.NullString{Valid: true, String: string(wildcardConfigJSON)},
		}
		// A task wildcard group with a different value must survive, not be dropped.
		taskWildcardGroup := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: wildcardName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "silver"}},
			}},
		}
		filterAttribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{taskWildcardGroup}}

		// When
		got, err := GetQAConversationFilters(period, users, templateWildcard, filterAttribute)

		// Then
		require.NoError(t, err)
		if diff := cmp.Diff(
			[]*analyticspb.MomentGroup{taskWildcardGroup, templateWildcardGroup},
			got.MomentGroups,
			protocmp.Transform(),
		); diff != "" {
			t.Errorf("unexpected moment groups (-want +got):\n%s", diff)
		}
	})

	t.Run("dedups per field across multiple template fields and keeps non-field task groups", func(t *testing.T) {
		// Given
		secondFieldName := momentpb.MomentName{CustomerID: "customer", ProfileID: "profile", MomentID: "template-moment-2"}.String()
		templateGroup1 := templateMomentGroup
		templateGroup2 := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: secondFieldName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "east"}},
			}},
		}
		multiConfigJSON, err := protojson.Marshal(&qapb.QAScoreConfig{
			ConversationFilters: &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{templateGroup1, templateGroup2}},
		})
		require.NoError(t, err)
		templateMultiField := &dbmodel.ScorecardTemplates{
			Customer: "customer", Profile: "profile", ResourceID: "template-multi", Revision: "rev1", Title: "multi",
			QaScoreConfig: sql.NullString{Valid: true, String: string(multiConfigJSON)},
		}
		// Overlaps both template fields (dropped) + a group with no moment (kept).
		taskOnField1 := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: momentName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "silver"}},
			}},
		}
		taskOnField2 := &analyticspb.MomentGroup{
			Moments: []*momentpb.Moment{{Name: secondFieldName}},
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "west"}},
			}},
		}
		taskWithoutMoment := &analyticspb.MomentGroup{
			MetadataValueAttributes: []*analyticspb.MetadataValueAttribute{{
				MetadataValue: &metadatapb.MetadataValue{Value: &metadatapb.MetadataValue_StringValue{StringValue: "keep-me"}},
			}},
		}
		filterAttribute := &analyticspb.Attribute{MomentGroups: []*analyticspb.MomentGroup{taskOnField1, taskOnField2, taskWithoutMoment}}

		// When
		got, err := GetQAConversationFilters(period, users, templateMultiField, filterAttribute)

		// Then
		require.NoError(t, err)
		if diff := cmp.Diff(
			[]*analyticspb.MomentGroup{taskWithoutMoment, templateGroup1, templateGroup2},
			got.MomentGroups,
			protocmp.Transform(),
		); diff != "" {
			t.Errorf("unexpected moment groups (-want +got):\n%s", diff)
		}
	})

	t.Run("adds no moment groups when the template has no condition", func(t *testing.T) {
		// When
		got, err := GetQAConversationFilters(period, users, templateWithoutCondition, nil)

		// Then
		require.NoError(t, err)
		require.Empty(t, got.MomentGroups)
	})
}
