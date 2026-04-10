package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatCompletionRequest represents an OpenAI-compatible chat completion request.
type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature *float64  `json:"temperature,omitempty"`
}

// ChatCompletionResponse represents an OpenAI-compatible chat completion response.
type ChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// LLMConfig represents the configuration for an LLM provider.
type LLMConfig struct {
	APIURL    string
	APIKey    string
	ModelName string
}

// LLMClient defines the interface for LLM operations.
type LLMClient interface {
	ChatCompletion(ctx context.Context, config LLMConfig, messages []Message, opts ...CallOption) (string, error)
	TestConnection(ctx context.Context, config LLMConfig) error
}

// CallOption allows optional parameters for LLM calls.
type CallOption func(*ChatCompletionRequest)

// WithTemperature sets the temperature for the LLM call.
func WithTemperature(t float64) CallOption {
	return func(req *ChatCompletionRequest) {
		req.Temperature = &t
	}
}

type llmClient struct {
	httpClient *http.Client
}

// New creates a new LLMClient instance.
func New() LLMClient {
	return &llmClient{
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ChatCompletion sends a chat completion request to the LLM provider.
func (c *llmClient) ChatCompletion(ctx context.Context, config LLMConfig, messages []Message, opts ...CallOption) (string, error) {
	reqBody := ChatCompletionRequest{
		Model:    config.ModelName,
		Messages: messages,
	}

	for _, opt := range opts {
		opt(&reqBody)
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	apiURL := config.APIURL
	// Ensure the URL ends with the chat completions path
	if apiURL != "" && apiURL[len(apiURL)-1] == '/' {
		apiURL = apiURL[:len(apiURL)-1]
	}
	// Auto-append /v1/chat/completions if not already present
	if len(apiURL) < len("/chat/completions") || apiURL[len(apiURL)-len("/chat/completions"):] != "/chat/completions" {
		apiURL += "/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+config.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM API returned status %d: %s", resp.StatusCode, string(body))
	}

	var completionResp ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&completionResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if len(completionResp.Choices) == 0 {
		return "", fmt.Errorf("no completion choices returned")
	}

	return completionResp.Choices[0].Message.Content, nil
}

// TestConnection tests the connectivity to the LLM provider.
func (c *llmClient) TestConnection(ctx context.Context, config LLMConfig) error {
	messages := []Message{
		{Role: "user", Content: "Hi"},
	}

	_, err := c.ChatCompletion(ctx, config, messages)
	if err != nil {
		return fmt.Errorf("connectivity test failed: %w", err)
	}

	return nil
}
