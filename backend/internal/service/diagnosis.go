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
const intentPrompt = `你是一个 Kubernetes 运维 AI 助手。用户会用自然语言提问关于集群的问题。

你的任务是判断用户的问题是否需要查询集群数据。如果需要，生成对应的 kubectl 命令。

请严格按以下 JSON 格式回复（不要包含其他内容）：
{
  "need_query": true/false,
  "commands": ["kubectl ..."],
  "reason": "简要说明为什么需要/不需要查询"
}

规则：
1. 如果用户问的是集群状态、资源信息、版本、日志等需要实际数据的问题，need_query=true
2. 如果用户问的是概念性问题、最佳实践、配置建议等不需要实时数据的问题，need_query=false
3. 如果用户在评价或追问之前的回答（如"不对"、"错了"、"再查一下"），根据上下文判断是否需要重新查询
4. commands 数组可以包含多条命令（最多3条），按需生成
5. 只生成只读命令（get/describe/logs/top/version/api-resources），不要生成写操作
6. 命令不要包含 --kubeconfig 或 --context 参数
7. 不要使用已废弃的参数（如 kubectl version 不要加 --short）

示例：
用户：集群版本是多少？
{"need_query":true,"commands":["kubectl version"],"reason":"需要查询集群版本信息"}

用户：所有 Pod 状态怎么样？
{"need_query":true,"commands":["kubectl get pods --all-namespaces"],"reason":"需要查看所有Pod状态"}

用户：什么是 DaemonSet？
{"need_query":false,"commands":[],"reason":"概念性问题，不需要查询集群"}

用户：节点资源使用情况如何？
{"need_query":true,"commands":["kubectl top nodes","kubectl get nodes -o wide"],"reason":"需要查看节点资源使用和状态"}

用户：上一个回答不对
{"need_query":false,"commands":[],"reason":"用户在评价之前的回答，不需要新查询"}`

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
	database.DB.Where("session_id = ?", sessionID).Order("created_at asc").Limit(10).Find(&prevRecords)

	// Step 1: Intent recognition — ask LLM if we need to query the cluster
	intent := s.recognizeIntent(question, llmConf, prevRecords)

	// Step 2: Execute kubectl commands if needed
	clusterData := ""
	if intent.NeedQuery && len(intent.Commands) > 0 {
		clusterData = s.executeCommands(intent.Commands)
	}

	// Step 3: Build final LLM messages with conversation history + cluster data
	messages := []llmclient.Message{
		{Role: "system", Content: `你是一个 Kubernetes 运维专家 AI 助手。请遵循以下规则：
1. 回答要简洁精炼，直接给出结论，避免冗长的解释
2. 如果有集群查询结果，基于实际数据回答，直接呈现关键信息
3. 如果查询失败（如 Agent 离线），用一两句话说明原因即可，不要长篇大论教用户怎么修
4. 使用 Markdown 格式化输出，善用表格、代码块展示数据
5. 不要重复用户的问题，不要说"根据查询结果"之类的废话，直接给答案`},
	}

	// Add conversation history
	for _, r := range prevRecords {
		messages = append(messages, llmclient.Message{Role: "user", Content: r.Question})
		if r.LLMAvailable && r.LLMResponse != "" {
			messages = append(messages, llmclient.Message{Role: "assistant", Content: r.LLMResponse})
		}
	}

	// Build current user message
	userContent := question
	if clusterData != "" {
		userContent = fmt.Sprintf("%s\n\n以下是从集群查询到的实际数据：\n```\n%s\n```\n请基于以上数据回答用户问题。", question, clusterData)
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

	// Include recent history for context
	for _, r := range history {
		if len(history) > 4 {
			break
		}
		messages = append(messages, llmclient.Message{Role: "user", Content: r.Question})
	}

	messages = append(messages, llmclient.Message{Role: "user", Content: question})

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	resp, err := s.llmClient.ChatCompletion(ctx, llmConf, messages)
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
