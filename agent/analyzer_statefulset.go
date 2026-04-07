package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSetAnalyzer checks for StatefulSet issues including replica mismatches,
// headless Service validation, StorageClass verification, and per-Pod status.
type StatefulSetAnalyzer struct{}

func (s *StatefulSetAnalyzer) Name() string { return "StatefulSet" }

func (s *StatefulSetAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	stsList, err := client.Clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, sts := range stsList.Items {
		var failures []string

		// 1. Replica mismatch check
		if sts.Spec.Replicas != nil {
			desired := *sts.Spec.Replicas
			ready := sts.Status.ReadyReplicas
			if desired != ready {
				failures = append(failures, fmt.Sprintf(
					"StatefulSet %s/%s has %d desired replicas but only %d are ready (current=%d, updated=%d)",
					sts.Namespace, sts.Name, desired, ready,
					sts.Status.CurrentReplicas, sts.Status.UpdatedReplicas))
			}
		}

		// 2. Headless Service validation
		if sts.Spec.ServiceName != "" {
			svc, err := client.Clientset.CoreV1().Services(sts.Namespace).Get(ctx, sts.Spec.ServiceName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"StatefulSet %s/%s references non-existent headless Service %q",
					sts.Namespace, sts.Name, sts.Spec.ServiceName))
			} else if svc.Spec.ClusterIP != "None" {
				failures = append(failures, fmt.Sprintf(
					"StatefulSet %s/%s references Service %q which is not headless (clusterIP=%s)",
					sts.Namespace, sts.Name, sts.Spec.ServiceName, svc.Spec.ClusterIP))
			}
		}

		// 3. StorageClass validation for VolumeClaimTemplates
		for _, vct := range sts.Spec.VolumeClaimTemplates {
			if vct.Spec.StorageClassName != nil && *vct.Spec.StorageClassName != "" {
				scName := *vct.Spec.StorageClassName
				_, err := client.Clientset.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"StatefulSet %s/%s VolumeClaimTemplate %q references non-existent StorageClass %q",
						sts.Namespace, sts.Name, vct.Name, scName))
				}
			}
		}

		// 4. Per-Pod status check via label selector
		if sts.Spec.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(sts.Spec.Selector)
			if err == nil {
				pods, err := client.Clientset.CoreV1().Pods(sts.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil {
					for _, pod := range pods.Items {
						for _, cs := range pod.Status.ContainerStatuses {
							if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
								failures = append(failures, fmt.Sprintf(
									"Pod %s in StatefulSet %s/%s container %q is waiting: %s",
									pod.Name, sts.Namespace, sts.Name, cs.Name, cs.State.Waiting.Reason))
							}
							if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
								failures = append(failures, fmt.Sprintf(
									"Pod %s in StatefulSet %s/%s container %q terminated with exit code %d: %s",
									pod.Name, sts.Namespace, sts.Name, cs.Name,
									cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
							}
						}
					}
				}
			}
		}

		// 5. Event check
		if len(failures) > 0 {
			evtReason, evtMsg, err := client.FetchLatestEvent(ctx, sts.Namespace, sts.Name)
			if err == nil && evtReason != "" {
				failures = append(failures, fmt.Sprintf("Event: [%s] %s", evtReason, evtMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "StatefulSet",
				Name:  fmt.Sprintf("%s/%s", sts.Namespace, sts.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// ReplicaSetAnalyzer checks for standalone ReplicaSet issues (not owned by Deployment).
type ReplicaSetAnalyzer struct{}

func (r *ReplicaSetAnalyzer) Name() string { return "ReplicaSet" }

func (r *ReplicaSetAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	rsList, err := client.Clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, rs := range rsList.Items {
		// Skip ReplicaSets owned by Deployments (they are handled by DeploymentAnalyzer)
		if len(rs.OwnerReferences) > 0 {
			continue
		}

		var failures []string

		if rs.Spec.Replicas == nil {
			continue
		}

		desired := *rs.Spec.Replicas
		ready := rs.Status.ReadyReplicas

		if desired != ready {
			failures = append(failures, fmt.Sprintf(
				"ReplicaSet %s/%s has %d replicas but %d are ready",
				rs.Namespace, rs.Name, desired, ready))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ReplicaSet",
				Name:  fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// DaemonSetAnalyzer checks for DaemonSet scheduling issues.
type DaemonSetAnalyzer struct{}

func (d *DaemonSetAnalyzer) Name() string { return "DaemonSet" }

func (d *DaemonSetAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	dsList, err := client.Clientset.AppsV1().DaemonSets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, ds := range dsList.Items {
		var failures []string

		desired := ds.Status.DesiredNumberScheduled
		ready := ds.Status.NumberReady
		unavailable := ds.Status.NumberUnavailable

		if desired != ready {
			failures = append(failures, fmt.Sprintf(
				"DaemonSet %s/%s has %d desired but %d ready (%d unavailable)",
				ds.Namespace, ds.Name, desired, ready, unavailable))
		}

		if ds.Status.NumberMisscheduled > 0 {
			failures = append(failures, fmt.Sprintf(
				"DaemonSet %s/%s has %d misscheduled pods",
				ds.Namespace, ds.Name, ds.Status.NumberMisscheduled))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "DaemonSet",
				Name:  fmt.Sprintf("%s/%s", ds.Namespace, ds.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
