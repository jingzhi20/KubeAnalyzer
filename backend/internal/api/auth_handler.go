package api

import (
	"aiops-backend/internal/auth"
	"aiops-backend/internal/database"
	"aiops-backend/internal/feishuclient"
	"aiops-backend/internal/model"
	"aiops-backend/pkg/response"
	"fmt"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest represents the login request payload.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response.
type LoginResponse struct {
	Token     string     `json:"token"`
	ExpiresAt int64      `json:"expires_at"`
	User      UserResponse `json:"user"`
}

// UserResponse represents user information in response.
type UserResponse struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	AvatarURL   string `json:"avatar_url"`
}

// FeishuConfigResponse represents the Feishu SSO configuration status.
type FeishuConfigResponse struct {
	Enabled bool   `json:"enabled"`
	AppID   string `json:"app_id"`
}

// FeishuCallbackRequest represents the Feishu OAuth callback request.
type FeishuCallbackRequest struct {
	Code string `json:"code" binding:"required"`
}

// Login handles user login and returns a JWT token.
func Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Find user
	var user model.User
	if err := database.DB.Where("username = ?", req.Username).First(&user).Error; err != nil {
		// User not found, return generic error message
		response.Unauthorized(c, "INVALID_CREDENTIALS", "用户名或密码错误", "")
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		// Wrong password, return generic error message
		response.Unauthorized(c, "INVALID_CREDENTIALS", "用户名或密码错误", "")
		return
	}

	// Generate token
	token, expiresAt, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		response.InternalError(c, "TOKEN_GENERATION_FAILED", "生成令牌失败", "请稍后重试")
		return
	}

	// Build user response
	userResp := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		AvatarURL:   user.AvatarURL,
	}

	c.JSON(200, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
		User:      userResp,
	})
}

// GetCurrentUser returns the current authenticated user info.
func GetCurrentUser(c *gin.Context) {
	userID := c.GetUint("user_id")

	// Fetch complete user info from database
	var user model.User
	if err := database.DB.First(&user, userID).Error; err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "用户不存在", "")
		return
	}

	c.JSON(200, UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		AvatarURL:   user.AvatarURL,
	})
}

// GetFeishuConfig returns the Feishu SSO configuration status.
func GetFeishuConfig(c *gin.Context) {
	enabled := feishuclient.IsConfigured()
	appID := ""
	if enabled {
		appID = feishuclient.GetAppID()
	}

	c.JSON(200, FeishuConfigResponse{
		Enabled: enabled,
		AppID:   appID,
	})
}

// FeishuCallback handles Feishu OAuth callback.
func FeishuCallback(c *gin.Context) {
	var req FeishuCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Exchange code for token
	accessToken, err := feishuclient.ExchangeCodeForToken(req.Code)
	if err != nil {
		response.Unauthorized(c, "INVALID_CODE", "授权码无效或已过期", err.Error())
		return
	}

	// Get user info from Feishu
	feishuUser, err := feishuclient.GetUserInfo(accessToken)
	if err != nil {
		response.BadGateway(c, "FEISHU_API_ERROR", "获取飞书用户信息失败", err.Error())
		return
	}

	// Find or create user by open_id
	var user model.User
	err = database.DB.Where("feishu_open_id = ?", feishuUser.OpenID).First(&user).Error

	if err != nil {
		// First time login: create new user
		// Generate a unique username from open_id
		username := fmt.Sprintf("feishu_%s", feishuUser.OpenID[len(feishuUser.OpenID)-8:])

		user = model.User{
			Username:     username,
			DisplayName:  feishuUser.Name,
			Role:         "user",
			FeishuOpenID: feishuUser.OpenID,
			AvatarURL:    feishuUser.AvatarURL,
		}

		if err := database.DB.Create(&user).Error; err != nil {
			response.InternalError(c, "USER_CREATION_FAILED", "创建用户失败", "请稍后重试")
			return
		}
	} else {
		// Not first time: update user info
		updates := map[string]interface{}{
			"display_name": feishuUser.Name,
			"avatar_url":   feishuUser.AvatarURL,
		}
		if err := database.DB.Model(&user).Updates(updates).Error; err != nil {
			// Log error but continue (non-critical)
			fmt.Printf("Failed to update user info: %v\n", err)
		}
	}

	// Generate JWT token
	token, expiresAt, err := auth.GenerateToken(user.ID, user.Username, user.Role)
	if err != nil {
		response.InternalError(c, "TOKEN_GENERATION_FAILED", "生成令牌失败", "请稍后重试")
		return
	}

	// Build user response
	userResp := UserResponse{
		ID:          user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		AvatarURL:   user.AvatarURL,
	}

	c.JSON(200, LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt.Unix(),
		User:      userResp,
	})
}
