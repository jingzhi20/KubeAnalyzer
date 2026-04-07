package api

import (
	"aiops-backend/internal/model"
	"aiops-backend/internal/service"
	"aiops-backend/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

var inspectionSvc *service.InspectionScheduler

// initInspectionService initializes the inspection service (called after database is ready)
func initInspectionService() {
	if inspectionSvc == nil {
		inspectionSvc = service.NewInspectionScheduler()
	}
}

// GetInspectionConfig returns inspection configuration.
func GetInspectionConfig(c *gin.Context) {
	config, err := inspectionSvc.GetConfig()
	if err != nil {
		response.InternalError(c, "CONFIG_ERROR", "获取配置失败", err.Error())
		return
	}

	c.JSON(200, config)
}

// UpdateInspectionConfig updates inspection configuration.
func UpdateInspectionConfig(c *gin.Context) {
	var req struct {
		CronExpression string `json:"cron_expression" binding:"required"`
		Enabled        bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	config, err := inspectionSvc.UpdateConfig(req.CronExpression, req.Enabled)
	if err != nil {
		response.BadRequest(c, "CONFIG_UPDATE_FAILED", "更新配置失败", err.Error())
		return
	}

	c.JSON(200, config)
}

// TriggerInspection manually triggers an inspection.
func TriggerInspection(c *gin.Context) {
	var req struct {
		ClusterID uint `json:"cluster_id"`
	}
	// Allow empty body (defaults to active cluster)
	c.ShouldBindJSON(&req)

	task, err := inspectionSvc.TriggerManual(req.ClusterID)
	if err != nil {
		response.InternalError(c, "TRIGGER_FAILED", "触发巡检失败", err.Error())
		return
	}

	c.JSON(200, task)
}

// GetInspectionTasks returns inspection task list.
func GetInspectionTasks(c *gin.Context) {
	tasks, err := inspectionSvc.GetTasks()
	if err != nil {
		response.InternalError(c, "TASK_LIST_FAILED", "获取任务列表失败", err.Error())
		return
	}

	c.JSON(200, tasks)
}

// GetInspectionTask returns a specific inspection task.
func GetInspectionTask(c *gin.Context) {
	taskID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "任务 ID 格式错误", "")
		return
	}

	task, err := inspectionSvc.GetTask(uint(taskID))
	if err != nil {
		response.NotFound(c, "TASK_NOT_FOUND", "任务不存在", "")
		return
	}

	c.JSON(200, task)
}

// CreateInspectionRule creates a new inspection rule.
func CreateInspectionRule(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		RuleType       string `json:"rule_type" binding:"required"`
		ResourceType   string `json:"resource_type"`
		CheckItems     string `json:"check_items"`
		Command        string `json:"command"`
		Script         string `json:"script"`
		ScriptType     string `json:"script_type"`
		Timeout        int    `json:"timeout"`
		TargetNodes    string `json:"target_nodes"`
		Namespaces     string `json:"namespaces"`
		ClusterID      uint   `json:"cluster_id"`
		CronExpression string `json:"cron_expression"`
		Enabled        bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	rule := &model.InspectionRule{
		Name:           req.Name,
		RuleType:       req.RuleType,
		ResourceType:   req.ResourceType,
		CheckItems:     req.CheckItems,
		Command:        req.Command,
		Script:         req.Script,
		ScriptType:     req.ScriptType,
		Timeout:        req.Timeout,
		TargetNodes:    req.TargetNodes,
		Namespaces:     req.Namespaces,
		ClusterID:      req.ClusterID,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
	}

	created, err := inspectionSvc.CreateRule(rule)
	if err != nil {
		response.BadRequest(c, "RULE_CREATE_FAILED", "创建规则失败", err.Error())
		return
	}

	c.JSON(201, created)
}

// UpdateInspectionRule updates an existing inspection rule.
func UpdateInspectionRule(c *gin.Context) {
	ruleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "规则 ID 格式错误", "")
		return
	}

	var req struct {
		Name           string `json:"name" binding:"required"`
		RuleType       string `json:"rule_type" binding:"required"`
		ResourceType   string `json:"resource_type"`
		CheckItems     string `json:"check_items"`
		Command        string `json:"command"`
		Script         string `json:"script"`
		ScriptType     string `json:"script_type"`
		Timeout        int    `json:"timeout"`
		TargetNodes    string `json:"target_nodes"`
		Namespaces     string `json:"namespaces"`
		ClusterID      uint   `json:"cluster_id"`
		CronExpression string `json:"cron_expression"`
		Enabled        bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "INVALID_INPUT", "请求参数错误", err.Error())
		return
	}

	updates := &model.InspectionRule{
		Name:           req.Name,
		RuleType:       req.RuleType,
		ResourceType:   req.ResourceType,
		CheckItems:     req.CheckItems,
		Command:        req.Command,
		Script:         req.Script,
		ScriptType:     req.ScriptType,
		Timeout:        req.Timeout,
		TargetNodes:    req.TargetNodes,
		Namespaces:     req.Namespaces,
		ClusterID:      req.ClusterID,
		CronExpression: req.CronExpression,
		Enabled:        req.Enabled,
	}

	rule, err := inspectionSvc.UpdateRule(uint(ruleID), updates)
	if err != nil {
		response.InternalError(c, "RULE_UPDATE_FAILED", "更新规则失败", err.Error())
		return
	}

	c.JSON(200, rule)
}

// ListInspectionRules returns all inspection rules.
func ListInspectionRules(c *gin.Context) {
	rules, err := inspectionSvc.ListRules()
	if err != nil {
		response.InternalError(c, "RULE_LIST_FAILED", "获取规则列表失败", err.Error())
		return
	}

	c.JSON(200, rules)
}

// DeleteInspectionRule deletes an inspection rule.
func DeleteInspectionRule(c *gin.Context) {
	ruleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "INVALID_ID", "规则 ID 格式错误", "")
		return
	}

	if err := inspectionSvc.DeleteRule(uint(ruleID)); err != nil {
		response.InternalError(c, "RULE_DELETE_FAILED", "删除规则失败", err.Error())
		return
	}

	c.JSON(200, gin.H{"message": "规则已删除"})
}
