package shared

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	// https://stackoverflow.com/questions/47794740/gos-time-doesnt-work-under-the-docker-image-from-scratch
	_ "time/tzdata"

	internaluserpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/nonpublic/user"
	analyticspb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/analytics"
	behaviorpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/behavior"
	timepb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/common/time"
	conversationpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/conversation"

	"github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/customer"
	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/set"
	"github.com/cresta/shared-go/fn"
	"github.com/cresta/shared-go/framework/client/grpc/registry"
	"github.com/cresta/shared-go/framework/stats"
	"github.com/cresta/shared-go/util/envflag"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/cresta/go-servers/config/shared/grpcclient/config"
	"github.com/cresta/go-servers/shared/auth"
	"github.com/cresta/go-servers/shared/consts"
	"github.com/cresta/go-servers/shared/conversation"
	"github.com/cresta/go-servers/shared/fnutils"
)

const (
	StarPlacerHolder                         = "*"
	DefaultTimeZone                          = "UTC"
	invalidCustomerErrorFormat               = "%v is not a valid parent resource name. The valid format is customers/{customer_id}"
	invalidCustomerProfileErrorFormat        = "%v is not a valid parent resource name. The valid format is customers/{customer_id}/profiles/{profile_id}"
	invalidCustomerProfileUsecaseErrorFormat = "%v is not a valid parent resource name. The valid format is customers/{customer_id}/profiles/{profile_id}/usecases/{usecase_id}"
	invalidCustomerUsecaseErrorFormat        = "%v is not a valid parent resource name. The valid format is customers/{customer_id}/usecases/{usecase_id}"
	DefaultListUsersPageSize                 = 500
	defaultPageSizeForElasticsearch          = 50
	customerIDTagFormat                      = "customer_id:%s"
	profileIDTagFormat                       = "profile_id:%s"
	NoTask                                   = "__no__task__"
)

var (
	CustomerRegex                   = regexp.MustCompile("^customers/([^/]+)$")
	CustomerProfileRegex            = regexp.MustCompile("^customers/([^/]+)/profiles/([^/]+)$")
	CustomerProfileUsecaseRegex     = regexp.MustCompile("^customers/([^/]+)/profiles/([^/]+)/usecases/([^/]+)$")
	CustomerUsecaseRegex            = regexp.MustCompile("^customers/([^/]+)/usecases/([^/]+)$")
	intentFilterCounter             = stats.NewMetric("analytics.filter.intent")
	speakerRoleFilterCounter        = stats.NewMetric("analytics.filter.speaker_role")
	conversationReasonFilterCounter = stats.NewMetric("analytics.filter.conversation_reason")
	getListUsersCacheCounter        = stats.NewMetric("list_users_cache.get.success")
	setListUsersCacheCounter        = stats.NewMetric("list_users_cache.set.success")
)

var DefaultListGroupsPageSize = envflag.Int("DEFAULT_LIST_GROUPS_PAGE_SIZE", 200, "The page size shared.ListGroups uses when calling the user service ListGroups RPC.")

type Environment int

const (
	UnspecifiedEnvironment Environment = iota
	Chat
	Voice
)

type Stage int

const (
	UnspecifiedStage Stage = iota
	Staging
	Prod
)

type Customer struct {
	CustomerID string
}

type CustomerProfile struct {
	CustomerID string
	ProfileID  string
}

func (p CustomerProfile) ToPBProfileName() *customer.ProfileName {
	return &customer.ProfileName{
		CustomerID: p.CustomerID,
		ProfileID:  p.ProfileID,
	}
}

type CustomerProfileUsecase struct {
	CustomerID string
	ProfileID  string
	UsecaseID  string
}

type CustomerUsecase struct {
	CustomerID string
	UsecaseID  string
}

func ParseCustomer(name string) (*Customer, error) {
	matches := CustomerRegex.FindStringSubmatch(name)
	if matches == nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidCustomerErrorFormat, name)
	}
	return &Customer{
		CustomerID: matches[1],
	}, nil
}

func (p Customer) String() string {
	return fmt.Sprintf("customers/%v", p.CustomerID)
}

func ParseCustomerProfile(name string) (*CustomerProfile, error) {
	matches := CustomerProfileRegex.FindStringSubmatch(name)
	if matches == nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidCustomerProfileErrorFormat, name)
	}
	return &CustomerProfile{
		CustomerID: matches[1],
		ProfileID:  matches[2],
	}, nil
}

func ParseCustomerProfileUsecase(name string) (*CustomerProfileUsecase, error) {
	matches := CustomerProfileUsecaseRegex.FindStringSubmatch(name)
	if matches == nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidCustomerProfileUsecaseErrorFormat, name)
	}
	return &CustomerProfileUsecase{
		CustomerID: matches[1],
		ProfileID:  matches[2],
		UsecaseID:  matches[3],
	}, nil
}

func ParseCustomerUsecase(name string) (*CustomerUsecase, error) {
	matches := CustomerUsecaseRegex.FindStringSubmatch(name)
	if matches == nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidCustomerUsecaseErrorFormat, name)
	}
	return &CustomerUsecase{
		CustomerID: matches[1],
		UsecaseID:  matches[2],
	}, nil
}

func (p CustomerProfile) String() string {
	return fmt.Sprintf("customers/%v/profiles/%v", p.CustomerID, p.ProfileID)
}

// Parse the start day of week.
// Returns 1 for SUNDAY, 2 for SATURDAY, ..., 0 for MONDAY.
// Postgres's DATE_TRUNC('week') by default truncates week starting on monday.
// ElasticSearch's date_histogram by default truncates week starting on moneday.
// Default to Sunday if not specified in "metadata".
func ParseStartDayOfWeek(metadata *analyticspb.Metadata, isWeekly bool) int {
	var startDayOfWeek int
	if !isWeekly {
		return 0
	}
	if metadata == nil || &metadata.StartDayOfWeek == analyticspb.Metadata_DAY_OF_WEEK_UNSPECIFIED.Enum() {
		startDayOfWeek = 7
	} else {
		startDayOfWeek = int(metadata.StartDayOfWeek)
	}
	return (8 - startDayOfWeek) % 7
}

