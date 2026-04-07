package api

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/kubectlai"
	"aiops-backend/pkg/response"
	"context"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

var kubectlAIExecutor = kubectlai.NewExecutor()

// KubectlAIGenerate generates a kubectl command from natural language.
func KubectlAIGenerate(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()

	result, err := kubectlAIExecutor.Generate(ctx, req.Prompt)
	if err != nil {
		errMsg := err.Error()
		// Distinguish LLM not configured error with a specific code
		if strings.HasPrefix(errMsg, "未配置 LLM") {
			response.BadRequest(c, "LLM_NOT_CONFIGURED", errMsg, "")
			return
		}
		response.BadGateway(c, "KUBECTL_AI_FAILED", "kubectl-ai 生成命令失败", errMsg)
		return
	}

	c.JSON(200, result)
}

// KubectlAIExecute generates and executes a kubectl command.
func KubectlAIExecute(c *gin.Context) {
	var req struct {
		Prompt string `json:"prompt" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	userID := c.GetUint("user_id")

	// Get active cluster ID
	var clusterID uint
	if ac, err := cluster.GetActiveCluster(); err == nil {
		clusterID = ac.ID
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	result, err := kubectlAIExecutor.Execute(ctx, req.Prompt, userID, clusterID)
	if err != nil {
		// Still return partial result even on error
		if result != nil {
			c.JSON(200, gin.H{
				"result":  result,
				"warning": err.Error(),
			})
			return
		}
		response.BadGateway(c, "KUBECTL_AI_EXECUTE_FAILED", "kubectl-ai 执行失败", err.Error())
		return
	}

	c.JSON(200, result)
}

// GetKubectlAIConfig returns kubectl-ai configuration.
func GetKubectlAIConfig(c *gin.Context) {
	config, err := kubectlAIExecutor.GetConfig()
	if err != nil {
		response.InternalError(c, "CONFIG_ERROR", "获取 kubectl-ai 配置失败", err.Error())
		return
	}
	c.JSON(200, config)
}

// UpdateKubectlAIConfig updates kubectl AI assistant configuration.
func UpdateKubectlAIConfig(c *gin.Context) {
	var req kubectlai.UpdateConfigRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	config, err := kubectlAIExecutor.UpdateConfig(req)
	if err != nil {
		response.InternalError(c, "CONFIG_UPDATE_FAILED", "更新配置失败", err.Error())
		return
	}

	c.JSON(200, config)
}

// GetKubectlAIHistory returns command history.
func GetKubectlAIHistory(c *gin.Context) {
	userID := c.GetUint("user_id")

	history, err := kubectlAIExecutor.GetHistory(userID)
	if err != nil {
		response.InternalError(c, "HISTORY_ERROR", "获取命令历史失败", err.Error())
		return
	}

	c.JSON(200, history)
}
