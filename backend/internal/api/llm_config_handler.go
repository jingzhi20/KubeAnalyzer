package api

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"aiops-backend/pkg/response"
	"context"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// CreateLLMConfig handles creating a new LLM configuration.
func CreateLLMConfig(c *gin.Context) {
	var config model.LLMConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Validate API URL format
	if _, err := url.ParseRequestURI(config.APIURL); err != nil {
		response.BadRequest(c, "INVALID_API_URL", "API 地址格式不合法", "请输入合法的 URL 地址")
		return
	}

	// Validate API Key is not empty
	if config.APIKey == "" {
		response.BadRequest(c, "INVALID_API_KEY", "API Key 不能为空", "请输入 API Key")
		return
	}

	config.Status = "unavailable"
	if err := database.DB.Create(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "创建配置失败", err.Error())
		return
	}

	config.APIKey = ""
	c.JSON(201, config)
}

// ListLLMConfigs handles listing all LLM configurations.
func ListLLMConfigs(c *gin.Context) {
	var configs []model.LLMConfig
	if err := database.DB.Order("created_at desc").Find(&configs).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "查询配置失败", err.Error())
		return
	}

	for i := range configs {
		configs[i].APIKey = ""
	}
	c.JSON(200, configs)
}

// UpdateLLMConfig handles updating an LLM configuration.
func UpdateLLMConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "配置 ID 格式错误", "")
		return
	}

	var config model.LLMConfig
	if err := database.DB.First(&config, id).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "配置不存在", "")
		return
	}

	var updates model.LLMConfig
	if err := c.ShouldBindJSON(&updates); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	// Validate API URL if provided
	if updates.APIURL != "" {
		if _, err := url.ParseRequestURI(updates.APIURL); err != nil {
			response.BadRequest(c, "INVALID_API_URL", "API 地址格式不合法", "请输入合法的 URL 地址")
			return
		}
		config.APIURL = updates.APIURL
	}

	if updates.ModelName != "" {
		config.ModelName = updates.ModelName
	}

	if updates.Name != "" {
		config.Name = updates.Name
	}

	// If API Key is updated, reset status to unavailable
	if updates.APIKey != "" {
		config.APIKey = updates.APIKey
		config.Status = "unavailable"
	}

	config.UpdatedAt = time.Now()
	if err := database.DB.Save(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "更新配置失败", err.Error())
		return
	}

	config.APIKey = ""
	c.JSON(200, config)
}

// DeleteLLMConfig handles deleting an LLM configuration.
func DeleteLLMConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "配置 ID 格式错误", "")
		return
	}

	var config model.LLMConfig
	if err := database.DB.First(&config, id).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "配置不存在", "")
		return
	}

	// Prevent deleting default config
	if config.IsDefault {
		response.Conflict(c, "DEFAULT_CONFIG_PROTECTED", "无法删除默认配置", "请先切换其他配置为默认配置")
		return
	}

	if err := database.DB.Delete(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "删除配置失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "配置已删除"})
}

// TestLLMConfig handles testing LLM connectivity.
func TestLLMConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "配置 ID 格式错误", "")
		return
	}

	var config model.LLMConfig
	if err := database.DB.First(&config, id).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "配置不存在", "")
		return
	}

	client := llmclient.New()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	llmConfig := llmclient.LLMConfig{
		APIURL:    config.APIURL,
		APIKey:    config.APIKey,
		ModelName: config.ModelName,
	}

	if err := client.TestConnection(ctx, llmConfig); err != nil {
		config.Status = "unavailable"
		database.DB.Save(&config)
		response.BadGateway(c, "LLM_CONNECTION_FAILED", "连通性测试失败", err.Error())
		return
	}

	config.Status = "available"
	config.UpdatedAt = time.Now()
	database.DB.Save(&config)

	c.JSON(200, gin.H{"message": "连通性测试成功", "status": "available"})
}

// SetDefaultLLMConfig handles setting a default LLM configuration.
func SetDefaultLLMConfig(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "配置 ID 格式错误", "")
		return
	}

	var config model.LLMConfig
	if err := database.DB.First(&config, id).Error; err != nil {
		response.NotFound(c, "CONFIG_NOT_FOUND", "配置不存在", "")
		return
	}

	// Reset all configs to non-default
	if err := database.DB.Model(&model.LLMConfig{}).Where("1 = 1").Updates(map[string]interface{}{"is_default": false}).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "设置默认配置失败", err.Error())
		return
	}

	// Set this config as default
	config.IsDefault = true
	config.UpdatedAt = time.Now()
	if err := database.DB.Save(&config).Error; err != nil {
		response.InternalError(c, "DATABASE_ERROR", "设置默认配置失败", err.Error())
		return
	}

	config.APIKey = ""
	c.JSON(200, gin.H{"message": "默认配置已更新", "config": config})
}

// GetDefaultLLMConfig returns the default LLM configuration.
func GetDefaultLLMConfig(c *gin.Context) {
	var config model.LLMConfig
	if err := database.DB.Where("is_default = ?", true).First(&config).Error; err != nil {
		response.NotFound(c, "NO_DEFAULT_CONFIG", "未设置默认配置", "请先设置一个默认认为配置")
		return
	}

	config.APIKey = ""
	c.JSON(200, config)
}