func ParseTimeRange(timeRange interface{}, fieldName string) (time.Time, time.Time, error) {
	startTime, endTime := time.Time{}, time.Time{}
	if timeRange == nil || (reflect.ValueOf(timeRange).Kind() == reflect.Ptr && reflect.ValueOf(timeRange).IsNil()) {
		return startTime, endTime, status.Errorf(codes.InvalidArgument, "`%s` must be set.", fieldName)
	}
	if attribute, isAttribute := timeRange.(*analyticspb.TimeRange); isAttribute {
		startTime, endTime = attribute.StartTimestamp.AsTime(), attribute.EndTimestamp.AsTime()
	} else if attribute, isAttribute := timeRange.(*timepb.TimestampRange); isAttribute {
		startTime, endTime = attribute.StartTimestamp.AsTime(), attribute.EndTimestamp.AsTime()
	} else {
		return startTime, endTime, status.Errorf(codes.InvalidArgument, "unsupported `%s` type.", fieldName)
	}
	if startTime.After(endTime) {
		return startTime, endTime, status.Errorf(codes.InvalidArgument, "`start_timestamp` %s must be no latter than `end_timestamp` %s.", startTime, endTime)
	}
	return startTime, endTime, nil
}

func ParseFilterByTimeRange(filterByTimeRange interface{}) (time.Time, time.Time, error) {
	return ParseTimeRange(filterByTimeRange, "filter_by_time_range")
}

func ParseTimeZone(metadata *analyticspb.Metadata) (string, error) {
	if metadata == nil || metadata.TimeZoneId == "" {
		return DefaultTimeZone, nil
	}
	timezone, err := ConvertTimezoneToOffset(metadata.TimeZoneId)
	if err != nil {
		return "", err
	}
	return timezone, nil
}

func ConvertTimezoneToOffset(timezoneID string) (string, error) {
	offsetSecs, err := TimezoneOffsetSecs(timezoneID)
	if err != nil {
		return "", err
	}
	// Format the offset as "-07:00" format
	offsetHours := offsetSecs / 3600
	offsetMinutes := (offsetSecs % 3600) / 60
	return TimezoneOffsetString(offsetHours, offsetMinutes), nil
}

func TimezoneOffsetString(hourOffset, minuteOffset int) string {
	// Format the offset as "-07:00" format
	return fmt.Sprintf("%c%02d:%02d", getSign(hourOffset), abs(hourOffset), minuteOffset)
}

func TimezoneOffsetSecs(timezoneID string) (int, error) {
	loc, err := time.LoadLocation(timezoneID)
	if err != nil {
		return 0, status.Errorf(codes.InvalidArgument, "Unknown `metadata.time_zone_id` %s: %s", timezoneID, err)
	}

	// Get the current time in the specified timezone
	now := time.Now().In(loc)

	// Extract the UTC offset from the time
	_, offset := now.Zone()
	return offset, nil
}

func getSign(value int) rune {
	if value < 0 {
		return '-'
	}
	return '+'
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func ParseFrequency(frequency *analyticspb.Frequency) (string, error) {
	switch *frequency {
	case analyticspb.Frequency_DAILY:
		return "day", nil
	case analyticspb.Frequency_WEEKLY:
		return "week", nil
	case analyticspb.Frequency_MONTHLY:
		return "month", nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "`frequency` must be set.")
	}
}

func SetChatBotAttributeAsGroupByKey(attributeTypes []analyticspb.ChatBotAttributeType, customerID string, groupByKeyValues ...[]string) *analyticspb.ChatBotAttribute {
	if len(attributeTypes) == 0 {
		return &analyticspb.ChatBotAttribute{
			Intents: []*analyticspb.ChatBotAttribute_Intent{{Name: groupByKeyValues[0][0]}},
			Users:   []*userpb.User{{Name: groupByKeyValues[0][0], DisplayName: groupByKeyValues[0][1]}},
		}
	}
	attribute := analyticspb.ChatBotAttribute{}
	for i := 0; i < len(attributeTypes) && i < len(groupByKeyValues); i++ {
		switch attributeTypes[i] {
		case analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_USER:
			user := &userpb.User{
				Name:        userpb.UserName{CustomerID: customerID, UserID: groupByKeyValues[i][0]}.String(),
				DisplayName: groupByKeyValues[i][1],
			}
			attribute.Users = append(attribute.Users, user)
		case analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_INTENT:
			intent := &analyticspb.ChatBotAttribute_Intent{Name: groupByKeyValues[i][0]}
			attribute.Intents = append(attribute.Intents, intent)
		case analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_USER_RESPONSE_TYPE:
			enum, _ := strconv.ParseInt(groupByKeyValues[i][0], 10, 32)
			userResponseType := analyticspb.ChatBotAttribute_UserResponseType(enum)
			attribute.UserResponseTypes = append(attribute.UserResponseTypes, userResponseType)
		case analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_DEFLECTION_TYPE:
			enum, _ := strconv.ParseInt(groupByKeyValues[i][0], 10, 32)
			deflectionType := analyticspb.ChatBotAttribute_DeflectionType(enum)
			attribute.DeflectionTypes = append(attribute.DeflectionTypes, deflectionType)
		case analyticspb.ChatBotAttributeType_CHAT_BOT_ATTRIBUTE_TYPE_VERSION:
			majorVersion, _ := strconv.ParseInt(groupByKeyValues[i][0], 10, 32)
			minorVersion, _ := strconv.ParseInt(groupByKeyValues[i][1], 10, 32)
			revisionVersion, _ := strconv.ParseInt(groupByKeyValues[i][2], 10, 32)
			version := &analyticspb.ChatBotAttribute_Version{
				MajorVersion:    int32(majorVersion),
				MinorVersion:    int32(minorVersion),
				RevisionVersion: int32(revisionVersion),
			}
			attribute.Versions = append(attribute.Versions, version)
		}
	}
	return &attribute
}

func ConvertTimeToUnixEpoch(time time.Time) int64 {
	return time.UnixMilli() / 1000
}

func ConvertTimeToAPITimestamp(time time.Time) *timestamppb.Timestamp {
	return timestamppb.New(time)
}

func ConvertStringToAPITimestamp(timeStr string) (*timestamppb.Timestamp, error) {
	t, err := time.Parse(time.RFC3339, timeStr)
	if err != nil {
		return nil, err
	}
	return timestamppb.New(t), nil
}

func ConvertSecondToAPIDuration(second int64) *durationpb.Duration {
	return durationpb.New(time.Duration(second * int64(time.Second)))
}

func CountFilter(ctx context.Context, filterByAttribute *analyticspb.Attribute) {
	if filterByAttribute == nil {
		return
	}
	if len(filterByAttribute.Intents) != 0 {
		intentFilterCounter.Incr(ctx)
	}
	if len(filterByAttribute.SpeakerRoles) != 0 {
		speakerRoleFilterCounter.Incr(ctx)
	}
	if len(filterByAttribute.ConversationReasons) != 0 {
		conversationReasonFilterCounter.Incr(ctx)
	}
}

