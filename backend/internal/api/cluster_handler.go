package api

import (
	"aiops-backend/internal/service"
	"aiops-backend/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

var clusterSvc = service.NewClusterService()

// ListClusters returns all cluster configurations.
func ListClusters(c *gin.Context) {
	clusters, err := clusterSvc.ListClusters()
	if err != nil {
		response.InternalError(c, "CLUSTER_LIST_FAILED", "获取集群列表失败", err.Error())
		return
	}
	c.JSON(200, clusters)
}

// CreateCluster creates a new cluster configuration.
func CreateCluster(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		KubeConfig string `json:"kubeconfig"`
		Context    string `json:"context"`
		ServerURL  string `json:"server_url"`
		ConnMode   string `json:"conn_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误: "+err.Error(), "")
		return
	}

	connMode := req.ConnMode
	if connMode == "" {
		connMode = "direct"
	}
	if connMode == "direct" && req.KubeConfig == "" {
		response.BadRequest(c, "INVALID_INPUT", "直连模式需要提供 kubeconfig", "")
		return
	}

	cluster, err := clusterSvc.CreateCluster(req.Name, req.KubeConfig, req.Context, req.ServerURL, connMode)
	if err != nil {
		response.InternalError(c, "CLUSTER_CREATE_FAILED", "创建集群配置失败", err.Error())
		return
	}

	c.JSON(201, cluster)
}

// UpdateCluster updates an existing cluster configuration.
func UpdateCluster(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "集群 ID 格式错误", "")
		return
	}

	var req struct {
		Name       string `json:"name" binding:"required"`
		KubeConfig string `json:"kubeconfig"`
		Context    string `json:"context"`
		ServerURL  string `json:"server_url"`
		ConnMode   string `json:"conn_mode"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	cluster, err := clusterSvc.UpdateCluster(uint(id), req.Name, req.KubeConfig, req.Context, req.ServerURL, req.ConnMode)
	if err != nil {
		response.InternalError(c, "CLUSTER_UPDATE_FAILED", "更新集群配置失败", err.Error())
		return
	}

	c.JSON(200, cluster)
}

// DeleteCluster deletes a cluster configuration.
func DeleteCluster(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "集群 ID 格式错误", "")
		return
	}

	if err := clusterSvc.DeleteCluster(uint(id)); err != nil {
		response.BadRequest(c, "CLUSTER_DELETE_FAILED", "删除集群失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "集群配置已删除"})
}

// SetActiveCluster sets a cluster as the active one.
func SetActiveCluster(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "集群 ID 格式错误", "")
		return
	}

	cluster, err := clusterSvc.SetActiveCluster(uint(id))
	if err != nil {
		response.InternalError(c, "SET_ACTIVE_FAILED", "切换活跃集群失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "已切换活跃集群", "cluster": cluster})
}

// TestCluster tests cluster connectivity.
func TestCluster(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "集群 ID 格式错误", "")
		return
	}

	output, err := clusterSvc.TestCluster(uint(id))
	if err != nil {
		response.BadGateway(c, "CLUSTER_TEST_FAILED", "集群连通性测试失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "集群连通性测试成功", "output": output})
}
