package api

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"aiops-backend/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// CreateUserRequest represents the request to create a user.
type CreateUserRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role" binding:"required"`
}

// UpdateRoleRequest represents the request to update user role.
type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// ListUsers returns all users.
func ListUsers(c *gin.Context) {
	var users []model.User
	if err := database.DB.Order("created_at desc").Find(&users).Error; err != nil {
		response.InternalError(c, "QUERY_FAILED", "查询用户列表失败", "请稍后重试")
		return
	}

	// Build response list
	type UserInfo struct {
		ID          uint   `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
		Role        string `json:"role"`
		AvatarURL   string `json:"avatar_url"`
		LoginMethod string `json:"login_method"`
		CreatedAt   string `json:"created_at"`
	}

	userList := make([]UserInfo, 0, len(users))
	for _, u := range users {
		// Determine login method
		loginMethod := "密码登录"
		if u.FeishuOpenID != "" {
			loginMethod = "飞书登录"
		}

		userList = append(userList, UserInfo{
			ID:          u.ID,
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Role:        u.Role,
			AvatarURL:   u.AvatarURL,
			LoginMethod: loginMethod,
			CreatedAt:   u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}

	c.JSON(200, userList)
}

// CreateUser creates a new user.
func CreateUser(c *gin.Context) {
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		response.BadRequest(c, "INVALID_ROLE", "无效的角色", "角色必须是 admin 或 user")
		return
	}

	// Check if username already exists
	var existing model.User
	if err := database.DB.Where("username = ?", req.Username).First(&existing).Error; err == nil {
		response.Conflict(c, "USERNAME_EXISTS", "用户名已存在", "请使用其他用户名")
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.InternalError(c, "PASSWORD_HASH_FAILED", "密码加密失败", "请稍后重试")
		return
	}

	// Create user
	user := model.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		DisplayName:  req.DisplayName,
		Role:         req.Role,
	}

	if err := database.DB.Create(&user).Error; err != nil {
		response.InternalError(c, "USER_CREATION_FAILED", "创建用户失败", "请稍后重试")
		return
	}

	c.JSON(201, gin.H{
		"id":           user.ID,
		"username":     user.Username,
		"display_name": user.DisplayName,
		"role":         user.Role,
		"avatar_url":   user.AvatarURL,
		"created_at":   user.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}

// UpdateUserRole updates a user's role.
func UpdateUserRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "无效的用户ID", "")
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Validate role
	if req.Role != "admin" && req.Role != "user" {
		response.BadRequest(c, "INVALID_ROLE", "无效的角色", "角色必须是 admin 或 user")
		return
	}

	// Find user
	var user model.User
	if err := database.DB.First(&user, uint(id)).Error; err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "用户不存在", "")
		return
	}

	// Update role
	if err := database.DB.Model(&user).Update("role", req.Role).Error; err != nil {
		response.InternalError(c, "UPDATE_FAILED", "更新角色失败", "请稍后重试")
		return
	}

	c.JSON(200, gin.H{
		"id":   user.ID,
		"role": user.Role,
	})
}

// DeleteUser deletes a user.
func DeleteUser(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "无效的用户ID", "")
		return
	}

	currentUserID := c.GetUint("user_id")
	if uint(id) == currentUserID {
		response.BadRequest(c, "CANNOT_DELETE_SELF", "不能删除当前登录用户", "")
		return
	}

	// Find user
	var user model.User
	if err := database.DB.First(&user, uint(id)).Error; err != nil {
		response.NotFound(c, "USER_NOT_FOUND", "用户不存在", "")
		return
	}

	// Delete user
	if err := database.DB.Delete(&user).Error; err != nil {
		response.InternalError(c, "DELETE_FAILED", "删除用户失败", "请稍后重试")
		return
	}

	c.JSON(200, gin.H{
		"message": "用户已删除",
	})
}
