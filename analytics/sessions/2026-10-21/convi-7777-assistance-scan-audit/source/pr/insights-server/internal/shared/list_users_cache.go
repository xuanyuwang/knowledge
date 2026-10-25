package shared

import (
	"context"
	"time"

	userpb "github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/user"
	"github.com/cresta/shared-go/container/cache"
	"github.com/cresta/shared-go/util/envflag"
)

var (
	ListUsersCacheTimeout        = envflag.Duration("LIST_USERS_CACHE_TIMEOUT", 30*time.Minute, "The cache timeout duration for ListUsers response.")
	ListUsersCacheCleanupTimeout = envflag.Duration("LIST_USERS_CACHE_CLEANUP_TIMEOUT", 45*time.Minute, "The cache cleanup timeout duration for ListUsers response.")
)

type ListUsersCache struct {
	UsersToGroupsCache cache.Cache[string, map[string][]string]
	UsersCache         cache.Cache[string, []*userpb.User]
	GroupsCache        cache.Cache[string, []*userpb.Group]
}

func (c *ListUsersCache) SetListUsersResp(ctx context.Context, customerID, profileID string, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, agentOnly bool, usersToGroups map[string][]string, groups []*userpb.Group, users []*userpb.User) {
	cacheKey := getCacheKey(customerID, profileID, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, agentOnly)
	c.UsersToGroupsCache.Set(cacheKey, usersToGroups)
	c.UsersCache.Set(cacheKey, users)
	c.GroupsCache.Set(cacheKey, groups)
}

func (c *ListUsersCache) GetListUsersResp(ctx context.Context, customerID, profileID string, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, agentOnly bool) (map[string][]string, []*userpb.Group, []*userpb.User, bool) {
	cacheKey := getCacheKey(customerID, profileID, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, agentOnly)
	usersToGroups, existsUsersToGroups := c.UsersToGroupsCache.Get(cacheKey)
	users, existsUsers := c.UsersCache.Get(cacheKey)
	groups, existsGroups := c.GroupsCache.Get(cacheKey)
	return usersToGroups, groups, users, (existsUsersToGroups && existsUsers && existsGroups)
}

func getCacheKey(customerID, profileID string, hasAgentAsGroupByKey, includeDirectGroupMembershipsOnly, agentOnly bool) string {
	cacheKey := customerID + ":" + profileID
	if hasAgentAsGroupByKey {
		cacheKey += ":groupByAgent"
	}
	if includeDirectGroupMembershipsOnly {
		cacheKey += ":directMembersOnly"
	}
	if agentOnly {
		cacheKey += ":agentOnly"
	}
	return cacheKey
}
