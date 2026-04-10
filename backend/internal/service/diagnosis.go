package service

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/database"
	"aiops-backend/internal/k8sgpt"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// DiagnosisService handles diagnosis operations.
type DiagnosisService struct {
	k8sgptExec *k8sgpt.Executor
	llmClient  llmclient.LLMClient
}

// NewDiagnosisService creates a new DiagnosisService instance.
func NewDiagnosisService() *DiagnosisService {
	return &DiagnosisService{
		k8sgptExec: k8sgpt.NewExecutor(),
		llmClient:  llmclient.New(),
	}
}

// CreateSession creates a new diagnosis session.
func (s *DiagnosisService) CreateSession(userID uint, title string) (*model.DiagnosisSession, error) {
	session := model.DiagnosisSession{
		UserID: userID,
		Title:  title,
	}

	if err := database.DB.Create(&session).Error; err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

// ListSessions returns all diagnosis sessions for a user.
func (s *DiagnosisService) ListSessions(userID uint) ([]model.DiagnosisSession, error) {
	var sessions []model.DiagnosisSession
	if err := database.DB.Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	return sessions, nil
}

// GetSession returns a session with its records.
func (s *DiagnosisService) GetSession(sessionID uint, userID uint) (*model.DiagnosisSession, error) {
	var session model.DiagnosisSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).
		Preload("Records").
		First(&session).Error; err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	return &session, nil
}

// DeleteSession deletes a diagnosis session and its records.
func (s *DiagnosisService) DeleteSession(sessionID uint, userID uint) error {
	// First verify ownership
	var session model.DiagnosisSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return fmt.Errorf("session not found")
	}
	// Delete associated records first
	database.DB.Where("session_id = ?", sessionID).Delete(&model.DiagnosisRecord{})
	// Delete the session
	if err := database.DB.Delete(&session).Error; err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

// RenameSession renames a diagnosis session.
func (s *DiagnosisService) RenameSession(sessionID uint, userID uint, title string) (*model.DiagnosisSession, error) {
	var session model.DiagnosisSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}
	session.Title = title
	if err := database.DB.Save(&session).Error; err != nil {
		return nil, fmt.Errorf("failed to rename session: %w", err)
	}
	return &session, nil
}

// intentPrompt is the system prompt for intent recognition and command generation.
const intentPrompt = `你是一个 Kubernetes 运维助手。根据用户问题生成 kubectl 只读命令。

规则：
1. 只生成 get、describe、logs、top、version、cluster-info、api-resources 命令
2. 未指定 namespace 则用 -A
3. 最多 3 条命令
4. 不需要查集群的问题（概念、反馈等），need_query=false
5. 直接用用户提到的资源名称，不要自作主张加 API group 后缀

回复 JSON：
{"need_query":true/false,"commands":["kubectl ..."],"reason":"说明"}`

// intentResult represents the LLM's intent recognition result.
type intentResult struct {
	NeedQuery bool     `json:"need_query"`
	Commands  []string `json:"commands"`
	Reason    string   `json:"reason"`
}

// SubmitQuery implements an AI Agent flow: intent recognition → kubectl execution → LLM summary.
func (s *DiagnosisService) SubmitQuery(sessionID uint, userID uint, question string) (*model.DiagnosisRecord, error) {
	// Verify session belongs to user
	var session model.DiagnosisSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Get default LLM config
	var llmConfig model.LLMConfig
	if err := database.DB.Where("is_default = ?", true).First(&llmConfig).Error; err != nil {
		record := &model.DiagnosisRecord{
			SessionID:    sessionID,
			Question:     question,
			LLMResponse:  "LLM 未配置：请先在「系统管理 → LLM 配置」中添加并设为默认。",
			LLMAvailable: false,
		}
		database.DB.Create(record)
		return record, nil
	}

	llmConf := llmclient.LLMConfig{
		APIURL:    llmConfig.APIURL,
		APIKey:    llmConfig.APIKey,
		ModelName: llmConfig.ModelName,
	}

	// Load conversation history for multi-turn context
	var prevRecords []model.DiagnosisRecord
	database.DB.Where("session_id = ?", sessionID).Order("created_at desc").Limit(10).Find(&prevRecords)
	// Reverse to chronological order for LLM context
	for i, j := 0, len(prevRecords)-1; i < j; i, j = i+1, j-1 {
		prevRecords[i], prevRecords[j] = prevRecords[j], prevRecords[i]
	}

	// Step 1: Intent recognition — ask LLM if we need to query the cluster
	intent := s.recognizeIntent(question, llmConf, prevRecords)

	// Step 2: Execute kubectl commands if needed
	clusterData := ""
	if intent.NeedQuery && len(intent.Commands) > 0 {
		clusterData = s.executeCommands(intent.Commands)
	} else if intent.NeedQuery {
		// Commands were empty or all filtered — hint LLM to answer from knowledge
		clusterData = "[无可用查询命令，请基于专业知识回答]"
	}

	// Step 3: Build final LLM messages with conversation history + cluster data
	messages := []llmclient.Message{
		{Role: "system", Content: `你是一个 Kubernetes 运维专家 AI 助手。请遵循以下规则：
1. 简洁精炼，直接给结论
2. 本次实时查询的数据是最权威的，优先基于它回答，忽略历史中可能过时的数据
3. 善用 Markdown 表格、代码块展示数据
4. 不要重复问题，不要说"根据查询结果"，直接给答案
5. 用户说之前回答有误时，回顾对话历史纠正错误，不要查新资源
6. 查询失败时用专业知识回答，不要只说"查询失败"就结束`},
	}

	// Add conversation history (question + answer only, no old cluster data to avoid confusion)
	for _, r := range prevRecords {
		messages = append(messages, llmclient.Message{Role: "user", Content: r.Question})
		if r.LLMAvailable && r.LLMResponse != "" {
			messages = append(messages, llmclient.Message{Role: "assistant", Content: r.LLMResponse})
		}
	}

	// Build current user message — clearly mark fresh data
	userContent := question
	if clusterData != "" {
		userContent = fmt.Sprintf("%s\n\n以下是【本次实时查询】的集群数据（以此为准）：\n```\n%s\n```", question, clusterData)
	}
	messages = append(messages, llmclient.Message{Role: "user", Content: userContent})

	ctxLLM, cancelLLM := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelLLM()

	llmResponse, llmErr := s.llmClient.ChatCompletion(ctxLLM, llmConf, messages)

	// Create diagnosis record
	record := &model.DiagnosisRecord{
		SessionID:    sessionID,
		Question:     question,
		GRPCResponse: clusterData,
		LLMAvailable: llmErr == nil,
	}

	if llmErr != nil {
		record.LLMResponse = fmt.Sprintf("LLM 分析失败：%s", llmErr.Error())
	} else {
		record.LLMResponse = llmResponse
	}

	if err := database.DB.Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to save diagnosis record: %w", err)
	}

	return record, nil
}

