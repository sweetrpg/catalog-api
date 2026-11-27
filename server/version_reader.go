package server

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sweetrpg/authz-client.go/authz"
	"github.com/sweetrpg/catalog-api/constants"
	"github.com/sweetrpg/catalog-data.go/data"
	"github.com/sweetrpg/common.go/logging"
)

const versionReaderKey = "catalog.versionReader"

// internalReader is for this service's own lookups (review actions, events). Never use it to
// build a response for a caller without a review role.
var internalReader = data.VersionReader{Reviewer: true}

// resolveVersionReader identifies the caller of a version read route and marks the response as
// not shareable between callers. A request with no token, or one that can't be verified, stays
// a signed-out reader.
func resolveVersionReader(client *authz.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "private, no-store")
		c.Header("Vary", "Authorization")

		token, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok || token == "" {
			return
		}
		result, err := client.Check(c.Request.Context(), token, constants.ServiceName)
		if err != nil {
			logging.Logger.Warn("version read: caller not verified, using the signed-out view", "error", err.Error())
			return
		}
		if !result.Allowed {
			return
		}
		c.Set(versionReaderKey, data.VersionReader{
			UserID:   client.ResolveUserID(c.Request.Context(), token),
			Reviewer: isReviewer(result.Roles),
		})
	}
}

// versionReader returns the reader resolveVersionReader stored, or the signed-out reader.
func versionReader(c *gin.Context) data.VersionReader {
	reader, _ := c.Value(versionReaderKey).(data.VersionReader)
	return reader
}

// roleReader is the reader for a route already gated by authz.RequireAnyRole.
func roleReader(c *gin.Context) data.VersionReader {
	return data.VersionReader{UserID: authz.Viewer(c), Reviewer: isReviewer(authz.Roles(c))}
}

func isReviewer(roles []string) bool {
	return authz.HasRole(roles, authz.RoleAdmin) || authz.HasRole(roles, authz.RoleEditor)
}
