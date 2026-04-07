package service

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/k8sgpt"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// ScriptResult represents the result of a command/script execution.
type ScriptResult struct {
	RuleID   uint   `json:"rule_id"`
	RuleName string `json:"rule_name"`
	RuleType string `json:"rule_type"`
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration string `json:"duration"`
	Error    string `json:"error,omitempty"`
}

// InspectionScheduler handles inspection scheduling and execution.
type InspectionScheduler struct {
	cron       *cron.Cron
	cronIDs    map[uint]cron.EntryID // rule ID -> cron entry ID
	k8sgptExec *k8sgpt.Executor
	llmClient  llmclient.LLMClient
}

// NewInspectionScheduler creates a new InspectionScheduler instance.
func NewInspectionScheduler() *InspectionScheduler {
	scheduler := &InspectionScheduler{
		cron:       cron.New(),
		cronIDs:    make(map[uint]cron.EntryID),
		k8sgptExec: k8sgpt.NewExecutor(),
		llmClient:  llmclient.New(),
	}

	scheduler.cron.Start()
	scheduler.syncRuleSchedules()
	log.Println("Inspection scheduler started")

	return scheduler
}

// syncRuleSchedules loads all enabled rules with cron expressions and schedules them.
func (s *InspectionScheduler) syncRuleSchedules() {
	var rules []model.InspectionRule
	if err := database.DB.Where("enabled = ? AND cron_expression != ''", true).Find(&rules).Error; err != nil {
		log.Printf("Failed to load rules for scheduling: %v", err)
		return
	}
	for _, rule := range rules {
		s.scheduleRule(rule)
	}
}

// scheduleRule adds or updates a cron job for a specific rule.
func (s *InspectionScheduler) scheduleRule(rule model.InspectionRule) {
	// Remove existing job if any
	if entryID, ok := s.cronIDs[rule.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronIDs, rule.ID)
	}

	if !rule.Enabled || rule.CronExpression == "" {
		return
	}

	ruleID := rule.ID
	clusterID := rule.ClusterID
	entryID, err := s.cron.AddFunc(rule.CronExpression, func() {
		s.executeInspection("scheduled", clusterID)
	})
	if err != nil {
		log.Printf("Failed to schedule rule %d (%s): %v", ruleID, rule.Name, err)
		return
	}
	s.cronIDs[ruleID] = entryID
	log.Printf("Scheduled rule %d (%s) with cron: %s", ruleID, rule.Name, rule.CronExpression)
}

// GetConfig returns the inspection configuration.
func (s *InspectionScheduler) GetConfig() (*model.InspectionConfig, error) {
	var config model.InspectionConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get inspection config: %w", err)
	}
	return &config, nil
}

// UpdateConfig updates the inspection configuration and reschedules.
func (s *InspectionScheduler) UpdateConfig(cronExpr string, enabled bool) (*model.InspectionConfig, error) {
	var config model.InspectionConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to get inspection config: %w", err)
	}

	// Remove existing job
	s.cron.Stop()
	s.cron = cron.New()

	config.CronExpression = cronExpr
	config.Enabled = enabled
	config.UpdatedAt = time.Now()

	if err := database.DB.Save(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	// Schedule new job if enabled (global fallback)
	if enabled && cronExpr != "" {
		_, err := s.cron.AddFunc(cronExpr, func() {
			s.executeInspection("scheduled", 0)
		})
		if err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
	}
	s.cron.Start()

	return &config, nil
}

// TriggerManual triggers a manual inspection for a specific cluster.
func (s *InspectionScheduler) TriggerManual(clusterID uint) (*model.InspectionTask, error) {
	task := s.executeInspection("manual", clusterID)
	return task, nil
}

// GetTasks returns inspection tasks.
func (s *InspectionScheduler) GetTasks() ([]model.InspectionTask, error) {
	var tasks []model.InspectionTask
	if err := database.DB.Order("started_at desc").Find(&tasks).Error; err != nil {
		return nil, fmt.Errorf("failed to get tasks: %w", err)
	}
	return tasks, nil
}

// GetTask returns a specific task.
func (s *InspectionScheduler) GetTask(taskID uint) (*model.InspectionTask, error) {
	var task model.InspectionTask
	if err := database.DB.First(&task, taskID).Error; err != nil {
		return nil, fmt.Errorf("failed to get task: %w", err)
	}
	return &task, nil
}

