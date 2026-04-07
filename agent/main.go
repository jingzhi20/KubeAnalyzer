package main

import (
	"context"
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

// AgentConfig holds the agent configuration from environment variables.
type AgentConfig struct {
	ServerURL  string
	Token      string
	AllowWrite bool
}

// AgentMessage defines the WebSocket communication protocol between Agent and Server.
type AgentMessage struct {
	ID            string   `json:"id,omitempty"`
	Type          string   `json:"type"`                        // kubectl / analyze / result / error / ping / pong / register
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

func getConfig() *AgentConfig {
	serverURL := os.Getenv("AIOPS_SERVER_URL")
	if serverURL == "" {
		log.Fatal("[AIOps Agent] AIOPS_SERVER_URL is required")
	}
	token := os.Getenv("AIOPS_AGENT_TOKEN")
	if token == "" {
		log.Fatal("[AIOps Agent] AIOPS_AGENT_TOKEN is required")
	}
	return &AgentConfig{
		ServerURL:  serverURL,
		Token:      token,
		AllowWrite: os.Getenv("AIOPS_ALLOW_WRITE") == "true",
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("[AIOps Agent] Starting (client-go native)...")

	// Initialize Prometheus metrics
	InitMetrics()

	config := getConfig()

	// Initialize Kubernetes client via client-go
	log.Println("[AIOps Agent] Initializing Kubernetes client...")
	k8sClient, err := NewK8sClient()
	if err != nil {
		log.Printf("[AIOps Agent] WARNING: Failed to initialize K8s client: %v", err)
		log.Println("[AIOps Agent] Agent will run in kubectl-only mode (no native analysis)")
	} else {
		if err := k8sClient.TestConnection(); err != nil {
			log.Printf("[AIOps Agent] WARNING: K8s cluster not reachable: %v", err)
			log.Println("[AIOps Agent] Agent will run in kubectl-only mode")
			k8sClient = nil
		} else {
			log.Println("[AIOps Agent] Kubernetes client connected successfully!")
		}
	}

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
		err := connectAndServe(config, k8sClient)
		if err != nil {
			log.Printf("[AIOps Agent] Connection error: %v", err)
		}
		log.Println("[AIOps Agent] Reconnecting in 5 seconds...")
		time.Sleep(5 * time.Second)
	}
}

func connectAndServe(config *AgentConfig, k8sClient *K8sClient) error {
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

	// Send registration message with capabilities
	regMsg := AgentMessage{
		Type:       "register",
		AllowWrite: config.AllowWrite,
	}
	if err := conn.WriteJSON(regMsg); err != nil {
		return fmt.Errorf("failed to send registration: %w", err)
	}
	log.Printf("[AIOps Agent] Registered with allow_write=%v", config.AllowWrite)

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
			go handleKubectlCommand(conn, msg, config.AllowWrite)
		case "analyze":
			go handleAnalyzeCommand(conn, msg, k8sClient)
		case "pong":
			// Server acknowledged our ping
		default:
			log.Printf("[AIOps Agent] Unknown message type: %s", msg.Type)
		}
	}
}

