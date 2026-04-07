package cluster

import "context"

// KubeExecutor abstracts kubectl command execution.
// Both direct (local kubectl) and agent (remote WebSocket) modes implement this.
type KubeExecutor interface {
	// ExecKubectl executes a kubectl command with the given args and returns output.
	// Args should NOT include --kubeconfig or --context — the executor handles that.
	ExecKubectl(ctx context.Context, args []string) ([]byte, error)
}
