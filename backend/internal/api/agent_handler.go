package api

import (
	"aiops-backend/internal/agent"
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"aiops-backend/pkg/response"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Agent connections use token auth
	},
}

// AgentWebSocket handles WebSocket connections from remote agents.
// The agent authenticates via token query parameter: /api/agent/ws?token=xxx
func AgentWebSocket(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(401, gin.H{"error": "missing token"})
		return
	}

	// Find cluster by agent token
	var cluster model.ClusterConfig
	if err := database.DB.Where("agent_token = ? AND conn_mode = ?", token, "agent").First(&cluster).Error; err != nil {
		c.JSON(401, gin.H{"error": "invalid agent token"})
		return
	}

	// Upgrade to WebSocket
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	hub := agent.GetHub()
	if hub == nil {
		conn.Close()
		return
	}

	// Register and start handling messages
	agentConn := hub.RegisterAgent(cluster.ID, conn)
	hub.HandleAgentMessages(agentConn) // blocks until connection closes
}

// GenerateAgentToken generates (or regenerates) an agent token for a cluster.
func GenerateAgentToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "集群 ID 格式错误", "")
		return
	}

	var cluster model.ClusterConfig
	if err := database.DB.First(&cluster, id).Error; err != nil {
		response.BadRequest(c, "CLUSTER_NOT_FOUND", "集群不存在", "")
		return
	}

	// Generate secure random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		response.InternalError(c, "TOKEN_GEN_FAILED", "生成 Token 失败", err.Error())
		return
	}
	token := hex.EncodeToString(tokenBytes)

	// Update cluster
	cluster.AgentToken = token
	cluster.ConnMode = "agent"
	if err := database.DB.Save(&cluster).Error; err != nil {
		response.InternalError(c, "TOKEN_SAVE_FAILED", "保存 Token 失败", err.Error())
		return
	}

	// Build deployment YAML template
	serverURL := c.Request.Host
	scheme := "wss"
	if c.Request.TLS == nil {
		scheme = "ws"
	}
	wsURL := fmt.Sprintf("%s://%s/api/agent/ws", scheme, serverURL)

	c.JSON(200, gin.H{
		"token":     token,
		"ws_url":    wsURL,
		"deploy_cmd": fmt.Sprintf(
			"# 在远程集群中部署 AIOps Agent:\n"+
				"export AIOPS_SERVER_URL=%s\n"+
				"export AIOPS_AGENT_TOKEN=%s\n"+
				"# 可选：启用写权限（默认只读）\n"+
				"# export AIOPS_ALLOW_WRITE=true\n"+
				"./aiops-agent",
			wsURL, token),
	})
}