// recognizeIntent uses LLM to determine if the question needs cluster data.
func (s *DiagnosisService) recognizeIntent(question string, llmConf llmclient.LLMConfig, history []model.DiagnosisRecord) intentResult {
	// Default: no query needed
	fallback := intentResult{NeedQuery: false}

	messages := []llmclient.Message{
		{Role: "system", Content: intentPrompt},
	}

	// Include recent history questions only (no answers to avoid context pollution)
	start := 0
	if len(history) > 4 {
		start = len(history) - 4
	}
	for _, r := range history[start:] {
		messages = append(messages, llmclient.Message{Role: "user", Content: r.Question})
	}

	messages = append(messages, llmclient.Message{Role: "user", Content: question})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := s.llmClient.ChatCompletion(ctx, llmConf, messages, llmclient.WithTemperature(0.1))
	if err != nil {
		return fallback
	}

	// Extract JSON from response (handle markdown code blocks)
	jsonStr := extractJSON(resp)
	var result intentResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fallback
	}

	// Safety: limit to 3 commands, only allow read-only operations
	var safeCommands []string
	for _, cmd := range result.Commands {
		if len(safeCommands) >= 3 {
			break
		}
		// Reject commands with shell operators (pipes, redirects, subshells)
		if strings.ContainsAny(cmd, "|>&;$`") {
			continue
		}
		if isReadOnlyCommand(cmd) {
			safeCommands = append(safeCommands, cmd)
		}
	}
	result.Commands = safeCommands

	return result
}

// executeCommands runs kubectl commands against the active cluster and returns combined output.
func (s *DiagnosisService) executeCommands(commands []string) string {
	executor, err := cluster.GetActiveExecutor()
	if err != nil {
		return fmt.Sprintf("[集群连接失败: %s]", err.Error())
	}
	defer executor.Close()

	var results []string
	for _, cmd := range commands {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		// Parse command: remove "kubectl" prefix
		args := parseKubectlArgs(cmd)
		if len(args) == 0 {
			cancel()
			continue
		}

		output, execErr := executor.ExecKubectl(ctx, args)
		cancel()

		header := fmt.Sprintf("$ %s", cmd)
		if execErr != nil {
			results = append(results, fmt.Sprintf("%s\n[执行失败: %s]\n%s", header, execErr.Error(), string(output)))
		} else {
			results = append(results, fmt.Sprintf("%s\n%s", header, strings.TrimSpace(string(output))))
		}
	}

	return strings.Join(results, "\n\n")
}

// extractJSON extracts JSON from a string that may contain markdown code blocks.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Try to find JSON in code blocks
	re := regexp.MustCompile("(?s)```(?:json)?\\s*\\n?(\\{.*?\\})\\s*```")
	if matches := re.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}
	// Try to find raw JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

// parseKubectlArgs parses a kubectl command string into args (removing "kubectl" prefix).
func parseKubectlArgs(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "kubectl ")
	if cmd == "" {
		return nil
	}
	// Remove --kubeconfig and --context flags
	re := regexp.MustCompile(`\s*--(kubeconfig|context)\s+\S+`)
	cmd = re.ReplaceAllString(cmd, "")
	return strings.Fields(cmd)
}

// isReadOnlyCommand checks if a kubectl command is read-only (safe to execute).
func isReadOnlyCommand(cmd string) bool {
	cmd = strings.TrimSpace(strings.TrimPrefix(cmd, "kubectl"))
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}
	readOnlyVerbs := map[string]bool{
		"get": true, "describe": true, "logs": true, "top": true,
		"version": true, "api-resources": true, "api-versions": true,
		"explain": true, "cluster-info": true, "config": true,
		"auth": true, "diff": true,
	}
	return readOnlyVerbs[parts[0]]
}
