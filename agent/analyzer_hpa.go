package main

import (
	"context"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HPAAnalyzer checks for HorizontalPodAutoscaler issues including
// condition checks, ScaleTargetRef validation, and resource requirements.
type HPAAnalyzer struct{}

func (h *HPAAnalyzer) Name() string { return "HorizontalPodAutoscaler" }

func (h *HPAAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	hpaList, err := client.Clientset.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, hpa := range hpaList.Items {
		var failures []string

		// 1. Check HPA conditions
		for _, cond := range hpa.Status.Conditions {
			switch cond.Type {
			case autoscalingv2.ScalingActive:
				if cond.Status == "False" {
					failures = append(failures, fmt.Sprintf(
						"HPA %s/%s is not active: %s - %s",
						hpa.Namespace, hpa.Name, cond.Reason, cond.Message))
				}
			case autoscalingv2.AbleToScale:
				if cond.Status == "False" {
					failures = append(failures, fmt.Sprintf(
						"HPA %s/%s unable to scale: %s - %s",
						hpa.Namespace, hpa.Name, cond.Reason, cond.Message))
				}
			case autoscalingv2.ScalingLimited:
				if cond.Status == "True" && cond.Reason != "TooFewReplicas" {
					failures = append(failures, fmt.Sprintf(
						"HPA %s/%s scaling limited: %s - %s",
						hpa.Namespace, hpa.Name, cond.Reason, cond.Message))
				}
			}
		}

		// 2. Validate ScaleTargetRef exists
		ref := hpa.Spec.ScaleTargetRef
		switch ref.Kind {
		case "Deployment":
			_, err := client.Clientset.AppsV1().Deployments(hpa.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"HPA %s/%s references non-existent Deployment %q",
					hpa.Namespace, hpa.Name, ref.Name))
			}
		case "StatefulSet":
			_, err := client.Clientset.AppsV1().StatefulSets(hpa.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"HPA %s/%s references non-existent StatefulSet %q",
					hpa.Namespace, hpa.Name, ref.Name))
			}
		case "ReplicaSet":
			_, err := client.Clientset.AppsV1().ReplicaSets(hpa.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"HPA %s/%s references non-existent ReplicaSet %q",
					hpa.Namespace, hpa.Name, ref.Name))
			}
		}

		// 3. Check resource metrics - verify target has resource requests set
		for _, metric := range hpa.Spec.Metrics {
			if metric.Type == autoscalingv2.ResourceMetricSourceType && metric.Resource != nil {
				// Verify the target workload has resource requests
				if ref.Kind == "Deployment" {
					deploy, err := client.Clientset.AppsV1().Deployments(hpa.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
					if err == nil {
						hasRequests := false
						for _, container := range deploy.Spec.Template.Spec.Containers {
							if _, ok := container.Resources.Requests[metric.Resource.Name]; ok {
								hasRequests = true
								break
							}
						}
						if !hasRequests {
							failures = append(failures, fmt.Sprintf(
								"HPA %s/%s targets Deployment %q metric %q but no container has resource requests set for it",
								hpa.Namespace, hpa.Name, ref.Name, metric.Resource.Name))
						}
					}
				}
			}
		}

		// 4. Check for min > max misconfiguration
		if hpa.Spec.MinReplicas != nil && *hpa.Spec.MinReplicas > hpa.Spec.MaxReplicas {
			failures = append(failures, fmt.Sprintf(
				"HPA %s/%s has minReplicas (%d) > maxReplicas (%d)",
				hpa.Namespace, hpa.Name, *hpa.Spec.MinReplicas, hpa.Spec.MaxReplicas))
		}

		// 5. Check current vs desired mismatch
		if hpa.Status.CurrentReplicas != hpa.Status.DesiredReplicas {
			failures = append(failures, fmt.Sprintf(
				"HPA %s/%s has %d current replicas but desires %d",
				hpa.Namespace, hpa.Name, hpa.Status.CurrentReplicas, hpa.Status.DesiredReplicas))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "HorizontalPodAutoscaler",
				Name:  fmt.Sprintf("%s/%s", hpa.Namespace, hpa.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
