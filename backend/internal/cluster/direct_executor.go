package cluster

import (
	"context"
	"fmt"
	"os/exec"
)

// DirectExecutor executes kubectl commands locally using kubeconfig file.
type DirectExecutor struct {
	kubeconfigPath string
	kubeContext    string
	cleanup       func()
}

// NewDirectExecutor creates a DirectExecutor from kubeconfig content.
// Caller must call Close() when done to clean up the temp kubeconfig file.
func NewDirectExecutor(kubeconfigContent, kubeContext string) (*DirectExecutor, error) {
	kubeconfigPath, cleanup, err := WriteKubeconfig(kubeconfigContent)
	if err != nil {
		return nil, err
	}
	return &DirectExecutor{
		kubeconfigPath: kubeconfigPath,
		kubeContext:    kubeContext,
		cleanup:       cleanup,
	}, nil
}

// ExecKubectl executes kubectl locally with kubeconfig and context injected.
func (d *DirectExecutor) ExecKubectl(ctx context.Context, args []string) ([]byte, error) {
	fullArgs := append(args, "--kubeconfig", d.kubeconfigPath)
	if d.kubeContext != "" {
		fullArgs = append(fullArgs, "--context", d.kubeContext)
	}
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("kubectl %v failed: %s", args, string(output))
	}
	return output, nil
}

// Close cleans up the temporary kubeconfig file.
func (d *DirectExecutor) Close() {
	if d.cleanup != nil {
		d.cleanup()
	}
}
