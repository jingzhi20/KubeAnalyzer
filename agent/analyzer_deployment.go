package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeploymentAnalyzer checks for Deployment replica mismatches.
// Ported from k8sgpt pkg/analyzer/deployment.go
type DeploymentAnalyzer struct{}

func (d *DeploymentAnalyzer) Name() string { return "Deployment" }

func (d *DeploymentAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	deployments, err := client.Clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, dep := range deployments.Items {
		var failures []string

		if dep.Spec.Replicas == nil {
			continue
		}

		desired := *dep.Spec.Replicas
		ready := dep.Status.ReadyReplicas
		current := dep.Status.Replicas

		if desired != ready {
			if current > desired {
				failures = append(failures, fmt.Sprintf(
					"Deployment %s/%s has %d replicas in spec but %d replicas in status because status field is not updated yet after scaling and %d replicas are available with status running",
					dep.Namespace, dep.Name, desired, current, ready))
			} else {
				failures = append(failures, fmt.Sprintf(
					"Deployment %s/%s has %d replicas but %d are available with status running",
					dep.Namespace, dep.Name, desired, ready))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Deployment",
				Name:  fmt.Sprintf("%s/%s", dep.Namespace, dep.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
