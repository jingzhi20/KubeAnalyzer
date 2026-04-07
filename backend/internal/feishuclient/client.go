package feishuclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// FeishuClient handles Feishu webhook notifications.
type FeishuClient struct {
	httpClient *http.Client
}

// New creates a new FeishuClient instance.
func New() *FeishuClient {
	return &FeishuClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage sends a message to Feishu webhook.
func (c *FeishuClient) SendMessage(ctx context.Context, webhookURL string, message interface{}) error {
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// FormatInspectionCard formats an inspection result as a Feishu rich text card.
func (c *FeishuClient) FormatInspectionCard(title string, summary string, anomalies []string, timestamp time.Time) map[string]interface{} {
	elements := []map[string]interface{}{
		{
			"tag": "markdown",
			"content": fmt.Sprintf("**摘要**\n%s", summary),
		},
	}

	if len(anomalies) > 0 {
		anomalyContent := "**异常详情**\n"
		for i, anomaly := range anomalies {
			anomalyContent += fmt.Sprintf("%d. %s\n", i+1, anomaly)
		}
		elements = append(elements, map[string]interface{}{
			"tag":     "markdown",
			"content": anomalyContent,
		})
	}

	elements = append(elements, map[string]interface{}{
		"tag":     "markdown",
		"content": fmt.Sprintf("**执行时间**\n%s", timestamp.Format("2006-01-02 15:04:05")),
	})

	card := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]interface{}{
					"tag":     "plain_text",
					"content": title,
				},
			},
			"elements": elements,
		},
	}

	return card
}

// FormatTestMessage creates a test message for webhook validation.
func (c *FeishuClient) FormatTestMessage() map[string]interface{} {
	return map[string]interface{}{
		"msg_type": "text",
		"content": map[string]interface{}{
			"text": "AIOps 平台测试消息 - Webhook 配置验证成功",
		},
	}
}