// Validate the group by attributes.
func ValidateGroupByAttributes(groupByAttrs []analyticspb.AttributeType, frequency analyticspb.Frequency) error {
	groupByCount := 0
	// We use frequency to represent grouping by time range.
	// The frequency should be set in most default cases.
	if frequency != analyticspb.Frequency_FREQUENCY_UNSPECIFIED {
		groupByCount += 1
	}
	// All the other group-by keys.
	if len(groupByAttrs) > 0 {
		groupByCount += len(groupByAttrs)
		if ContainsAttributeType(groupByAttrs, analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT) &&
			ContainsAttributeType(groupByAttrs, analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP) {
			// We will convert group by groups to agents (and re-assemble results per group).
			groupByCount -= 1
		}
	}
	if groupByCount > 2 {
		return status.Errorf(codes.InvalidArgument, "not supporting more than 2 grouping attribute types with frequency %s and group keys %s.", frequency, groupByAttrs)
	}
	return nil
}

// Check if only the target QAAttributeType(s) are contained in the src
func OnlyContainsQAAttributeTypes(src []analyticspb.QAAttributeType, target ...analyticspb.QAAttributeType) bool {
	return cmp.Equal(
		src,
		target,
		cmpopts.SortSlices(
			func(a, b analyticspb.QAAttributeType) bool {
				return int32(a) < int32(b)
			},
		),
		cmpopts.EquateEmpty(),
	)
}

// Check if only the target AttributeType(s) are contained in the src
func OnlyContainsAttributeTypes(src []analyticspb.AttributeType, target ...analyticspb.AttributeType) bool {
	return cmp.Equal(
		src,
		target,
		cmpopts.SortSlices(
			func(a, b analyticspb.AttributeType) bool {
				return int32(a) < int32(b)
			},
		),
		cmpopts.EquateEmpty(),
	)
}

func ContainsAttributeType(attributeTypes []analyticspb.AttributeType, attributeType analyticspb.AttributeType) bool {
	return slices.Contains(attributeTypes, attributeType)
}

// Validates whether the customerID is correct.
func ValidateCustomerName(envCustomerID, customerID string) error {
	if customerID != envCustomerID {
		return fmt.Errorf("invalid customer ID %s, must be %s", customerID, envCustomerID)
	}
	return nil
}

// Validates whether the customerID and profileID are correct.
func ValidateProfileName(envCustomerID, envProfileID, customerID, profileID string) error {
	if customerID != envCustomerID {
		return fmt.Errorf("invalid customer ID %s, must be %s", customerID, envCustomerID)
	}
	if profileID != envProfileID {
		return fmt.Errorf("invalid profile ID %s, must be %s", profileID, envProfileID)
	}
	return nil
}

func ConvertESToAPISpeakerRole(speakerRole string) (conversationpb.ConversationMessage_SpeakerRole, error) {
	switch speakerRole {
	case conversation.DBSpeakerRoleAgent:
		return conversationpb.ConversationMessage_AGENT, nil
	case conversation.DBSpeakerRoleVisitor:
		return conversationpb.ConversationMessage_VISITOR, nil
	case conversation.DBSpeakerRoleSystem:
		return conversationpb.ConversationMessage_SYSTEM_MESSAGE, nil
	case conversation.DBSpeakerRoleBot:
		return conversationpb.ConversationMessage_BOT, nil
	default:
		return conversationpb.ConversationMessage_ROLE_UNSPECIFIED, status.Errorf(codes.Internal, "Unknown speaker role %v", speakerRole)
	}
}

func ConvertAPIToESSpeakerRoles(speakerRoles []string) ([]string, error) {
	return fn.MapE(speakerRoles, ConvertAPIToESSpeakerRole)
}

func ConvertAPIToESSpeakerRole(speakerRole string) (string, error) {
	switch strings.ToLower(speakerRole) {
	case conversation.DBSpeakerRoleAgent:
		return conversation.DBSpeakerRoleAgent, nil
	case conversation.DBSpeakerRoleVisitor:
		return conversation.DBSpeakerRoleVisitor, nil
	case conversation.DBSpeakerRoleSystem:
		return conversation.DBSpeakerRoleSystem, nil
	case conversation.DBSpeakerRoleBot:
		return conversation.DBSpeakerRoleBot, nil
	default:
		return "", status.Errorf(codes.Internal, "Unknown speaker role %v.", speakerRole)
	}
}

func ParsePageSizeAndPopulateDefaultValue(pageSize int32) int {
	if pageSize == 0 {
		return defaultPageSizeForElasticsearch
	}
	return int(pageSize)
}

func ParseFiltersToUsers(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	groupFilter []*userpb.Group,
	activeUserFilter, includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
) ([]*userpb.User, error) {
	if len(groupFilter) == 0 && !activeUserFilter {
		return nil, status.Errorf(codes.InvalidArgument, "at least one of groupFilter or activeUserFilter must be set")
	}

	internalUserClient, err := internalUserClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get internal user client for customer %v profile %v", customerID, profileID)
	}

	reqFilterGroupIDs := make([]string, len(groupFilter))
	for i, group := range groupFilter {
		groupName, err := userpb.ParseGroupName(group.Name)
		if err != nil {
			return nil, err
		}
		reqFilterGroupIDs[i] = groupName.GroupID
	}

	nextPageOffset := int32(0)
	users := []*userpb.User{}
	for {
		listUsersForAnalyticsReq := &internaluserpb.ListUsersForAnalyticsRequest{
			Parent:                          Customer{CustomerID: customerID}.String(),
			ProfileIds:                      []string{profileID},
			GroupIds:                        reqFilterGroupIDs,
			IncludeIndirectGroupMemberships: !includeDirectGroupMembershipsOnly,
			IncludeInactiveUsers:            true,
			PageOffset:                      nextPageOffset,
			AgentOnly:                       listAgentOnly,
		}
		if activeUserFilter {
			listUsersForAnalyticsReq.IncludeInactiveUsers = false
		}
		listUsers, err := internalUserClient.ListUsersForAnalytics(ctx, listUsersForAnalyticsReq)
		if err != nil {
			return nil, err
		}
		for _, user := range listUsers.Users {
			users = append(users, &userpb.User{
				Name:     userpb.UserName{CustomerID: customerID, UserID: user.UserId}.String(),
				Username: user.Username,
				FullName: user.FullName,
			})
		}
		if listUsers.NextPageOffset == 0 {
			break
		}
		nextPageOffset = listUsers.NextPageOffset
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].FullName < users[j].FullName
	})

	return users, nil
}

