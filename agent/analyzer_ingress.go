package main

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressAnalyzer checks for Ingress configuration issues including
// backend Service existence, TLS secrets, and IngressClass validation.
type IngressAnalyzer struct{}

func (i *IngressAnalyzer) Name() string { return "Ingress" }

func (i *IngressAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	ingresses, err := client.Clientset.NetworkingV1().Ingresses(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, ing := range ingresses.Items {
		var failures []string

		// 1. IngressClass validation (spec.ingressClassName - new format)
		if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
			className := *ing.Spec.IngressClassName
			_, err := client.Clientset.NetworkingV1().IngressClasses().Get(ctx, className, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"Ingress %s/%s references non-existent IngressClass %q",
					ing.Namespace, ing.Name, className))
			}
		} else {
			// Check legacy annotation: kubernetes.io/ingress.class
			if legacyClass, ok := ing.Annotations["kubernetes.io/ingress.class"]; ok && legacyClass != "" {
				// Validate legacy class against IngressClass resources
				_, err := client.Clientset.NetworkingV1().IngressClasses().Get(ctx, legacyClass, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"Ingress %s/%s annotation kubernetes.io/ingress.class=%q references non-existent IngressClass",
						ing.Namespace, ing.Name, legacyClass))
				}
			}
		}

		// 2. Backend Service existence check
		for _, rule := range ing.Spec.Rules {
			if rule.HTTP == nil {
				continue
			}
			for _, path := range rule.HTTP.Paths {
				if path.Backend.Service == nil {
					continue
				}
				svcName := path.Backend.Service.Name
				_, err := client.Clientset.CoreV1().Services(ing.Namespace).Get(ctx, svcName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"Ingress %s/%s references non-existent service %s",
						ing.Namespace, ing.Name, svcName))
				}
			}
		}

		// 3. Default backend check
		if ing.Spec.DefaultBackend != nil && ing.Spec.DefaultBackend.Service != nil {
			svcName := ing.Spec.DefaultBackend.Service.Name
			_, err := client.Clientset.CoreV1().Services(ing.Namespace).Get(ctx, svcName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"Ingress %s/%s default backend references non-existent service %s",
					ing.Namespace, ing.Name, svcName))
			}
		}

		// 4. TLS secrets check
		for _, tls := range ing.Spec.TLS {
			if tls.SecretName != "" {
				_, err := client.Clientset.CoreV1().Secrets(ing.Namespace).Get(ctx, tls.SecretName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"Ingress %s/%s references non-existent TLS secret %s",
						ing.Namespace, ing.Name, tls.SecretName))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Ingress",
				Name:  fmt.Sprintf("%s/%s", ing.Namespace, ing.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// PVCAnalyzer checks for PersistentVolumeClaim binding issues.
type PVCAnalyzer struct{}

func (p *PVCAnalyzer) Name() string { return "PersistentVolumeClaim" }

func (p *PVCAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	pvcs, err := client.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, pvc := range pvcs.Items {
		var failures []string

		if pvc.Status.Phase == v1.ClaimPending {
			failures = append(failures, fmt.Sprintf(
				"PVC %s/%s is in Pending state",
				pvc.Namespace, pvc.Name))

			// Check events for more details
			evtReason, evtMsg, err := client.FetchLatestEvent(ctx, pvc.Namespace, pvc.Name)
			if err == nil && evtReason != "" && evtMsg != "" {
				failures = append(failures, fmt.Sprintf("Event: [%s] %s", evtReason, evtMsg))
			}
		}

		if pvc.Status.Phase == v1.ClaimLost {
			failures = append(failures, fmt.Sprintf(
				"PVC %s/%s is in Lost state - the bound PersistentVolume has been deleted",
				pvc.Namespace, pvc.Name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "PersistentVolumeClaim",
				Name:  fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
				Error: failures,
			})
		}
	}

	return results, nil
}
