package cluster

import (
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// GetActiveCluster returns the currently active cluster.
func GetActiveCluster() (*model.ClusterConfig, error) {
	var cluster model.ClusterConfig
	if err := database.DB.Where("is_active = ?", true).First(&cluster).Error; err != nil {
		return nil, fmt.Errorf("no active cluster configured")
	}
	return &cluster, nil
}

// WriteKubeconfig writes kubeconfig content to a temp file and returns path + cleanup func.
func WriteKubeconfig(content string) (string, func(), error) {
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, fmt.Sprintf("aiops-kubeconfig-%d", time.Now().UnixNano()))

	if err := os.WriteFile(tmpFile, []byte(content), 0600); err != nil {
		return "", nil, fmt.Errorf("failed to write kubeconfig: %w", err)
	}

	cleanup := func() {
		os.Remove(tmpFile)
	}

	return tmpFile, cleanup, nil
}

// CloseableExecutor wraps KubeExecutor with a Close method for cleanup.
type CloseableExecutor struct {
	KubeExecutor
	CloseFunc func()
}

// Close calls the cleanup function.
func (c *CloseableExecutor) Close() {
	if c.CloseFunc != nil {
		c.CloseFunc()
	}
}

// GetActiveExecutor returns a KubeExecutor for the active cluster.
// Caller must call Close() on the returned executor when done.
func GetActiveExecutor() (*CloseableExecutor, error) {
	ac, err := GetActiveCluster()
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}
	return GetExecutorForCluster(ac)
}

// GetExecutorForCluster returns a KubeExecutor for a specific cluster config.
// Caller must call Close() on the returned executor when done.
func GetExecutorForCluster(c *model.ClusterConfig) (*CloseableExecutor, error) {
	if c.ConnMode == "agent" {
		exec, err := NewAgentExecutor(c.ID)
		if err != nil {
			return nil, err
		}
		return &CloseableExecutor{KubeExecutor: exec, CloseFunc: func() {}}, nil
	}

	// Default: direct mode
	if c.KubeConfig == "" {
		return nil, fmt.Errorf("cluster %s has no kubeconfig configured (direct mode requires kubeconfig)", c.Name)
	}
	exec, err := NewDirectExecutor(c.KubeConfig, c.Context)
	if err != nil {
		return nil, err
	}
	return &CloseableExecutor{KubeExecutor: exec, CloseFunc: exec.Close}, nil
}
