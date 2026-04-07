package main

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapAnalyzer checks for ConfigMap issues including
// referenced-but-missing, unused, empty, and oversized detection.
type ConfigMapAnalyzer struct{}

func (c *ConfigMapAnalyzer) Name() string { return "ConfigMap" }

func (c *ConfigMapAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	// Get all pods to find ConfigMap references
	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	// Collect all referenced ConfigMaps
	type cmRef struct {
		namespace string
		name      string
		podName   string
	}
	var refs []cmRef
	referencedCMs := make(map[string]bool) // ns/name -> referenced

	for _, pod := range pods.Items {
		// Check volumes
		for _, vol := range pod.Spec.Volumes {
			if vol.ConfigMap != nil {
				key := pod.Namespace + "/" + vol.ConfigMap.Name
				referencedCMs[key] = true
				refs = append(refs, cmRef{
					namespace: pod.Namespace,
					name:      vol.ConfigMap.Name,
					podName:   pod.Name,
				})
			}
			if vol.Projected != nil {
				for _, src := range vol.Projected.Sources {
					if src.ConfigMap != nil {
						key := pod.Namespace + "/" + src.ConfigMap.Name
						referencedCMs[key] = true
					}
				}
			}
		}
		// Check env refs
		for _, container := range pod.Spec.Containers {
			for _, envFrom := range container.EnvFrom {
				if envFrom.ConfigMapRef != nil {
					key := pod.Namespace + "/" + envFrom.ConfigMapRef.Name
					referencedCMs[key] = true
					refs = append(refs, cmRef{
						namespace: pod.Namespace,
						name:      envFrom.ConfigMapRef.Name,
						podName:   pod.Name,
					})
				}
			}
			for _, env := range container.Env {
				if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
					key := pod.Namespace + "/" + env.ValueFrom.ConfigMapKeyRef.Name
					referencedCMs[key] = true
					// Also add to refs for existence check (unless optional)
					optional := env.ValueFrom.ConfigMapKeyRef.Optional != nil && *env.ValueFrom.ConfigMapKeyRef.Optional
					if !optional {
						refs = append(refs, cmRef{
							namespace: pod.Namespace,
							name:      env.ValueFrom.ConfigMapKeyRef.Name,
							podName:   pod.Name,
						})
					}
				}
			}
		}
	}

	// Check each referenced ConfigMap exists
	checked := make(map[string]bool)
	var results []AnalyzeResult

	for _, ref := range refs {
		key := ref.namespace + "/" + ref.name
		if checked[key] {
			continue
		}
		checked[key] = true

		_, err := client.Clientset.CoreV1().ConfigMaps(ref.namespace).Get(ctx, ref.name, metav1.GetOptions{})
		if err != nil {
			results = append(results, AnalyzeResult{
				Kind:  "ConfigMap",
				Name:  key,
				Error: []string{fmt.Sprintf("ConfigMap %s referenced by Pod %s not found", key, ref.podName)},
			})
		}
	}

	// List all ConfigMaps for unused/empty/oversized checks
	allCMs, err := client.Clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return results, nil
	}

	// System ConfigMaps to skip
	systemCMs := map[string]bool{
		"kube-root-ca.crt": true, "extension-apiserver-authentication": true,
		"coredns": true, "kubeadm-config": true, "kubelet-config": true,
		"cluster-info": true, "kube-proxy": true,
	}
	systemNamespaces := map[string]bool{
		"kube-system": true, "kube-public": true, "kube-node-lease": true,
	}

	const maxCMSizeBytes = 1048576 // 1 MiB (K8s hard limit)
	const warnCMSizeBytes = 524288 // 512 KiB warning threshold

	for _, cm := range allCMs.Items {
		// Skip system namespaces and known system CMs
		if systemNamespaces[cm.Namespace] || systemCMs[cm.Name] {
			continue
		}
		// Skip CMs with owner references (managed by controllers)
		if len(cm.OwnerReferences) > 0 {
			continue
		}

		var failures []string
		key := cm.Namespace + "/" + cm.Name

		// Empty check
		if len(cm.Data) == 0 && len(cm.BinaryData) == 0 {
			failures = append(failures, fmt.Sprintf(
				"ConfigMap %s is empty (no data or binaryData)", key))
		}

		// Oversized check
		totalSize := 0
		for _, v := range cm.Data {
			totalSize += len(v)
		}
		for _, v := range cm.BinaryData {
			totalSize += len(v)
		}
		if totalSize > maxCMSizeBytes {
			failures = append(failures, fmt.Sprintf(
				"ConfigMap %s exceeds 1 MiB size limit (%d bytes)", key, totalSize))
		} else if totalSize > warnCMSizeBytes {
			failures = append(failures, fmt.Sprintf(
				"ConfigMap %s is large (%d bytes, warning threshold 512 KiB)", key, totalSize))
		}

		// Unused check
		if !referencedCMs[key] && len(failures) == 0 {
			// Also check if ConfigMap starts with known prefixes to skip
			if !strings.HasPrefix(cm.Name, "sh.helm.release") && !strings.HasPrefix(cm.Name, "ingress-controller-leader") {
				failures = append(failures, fmt.Sprintf(
					"ConfigMap %s appears unused (not referenced by any Pod)", key))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ConfigMap",
				Name:  key,
				Error: failures,
			})
		}
	}

	return results, nil
}