// CreateRule creates an inspection rule.
func (s *InspectionScheduler) CreateRule(rule *model.InspectionRule) (*model.InspectionRule, error) {
	// Validate rule type
	switch rule.RuleType {
	case "builtin":
		if rule.ResourceType == "" {
			return nil, fmt.Errorf("resource_type is required for builtin rules")
		}
	case "command":
		if rule.Command == "" {
			return nil, fmt.Errorf("command is required for command rules")
		}
	case "script":
		if rule.Script == "" {
			return nil, fmt.Errorf("script is required for script rules")
		}
		if rule.ScriptType == "" {
			rule.ScriptType = "bash"
		}
	default:
		return nil, fmt.Errorf("invalid rule_type: %s (must be builtin, command, or script)", rule.RuleType)
	}

	if rule.Timeout <= 0 {
		rule.Timeout = 60
	}

	if err := database.DB.Create(rule).Error; err != nil {
		return nil, fmt.Errorf("failed to create rule: %w", err)
	}

	// Schedule if has cron
	s.scheduleRule(*rule)

	return rule, nil
}

// UpdateRule updates an existing inspection rule.
func (s *InspectionScheduler) UpdateRule(ruleID uint, updates *model.InspectionRule) (*model.InspectionRule, error) {
	var rule model.InspectionRule
	if err := database.DB.First(&rule, ruleID).Error; err != nil {
		return nil, fmt.Errorf("rule not found: %w", err)
	}

	rule.Name = updates.Name
	rule.RuleType = updates.RuleType
	rule.ResourceType = updates.ResourceType
	rule.CheckItems = updates.CheckItems
	rule.Command = updates.Command
	rule.Script = updates.Script
	rule.ScriptType = updates.ScriptType
	rule.Enabled = updates.Enabled
	rule.TargetNodes = updates.TargetNodes
	rule.Namespaces = updates.Namespaces
	rule.ClusterID = updates.ClusterID
	rule.CronExpression = updates.CronExpression
	if updates.Timeout > 0 {
		rule.Timeout = updates.Timeout
	}

	if err := database.DB.Save(&rule).Error; err != nil {
		return nil, fmt.Errorf("failed to update rule: %w", err)
	}

	// Re-schedule cron if needed
	s.scheduleRule(rule)

	return &rule, nil
}

// ListRules returns all inspection rules.
func (s *InspectionScheduler) ListRules() ([]model.InspectionRule, error) {
	var rules []model.InspectionRule
	if err := database.DB.Find(&rules).Error; err != nil {
		return nil, fmt.Errorf("failed to get rules: %w", err)
	}
	return rules, nil
}

// DeleteRule deletes an inspection rule.
func (s *InspectionScheduler) DeleteRule(ruleID uint) error {
	// Remove cron job
	if entryID, ok := s.cronIDs[ruleID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronIDs, ruleID)
	}
	if err := database.DB.Delete(&model.InspectionRule{}, ruleID).Error; err != nil {
		return fmt.Errorf("failed to delete rule: %w", err)
	}
	return nil
}

