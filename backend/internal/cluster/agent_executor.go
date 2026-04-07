package cluster

import (
	"context"
	"fmt"
)

// AgentSender is the interface for sending commands to a remote agent.
// Implemented by agent.Hub.
type AgentSender interface {
	SendCommand(ctx context.Context, clusterID uint, args []string) ([]byte, error)
	SendAnalyze(ctx context.Context, clusterID uint, filters []string, namespace, labelSelector string) ([]byte, error)
	IsOnline(clusterID uint) bool
}

// agentHub is the global agent sender, set by agent.Hub on initialization.
var agentHub AgentSender

// SetAgentHub registers the global AgentSender (called by agent.Hub during init).
func SetAgentHub(hub AgentSender) {
	agentHub = hub
}

// GetAgentHub returns the global AgentSender.
func GetAgentHub() AgentSender {
	return agentHub
}

// AgentExecutor executes kubectl commands via a remote agent over WebSocket.
type AgentExecutor struct {
	clusterID uint
	hub       AgentSender
}

// NewAgentExecutor creates an AgentExecutor for the given cluster.
func NewAgentExecutor(clusterID uint) (*AgentExecutor, error) {
	if agentHub == nil {
		return nil, fmt.Errorf("agent hub not initialized")
	}
	if !agentHub.IsOnline(clusterID) {
		return nil, fmt.Errorf("agent for cluster %d is offline", clusterID)
	}
	return &AgentExecutor{
		clusterID: clusterID,
		hub:       agentHub,
	}, nil
}

// ExecKubectl sends kubectl command to the remote agent and returns output.
func (a *AgentExecutor) ExecKubectl(ctx context.Context, args []string) ([]byte, error) {
	return a.hub.SendCommand(ctx, a.clusterID, args)
}

// ExecAnalyze sends an analyze request to the remote agent and returns the structured response.
func (a *AgentExecutor) ExecAnalyze(ctx context.Context, filters []string, namespace, labelSelector string) ([]byte, error) {
	return a.hub.SendAnalyze(ctx, a.clusterID, filters, namespace, labelSelector)
}

// Close is a no-op for AgentExecutor (no temp files to clean up).
func (a *AgentExecutor) Close() {}
