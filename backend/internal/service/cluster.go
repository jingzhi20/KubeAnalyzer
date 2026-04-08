package service

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"context"
	"fmt"
	"time"
)

// ClusterService handles cluster management operations.
type ClusterService struct{}

// NewClusterService creates a new ClusterService instance.
func NewClusterService() *ClusterService {
	return &ClusterService{}
}

// ListClusters returns all cluster configurations.
// For agent-mode clusters, dynamically check heartbeat timeout to update agent_status.
func (s *ClusterService) ListClusters() ([]model.ClusterConfig, error) {
	var clusters []model.ClusterConfig
	if err := database.DB.Order("created_at desc").Find(&clusters).Error; err != nil {
		return nil, fmt.Errorf("failed to list clusters: %w", err)
	}

	// Check agent heartbeat timeout (60 seconds)
	now := time.Now()
	for i := range clusters {
		if clusters[i].ConnMode == "agent" && clusters[i].AgentStatus == "online" {
			if clusters[i].LastPingAt == nil || now.Sub(*clusters[i].LastPingAt) > 60*time.Second {
				clusters[i].AgentStatus = "offline"
				// Persist the status change
				database.DB.Model(&clusters[i]).Update("agent_status", "offline")
			}
		}
	}

	return clusters, nil
}

// CreateCluster creates a new cluster configuration.
func (s *ClusterService) CreateCluster(name, kubeconfig, kubeContext, serverURL, connMode string) (*model.ClusterConfig, error) {
	if connMode == "" {
		connMode = "direct"
	}
	cluster := model.ClusterConfig{
		Name:       name,
		KubeConfig: kubeconfig,
		Context:    kubeContext,
		ServerURL:  serverURL,
		ConnMode:   connMode,
		Status:     "unknown",
		AgentStatus: "offline",
	}

	if err := database.DB.Create(&cluster).Error; err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}
	return &cluster, nil
}

// UpdateCluster updates an existing cluster configuration.
func (s *ClusterService) UpdateCluster(id uint, name, kubeconfig, kubeContext, serverURL, connMode string) (*model.ClusterConfig, error) {
	var c model.ClusterConfig
	if err := database.DB.First(&c, id).Error; err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	c.Name = name
	c.KubeConfig = kubeconfig
	c.Context = kubeContext
	c.ServerURL = serverURL
	if connMode != "" {
		c.ConnMode = connMode
	}
	c.UpdatedAt = time.Now()

	if err := database.DB.Save(&c).Error; err != nil {
		return nil, fmt.Errorf("failed to update cluster: %w", err)
	}
	return &c, nil
}

// DeleteCluster deletes a cluster configuration.
func (s *ClusterService) DeleteCluster(id uint) error {
	var cluster model.ClusterConfig
	if err := database.DB.First(&cluster, id).Error; err != nil {
		return fmt.Errorf("cluster not found: %w", err)
	}
	if cluster.IsActive {
		return fmt.Errorf("cannot delete active cluster")
	}
	return database.DB.Delete(&cluster).Error
}

// SetActiveCluster sets a cluster as the active one.
func (s *ClusterService) SetActiveCluster(id uint) (*model.ClusterConfig, error) {
	var cluster model.ClusterConfig
	if err := database.DB.First(&cluster, id).Error; err != nil {
		return nil, fmt.Errorf("cluster not found: %w", err)
	}

	// Reset all clusters to inactive
	if err := database.DB.Model(&model.ClusterConfig{}).Where("1=1").Update("is_active", false).Error; err != nil {
		return nil, fmt.Errorf("failed to reset active: %w", err)
	}

	cluster.IsActive = true
	cluster.UpdatedAt = time.Now()
	if err := database.DB.Save(&cluster).Error; err != nil {
		return nil, fmt.Errorf("failed to set active: %w", err)
	}

	return &cluster, nil
}

// TestCluster tests cluster connectivity.
func (s *ClusterService) TestCluster(id uint) (string, error) {
	var c model.ClusterConfig
	if err := database.DB.First(&c, id).Error; err != nil {
		return "", fmt.Errorf("cluster not found: %w", err)
	}

	exec, err := cluster.GetExecutorForCluster(&c)
	if err != nil {
		c.Status = "disconnected"
		database.DB.Save(&c)
		return "", fmt.Errorf("failed to create executor: %w", err)
	}
	defer exec.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	output, err := exec.ExecKubectl(ctx, []string{"cluster-info"})
	if err != nil {
		c.Status = "disconnected"
		database.DB.Save(&c)
		return string(output), fmt.Errorf("cluster connectivity test failed: %s", string(output))
	}

	c.Status = "connected"
	c.UpdatedAt = time.Now()
	database.DB.Save(&c)
	return string(output), nil
}

