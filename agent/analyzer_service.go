package main

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceAnalyzer checks for Service selector/endpoint issues including
// NotReadyAddresses, Event scanning, and leader election service skipping.
type ServiceAnalyzer struct{}

func (s *ServiceAnalyzer) Name() string { return "Service" }

func (s *ServiceAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	services, err := client.Clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, svc := range services.Items {
		// Skip services without selectors (e.g. ExternalName, headless without selector)
		if len(svc.Spec.Selector) == 0 {
			continue
		}

		// Skip known leader election services (kube-system internal)
		if strings.HasSuffix(svc.Name, "-leader-election") || strings.HasSuffix(svc.Name, "-leader-locking") {
			continue
		}

		var failures []string

		// Check if endpoints exist
		endpoints, err := client.Clientset.CoreV1().Endpoints(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
		if err != nil {
			failures = append(failures, fmt.Sprintf("Service %s/%s has no associated endpoints object", svc.Namespace, svc.Name))
		} else {
			hasAddresses := false
			notReadyCount := 0

			for _, subset := range endpoints.Subsets {
				if len(subset.Addresses) > 0 {
					hasAddresses = true
				}
				notReadyCount += len(subset.NotReadyAddresses)
			}

			if !hasAddresses {
				failures = append(failures, fmt.Sprintf("Service %s/%s has no ready endpoints", svc.Namespace, svc.Name))
			}

			// Report NotReadyAddresses
			if notReadyCount > 0 {
				failures = append(failures, fmt.Sprintf(
					"Service %s/%s has %d not-ready endpoint addresses",
					svc.Namespace, svc.Name, notReadyCount))
			}
		}

		// Check for warning events on the Service
		evtReason, evtMsg, err := client.FetchLatestEvent(ctx, svc.Namespace, svc.Name)
		if err == nil && evtReason != "" {
			// Only report non-normal events
			if evtReason != "LeaderElection" {
				failures = append(failures, fmt.Sprintf("Event: [%s] %s", evtReason, evtMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Service",
				Name:  fmt.Sprintf("%s/%s", svc.Namespace, svc.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