func MoveFiltersToUserFilter(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	// only uses filterByAttribute.Users, filterByAttribute.Groups, filterByAttribute.ExcludeDeactivatedUsers
	filterByAttribute *analyticspb.Attribute,
	includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
) (*analyticspb.Attribute, error) {
	// Let's assume that we have
	// - 1) users A, B, C, D
	// - 2) team 1 has user A and team 2 has users B, C, D
	// - 3) user B is de-activated.
	//
	// We have several cases here.
	//	- a) if user filter is empty, all user filters are from the returned `users`
	//	- b) if user filter is non-empty (e.g. filterByAttribute.Users being [A, B]):
	//		- b1) if only group filter (e.g. team 2 => users [B, C, D]) is set,
	//			- filterByAttribute.Users => [A, B]
	//			- users => [B, C, D]
	//			- results should be [A, B, C, D] (append only)
	//		- b2) if only deactivated filter is set,
	//			- filterByAttribute.Users => [A, B]
	//			- users => [A, C, D]
	//			- results should be [A] (filter out only)
	//		- b3) if both group and de-activated user filter set,
	//			- filterByAttribute.Users => [A, B]
	//			- users => [C, D]
	//			- results should be [A, C, D]
	// That is why we might need to separately listUsers below during case b3).
	if len(filterByAttribute.GetUsers()) == 0 {
		// case a) - user filter empty.
		users, err := ParseFiltersToUsers(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			filterByAttribute.GetGroups(),
			filterByAttribute.GetExcludeDeactivatedUsers(),
			includeDirectGroupMembershipsOnly,
			listAgentOnly,
		)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = users
		filterByAttribute.Groups = nil
		return filterByAttribute, nil
	}

	// case b) - user filter non-empty.
	if len(filterByAttribute.GetGroups()) > 0 {
		// case b1) - group filter set
		users, err := ParseFiltersToUsers(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			filterByAttribute.GetGroups(),
			false,
			includeDirectGroupMembershipsOnly,
			listAgentOnly,
		)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = DedupUsers(append(filterByAttribute.Users, users...))
		filterByAttribute.Groups = nil
	}
	if filterByAttribute.GetExcludeDeactivatedUsers() {
		// case b2) - deactivated user filter set
		users, err := ParseFiltersToUsers(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, []*userpb.Group{}, true, includeDirectGroupMembershipsOnly, listAgentOnly)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = FilterUsers(filterByAttribute.Users, users)
	}
	// case b3) - both group filter and deactivated user filter set, this case is already covered as above.
	return filterByAttribute, nil
}

func MoveGroupFilterToUserFilterForQA(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	filterByAttribute *analyticspb.QAAttribute,
	includeDirectGroupMembershipsOnly bool,
	listAgentOnly bool,
) (*analyticspb.QAAttribute, error) {
	// Same mechanism as MoveFiltersToUserFilter()
	// case a) - user filter empty.
	if len(filterByAttribute.GetUsers()) == 0 {
		users, err := ParseFiltersToUsers(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			filterByAttribute.GetGroups(),
			filterByAttribute.GetExcludeDeactivatedUsers(),
			includeDirectGroupMembershipsOnly,
			listAgentOnly,
		)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = users
		filterByAttribute.Groups = nil
		return filterByAttribute, nil
	}

	// case b) - user filter non-empty.
	if len(filterByAttribute.GetGroups()) > 0 {
		// case b1) - group filter set
		users, err := ParseFiltersToUsers(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			filterByAttribute.GetGroups(),
			false,
			includeDirectGroupMembershipsOnly,
			listAgentOnly,
		)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = DedupUsers(append(filterByAttribute.Users, users...))
		filterByAttribute.Groups = nil
	}
	if filterByAttribute.GetExcludeDeactivatedUsers() {
		// case b2) - deactivated user filter set
		users, err := ParseFiltersToUsers(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			[]*userpb.Group{},
			true,
			includeDirectGroupMembershipsOnly,
			listAgentOnly,
		)
		if err != nil {
			return nil, err
		}
		filterByAttribute.Users = FilterUsers(filterByAttribute.Users, users)
	}
	// case b3) - both group filter and deactivated user filter set, this case is already covered as above.
	return filterByAttribute, nil
}

func DedupUsers(users []*userpb.User) []*userpb.User {
	return fnutils.DedupeBy(users, func(u *userpb.User) string { return u.GetName() })
}

func FilterUsers(users []*userpb.User, filteredUsers []*userpb.User) []*userpb.User {
	filteredUserIDs := set.NewFromSlice(fn.Map(filteredUsers, func(u *userpb.User) string { return u.GetName() }))
	return fn.Filter(users, func(u *userpb.User) bool {
		return filteredUserIDs.Has(u.GetName())
	})
}

func ListGroups(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	groups []*userpb.Group,
	onlyFilteredGroups bool,
) ([]*userpb.Group, error) {
	if *useInternalListGroupsForAnalytics {
		return listGroupsForAnalytics(ctx, internalUserClientRegistry, customerID, profileID, groups, onlyFilteredGroups)
	}
	return listGroupsFromPublicUserService(ctx, userClientRegistry, customerID, profileID, groups, onlyFilteredGroups)
}

func listGroupsFromPublicUserService(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	customerID, profileID string,
	groups []*userpb.Group,
	onlyFilteredGroups bool,
) ([]*userpb.Group, error) {
	userClient, err := userClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user client for customer %v profile %v", customerID, profileID)
	}

	listGroupsFilter := &userpb.ListGroupsRequest_ListGroupsFilter{
		CustomerProfiles: []string{CustomerProfile{CustomerID: customerID, ProfileID: profileID}.String()},
		GroupTypes:       []userpb.Group_Type{userpb.Group_TEAM, userpb.Group_DYNAMIC_GROUP},
	}
	for _, group := range groups {
		groupName, err := userpb.ParseGroupName(group.GetName())
		if err != nil {
			return nil, err
		}
		listGroupsFilter.GroupIds = append(listGroupsFilter.GroupIds, groupName.GroupID)
	}
	var (
		groupResults  = []*userpb.Group{}
		nextPageToken string
	)
	for {
		listGroups, err := userClient.ListGroups(
			ctx,
			&userpb.ListGroupsRequest{
				Parent:                          Customer{CustomerID: customerID}.String(),
				PageSize:                        int32(*DefaultListGroupsPageSize),
				PageToken:                       nextPageToken,
				Filter:                          listGroupsFilter,
				IncludeGroupMemberships:         !onlyFilteredGroups && len(groups) > 0,
				IncludeIndirectGroupMemberships: !onlyFilteredGroups && len(groups) > 0,
			},
		)
		if err != nil {
			return nil, err
		}
		// Parse team and child teams.
		for _, g := range listGroups.Groups {
			groupResults = append(groupResults, g)
			for _, m := range g.GetMembers() {
				if m.IsRoot {
					// Skip the `root` group as member.
					continue
				}
				if m.GetGroup() == g.Name && m.MemberType == userpb.GroupMembership_GROUP {
					childGroup := &userpb.Group{
						Name:        userpb.GroupName{CustomerID: customerID, GroupID: m.MemberId}.String(),
						DisplayName: m.MemberDisplayName,
						Type:        userpb.Group_TEAM,
						IsRoot:      false,
					}
					groupResults = append(groupResults, childGroup)
				}
			}
		}
		if listGroups.NextPageToken == "" {
			break
		}
		nextPageToken = listGroups.NextPageToken
	}

	return fnutils.DedupeBy(groupResults, func(g *userpb.Group) string { return g.GetName() }), nil
}

