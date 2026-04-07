package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogAnalyzer checks Pod container logs for error patterns.
// Only analyzes pods in non-Running/non-Succeeded states or pods with restart counts.
type LogAnalyzer struct{}

func (l *LogAnalyzer) Name() string { return "Log" }

func (l *LogAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, pod := range pods.Items {
		// Only check pods that might have issues
		if !shouldCheckLogs(pod) {
			continue
		}

		var failures []string

		for _, container := range pod.Spec.Containers {
			// Get last 50 lines of logs
			tailLines := int64(50)
			logOpts := &corev1.PodLogOptions{
				Container: container.Name,
				TailLines: &tailLines,
			}

			req := client.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, logOpts)
			stream, err := req.Stream(ctx)
			if err != nil {
				continue // Pod may not have logs yet
			}

			errors := scanLogsForErrors(stream, 5) // max 5 error lines
			stream.Close()

			for _, errLine := range errors {
				failures = append(failures, fmt.Sprintf(
					"Pod %s/%s container %q log: %s",
					pod.Namespace, pod.Name, container.Name, errLine))
			}

			// Also check previous container logs if there were restarts
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Name == container.Name && cs.RestartCount > 0 {
					prevLogOpts := &corev1.PodLogOptions{
						Container: container.Name,
						TailLines: &tailLines,
						Previous:  true,
					}
					prevReq := client.Clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, prevLogOpts)
					prevStream, err := prevReq.Stream(ctx)
					if err != nil {
						continue
					}
					prevErrors := scanLogsForErrors(prevStream, 3)
					prevStream.Close()

					for _, errLine := range prevErrors {
						failures = append(failures, fmt.Sprintf(
							"Pod %s/%s container %q previous log: %s",
							pod.Namespace, pod.Name, container.Name, errLine))
					}
				}
			}
		}

		if len(failures) > 0 {
			parent, hasParent := client.GetParent(pod.ObjectMeta)
			result := AnalyzeResult{
				Kind:  "Log",
				Name:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Error: failures,
			}
			if hasParent {
				result.ParentObj = parent
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// shouldCheckLogs returns true if the pod is likely to have meaningful error logs.
func shouldCheckLogs(pod corev1.Pod) bool {
	// Check non-running pods
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodPending {
		return true
	}

	// Check pods with restarts
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount > 0 {
			return true
		}
		if cs.State.Waiting != nil {
			return true
		}
	}

	return false
}

// scanLogsForErrors scans log lines for common error patterns.
func scanLogsForErrors(reader io.Reader, maxErrors int) []string {
	var errors []string
	scanner := bufio.NewScanner(reader)

	errorPatterns := []string{
		"error", "Error", "ERROR",
		"fatal", "Fatal", "FATAL",
		"panic", "PANIC",
		"exception", "Exception", "EXCEPTION",
		"OOMKilled",
		"failed", "Failed", "FAILED",
		"cannot", "Cannot",
		"refused", "timeout", "Timeout",
	}

	for scanner.Scan() {
		if len(errors) >= maxErrors {
			break
		}
		line := scanner.Text()
		for _, pattern := range errorPatterns {
			if strings.Contains(line, pattern) {
				// Truncate long lines
				if len(line) > 200 {
					line = line[:200] + "..."
				}
				errors = append(errors, line)
				break
			}
		}
	}

	return errors
}
