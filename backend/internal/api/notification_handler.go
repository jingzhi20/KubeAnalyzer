package api

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/feishuclient"
	"aiops-backend/internal/model"
	"aiops-backend/internal/service"
	"aiops-backend/pkg/response"
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

var feishuClient = feishuclient.New()

// GetNotificationConfig returns the notification configuration.
func GetNotificationConfig(c *gin.Context) {
	config, err := service.GetNotificationConfig()
	if err != nil {
		// Return empty config if not found
		c.JSON(200, model.NotificationConfig{})
		return
	}

	c.JSON(200, config)
}

// UpdateNotificationConfig updates the notification configuration.
func UpdateNotificationConfig(c *gin.Context) {
	var config model.NotificationConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "获取配置失败", err.Error())
		return
	}

	var updates model.NotificationConfig
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	config.WebhookURL = updates.WebhookURL
	config.SignKey = updates.SignKey
	config.Policy = updates.Policy
	config.Enabled = updates.Enabled
	config.UpdatedAt = time.Now()

	if err := database.DB.Save(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "更新配置失败", err.Error())
		return
	}

	c.JSON(200, config)
}

// TestNotification sends a test message to Feishu webhook.
func TestNotification(c *gin.Context) {
	var config model.NotificationConfig
	if err := database.DB.First(&config).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "未配置飞书 Webhook", "请先配置 Webhook 地址")
		return
	}

	if config.WebhookURL == "" {
		response.BadRequest(c, "MISSING_WEBHOOK", "Webhook 地址为空", "请先配置 Webhook 地址")
		return
	}

	message := feishuClient.FormatTestMessage()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := feishuClient.SendMessage(ctx, config.WebhookURL, message); err != nil {
		response.BadGateway(c, "WEBHOOK_TEST_FAILED", "测试消息发送失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "测试消息发送成功"})
}

// SendNotification manually sends a diagnosis result to Feishu.
func SendNotification(c *gin.Context) {
	var config model.NotificationConfig
	if err := database.DB.First(&config).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "未配置飞书 Webhook", "")
		return
	}

	if !config.Enabled {
		response.BadRequest(c, "NOTIFICATION_DISABLED", "通知功能未启用", "")
		return
	}

	var req struct {
		Title   string   `json:"title" binding:"required"`
		Summary string   `json:"summary" binding:"required"`
		Details []string `json:"details"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	message := feishuClient.FormatInspectionCard(req.Title, req.Summary, req.Details, time.Now())
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := feishuClient.SendMessage(ctx, config.WebhookURL, message); err != nil {
		response.BadGateway(c, "SEND_FAILED", "消息发送失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "消息发送成功"})
}

// ShouldNotify checks if notification should be sent based on policy.
func ShouldNotify(c *gin.Context) {
	// This function is now in service package, keeping for API compatibility
	c.JSON(200, gin.H{"message": "Use service.ShouldNotify instead"})
}
