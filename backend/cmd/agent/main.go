package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

// AgentMessage matches the server's message format.
type AgentMessage struct {
	ID        string   `json:"id,omitempty"`
	Type      string   `json:"type"`
	Args      []string `json:"args,omitempty"`
	Output    string   `json:"output,omitempty"`
	Error     string   `json:"error,omitempty"`
	ClusterID uint     `json:"cluster_id,omitempty"`
}

// Agent configuration from environment variables.
type AgentConfig struct {
	ServerURL string // WebSocket URL: wss://aiops.example.com/api/agent/ws
	Token     string // Authentication token
}

func getConfig() *AgentConfig {
	serverURL := os.Getenv("AIOPS_SERVER_URL")
	token := os.Getenv("AIOPS_AGENT_TOKEN")

	if serverURL == "" {
		log.Fatal("AIOPS_SERVER_URL is required (e.g., wss://aiops.example.com/api/agent/ws)")
	}
	if token == "" {
		log.Fatal("AIOPS_AGENT_TOKEN is required")
	}

	return &AgentConfig{
		ServerURL: serverURL,
		Token:     token,
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[AIOps Agent] Starting...")

	config := getConfig()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("[AIOps Agent] Shutting down...")
		os.Exit(0)
	}()

	// Main reconnection loop
	for {
		err := connectAndServe(config)
		if err != nil {
			log.Printf("[AIOps Agent] Connection error: %v", err)
		}
		log.Println("[AIOps Agent] Reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func connectAndServe(config *AgentConfig) error {
	// Build WebSocket URL with token
	u, err := url.Parse(config.ServerURL)
	if err != nil {
		return fmt.Errorf("invalid server URL: %w", err)
	}
	q := u.Query()
	q.Set("token", config.Token)
	u.RawQuery = q.Encode()

	log.Printf("[AIOps Agent] Connecting to %s ...", config.ServerURL)

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	log.Println("[AIOps Agent] Connected successfully!")

	// Start ping ticker
	pingTicker := time.NewTicker(25 * time.Second)
	defer pingTicker.Stop()

	// Ping goroutine
	go func() {
		for range pingTicker.C {
			msg := AgentMessage{Type: "ping"}
			if err := conn.WriteJSON(msg); err != nil {
				log.Printf("[AIOps Agent] Ping failed: %v", err)
				conn.Close()
				return
			}
		}
	}()

	// Main message loop
	for {
		var msg AgentMessage
		if err := conn.ReadJSON(&msg); err != nil {
			return fmt.Errorf("read error: %w", err)
		}

		switch msg.Type {
		case "kubectl":
			go handleKubectlCommand(conn, msg)
		case "pong":
			// Server acknowledged our ping
		default:
			log.Printf("[AIOps Agent] Unknown message type: %s", msg.Type)
		}
	}
}

// handleKubectlCommand executes a kubectl command and sends the result back.
func handleKubectlCommand(conn *websocket.Conn, msg AgentMessage) {
	log.Printf("[AIOps Agent] Executing: kubectl %s", strings.Join(msg.Args, " "))

	// Security: validate that the command is kubectl-safe
	if !isAllowedCommand(msg.Args) {
		resp := AgentMessage{
			ID:    msg.ID,
			Type:  "error",
			Error: "command not allowed by agent policy",
		}
		conn.WriteJSON(resp)
		return
	}

	// Execute kubectl command
	cmd := exec.Command("kubectl", msg.Args...)
	output, err := cmd.CombinedOutput()

	resp := AgentMessage{
		ID:     msg.ID,
		Type:   "result",
		Output: string(output),
	}

	if err != nil {
		resp.Type = "error"
		resp.Error = err.Error()
		resp.Output = string(output)
	}

	if writeErr := conn.WriteJSON(resp); writeErr != nil {
		log.Printf("[AIOps Agent] Failed to send response: %v", writeErr)
	}

	// Log result size
	log.Printf("[AIOps Agent] Command completed, output size: %d bytes", len(output))
}

// isAllowedCommand checks if a kubectl command is safe to execute.
// By default, only read-only commands are allowed.
func isAllowedCommand(args []string) bool {
	if len(args) == 0 {
		return false
	}

	// Allowed read-only subcommands
	allowedVerbs := map[string]bool{
		"get":          true,
		"describe":     true,
		"logs":         true,
		"top":          true,
		"cluster-info": true,
		"version":      true,
		"api-resources": true,
		"api-versions":  true,
		"explain":      true,
		"auth":         true,
	}

	verb := args[0]
	if allowedVerbs[verb] {
		return true
	}

	// Check AIOPS_ALLOW_WRITE env for write commands
	if os.Getenv("AIOPS_ALLOW_WRITE") == "true" {
		writeVerbs := map[string]bool{
			"apply":   true,
			"create":  true,
			"delete":  true,
			"patch":   true,
			"scale":   true,
			"edit":    true,
			"label":   true,
			"annotate": true,
			"rollout": true,
			"cordon":  true,
			"uncordon": true,
			"drain":   true,
			"taint":   true,
		}
		if writeVerbs[verb] {
			return true
		}
	}

	return false
}

// Ensure json import is used
var _ = json.Marshal
