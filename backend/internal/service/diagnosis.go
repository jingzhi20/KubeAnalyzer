package service

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/k8sgpt"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"context"
	"encoding/json"
	"fmt"
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

// SubmitQuery submits a diagnosis query and returns the analysis.
func (s *DiagnosisService) SubmitQuery(sessionID uint, userID uint, question string) (*model.DiagnosisRecord, error) {
	// Verify session belongs to user
	var session model.DiagnosisSession
	if err := database.DB.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Step 1: Call k8sgpt to get cluster diagnosis
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	k8sgptResp, k8sgptErr := s.k8sgptExec.Analyze(ctx, []string{}, "", true)

	k8sgptResponseJSON := ""
	if k8sgptErr != nil {
		// Return structured error
		return nil, fmt.Errorf("GRPC_ERROR|K8sGPT 分析失败|%s", k8sgptErr.Error())
	}

	// Serialize k8sgpt response
	if k8sgptResp != nil {
		if jsonData, err := json.Marshal(k8sgptResp); err == nil {
			k8sgptResponseJSON = string(jsonData)
		}
	}

	// Step 2: Get default LLM config
	var llmConfig model.LLMConfig
	if err := database.DB.Where("is_default = ?", true).First(&llmConfig).Error; err != nil {
		// LLM not available, return raw k8sgpt data
		record := &model.DiagnosisRecord{
			SessionID:    sessionID,
			Question:     question,
			GRPCResponse: k8sgptResponseJSON,
			LLMResponse:  "LLM 分析不可用：未设置默认 LLM 配置",
			LLMAvailable: false,
		}
		database.DB.Create(record)
		return record, nil
	}

	// Step 3: Send to LLM for analysis
	llmMessages := []llmclient.Message{
		{Role: "system", Content: "你是一个 Kubernetes 运维专家。请分析以下集群诊断数据并给出专业的诊断结果和修复建议。"},
		{Role: "user", Content: fmt.Sprintf("用户问题：%s\n\n集群诊断数据：%s", question, k8sgptResponseJSON)},
	}

	llmConfigClient := llmclient.LLMConfig{
		APIURL:    llmConfig.APIURL,
		APIKey:    llmConfig.APIKey,
		ModelName: llmConfig.ModelName,
	}

	ctxLLM, cancelLLM := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelLLM()

	llmResponse, llmErr := s.llmClient.ChatCompletion(ctxLLM, llmConfigClient, llmMessages)

	// Step 4: Create diagnosis record
	record := &model.DiagnosisRecord{
		SessionID:    sessionID,
		Question:     question,
		GRPCResponse: k8sgptResponseJSON,
		LLMAvailable: llmErr == nil,
	}

	if llmErr != nil {
		// LLM failed, return raw gRPC data
		record.LLMResponse = fmt.Sprintf("LLM 分析失败：%s", llmErr.Error())
	} else {
		record.LLMResponse = llmResponse
	}

	if err := database.DB.Create(record).Error; err != nil {
		return nil, fmt.Errorf("failed to save diagnosis record: %w", err)
	}

	return record, nil
}