// handleKubectlCommand executes a kubectl command and sends the result back.
func handleKubectlCommand(conn *websocket.Conn, msg AgentMessage, allowWrite bool) {
	log.Printf("[AIOps Agent] Executing: kubectl %s", strings.Join(msg.Args, " "))

	// Security: validate that the command is kubectl-safe
	verb := ""
	if len(msg.Args) > 0 {
		verb = strings.ToLower(msg.Args[0])
	}

	if isHighRiskCommand(verb, msg.Args) {
		errMsg := fmt.Sprintf("\u9ad8\u5371\u64cd\u4f5c\u5df2\u62e6\u622a: kubectl %s \u2014 \u6b64\u64cd\u4f5c\u5c5e\u4e8e\u4e0d\u53ef\u9006\u7834\u574f\u6027\u64cd\u4f5c\uff0c\u59cb\u7ec8\u88ab\u7981\u6b62", strings.Join(msg.Args, " "))
		resp := AgentMessage{
			ID:     msg.ID,
			Type:   "error",
			Error:  errMsg,
			Output: errMsg,
		}
		conn.WriteJSON(resp)
		return
	}

	if !isAllowedCommand(msg.Args, allowWrite) {
		errMsg := fmt.Sprintf("\u5199\u64cd\u4f5c\u672a\u6388\u6743: kubectl %s \u2014 Agent \u5f53\u524d\u4e3a\u53ea\u8bfb\u6a21\u5f0f\uff0c\u9700\u8bbe\u7f6e AIOPS_ALLOW_WRITE=true \u5e76\u91cd\u542f Agent", strings.Join(msg.Args, " "))
		resp := AgentMessage{
			ID:     msg.ID,
			Type:   "error",
			Error:  errMsg,
			Output: errMsg,
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

	log.Printf("[AIOps Agent] Command completed, output size: %d bytes", len(output))
}

// handleAnalyzeCommand runs the native analysis engine and returns structured results.
func handleAnalyzeCommand(conn *websocket.Conn, msg AgentMessage, k8sClient *K8sClient) {
	log.Printf("[AIOps Agent] Analyze request: filters=%v namespace=%s", msg.Filters, msg.Namespace)

	if k8sClient == nil {
		resp := AgentMessage{
			ID:    msg.ID,
			Type:  "error",
			Error: "Kubernetes client not available on this agent, cannot perform native analysis",
		}
		conn.WriteJSON(resp)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Run native analysis
	analyzeResp := RunAnalysis(ctx, k8sClient, msg.Filters, msg.Namespace, msg.LabelSelector)

	// Serialize to JSON
	jsonData, err := json.Marshal(analyzeResp)
	if err != nil {
		resp := AgentMessage{
			ID:    msg.ID,
			Type:  "error",
			Error: fmt.Sprintf("failed to serialize analysis results: %v", err),
		}
		conn.WriteJSON(resp)
		return
	}

	resp := AgentMessage{
		ID:     msg.ID,
		Type:   "result",
		Output: string(jsonData),
	}

	if writeErr := conn.WriteJSON(resp); writeErr != nil {
		log.Printf("[AIOps Agent] Failed to send analyze response: %v", writeErr)
	}

	log.Printf("[AIOps Agent] Analysis completed: %d problems found", analyzeResp.Problems)
}

// isAllowedCommand checks if a kubectl command is safe to execute.
// Three-tier security policy:
//   - Read-only commands: always allowed
//   - Write commands (low-medium risk): controlled by allowWrite switch
//   - High-risk destructive commands: ALWAYS blocked, no override
func isAllowedCommand(args []string, allowWrite bool) bool {
	if len(args) == 0 {
		return false
	}

	verb := strings.ToLower(args[0])

	// ===== Tier 0: ALWAYS BLOCKED — catastrophic / irreversible operations =====
	// These are NEVER allowed regardless of allowWrite setting.
	if isHighRiskCommand(verb, args) {
		log.Printf("[AIOps Agent] BLOCKED high-risk command: kubectl %s", strings.Join(args, " "))
		return false
	}

	// ===== Tier 1: Read-only commands — always allowed =====
	readOnlyVerbs := map[string]bool{
		"get":           true,
		"describe":      true,
		"logs":          true,
		"top":           true,
		"cluster-info":  true,
		"version":       true,
		"api-resources": true,
		"api-versions":  true,
		"explain":       true,
		"auth":          true,
		"diff":          true,
		"wait":          true,
	}
	if readOnlyVerbs[verb] {
		return true
	}

	// ===== Tier 2: Write commands (low-medium risk) — controlled by allowWrite =====
	if allowWrite {
		writeVerbs := map[string]bool{
			"apply":    true,
			"create":   true,
			"delete":   true, // delete pod/deployment etc. (high-risk targets filtered in Tier 0)
			"patch":    true,
			"scale":    true,
			"label":    true,
			"annotate": true,
			"rollout":  true,
			"set":      true,
			"replace":  true,
			"cp":       true,
		}
		if writeVerbs[verb] {
			return true
		}
	}

	return false
}

// isHighRiskCommand detects destructive operations that should NEVER be allowed.
// Returns true if the command is considered high-risk / catastrophic.
func isHighRiskCommand(verb string, args []string) bool {
	// Join all args for pattern matching
	fullCmd := strings.ToLower(strings.Join(args, " "))

	// ----- Always blocked verbs (no legitimate remote use) -----
	blockedVerbs := map[string]bool{
		"exec":   true, // interactive shell — security risk
		"attach": true, // attach to running container
		"proxy":  true, // opens local proxy to API server
		"port-forward": true, // port-forwarding (no use via agent)
		"edit":   true, // interactive editing (no TTY via agent)
		"run":    true, // creates arbitrary pods
		"debug":  true, // ephemeral debug containers
	}
	if blockedVerbs[verb] {
		return true
	}

	// ----- delete + high-risk resource types -----
	if verb == "delete" {
		// Resources whose deletion is catastrophic
		highRiskResources := []string{
			"namespace", "namespaces", "ns",
			"node", "nodes", "no",
			"clusterrole", "clusterroles",
			"clusterrolebinding", "clusterrolebindings",
			"persistentvolume", "persistentvolumes", "pv",
			"storageclass", "storageclasses", "sc",
			"customresourcedefinition", "customresourcedefinitions", "crd", "crds",
			"apiservice", "apiservices",
			"mutatingwebhookconfiguration", "mutatingwebhookconfigurations",
			"validatingwebhookconfiguration", "validatingwebhookconfigurations",
			"priorityclass", "priorityclasses", "pc",
			"runtimeclass", "runtimeclasses",
			"serviceaccount", "serviceaccounts", "sa",
		}
		for _, res := range highRiskResources {
			if containsArg(args[1:], res) {
				return true
			}
		}

		// delete --all or delete --all-namespaces (mass deletion)
		if strings.Contains(fullCmd, "--all") {
			return true
		}
	}

	// ----- drain / cordon / uncordon / taint on nodes — always high risk -----
	if verb == "drain" || verb == "cordon" || verb == "uncordon" || verb == "taint" {
		return true
	}

	// ----- apply/create + cluster-scoped dangerous resources -----
	if verb == "apply" || verb == "create" || verb == "replace" {
		clusterScopedDangerous := []string{
			"clusterrole", "clusterrolebinding",
			"customresourcedefinition", "crd",
			"namespace",
			"priorityclass",
		}
		for _, res := range clusterScopedDangerous {
			if containsArg(args[1:], res) {
				return true
			}
		}
	}

	// ----- patch on critical resources -----
	if verb == "patch" {
		criticalPatchTargets := []string{
			"node", "nodes",
			"namespace", "namespaces",
			"clusterrole", "clusterrolebinding",
		}
		for _, res := range criticalPatchTargets {
			if containsArg(args[1:], res) {
				return true
			}
		}
	}

	return false
}

// containsArg checks if any argument matches the target (case-insensitive).
func containsArg(args []string, target string) bool {
	for _, a := range args {
		if strings.ToLower(a) == target {
			return true
		}
	}
	return false
}
