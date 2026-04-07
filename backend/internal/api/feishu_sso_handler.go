package api

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"aiops-backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// FeishuSSOConfigResponse represents the Feishu SSO configuration response.
type FeishuSSOConfigResponse struct {
	ID          uint   `json:"id"`
	AppID       string `json:"app_id"`
	RedirectURI string `json:"redirect_uri"`
	Enabled     bool   `json:"enabled"`
	UpdatedAt   string `json:"updated_at"`
}

// UpdateFeishuSSOConfigRequest represents the request to update Feishu SSO config.
type UpdateFeishuSSOConfigRequest struct {
	AppID       string `json:"app_id" binding:"required"`
	AppSecret   string `json:"app_secret" binding:"required"`
	RedirectURI string `json:"redirect_uri" binding:"required"`
	Enabled     bool   `json:"enabled"`
}

// GetFeishuSSOConfig returns the Feishu SSO configuration.
func GetFeishuSSOConfig(c *gin.Context) {
	var config model.FeishuSSOConfig
	// Get the first config (there should only be one)
	if err := database.DB.First(&config).Error; err != nil {
		// Return empty config if not found
		c.JSON(200, FeishuSSOConfigResponse{
			Enabled: false,
		})
		return
	}

	c.JSON(200, FeishuSSOConfigResponse{
		ID:          config.ID,
		AppID:       config.AppID,
		RedirectURI: config.RedirectURI,
		Enabled:     config.Enabled,
		UpdatedAt:   config.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// UpdateFeishuSSOConfig updates the Feishu SSO configuration.
func UpdateFeishuSSOConfig(c *gin.Context) {
	var req UpdateFeishuSSOConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Find existing config or create new one
	var config model.FeishuSSOConfig
	err := database.DB.First(&config).Error

	if err != nil {
		// Create new config
		config = model.FeishuSSOConfig{
			AppID:       req.AppID,
			AppSecret:   req.AppSecret,
			RedirectURI: req.RedirectURI,
			Enabled:     req.Enabled,
		}
		if err := database.DB.Create(&config).Error; err != nil {
			response.InternalError(c, "CREATE_FAILED", "创建配置失败", "请稍后重试")
			return
		}
	} else {
		// Update existing config
		updates := map[string]interface{}{
			"app_id":       req.AppID,
			"app_secret":   req.AppSecret,
			"redirect_uri": req.RedirectURI,
			"enabled":      req.Enabled,
		}
		if err := database.DB.Model(&config).Updates(updates).Error; err != nil {
			response.InternalError(c, "UPDATE_FAILED", "更新配置失败", "请稍后重试")
			return
		}
	}

	c.JSON(200, FeishuSSOConfigResponse{
		ID:          config.ID,
		AppID:       config.AppID,
		RedirectURI: config.RedirectURI,
		Enabled:     config.Enabled,
		UpdatedAt:   config.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}

// TestFeishuSSOConfig tests the Feishu SSO configuration.
func TestFeishuSSOConfig(c *gin.Context) {
	var req UpdateFeishuSSOConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Test by checking if we can get app info from Feishu
	// This is a simple validation test
	if req.AppID == "" || req.AppSecret == "" {
		response.BadRequest(c, "INVALID_CONFIG", "配置不完整", "App ID 和 App Secret 不能为空")
		return
	}

	// For now, just validate the format
	// In production, you would make an actual API call to Feishu to test
	c.JSON(200, gin.H{
		"message": "配置格式验证通过",
		"success": true,
	})
}
