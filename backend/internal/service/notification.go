package service

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/feishuclient"
	"aiops-backend/internal/model"
	"context"
	"time"
)

// ShouldNotify checks if notification should be sent based on policy.
func ShouldNotify(config model.NotificationConfig, anomalyCount int) bool {
	if !config.Enabled {
		return false
	}

	switch config.Policy {
	case "always":
		return true
	case "anomaly_only":
		return anomalyCount > 0
	case "disabled":
		return false
	default:
		return false
	}
}

// SendInspectionNotification sends inspection result notification.
func SendInspectionNotification(config model.NotificationConfig, title string, summary string, anomalies []string, anomalyCount int) error {
	if !ShouldNotify(config, anomalyCount) {
		return nil
	}

	feishuClient := feishuclient.New()
	message := feishuClient.FormatInspectionCard(title, summary, anomalies, time.Now())
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return feishuClient.SendMessage(ctx, config.WebhookURL, message)
}

// GetNotificationConfig retrieves notification config from database.
func GetNotificationConfig() (model.NotificationConfig, error) {
	var config model.NotificationConfig
	if err := database.DB.First(&config).Error; err != nil {
		return config, err
	}
	return config, nil
}
