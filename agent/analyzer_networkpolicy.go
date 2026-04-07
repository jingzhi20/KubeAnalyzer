package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkPolicyAnalyzer checks for NetworkPolicy issues including
// empty podSelector detection and unmatched Pod detection.
type NetworkPolicyAnalyzer struct{}

func (n *NetworkPolicyAnalyzer) Name() string { return "NetworkPolicy" }

func (n *NetworkPolicyAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	npList, err := client.Clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, np := range npList.Items {
		var failures []string

		// 1. Check for empty podSelector (selects ALL pods in namespace)
		if len(np.Spec.PodSelector.MatchLabels) == 0 && len(np.Spec.PodSelector.MatchExpressions) == 0 {
			failures = append(failures, fmt.Sprintf(
				"NetworkPolicy %s/%s has empty podSelector (applies to ALL pods in namespace)",
				np.Namespace, np.Name))
		} else {
			// 2. Check if podSelector matches any existing pods
			selector, err := metav1.LabelSelectorAsSelector(&np.Spec.PodSelector)
			if err == nil {
				pods, err := client.Clientset.CoreV1().Pods(np.Namespace).List(ctx, metav1.ListOptions{
					LabelSelector: selector.String(),
				})
				if err == nil && len(pods.Items) == 0 {
					failures = append(failures, fmt.Sprintf(
						"NetworkPolicy %s/%s podSelector matches no existing pods",
						np.Namespace, np.Name))
				}
			}
		}

		// 3. Check ingress rules for issues
		for i, ingress := range np.Spec.Ingress {
			for j, from := range ingress.From {
				if from.PodSelector != nil {
					selector, err := metav1.LabelSelectorAsSelector(from.PodSelector)
					if err == nil {
						ns := np.Namespace
						if from.NamespaceSelector != nil {
							// Cross-namespace: skip pod match verification
							continue
						}
						pods, err := client.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
							LabelSelector: selector.String(),
						})
						if err == nil && len(pods.Items) == 0 {
							failures = append(failures, fmt.Sprintf(
								"NetworkPolicy %s/%s ingress rule[%d].from[%d] podSelector matches no pods",
								np.Namespace, np.Name, i, j))
						}
					}
				}
			}
		}

		// 4. Check egress rules for issues
		for i, egress := range np.Spec.Egress {
			for j, to := range egress.To {
				if to.PodSelector != nil {
					selector, err := metav1.LabelSelectorAsSelector(to.PodSelector)
					if err == nil {
						ns := np.Namespace
						if to.NamespaceSelector != nil {
							continue
						}
						pods, err := client.Clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
							LabelSelector: selector.String(),
						})
						if err == nil && len(pods.Items) == 0 {
							failures = append(failures, fmt.Sprintf(
								"NetworkPolicy %s/%s egress rule[%d].to[%d] podSelector matches no pods",
								np.Namespace, np.Name, i, j))
						}
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "NetworkPolicy",
				Name:  fmt.Sprintf("%s/%s", np.Namespace, np.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
