package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PDBAnalyzer checks for PodDisruptionBudget issues including
// DisruptionsAllowed condition and selector validation.
type PDBAnalyzer struct{}

func (p *PDBAnalyzer) Name() string { return "PodDisruptionBudget" }

func (p *PDBAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	pdbList, err := client.Clientset.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, pdb := range pdbList.Items {
		var failures []string

		// 1. Check DisruptionsAllowed
		if pdb.Status.DisruptionsAllowed == 0 && pdb.Status.CurrentHealthy > 0 {
			failures = append(failures, fmt.Sprintf(
				"PDB %s/%s allows 0 disruptions (currentHealthy=%d, desiredHealthy=%d, expectedPods=%d)",
				pdb.Namespace, pdb.Name,
				pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods))
		}

		// 2. Check if PDB selector matches any pods
		if pdb.Spec.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err == nil {
				pods, err := client.Clientset.CoreV1().Pods(pdb.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil && len(pods.Items) == 0 {
					failures = append(failures, fmt.Sprintf(
						"PDB %s/%s selector matches no existing pods",
						pdb.Namespace, pdb.Name))
				}
			}
		}

		// 3. Check for unhealthy pods
		if pdb.Status.CurrentHealthy < pdb.Status.ExpectedPods {
			failures = append(failures, fmt.Sprintf(
				"PDB %s/%s has %d healthy pods but expects %d",
				pdb.Namespace, pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.ExpectedPods))
		}

		// 4. Check conditions
		for _, cond := range pdb.Status.Conditions {
			if cond.Status == "False" && cond.Type == "SufficientPods" {
				failures = append(failures, fmt.Sprintf(
					"PDB %s/%s condition SufficientPods is False: %s",
					pdb.Namespace, pdb.Name, cond.Message))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "PodDisruptionBudget",
				Name:  fmt.Sprintf("%s/%s", pdb.Namespace, pdb.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