func ListUsersMappedToGroups(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	reqFilterGroups []*userpb.Group,
	hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly bool,
	enableListUsersCache bool,
	listUsersCache ListUsersCache,
	listAgentOnly bool,
) (map[string][]string, []*userpb.Group, []*userpb.User, error) {
	var (
		usersToGroups    = make(map[string][]string)
		users            = []*userpb.User{}
		groupNameToGroup = make(map[string]*userpb.Group)
	)
	if enableListUsersCache && len(reqFilterGroups) == 0 {
		usersToGroups, groups, users, exists := listUsersCache.GetListUsersResp(ctx, customerID, profileID, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, listAgentOnly)
		if exists {
			getListUsersCacheCounter.Incr(ctx, withTags(customerID, profileID)...)
			return usersToGroups, groups, users, nil
		}
	}

	// We always need to list groups and filter on customer profiles of the listed groups.
	//
	// As of 10/08/2024, we noticed a group in holidayinn-transfers-voice owning other holidayinn
	// profiles' agents. In this case, we cannot safely assume the empty group filter means all
	// groups. Instead it should be the groups only belonging to the specific customer profile
	// from Director page.
	groups, err := ListGroups(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, reqFilterGroups, false)
	if err != nil {
		return nil, nil, nil, err
	}

	filterTeamGroupNames := []string{}
	for _, group := range groups {
		if group.Type != userpb.Group_TEAM {
			continue
		}
		filterTeamGroupNames = append(filterTeamGroupNames, group.Name)
	}

	internalUserClient, err := internalUserClientRegistry.CreateClient(customerID, profileID)
	if err != nil {
		return nil, nil, nil, status.Errorf(codes.Internal, "failed to get internal user client for customer %v profile %v", customerID, profileID)
	}
	reqFilterGroupIDs := []string{}
	for _, group := range reqFilterGroups {
		groupName, err := userpb.ParseGroupName(group.Name)
		if err != nil {
			return nil, nil, nil, err
		}
		reqFilterGroupIDs = append(reqFilterGroupIDs, groupName.GroupID)
	}
	var nextPageOffset int32
	for {
		listUsersForAnalyticsReq := &internaluserpb.ListUsersForAnalyticsRequest{
			Parent:                          Customer{CustomerID: customerID}.String(),
			ProfileIds:                      []string{profileID},
			GroupIds:                        reqFilterGroupIDs,
			AgentOnly:                       listAgentOnly,
			IncludeIndirectGroupMemberships: !includeDirectGroupMembershipsOnly,
			IncludeInactiveUsers:            true,
			PageOffset:                      nextPageOffset,
		}
		listUsers, err := internalUserClient.ListUsersForAnalytics(ctx, listUsersForAnalyticsReq)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, user := range listUsers.Users {
			userName := userpb.UserName{CustomerID: customerID, UserID: user.UserId}.String()
			for _, membership := range user.Memberships {
				groupID := membership.Group.GroupId
				if !hasAgentAsGroupByKey && groupID == consts.RootGroupID {
					// Skip the root group for team leaderboard.
					continue
				}
				if !hasAgentAsGroupByKey && groupID == consts.DefaultGroupID {
					// Skip the default group for team leaderboard.
					continue
				}
				if (hasAgentAsGroupByKey || includeDirectGroupMembershipsOnly) && !membership.IsDirectMember {
					// Skip the indirect group for agent leaderboard.
					continue
				}
				if membership.Group.GroupType != internaluserpb.LiteGroup_TEAM {
					// Skip when a group is not team, i.e. when a group is virtual group
					continue
				}

				groupName := userpb.GroupName{CustomerID: customerID, GroupID: groupID}.String()
				usersToGroups[userName] = append(usersToGroups[userName], groupName)
				// If we have a group key by agent, we should return the team group for the agent.
				// Otherwise, when we have a non-empty team filter, we should only return a subset of
				// groups from user's group memberships; otherwise, it might return some
				// unnecessary groups.
				if hasAgentAsGroupByKey || len(filterTeamGroupNames) == 0 || slices.Contains(filterTeamGroupNames, groupName) {
					groupNameToGroup[groupName] = &userpb.Group{
						Name:        groupName,
						DisplayName: membership.Group.DisplayName,
						Type:        userpb.Group_TEAM,
						IsRoot:      false,
					}
				}
			}
			users = append(users, &userpb.User{
				Name:     userName,
				Username: user.Username,
				FullName: user.FullName,
			})
		}
		if listUsers.NextPageOffset == 0 {
			break
		}
		nextPageOffset = listUsers.NextPageOffset
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].FullName < users[j].FullName
	})

	if enableListUsersCache && len(reqFilterGroups) == 0 {
		listUsersCache.SetListUsersResp(ctx, customerID, profileID, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, listAgentOnly, usersToGroups, fn.MapValueToSlice(groupNameToGroup), users)
		setListUsersCacheCounter.Incr(ctx, withTags(customerID, profileID)...)
	}
	return usersToGroups, fn.MapValueToSlice(groupNameToGroup), users, nil
}

func withTags(customerID, profileID string) []string {
	return []string{
		fmt.Sprintf(customerIDTagFormat, customerID),
		fmt.Sprintf(profileIDTagFormat, profileID),
	}
}

func UpdateMomentGroupsWithAllBehaviors(attribute *analyticspb.Attribute) error {
	if attribute == nil || len(attribute.MomentGroups) == 0 {
		return nil
	}
	newMomentGroups := make([]*analyticspb.MomentGroup, 0, len(attribute.MomentGroups))
	for _, momentGroup := range attribute.MomentGroups {
		if len(momentGroup.Behaviors) > 0 && momentGroup.BooleanLogicOperator == analyticspb.MomentGroup_BOOLEAN_LOGIC_OPERATOR_AND {
			if len(momentGroup.AdherenceTypesForMoments) > 0 && len(momentGroup.AdherenceTypesForMoments) != len(momentGroup.Behaviors) {
				return status.Error(codes.InvalidArgument, "The lengths between Behaviors and AdherenceTypesForMoments mismatch.")
			}
			for i, behavior := range momentGroup.Behaviors {
				newMomentGroup := &analyticspb.MomentGroup{
					Behaviors: []*behaviorpb.Behavior{behavior},
				}
				if len(momentGroup.AdherenceTypesForMoments) > 0 {
					newMomentGroup.AdherenceTypesForMoments = []*analyticspb.MomentGroup_MomentAdherenceTypeConfig{momentGroup.AdherenceTypesForMoments[i]}
				}
				newMomentGroups = append(newMomentGroups, newMomentGroup)
			}
		} else {
			newMomentGroups = append(newMomentGroups, momentGroup)
		}
	}
	attribute.MomentGroups = newMomentGroups
	return nil
}

