package api

import (
	"aiops-backend/internal/service"
	"aiops-backend/pkg/response"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var diagnosisSvc = service.NewDiagnosisService()

// CreateSession handles creating a new diagnosis session.
func CreateSession(c *gin.Context) {
	userID := c.GetUint("user_id")

	var req struct {
		Title string `json:"title" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	session, err := diagnosisSvc.CreateSession(userID, req.Title)
	if err != nil {
		response.InternalError(c, "SESSION_CREATE_FAILED", "创建会话失败", err.Error())
		return
	}

	c.JSON(201, session)
}

// ListSessions handles listing diagnosis sessions.
func ListSessions(c *gin.Context) {
	userID := c.GetUint("user_id")

	sessions, err := diagnosisSvc.ListSessions(userID)
	if err != nil {
		response.InternalError(c, "SESSION_LIST_FAILED", "获取会话列表失败", err.Error())
		return
	}

	c.JSON(200, sessions)
}

// GetSession handles getting a session detail.
func GetSession(c *gin.Context) {
	userID := c.GetUint("user_id")
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "会话 ID 格式错误", "")
		return
	}

	session, err := diagnosisSvc.GetSession(uint(sessionID), userID)
	if err != nil {
		response.NotFound(c, "SESSION_NOT_FOUND", "会话不存在", "")
		return
	}

	c.JSON(200, session)
}

// SubmitQuery handles submitting a diagnosis query.
func SubmitQuery(c *gin.Context) {
	userID := c.GetUint("user_id")
	sessionID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "会话 ID 格式错误", "")
		return
	}

	var req struct {
		Question string `json:"question" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	record, err := diagnosisSvc.SubmitQuery(uint(sessionID), userID, req.Question)
	if err != nil {
		// Check if it's a gRPC error
		if strings.HasPrefix(err.Error(), "GRPC_ERROR|") {
			parts := strings.SplitN(err.Error(), "|", 3)
			errorType := parts[0]
			message := parts[1]
			suggestion := ""
			if len(parts) > 2 {
				suggestion = parts[2]
			}
			response.BadGateway(c, errorType, message, suggestion)
			return
		}
		response.InternalError(c, "QUERY_FAILED", "提交问题失败", err.Error())
		return
	}

	c.JSON(200, record)
}