// executeInspection performs the actual inspection.
// clusterID=0 means use the active cluster.
func (s *InspectionScheduler) executeInspection(triggerType string, clusterID uint) *model.InspectionTask {
	task := &model.InspectionTask{
		TriggerType: triggerType,
		Status:      "running",
		ClusterID:   clusterID,
		StartedAt:   time.Now(),
	}

	// Resolve cluster name
	if clusterID > 0 {
		var cluster model.ClusterConfig
		if err := database.DB.First(&cluster, clusterID).Error; err == nil {
			task.ClusterName = cluster.Name
		}
	} else {
		var cluster model.ClusterConfig
		if err := database.DB.Where("is_active = ?", true).First(&cluster).Error; err == nil {
			task.ClusterID = cluster.ID
			task.ClusterName = cluster.Name
		}
	}

	if err := database.DB.Create(task).Error; err != nil {
		log.Printf("Failed to create inspection task: %v", err)
		return nil
	}

	// Step 1: Call k8sgpt for cluster inspection
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	k8sgptResp, k8sgptErr := s.k8sgptExec.Analyze(ctx, []string{}, "", true)

	if k8sgptErr != nil {
		task.Status = "failed"
		task.ErrorMessage = k8sgptErr.Error()
		task.CompletedAt = time.Now()
		database.DB.Save(task)
		log.Printf("Inspection failed: %v", k8sgptErr)
		return task
	}

	// Serialize k8sgpt response
	if jsonData, err := json.Marshal(k8sgptResp); err == nil {
		task.GRPCResponse = string(jsonData)
	}

	// Step 2: Execute command/script rules
	scriptResults := s.executeRules(task.ClusterID)
	if len(scriptResults) > 0 {
		if jsonData, err := json.Marshal(scriptResults); err == nil {
			task.ScriptResults = string(jsonData)
		}
	}

	// Step 3: Send to LLM for summary (include script results)
	var llmConfig model.LLMConfig
	if err := database.DB.Where("is_default = ?", true).First(&llmConfig).Error; err == nil {
		scriptContext := ""
		if task.ScriptResults != "" {
			scriptContext = fmt.Sprintf("\n\n自定义巡检脚本结果：%s", task.ScriptResults)
		}

		messages := []llmclient.Message{
			{Role: "system", Content: "你是一个 Kubernetes 运维专家。请分析以下集群巡检数据，生成巡检摘要和异常分析报告。"},
			{Role: "user", Content: fmt.Sprintf("巡检数据：%s%s", task.GRPCResponse, scriptContext)},
		}

		llmConfigClient := llmclient.LLMConfig{
			APIURL:    llmConfig.APIURL,
			APIKey:    llmConfig.APIKey,
			ModelName: llmConfig.ModelName,
		}

		ctxLLM, cancelLLM := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancelLLM()

		llmResponse, llmErr := s.llmClient.ChatCompletion(ctxLLM, llmConfigClient, messages)
		if llmErr == nil {
			task.LLMSummary = llmResponse
		}
	}

	// Step 4: Update task status
	task.Status = "completed"
	task.AnomalyCount = k8sgptResp.Problems
	// Also count script failures as anomalies
	for _, sr := range scriptResults {
		if sr.ExitCode != 0 {
			task.AnomalyCount++
		}
	}
	task.CompletedAt = time.Now()
	database.DB.Save(task)

	// Step 5: Send notification if needed (including script alerts)
	notifConfig, err := GetNotificationConfig()
	if err == nil {
		if ShouldNotify(notifConfig, task.AnomalyCount) {
			anomalies := []string{}
			if err := json.Unmarshal([]byte(task.GRPCResponse), &anomalies); err != nil {
				// Ignore parse error
			}

			// Append script alert summaries
			for _, sr := range scriptResults {
				if sr.ExitCode != 0 {
					summary := fmt.Sprintf("[脚本预警] %s (退出码:%d)", sr.RuleName, sr.ExitCode)
					if sr.Stdout != "" {
						// Extract alert lines from stdout
						lines := strings.Split(sr.Stdout, "\n")
						for _, line := range lines {
							if strings.Contains(line, "预警") {
								summary += "\n  " + strings.TrimSpace(line)
							}
						}
					}
					anomalies = append(anomalies, summary)
				}
			}

			if notifyErr := SendInspectionNotification(
				notifConfig,
				"AIOps 集群巡检报告",
				task.LLMSummary,
				anomalies,
				task.AnomalyCount,
			); notifyErr != nil {
				log.Printf("Failed to send notification: %v", notifyErr)
			}
		}
	}

	log.Printf("Inspection completed: %d anomalies found", task.AnomalyCount)
	return task
}

// executeRules runs all enabled command/script rules and returns results.
func (s *InspectionScheduler) executeRules(clusterID uint) []ScriptResult {
	var rules []model.InspectionRule
	if err := database.DB.Where("enabled = ? AND rule_type IN ?", true, []string{"command", "script"}).Find(&rules).Error; err != nil {
		log.Printf("Failed to load inspection rules: %v", err)
		return nil
	}

	// Get cluster kubeconfig for kubectl commands
	kubeconfigPath, cleanup := s.prepareKubeconfig(clusterID)
	if cleanup != nil {
		defer cleanup()
	}

	var results []ScriptResult
	for _, rule := range rules {
		result := s.executeOneRule(rule, kubeconfigPath)
		results = append(results, result)
	}
	return results
}

