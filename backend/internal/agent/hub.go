package agent

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/database"
	"aiops-backend/internal/model"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Message types for Agent <-> Server communication.
type AgentMessage struct {
	ID            string   `json:"id,omitempty"`
	Type          string   `json:"type"`                        // kubectl / analyze / result / ping / pong / error / register
	Args          []string `json:"args,omitempty"`              // kubectl args (server -> agent)
	Output        string   `json:"output,omitempty"`            // command output (agent -> server)
	Error         string   `json:"error,omitempty"`             // error message
	ClusterID     uint     `json:"cluster_id,omitempty"`
	// Analyze request fields (server -> agent)
	Filters       []string `json:"filters,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	LabelSelector string   `json:"label_selector,omitempty"`
	// Agent registration fields (agent -> server)
	AllowWrite    bool     `json:"allow_write,omitempty"`       // Agent 是否允许写操作
}

// AgentConn represents a connected agent.
type AgentConn struct {
	ClusterID  uint
	Conn       *websocket.Conn
	LastPing   time.Time
	mu         sync.Mutex
	pending    map[string]chan *AgentMessage // request ID -> response channel
	pendingMu  sync.Mutex
}

// Hub manages all connected agents.
type Hub struct {
	agents map[uint]*AgentConn // clusterID -> agent connection
	mu     sync.RWMutex
}

var globalHub *Hub

// NewHub creates and initializes the global Agent Hub.
func NewHub() *Hub {
	globalHub = &Hub{
		agents: make(map[uint]*AgentConn),
	}
	// Register as the global agent sender for cluster package.
	cluster.SetAgentHub(globalHub)
	// Start health checker
	go globalHub.healthChecker()
	return globalHub
}

// GetHub returns the global hub instance.
func GetHub() *Hub {
	return globalHub
}

// RegisterAgent adds a new agent connection.
func (h *Hub) RegisterAgent(clusterID uint, conn *websocket.Conn) *AgentConn {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Close existing connection if any
	if existing, ok := h.agents[clusterID]; ok {
		existing.Conn.Close()
	}

	agent := &AgentConn{
		ClusterID: clusterID,
		Conn:      conn,
		LastPing:  time.Now(),
		pending:   make(map[string]chan *AgentMessage),
	}
	h.agents[clusterID] = agent

	// Update database status (allow_write will be updated when register message received)
	h.updateAgentStatus(clusterID, "online", false)
	log.Printf("[AgentHub] Agent registered for cluster %d", clusterID)

	return agent
}

// UnregisterAgent removes an agent connection.
func (h *Hub) UnregisterAgent(clusterID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if agent, ok := h.agents[clusterID]; ok {
		// Cancel all pending requests
		agent.pendingMu.Lock()
		for _, ch := range agent.pending {
			close(ch)
		}
		agent.pending = make(map[string]chan *AgentMessage)
		agent.pendingMu.Unlock()

		agent.Conn.Close()
		delete(h.agents, clusterID)
	}

	h.updateAgentStatus(clusterID, "offline", false)
	log.Printf("[AgentHub] Agent unregistered for cluster %d", clusterID)
}

// IsOnline checks if an agent is connected for a cluster.
func (h *Hub) IsOnline(clusterID uint) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.agents[clusterID]
	return ok
}

// SendCommand sends a kubectl command to a remote agent and waits for the response.
// Implements cluster.AgentSender interface.
func (h *Hub) SendCommand(ctx context.Context, clusterID uint, args []string) ([]byte, error) {
	msg := AgentMessage{
		Type: "kubectl",
		Args: args,
	}
	return h.sendAndWait(ctx, clusterID, msg)
}

// SendAnalyze sends an analyze request to a remote agent and waits for the response.
// Implements cluster.AgentSender interface.
func (h *Hub) SendAnalyze(ctx context.Context, clusterID uint, filters []string, namespace, labelSelector string) ([]byte, error) {
	msg := AgentMessage{
		Type:          "analyze",
		Filters:       filters,
		Namespace:     namespace,
		LabelSelector: labelSelector,
	}
	return h.sendAndWait(ctx, clusterID, msg)
}

// sendAndWait is the generic request-response handler for agent communication.
func (h *Hub) sendAndWait(ctx context.Context, clusterID uint, msg AgentMessage) ([]byte, error) {
	h.mu.RLock()
	agent, ok := h.agents[clusterID]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("agent for cluster %d is not connected", clusterID)
	}

	// Generate unique request ID
	reqID := fmt.Sprintf("req-%d-%d", clusterID, time.Now().UnixNano())
	msg.ID = reqID

	// Create response channel
	respCh := make(chan *AgentMessage, 1)
	agent.pendingMu.Lock()
	agent.pending[reqID] = respCh
	agent.pendingMu.Unlock()

	defer func() {
		agent.pendingMu.Lock()
		delete(agent.pending, reqID)
		agent.pendingMu.Unlock()
	}()

	// Send message to agent
	agent.mu.Lock()
	err := agent.Conn.WriteJSON(msg)
	agent.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send message to agent: %w", err)
	}

	// Wait for response with context timeout
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp, ok := <-respCh:
		if !ok {
			return nil, fmt.Errorf("agent connection closed before response")
		}
		if resp.Error != "" {
			return []byte(resp.Output), fmt.Errorf("agent error: %s", resp.Error)
		}
		return []byte(resp.Output), nil
	}
}

// HandleAgentMessages reads messages from an agent connection and dispatches them.
func (h *Hub) HandleAgentMessages(agent *AgentConn) {
	defer h.UnregisterAgent(agent.ClusterID)

	for {
		var msg AgentMessage
		err := agent.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[AgentHub] Agent %d read error: %v", agent.ClusterID, err)
			}
			return
		}

		switch msg.Type {
		case "register":
			// Agent reports its capabilities (allow_write)
			agent.LastPing = time.Now()
			log.Printf("[AgentHub] Agent %d registered with allow_write=%v", agent.ClusterID, msg.AllowWrite)
			h.updateAgentStatus(agent.ClusterID, "online", msg.AllowWrite)

		case "ping":
			agent.LastPing = time.Now()
			// Update last_ping_at in database so heartbeat timeout check stays accurate
			h.updateLastPing(agent.ClusterID)
			agent.mu.Lock()
			agent.Conn.WriteJSON(AgentMessage{Type: "pong"})
			agent.mu.Unlock()

		case "result", "error":
			// Dispatch response to waiting SendCommand call
			if msg.ID != "" {
				agent.pendingMu.Lock()
				if ch, ok := agent.pending[msg.ID]; ok {
					ch <- &msg
				}
				agent.pendingMu.Unlock()
			}
		}
	}
}

// healthChecker periodically checks agent health and marks stale ones as offline.
func (h *Hub) healthChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.mu.RLock()
		var stale []uint
		for id, agent := range h.agents {
			if time.Since(agent.LastPing) > 90*time.Second {
				stale = append(stale, id)
			}
		}
		h.mu.RUnlock()

		for _, id := range stale {
			log.Printf("[AgentHub] Agent %d timed out, disconnecting", id)
			h.UnregisterAgent(id)
		}
	}
}

// updateAgentStatus updates the agent_status and last_ping_at in database.
func (h *Hub) updateAgentStatus(clusterID uint, status string, allowWrite bool) {
	if database.DB == nil {
		return
	}
	updates := map[string]interface{}{
		"agent_status": status,
	}
	if status == "online" {
		now := time.Now()
		updates["last_ping_at"] = &now
		updates["allow_write"] = allowWrite
		updates["status"] = "connected"
	} else {
		updates["status"] = "disconnected"
	}
	database.DB.Model(&model.ClusterConfig{}).Where("id = ?", clusterID).Updates(updates)
}

// updateLastPing updates only the last_ping_at timestamp in database.
func (h *Hub) updateLastPing(clusterID uint) {
	if database.DB == nil {
		return
	}
	now := time.Now()
	database.DB.Model(&model.ClusterConfig{}).Where("id = ?", clusterID).
		Updates(map[string]interface{}{
			"last_ping_at":  &now,
			"agent_status":  "online",
		})
}

// GetOnlineAgentIDs returns cluster IDs of all online agents.
func (h *Hub) GetOnlineAgentIDs() []uint {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var ids []uint
	for id := range h.agents {
		ids = append(ids, id)
	}
	return ids
}
