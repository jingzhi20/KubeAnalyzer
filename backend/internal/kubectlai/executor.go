package kubectlai

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/database"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const defaultSystemPrompt = `You are a Kubernetes expert. The user will describe what they want to do in natural language.
Your job is to generate the correct kubectl command.

Rules:
1. Output ONLY the kubectl command, nothing else
2. Do not include any explanation or markdown formatting
3. The command must be a valid kubectl command
4. If the user's request is ambiguous, generate the most likely intended command
5. Always use explicit resource names when possible
6. For listing resources, default to all namespaces unless specified

Examples:
User: list all pods that are not running
Output: kubectl get pods --all-namespaces --field-selector=status.phase!=Running

User: show me the logs of pod nginx in namespace default
Output: kubectl logs -n default nginx

User: scale deployment web to 3 replicas
Output: kubectl scale deployment web --replicas=3

User: get events sorted by time
Output: kubectl get events --all-namespaces --sort-by='.lastTimestamp'`

// ExecuteResult represents the result of a command generation/execution.
type ExecuteResult struct {
	Prompt  string `json:"prompt"`
	Command string `json:"command"`
	Output  string `json:"output,omitempty"`
}

// Executor handles kubectl command generation via LLM (replaces kubectl-ai CLI).
type Executor struct {
	llmClient llmclient.LLMClient
}

func NewExecutor() *Executor {
	return &Executor{llmClient: llmclient.New()}
}

func getAIConfig() model.KubectlAIConfig {
	var config model.KubectlAIConfig
	database.DB.FirstOrCreate(&config)
	return config
}

// getLLMConfig returns the system default LLM configuration.
func (e *Executor) getLLMConfig() (*llmclient.LLMConfig, error) {
	var llmConf model.LLMConfig
	if err := database.DB.Where("is_default = ?", true).First(&llmConf).Error; err != nil {
		return nil, fmt.Errorf("\u672a\u914d\u7f6e LLM\uff1a\u7cfb\u7edf\u672a\u8bbe\u7f6e\u9ed8\u8ba4\u5927\u6a21\u578b\uff0c\u8bf7\u524d\u5f80 \u201cLLM \u914d\u7f6e\u201d \u9875\u9762\u6dfb\u52a0\u5e76\u8bbe\u7f6e\u9ed8\u8ba4\u6a21\u578b")
	}
	return &llmclient.LLMConfig{
		APIURL:    llmConf.APIURL,
		APIKey:    llmConf.APIKey,
		ModelName: llmConf.ModelName,
	}, nil
}

// getSystemPrompt returns the system prompt for kubectl command generation.
func getSystemPrompt() string {
	config := getAIConfig()
	if config.SystemPrompt != "" {
		return config.SystemPrompt
	}
	return defaultSystemPrompt
}

// Generate generates a kubectl command from natural language using LLM.
func (e *Executor) Generate(ctx context.Context, prompt string) (*ExecuteResult, error) {
	llmConf, err := e.getLLMConfig()
	if err != nil {
		return nil, err
	}

	messages := []llmclient.Message{
		{Role: "system", Content: getSystemPrompt()},
		{Role: "user", Content: prompt},
	}

	llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	response, err := e.llmClient.ChatCompletion(llmCtx, *llmConf, messages)
	if err != nil {
		return nil, fmt.Errorf("LLM 生成命令失败: %w", err)
	}

	command := extractKubectlCommand(response)

	return &ExecuteResult{
		Prompt:  prompt,
		Command: command,
		Output:  response,
	}, nil
}

// Execute generates a kubectl command via LLM and executes it.
func (e *Executor) Execute(ctx context.Context, prompt string, userID, clusterID uint) (*ExecuteResult, error) {
	// Step 1: Generate the command
	genResult, err := e.Generate(ctx, prompt)
	if err != nil {
		return nil, err
	}

	if genResult.Command == "" {
		return genResult, fmt.Errorf("LLM 未返回有效的 kubectl 命令")
	}

	// Step 2: Get executor for active cluster
	executor, err := cluster.GetActiveExecutor()
	if err != nil {
		return genResult, fmt.Errorf("no active cluster: %w", err)
	}
	defer executor.Close()

	// Step 3: Parse and execute the kubectl command
	execCmd := genResult.Command
	// Remove any existing --kubeconfig or --context flags (executor handles them)
	execCmd = removeKubeconfigFlags(execCmd)

	cmdParts := splitCommand(execCmd)
	if len(cmdParts) < 2 {
		return genResult, fmt.Errorf("无效的命令: %s", execCmd)
	}

	// Remove "kubectl" prefix, executor adds it
	args := cmdParts[1:]

	cmdCtx, cmdCancel := context.WithTimeout(ctx, 60*time.Second)
	defer cmdCancel()

	output, execErr := executor.ExecKubectl(cmdCtx, args)
	outputStr := strings.TrimSpace(string(output))

	// If output is empty but we have an error, use the error message as output
	// This ensures Agent security policy messages (high-risk block / read-only) are captured
	if outputStr == "" && execErr != nil {
		outputStr = execErr.Error()
		// Strip "agent error: " prefix if present
		outputStr = strings.TrimPrefix(outputStr, "agent error: ")
	}

	// Save history regardless of execution result
	history := model.KubectlAIHistory{
		UserID:    userID,
		ClusterID: clusterID,
		Prompt:    prompt,
		Command:   genResult.Command,
		Output:    outputStr,
		Executed:  execErr == nil,
	}
	database.DB.Create(&history)

	result := &ExecuteResult{
		Prompt:  prompt,
		Command: genResult.Command,
		Output:  outputStr,
	}

	if execErr != nil {
		return result, fmt.Errorf("%s", outputStr)
	}

	return result, nil
}

// removeKubeconfigFlags removes --kubeconfig and --context flags from a command string.
func removeKubeconfigFlags(cmd string) string {
	// Remove --kubeconfig <path>
	re := regexp.MustCompile(`\s*--kubeconfig\s+\S+`)
	cmd = re.ReplaceAllString(cmd, "")
	// Remove --context <name>
	re2 := regexp.MustCompile(`\s*--context\s+\S+`)
	cmd = re2.ReplaceAllString(cmd, "")
	return strings.TrimSpace(cmd)
}

// GetConfig returns the current configuration.
func (e *Executor) GetConfig() (*model.KubectlAIConfig, error) {
	var config model.KubectlAIConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateConfig updates the configuration.
func (e *Executor) UpdateConfig(req UpdateConfigRequest) (*model.KubectlAIConfig, error) {
	var config model.KubectlAIConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if err := database.DB.Model(&config).Updates(map[string]interface{}{
		"temperature":     req.Temperature,
		"system_prompt":   req.SystemPrompt,
		"custom_examples": req.CustomExamples,
		"updated_at":      time.Now(),
	}).Error; err != nil {
		return nil, err
	}

	// Reload to return the persisted state
	database.DB.First(&config, config.ID)
	return &config, nil
}

// UpdateConfigRequest represents the request payload for config update.
type UpdateConfigRequest struct {
	Temperature    float64 `json:"temperature"`
	SystemPrompt   string  `json:"system_prompt"`
	CustomExamples string  `json:"custom_examples"`
}

// GetHistory returns command history for a user.
func (e *Executor) GetHistory(userID uint) ([]model.KubectlAIHistory, error) {
	var history []model.KubectlAIHistory
	if err := database.DB.Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(50).
		Find(&history).Error; err != nil {
		return nil, err
	}
	return history, nil
}

// extractKubectlCommand extracts a kubectl command from LLM response text.
func extractKubectlCommand(text string) string {
	text = strings.TrimSpace(text)

	// Remove markdown code block markers
	text = strings.TrimPrefix(text, "```bash")
	text = strings.TrimPrefix(text, "```shell")
	text = strings.TrimPrefix(text, "```sh")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// Try to find kubectl command with regex
	re := regexp.MustCompile(`(?m)^(kubectl\s+.+)$`)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If the entire text starts with kubectl
	if strings.HasPrefix(text, "kubectl ") {
		// Return first line only
		lines := strings.SplitN(text, "\n", 2)
		return strings.TrimSpace(lines[0])
	}

	// Search line by line
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "kubectl ") {
			return line
		}
	}

	return text
}

// splitCommand splits a command string into arguments, respecting quotes.
func splitCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		} else if c == '\'' || c == '"' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' || c == '\t' {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