func BuildResourceACLOptions(users []*userpb.User, groups []*userpb.Group, includePeerUserStats bool) ([]auth.ResourceACLOption, error) {
	userIDs, groupIDs, err := ConvertUsersAndGroupsToIDs(users, groups)
	if err != nil {
		return nil, err
	}
	aclOptions := []auth.ResourceACLOption{}
	if len(userIDs) > 0 || len(groupIDs) == 0 {
		aclOptions = append(aclOptions,
			auth.IncludeManagedUsers(),
			auth.WithFilteredUserIDs(userIDs))
	}
	if len(groupIDs) > 0 {
		aclOptions = append(aclOptions,
			auth.IncludeManagedGroups(),
			auth.WithFilteredGroupIDs(groupIDs))
	}
	if includePeerUserStats {
		aclOptions = append(aclOptions, auth.IncludePeerUsers())
	}
	return aclOptions, nil
}

func ConvertUsersAndGroupsToIDs(users []*userpb.User, groups []*userpb.Group) ([]string, []string, error) {
	userIDs, err := ConvertUsersToUserIDs(users)
	if err != nil {
		return nil, nil, err
	}
	groupIDs, err := ConvertGroupsToGroupIDs(groups)
	if err != nil {
		return nil, nil, err
	}
	return userIDs, groupIDs, nil
}

func ConvertUsersToUserIDs(users []*userpb.User) ([]string, error) {
	return fn.MapE(users, func(user *userpb.User) (string, error) {
		userName, err := userpb.ParseUserName(user.GetName())
		if err != nil {
			return "", err
		}
		return userName.UserID, nil
	})
}

func ConvertGroupsToGroupIDs(groups []*userpb.Group) ([]string, error) {
	return fn.MapE(groups, func(group *userpb.Group) (string, error) {
		groupName, err := userpb.ParseGroupName(group.GetName())
		if err != nil {
			return "", err
		}
		return groupName.GroupID, nil
	})
}

func ConvertUserIDsToUsers(customerID string, userIDs []string) []*userpb.User {
	return fn.Map(userIDs, func(userID string) *userpb.User {
		return &userpb.User{
			Name: userpb.UserName{CustomerID: customerID, UserID: userID}.String(),
		}
	})
}

func ConvertGroupIDsToGroups(customerID string, groupIDs []string) []*userpb.Group {
	return fn.Map(groupIDs, func(groupID string) *userpb.Group {
		return &userpb.Group{
			Name: userpb.GroupName{CustomerID: customerID, GroupID: groupID}.String(),
		}
	})
}

func IsIncludeDirectGroupMembershipsOnly(filter *analyticspb.GroupMembershipFilter) bool {
	return filter.GetType() == analyticspb.GroupMembershipFilter_DIRECT_GROUP_MEMBERSHIPS_ONLY
}

func ConvertGroupToAgentAttributeType(types []analyticspb.AttributeType) []analyticspb.AttributeType {
	res := []analyticspb.AttributeType{analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT}
	for _, t := range types {
		if t != analyticspb.AttributeType_ATTRIBUTE_TYPE_AGENT &&
			t != analyticspb.AttributeType_ATTRIBUTE_TYPE_GROUP {
			res = append(res, t)
		}
	}
	return res
}

// Update the users/groups filters to respect the resource ACL on the authenticated user.
// If the user has root access, the filters will not be updated.
func ApplyResourceACL(
	ctx context.Context, configClient config.Client,
	customerID string, resourceACLHelper auth.ResourceACLHelper,
	users []*userpb.User, groups []*userpb.Group,
	includePeerUserStats bool,
) ([]*userpb.User, []*userpb.Group, error) {
	customerConfig, err := configClient.Get(customerID)
	if err != nil {
		return nil, nil, err
	}
	if customerConfig.GetEnableAdvancedDataAccessControl() {
		aclOptions, err := BuildResourceACLOptions(users, groups, includePeerUserStats)
		if err != nil {
			return nil, nil, err
		}
		resourceACL, err := resourceACLHelper.GetResourceACL(ctx, customerID, aclOptions...)
		if err != nil {
			return nil, nil, err
		}
		if !resourceACL.IsRootAccess {
			users = ConvertUserIDsToUsers(customerID, resourceACL.GetAllUserIDs())
		}
		if !resourceACL.IsRootAccess {
			groups = ConvertGroupIDsToGroups(customerID, resourceACL.GetAllGroupIDs())
		}
	}
	return users, groups, nil
}

func ApplyResourceACLWithFilterGroups(
	ctx context.Context, configClient config.Client,
	customerID string, resourceACLHelper auth.ResourceACLHelper,
	filterGroups []*userpb.Group,
	includePeerUserStats bool,
) ([]*userpb.User, []*userpb.Group, bool, error) {
	customerConfig, err := configClient.Get(customerID)
	if err != nil {
		return nil, nil, false, err
	}

	users := []*userpb.User{}
	groups := []*userpb.Group{}
	isRootAccess := true
	if customerConfig.GetEnableAdvancedDataAccessControl() || customerConfig.GetEnableConversationAccessControl() {
		_, filterGroupIDs, err := ConvertUsersAndGroupsToIDs([]*userpb.User{}, filterGroups)
		if err != nil {
			return nil, nil, false, err
		}
		aclOptions := []auth.ResourceACLOption{}
		aclOptions = append(aclOptions, auth.IncludeManagedUsers())
		if len(filterGroupIDs) > 0 {
			aclOptions = append(aclOptions,
				auth.IncludeManagedGroups(),
				auth.WithFilteredGroupIDs(filterGroupIDs))
		}
		if includePeerUserStats {
			aclOptions = append(aclOptions, auth.IncludePeerUsers())
		}
		resourceACL, err := resourceACLHelper.GetResourceACL(ctx, customerID, aclOptions...)
		if err != nil {
			return nil, nil, false, err
		}
		isRootAccess = resourceACL.IsRootAccess
		if !isRootAccess {
			users = ConvertUserIDsToUsers(customerID, resourceACL.GetAllUserIDs())
		}
		if !isRootAccess {
			groups = ConvertGroupIDsToGroups(customerID, resourceACL.GetAllGroupIDs())
		}
	}
	return users, groups, isRootAccess, nil
}