// prepareKubeconfig writes the specified (or active) cluster's kubeconfig to a temp file.
func (s *InspectionScheduler) prepareKubeconfig(clusterID uint) (string, func()) {
	var cluster model.ClusterConfig
	var err error

	if clusterID > 0 {
		err = database.DB.First(&cluster, clusterID).Error
	} else {
		err = database.DB.Where("is_active = ?", true).First(&cluster).Error
	}

	if err != nil {
		log.Printf("No cluster found for inspection scripts: %v", err)
		return "", nil
	}
	if cluster.KubeConfig == "" {
		log.Printf("Cluster %s has no kubeconfig (agent mode?), scripts will use default kubeconfig", cluster.Name)
		return "", nil
	}

	tmpFile, err := os.CreateTemp("", "inspection-kubeconfig-*")
	if err != nil {
		log.Printf("Failed to create temp kubeconfig: %v", err)
		return "", nil
	}
	if _, err := tmpFile.WriteString(cluster.KubeConfig); err != nil {
		os.Remove(tmpFile.Name())
		log.Printf("Failed to write kubeconfig: %v", err)
		return "", nil
	}
	tmpFile.Close()
	os.Chmod(tmpFile.Name(), 0600)

	return tmpFile.Name(), func() { os.Remove(tmpFile.Name()) }
}

// executeOneRule executes a single command or script rule.
func (s *InspectionScheduler) executeOneRule(rule model.InspectionRule, kubeconfigPath string) ScriptResult {
	start := time.Now()
	result := ScriptResult{
		RuleID:   rule.ID,
		RuleName: rule.Name,
		RuleType: rule.RuleType,
	}

	timeout := time.Duration(rule.Timeout) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd

	switch rule.RuleType {
	case "command":
		cmd = exec.CommandContext(ctx, "sh", "-c", rule.Command)

	case "script":
		// Write script to temp file and execute
		interpreter := s.getInterpreter(rule.ScriptType)
		ext := s.getScriptExt(rule.ScriptType)

		tmpFile, err := os.CreateTemp("", fmt.Sprintf("inspection-*%s", ext))
		if err != nil {
			result.Error = fmt.Sprintf("failed to create temp file: %v", err)
			result.Duration = time.Since(start).String()
			return result
		}
		defer os.Remove(tmpFile.Name())

		if _, err := tmpFile.WriteString(rule.Script); err != nil {
			result.Error = fmt.Sprintf("failed to write script: %v", err)
			result.Duration = time.Since(start).String()
			return result
		}
		tmpFile.Close()

		// Make executable
		os.Chmod(tmpFile.Name(), 0700)

		cmd = exec.CommandContext(ctx, interpreter, tmpFile.Name())

	default:
		result.Error = fmt.Sprintf("unsupported rule type: %s", rule.RuleType)
		result.Duration = time.Since(start).String()
		return result
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Set up environment with safe PATH, kubeconfig, and rule config
	cmd.Env = append(os.Environ(), "PATH="+sanitizePath())
	if kubeconfigPath != "" {
		cmd.Env = append(cmd.Env, "KUBECONFIG="+kubeconfigPath)
	}
	// Inject rule-level config as env vars for scripts to use
	if rule.TargetNodes != "" {
		cmd.Env = append(cmd.Env, "INSPECTION_TARGET_NODES="+rule.TargetNodes)
	}
	if rule.Namespaces != "" {
		cmd.Env = append(cmd.Env, "INSPECTION_NAMESPACES="+rule.Namespaces)
	}

	err := cmd.Run()
	result.Stdout = truncateOutput(stdout.String(), 64*1024)
	result.Stderr = truncateOutput(stderr.String(), 16*1024)
	result.Duration = time.Since(start).String()

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	log.Printf("Rule [%s] executed: exit_code=%d, duration=%s", rule.Name, result.ExitCode, result.Duration)
	return result
}

func (s *InspectionScheduler) getInterpreter(scriptType string) string {
	switch strings.ToLower(scriptType) {
	case "python":
		return "python3"
	case "perl":
		return "perl"
	default:
		return "bash"
	}
}

func (s *InspectionScheduler) getScriptExt(scriptType string) string {
	switch strings.ToLower(scriptType) {
	case "python":
		return ".py"
	case "perl":
		return ".pl"
	default:
		return ".sh"
	}
}

// sanitizePath returns a safe PATH value.
func sanitizePath() string {
	return strings.Join([]string{"/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin"}, string(filepath.ListSeparator))
}

// truncateOutput truncates output to maxLen bytes.
func truncateOutput(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... [truncated]"
}
