package api

import (
	"aiops-backend/internal/k8sgpt"
	"aiops-backend/pkg/response"
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

var k8sgptExecutor = k8sgpt.NewExecutor()

// K8sGPTAnalyze runs k8sgpt analysis.
func K8sGPTAnalyze(c *gin.Context) {
	var req struct {
		Filters       []string `json:"filters"`
		Namespace     string   `json:"namespace"`
		LabelSelector string   `json:"label_selector"`
		Explain       bool     `json:"explain"`
		WithStats     bool     `json:"with_stats"`
		UseCache      bool     `json:"use_cache"`
		ClusterID     uint     `json:"cluster_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	var result *k8sgpt.AnalyzeResponse
	var err error
	if req.UseCache {
		result, err = k8sgptExecutor.AnalyzeWithCacheForCluster(ctx, req.ClusterID, req.Filters, req.Namespace, req.LabelSelector, req.Explain, req.WithStats)
	} else {
		result, err = k8sgptExecutor.AnalyzeWithCluster(ctx, req.ClusterID, req.Filters, req.Namespace, req.LabelSelector, req.Explain, req.WithStats)
	}
	if err != nil {
		response.BadGateway(c, "K8SGPT_ANALYZE_FAILED", "K8sGPT 分析失败", err.Error())
		return
	}

	c.JSON(200, result)
}

// K8sGPTListFilters returns available k8sgpt filters.
func K8sGPTListFilters(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	filters, err := k8sgptExecutor.ListFilters(ctx)
	if err != nil {
		response.BadGateway(c, "K8SGPT_FILTERS_FAILED", "获取过滤器列表失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"filters": filters})
}

// K8sGPTListNamespaces returns available namespaces from the specified or active cluster.
func K8sGPTListNamespaces(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var clusterID uint
	if idStr := c.Query("cluster_id"); idStr != "" {
		if id, err := strconv.Atoi(idStr); err == nil && id > 0 {
			clusterID = uint(id)
		}
	}

	namespaces, err := k8sgptExecutor.ListNamespacesForCluster(ctx, clusterID)
	if err != nil {
		response.BadGateway(c, "NAMESPACE_LIST_FAILED", "获取命名空间列表失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"namespaces": namespaces})
}

// GetK8sGPTConfig returns k8sgpt configuration.
func GetK8sGPTConfig(c *gin.Context) {
	config, err := k8sgptExecutor.GetConfig()
	if err != nil {
		response.InternalError(c, "CONFIG_ERROR", "获取 K8sGPT 配置失败", err.Error())
		return
	}
	c.JSON(200, config)
}

// UpdateK8sGPTConfig updates cluster analysis configuration.
func UpdateK8sGPTConfig(c *gin.Context) {
	var req struct {
		Backend       string `json:"backend"`
		Model         string `json:"model"`
		BaseURL       string `json:"base_url"`
		Language      string `json:"language"`
		Anonymize     bool   `json:"anonymize"`
		UseBuiltinLLM bool   `json:"use_builtin_llm"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	config, err := k8sgptExecutor.UpdateConfig(req.Backend, req.Model, req.BaseURL, req.Language, req.Anonymize, req.UseBuiltinLLM)
	if err != nil {
		response.InternalError(c, "CONFIG_UPDATE_FAILED", "更新分析配置失败", err.Error())
		return
	}

	c.JSON(200, config)
}

// TestK8sGPTConnection tests kubectl cluster connectivity.
func TestK8sGPTConnection(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	var req struct {
		ClusterID uint `json:"cluster_id"`
	}
	c.ShouldBindJSON(&req)

	info, err := k8sgptExecutor.TestConnectionForCluster(ctx, req.ClusterID)
	if err != nil {
		response.BadGateway(c, "CLUSTER_NOT_AVAILABLE", "集群连接失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "集群连接正常", "info": info})
}

// InvalidateK8sGPTCache clears the analysis result cache.
func InvalidateK8sGPTCache(c *gin.Context) {
	k8sgpt.InvalidateCache()
	c.JSON(200, gin.H{"message": "缓存已清除"})
}