// SplitGroupsByType resolves requested groups through UserService and returns them
// grouped into teams and dynamic groups. Analytics requests carry group names without
// their type, so the type has to be looked up. Groups that do not resolve are dropped;
// callers that must not turn a dropped selection into "no filter" track the requested
// count themselves.
func SplitGroupsByType(
	ctx context.Context,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	groups []*userpb.Group,
) (teamGroups, dynamicGroups []*userpb.Group, err error) {
	if len(groups) == 0 {
		return nil, nil, nil
	}
	for _, group := range groups {
		name, err := userpb.ParseGroupName(group.GetName())
		if err != nil {
			return nil, nil, err
		}
		if name.CustomerID != customerID {
			return nil, nil, fmt.Errorf("group %q belongs to a different customer", group.GetName())
		}
	}

	resolvedGroups, err := ListGroups(ctx, userClientRegistry, internalUserClientRegistry, customerID, profileID, groups, true)
	if err != nil {
		return nil, nil, fmt.Errorf("listing requested groups for %s/%s: %w", customerID, profileID, err)
	}
	groupsByName := make(map[string]*userpb.Group, len(resolvedGroups))
	for _, group := range resolvedGroups {
		groupsByName[group.GetName()] = group
	}

	for _, requested := range groups {
		group, ok := groupsByName[requested.GetName()]
		if !ok {
			continue
		}
		switch group.GetType() {
		case userpb.Group_TEAM:
			teamGroups = append(teamGroups, group)
		case userpb.Group_DYNAMIC_GROUP:
			dynamicGroups = append(dynamicGroups, group)
		default:
			return nil, nil, fmt.Errorf("unsupported group type %v for %q", group.GetType(), group.GetName())
		}
	}
	return teamGroups, dynamicGroups, nil
}

// ApplyResourceACLToAttributeFilter applies ACL logic to an attribute filter in-place.
// It returns true when the caller should return an empty response for security or
// no-overlap cases after ACL filtering/intersection.
func ApplyResourceACLToAttributeFilter(
	ctx context.Context,
	configClient config.Client,
	resourceACLHelper auth.ResourceACLHelper,
	userClientRegistry registry.Registry[userpb.UserServiceClient],
	internalUserClientRegistry registry.Registry[internaluserpb.InternalUserServiceClient],
	customerID, profileID string,
	filterByAttribute *analyticspb.Attribute,
	enableNewACLIntersectionWithUserFilter bool,
	allFilterByAttributeUsers []*userpb.User,
	returnEmptyResponseWhenNoUsersAfterMove bool,
) (bool, error) {
	aclUsers := []*userpb.User{}
	isRootAccess := true
	var err error
	originalFilterByUsers := set.NewFromSlice(fn.Map(
		filterByAttribute.GetUsers(),
		func(u *userpb.User) string { return u.GetName() },
	))
	hadGroupFilter := len(filterByAttribute.GetGroups()) > 0
	// Members of `Attribute.dynamic_groups`, expanded by the caller before this call.
	dynamicGroupMemberUsers := fn.Filter(allFilterByAttributeUsers, func(u *userpb.User) bool {
		return !originalFilterByUsers.Has(u.GetName())
	})
	// We have very complicated assumptions about user/group filters with ACL control.
	// When updating the code, please carefully consider the following scenarios:
	// Only teams are ACL-managed resources. A dynamic group in `groups` never matches
	// the team-only ACL intersection, so it is kept and narrowed through its member
	// users instead.
	// - a) empty user filter, empty group filter => aclUsers non-empty, aclGroups empty
	//   - a1) filter by aclUsers only
	// - b) non-empty user filter, empty group filter => aclUsers non-empty, aclGroups empty
	//   - b1) user filter is within ACL filtered users => intersection of both
	//   - b2) user filter is out of ACL filtered users => empty response
	//     - Note: b2) is not going to happen in prod, just for security concern in API design.
	// - c) empty user filter, non-empty group filter => aclUsers non-empty
	//   - c1) requested TEAM out of ACL filtered groups => team dropped; empty response
	//     when no dynamic group was requested alongside it
	//   - c2) requested TEAM within ACL filtered groups => intersection of both => move team to user filter
	//   - c3) requested DYNAMIC_GROUP => not ACL-managed, always kept => expanded to member users,
	//     then intersected with aclUsers => empty response when no member is in scope
	// - d) non-empty user filter, non-empty group filter => non-empty aclUsers
	//   - d1) team filter out of ACL => empty response
	//   - d2) team filter within ACL => non-empty aclUsers filter, team moved to user filter => intersection of both user filters
	// TODO(xiaoyi.ge): refactor code to make it more readable.
	if enableNewACLIntersectionWithUserFilter {
		var aclFilterGroups []*userpb.Group
		// The helper intersects the requested groups with the team-only managed group
		// list, so dynamic group IDs simply match nothing here.
		aclUsers, aclFilterGroups, isRootAccess, err = ApplyResourceACLWithFilterGroups(
			ctx,
			configClient,
			customerID,
			resourceACLHelper,
			filterByAttribute.GetGroups(),
			false, // includePeerUserStats
		)
		if err != nil {
			return false, err
		}
		if !isRootAccess {
			// The ACL universe is team-only, so its intersection with the requested
			// groups cannot tell a dynamic group apart from a team outside the ACL.
			// Look the types up only here: root and ACL-disabled callers keep the
			// requested groups and need no extra lookup.
			teamGroups, dynamicGroups, err := SplitGroupsByType(
				ctx,
				userClientRegistry,
				internalUserClientRegistry,
				customerID,
				profileID,
				filterByAttribute.GetGroups(),
			)
			if err != nil {
				return false, err
			}
			// A requested TEAM outside the ACL is dropped. It is an empty audience only
			// when no in-scope dynamic group was requested alongside it; dynamic groups
			// are not ACL-managed and are narrowed through their member users below.
			if len(teamGroups) > 0 && len(aclFilterGroups) == 0 && len(dynamicGroups) == 0 {
				return true, nil
			}
			// Retained dynamic groups are narrowed below through their member users.
			filterByAttribute.Groups = append(aclFilterGroups, dynamicGroups...)
		}
		// A selected group that no longer resolves is an empty audience, not the
		// unfiltered request represented by an empty group filter.
		if hadGroupFilter && len(filterByAttribute.GetGroups()) == 0 {
			return true, nil
		}
	} else {
		filterByAttribute.Users, filterByAttribute.Groups, err = ApplyResourceACL(
			ctx,
			configClient,
			customerID,
			resourceACLHelper,
			filterByAttribute.Users,
			filterByAttribute.Groups,
			false, // includePeerUserStats
		)
		if err != nil {
			return false, err
		}
	}

	if len(filterByAttribute.Groups) > 0 || filterByAttribute.ExcludeDeactivatedUsers {
		updatedFilterByAttribute, err := MoveFiltersToUserFilter(
			ctx,
			userClientRegistry,
			internalUserClientRegistry,
			customerID,
			profileID,
			filterByAttribute,
			false, // includeDirectGroupMembershipsOnly
			false, // listAgentOnly
		)
		if err != nil {
			return false, err
		}
		filterByAttribute.Users = updatedFilterByAttribute.Users
		filterByAttribute.Groups = updatedFilterByAttribute.Groups

		// A requested group that expands to no users is an empty audience. This must
		// not fall through to the ACL-user fallback below, which would widen the query
		// to every conversation the caller can see.
		if (hadGroupFilter || returnEmptyResponseWhenNoUsersAfterMove) && len(filterByAttribute.Users) == 0 {
			return true, nil
		}
	}

	// Dynamic groups in `Attribute.dynamic_groups` are not team-ACL resources: their
	// member users are the filter. Merge them for root callers too, otherwise the
	// group filter is dropped and the query widens to every conversation.
	if enableNewACLIntersectionWithUserFilter && len(filterByAttribute.GetDynamicGroups()) > 0 {
		filterByAttribute.Users = DedupUsers(append(filterByAttribute.GetUsers(), dynamicGroupMemberUsers...))
		// An explicitly selected dynamic group with no resolved members is an empty
		// audience, not the unfiltered request.
		if len(filterByAttribute.Users) == 0 {
			return true, nil
		}
	}

	// Apply intersection between ACL users and expanded users from ACL-filtered groups.
	if enableNewACLIntersectionWithUserFilter && !isRootAccess && len(aclUsers) > 0 {
		// Intersect against the union of:
		// 1) users after group/deactivated filters are moved to user filters, and
		// 2) precomputed users (e.g. dynamic group expansion) collected before mutation.
		// This preserves users introduced during move while keeping precomputed expansions.
		usersToIntersect := DedupUsers(append(filterByAttribute.Users, dynamicGroupMemberUsers...))
		if len(usersToIntersect) == 0 {
			usersToIntersect = filterByAttribute.Users
		}
		if len(usersToIntersect) > 0 {
			filterByAttribute.Users = FilterUsers(usersToIntersect, aclUsers)
			if len(filterByAttribute.Users) == 0 {
				return true, nil
			}
		} else {
			// If no user filter, only apply ACL user filter.
			filterByAttribute.Users = aclUsers
		}
	}
	return false, nil
}

