package main

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NodeAnalyzer checks for Node condition issues (NotReady, MemoryPressure, DiskPressure, etc.).
type NodeAnalyzer struct{}

func (n *NodeAnalyzer) Name() string { return "Node" }

func (n *NodeAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	// Nodes are cluster-scoped, ignore namespace
	nodes, err := client.Clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, node := range nodes.Items {
		var failures []string

		for _, cond := range node.Status.Conditions {
			switch cond.Type {
			case v1.NodeReady:
				if cond.Status != v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node %s is not Ready: %s", node.Name, cond.Message))
				}
			case v1.NodeMemoryPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node %s has MemoryPressure: %s", node.Name, cond.Message))
				}
			case v1.NodeDiskPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node %s has DiskPressure: %s", node.Name, cond.Message))
				}
			case v1.NodePIDPressure:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node %s has PIDPressure: %s", node.Name, cond.Message))
				}
			case v1.NodeNetworkUnavailable:
				if cond.Status == v1.ConditionTrue {
					failures = append(failures, fmt.Sprintf("Node %s has NetworkUnavailable: %s", node.Name, cond.Message))
				}
			}
		}

		// Check taints for unschedulable
		for _, taint := range node.Spec.Taints {
			if taint.Key == "node.kubernetes.io/unschedulable" {
				failures = append(failures, fmt.Sprintf("Node %s is cordoned (unschedulable)", node.Name))
			}
			if taint.Key == "node.kubernetes.io/not-ready" {
				failures = append(failures, fmt.Sprintf("Node %s has not-ready taint", node.Name))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Node",
				Name:  node.Name,
				Error: failures,
			})
		}
	}

	return results, nil
}