// MutatingWebhookAnalyzer checks for MutatingWebhookConfiguration issues.
type MutatingWebhookAnalyzer struct{}

func (m *MutatingWebhookAnalyzer) Name() string { return "MutatingWebhookConfiguration" }

func (m *MutatingWebhookAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	webhooks, err := client.Clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, wh := range webhooks.Items {
		var failures []string

		for _, hook := range wh.Webhooks {
			if hook.ClientConfig.Service != nil {
				svcName := hook.ClientConfig.Service.Name
				svcNamespace := hook.ClientConfig.Service.Namespace

				_, err := client.Clientset.CoreV1().Services(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"MutatingWebhook %s references non-existent service %s/%s",
						wh.Name, svcNamespace, svcName))
				}
			}

			// Check failure policy
			if hook.FailurePolicy != nil && strings.EqualFold(string(*hook.FailurePolicy), "fail") {
				// Webhook with Fail policy and no service is critical
				if hook.ClientConfig.Service != nil {
					svcName := hook.ClientConfig.Service.Name
					svcNamespace := hook.ClientConfig.Service.Namespace
					endpoints, err := client.Clientset.CoreV1().Endpoints(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
					if err == nil {
						hasAddresses := false
						for _, subset := range endpoints.Subsets {
							if len(subset.Addresses) > 0 {
								hasAddresses = true
								break
							}
						}
						if !hasAddresses {
							failures = append(failures, fmt.Sprintf(
								"MutatingWebhook %s (FailurePolicy=Fail) service %s/%s has no ready endpoints",
								wh.Name, svcNamespace, svcName))
						}
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "MutatingWebhookConfiguration",
				Name:  wh.Name,
				Error: failures,
			})
		}
	}

	return results, nil
}

// ValidatingWebhookAnalyzer checks for ValidatingWebhookConfiguration issues.
type ValidatingWebhookAnalyzer struct{}

func (v *ValidatingWebhookAnalyzer) Name() string { return "ValidatingWebhookConfiguration" }

func (v *ValidatingWebhookAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	webhooks, err := client.Clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult

	for _, wh := range webhooks.Items {
		var failures []string

		for _, hook := range wh.Webhooks {
			if hook.ClientConfig.Service != nil {
				svcName := hook.ClientConfig.Service.Name
				svcNamespace := hook.ClientConfig.Service.Namespace

				_, err := client.Clientset.CoreV1().Services(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"ValidatingWebhook %s references non-existent service %s/%s",
						wh.Name, svcNamespace, svcName))
				}
			}

			// Check failure policy
			if hook.FailurePolicy != nil && strings.EqualFold(string(*hook.FailurePolicy), "fail") {
				if hook.ClientConfig.Service != nil {
					svcName := hook.ClientConfig.Service.Name
					svcNamespace := hook.ClientConfig.Service.Namespace
					endpoints, err := client.Clientset.CoreV1().Endpoints(svcNamespace).Get(ctx, svcName, metav1.GetOptions{})
					if err == nil {
						hasAddresses := false
						for _, subset := range endpoints.Subsets {
							if len(subset.Addresses) > 0 {
								hasAddresses = true
								break
							}
						}
						if !hasAddresses {
							failures = append(failures, fmt.Sprintf(
								"ValidatingWebhook %s (FailurePolicy=Fail) service %s/%s has no ready endpoints",
								wh.Name, svcNamespace, svcName))
						}
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ValidatingWebhookConfiguration",
				Name:  wh.Name,
				Error: failures,
			})
		}
	}

	return results, nil
}