func IncludeConversationSourceBot(
	ctx context.Context, configClient config.Client,
	customerID, profileID string,
) (bool, error) {
	profile, err := configClient.GetProfileConfig(customerID, profileID)
	if err != nil {
		return false, err
	}
	return !profile.GetFeatures().GetInsights().GetDisableStatsForConversationSourceBot(), nil
}

// IncludeIngestionPipelineInAgentStats returns true if INGESTION_PIPELINE (source 8)
// should be included in agent stats. Returns false if the customer has configured
// exclude_ingestion_pipeline_from_agent_stats=true in their config.
func IncludeIngestionPipelineInAgentStats(
	ctx context.Context, configClient config.Client,
	customerID, profileID string,
) (bool, error) {
	profile, err := configClient.GetProfileConfig(customerID, profileID)
	if err != nil {
		return false, err
	}
	exclude := profile.GetFeatures() != nil &&
		profile.GetFeatures().GetInsights() != nil &&
		profile.GetFeatures().GetInsights().GetExcludeIngestionPipelineFromAgentStats()
	return !exclude, nil
}

func ApplyResourceACLWithFilterGroupIDs(
	ctx context.Context, configClient config.Client,
	customerID string, resourceACLHelper auth.ResourceACLHelper,
	filterGroupIDs []string,
	includePeerUserStats bool,
	checkConversationAccessControl bool,
) ([]*userpb.User, []*userpb.Group, bool, error) {
	customerConfig, err := configClient.Get(customerID)
	if err != nil {
		return nil, nil, false, err
	}

	users := []*userpb.User{}
	groups := []*userpb.Group{}
	isRootAccess := true
	if customerConfig.GetEnableAdvancedDataAccessControl() || (checkConversationAccessControl && customerConfig.GetEnableConversationAccessControl()) {
		aclOptions := []auth.ResourceACLOption{}
		aclOptions = append(aclOptions, auth.IncludeManagedUsers())
		if len(filterGroupIDs) > 0 {
			aclOptions = append(aclOptions,
				auth.IncludeManagedGroups(),
				auth.WithFilteredGroupIDs(filterGroupIDs))
		}
		if includePeerUserStats {
			aclOptions = append(aclOptions, auth.IncludePeerUsers())
		}
		resourceACL, err := resourceACLHelper.GetResourceACL(ctx, customerID, aclOptions...)
		if err != nil {
			return nil, nil, false, err
		}
		isRootAccess = resourceACL.IsRootAccess
		if !isRootAccess {
			users = ConvertUserIDsToUsers(customerID, resourceACL.GetAllUserIDs())
		}
		if !isRootAccess {
			groups = ConvertGroupIDsToGroups(customerID, resourceACL.GetAllGroupIDs())
		}
	}
	return users, groups, isRootAccess, nil
}

func ListUsersFromDynamicGroups(ctx context.Context, userClient userpb.UserServiceClient, customerID string, profileID string, dynamicGroups []*userpb.Group) ([]*userpb.User, error) {
	if len(dynamicGroups) == 0 {
		// Return early - there are no dynamic groups to expand.
		return nil, nil
	}

	users := []*userpb.User{}
	nextPageToken := ""
	groupNames := fn.Map(dynamicGroups, func(group *userpb.Group) string {
		return group.Name
	})
	for {
		response, err := userClient.ListUsers(ctx, &userpb.ListUsersRequest{
			Parent:    Customer{CustomerID: customerID}.String(),
			ProfileId: profileID,
			PageToken: nextPageToken,
			Filter: &userpb.ListUsersRequest_ListUsersFilter{
				Groups: groupNames,
			},
		})
		if err != nil {
			return nil, err
		}
		users = append(users, response.Users...)
		if response.NextPageToken == "" {
			break
		}
		nextPageToken = response.NextPageToken
	}

	return users, nil
}

func ToUsecaseIDsIncludingAllUsecase(usecases []string) []string {
	usecaseIDs := make([]string, 0, len(usecases)+1)
	for _, usecase := range usecases {
		usecaseName, err := customer.ParseUsecaseName(usecase)
		// Ignore invalid usecase name
		if err == nil {
			usecaseIDs = append(usecaseIDs, usecaseName.GetUsecaseID())
		}
	}
	usecaseIDs = append(usecaseIDs, consts.AllUsecase)
	return usecaseIDs
}
