package middleware

import (
	"aiops-backend/internal/auth"
	"aiops-backend/pkg/response"
	"strings"

	"github.com/gin-gonic/gin"
)

// JWTAuth returns a Gin middleware that validates JWT tokens.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Unauthorized(c, "UNAUTHORIZED", "未提供认证令牌", "请先登录获取令牌")
			c.Abort()
			return
		}

		// Bearer <token>
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			response.Unauthorized(c, "UNAUTHORIZED", "认证令牌格式错误", "请使用 Bearer <token> 格式")
			c.Abort()
			return
		}

		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			response.Unauthorized(c, "TOKEN_EXPIRED", "认证令牌已过期或无效", "请重新登录")
			c.Abort()
			return
		}

		// Set user info in context
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)

		// Extract role from claims; default to "user" for old tokens missing the role field
		role := claims.Role
		if role == "" {
			role = "user"
		}
		c.Set("role", role)

		c.Next()
	}
}
