package main

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodAnalyzer checks for Pod issues (Pending, CrashLoopBackOff, ImagePullBackOff, etc.).
// Ported from k8sgpt pkg/analyzer/pod.go
type PodAnalyzer struct{}

func (p *PodAnalyzer) Name() string { return "Pod" }

// containerWaitingErrorReasons are error states for waiting containers.
var containerWaitingErrorReasons = map[string]bool{
	"CrashLoopBackOff":           true,
	"ImagePullBackOff":           true,
	"CreateContainerConfigError": true,
	"PreCreateHookError":         true,
	"CreateContainerError":       true,
	"PreStartHookError":          true,
	"RunContainerError":          true,
	"ImageInspectError":          true,
	"ErrImagePull":               true,
	"ErrImageNeverPull":          true,
	"InvalidImageName":           true,
}

// evtErrorReasons are event reasons that indicate container creation issues.
var evtErrorReasons = map[string]bool{
	"FailedCreatePodSandBox": true,
	"FailedMount":            true,
}

func (p *PodAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, pod := range pods.Items {
		var failures []string

		// Check for pending pods
		if pod.Status.Phase == v1.PodPending {
			for _, cond := range pod.Status.Conditions {
				if cond.Type == v1.PodScheduled && cond.Reason == "Unschedulable" && cond.Message != "" {
					failures = append(failures, cond.Message)
				}
			}
		}

		// Check init container statuses
		failures = append(failures, analyzeContainerStatuses(ctx, client, pod.Status.InitContainerStatuses, pod.Name, pod.Namespace, string(pod.Status.Phase))...)

		// Check container statuses
		failures = append(failures, analyzeContainerStatuses(ctx, client, pod.Status.ContainerStatuses, pod.Name, pod.Namespace, string(pod.Status.Phase))...)

		if len(failures) > 0 {
			result := AnalyzeResult{
				Kind:  "Pod",
				Name:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Error: failures,
			}
			parent, found := client.GetParent(pod.ObjectMeta)
			if found {
				result.ParentObj = parent
			}
			results = append(results, result)
		}
	}

	return results, nil
}

// analyzeContainerStatuses checks container statuses for error conditions.
func analyzeContainerStatuses(ctx context.Context, client *K8sClient, statuses []v1.ContainerStatus, podName, namespace, phase string) []string {
	var failures []string

	for _, cs := range statuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			if reason == "ContainerCreating" && phase == "Pending" {
				// Check events for more details
				evtReason, evtMsg, err := client.FetchLatestEvent(ctx, namespace, podName)
				if err == nil && evtErrorReasons[evtReason] && evtMsg != "" {
					failures = append(failures, evtMsg)
				}
			} else if reason == "CrashLoopBackOff" && cs.LastTerminationState.Terminated != nil {
				failures = append(failures, fmt.Sprintf("the last termination reason is %s container=%s pod=%s",
					cs.LastTerminationState.Terminated.Reason, cs.Name, podName))
			} else if containerWaitingErrorReasons[reason] && cs.State.Waiting.Message != "" {
				failures = append(failures, cs.State.Waiting.Message)
			}
		} else if cs.State.Terminated != nil {
			if cs.State.Terminated.ExitCode != 0 {
				reason := cs.State.Terminated.Reason
				if reason == "" {
					reason = "Unknown"
				}
				failures = append(failures, fmt.Sprintf("the termination reason is %s exitCode=%d container=%s pod=%s",
					reason, cs.State.Terminated.ExitCode, cs.Name, podName))
			}
		} else {
			// Running but ReadinessProbe fails
			if !cs.Ready && phase == "Running" {
				evtReason, evtMsg, err := client.FetchLatestEvent(ctx, namespace, podName)
				if err == nil && evtReason == "Unhealthy" && evtMsg != "" {
					failures = append(failures, evtMsg)
				}
			}
		}
	}

	return failures
}
