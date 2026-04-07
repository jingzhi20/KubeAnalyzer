package middleware

import (
	"aiops-backend/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

// RBACAuth returns a Gin middleware that enforces role-based access control.
// admin: access to all routes
// user: only allowed to access whitelisted paths
func RBACAuth() gin.HandlerFunc {
	// Whitelist paths that user role can access
	userWhitelist := []string{
		"/api/k8sgpt",
		"/api/kubectl-ai",
		"/api/diagnosis",
		"/api/inspections",
		"/api/auth",
	}

	return func(c *gin.Context) {
		role := c.GetString("role")

		// Admin has access to everything
		if role == "admin" {
			c.Next()
			return
		}

		// For user role, check against whitelist
		path := c.Request.URL.Path

		// Check if path matches any whitelist entry
		allowed := false
		for _, whitelistPath := range userWhitelist {
			if strings.HasPrefix(path, whitelistPath) {
				allowed = true
				break
			}
		}

		if !allowed {
			response.Forbidden(c, "FORBIDDEN", "权限不足", "您没有权限访问该资源")
			c.Abort()
			return
		}

		c.Next()
	}
}
