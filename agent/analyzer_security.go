package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityAnalyzer checks for security issues including default ServiceAccount usage,
// RBAC wildcard permissions, privileged containers, and missing SecurityContext.
type SecurityAnalyzer struct{}

func (s *SecurityAnalyzer) Name() string { return "Security" }

func (s *SecurityAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// 1. Check Pods for security issues
	pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}

	for _, pod := range pods.Items {
		var failures []string

		// Check default ServiceAccount usage
		if pod.Spec.ServiceAccountName == "default" || pod.Spec.ServiceAccountName == "" {
			failures = append(failures, fmt.Sprintf(
				"Pod %s/%s uses the default ServiceAccount, consider using a dedicated one",
				pod.Namespace, pod.Name))
		}

		// Check automountServiceAccountToken
		if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
			// Only flag if using default SA
			if pod.Spec.ServiceAccountName == "default" || pod.Spec.ServiceAccountName == "" {
				failures = append(failures, fmt.Sprintf(
					"Pod %s/%s has automountServiceAccountToken enabled with default SA",
					pod.Namespace, pod.Name))
			}
		}

		// Check Pod-level SecurityContext
		if pod.Spec.SecurityContext == nil {
			failures = append(failures, fmt.Sprintf(
				"Pod %s/%s has no PodSecurityContext defined",
				pod.Namespace, pod.Name))
		} else {
			if pod.Spec.SecurityContext.RunAsNonRoot == nil || !*pod.Spec.SecurityContext.RunAsNonRoot {
				failures = append(failures, fmt.Sprintf(
					"Pod %s/%s does not enforce runAsNonRoot",
					pod.Namespace, pod.Name))
			}
		}

		// Check containers for security issues
		for _, container := range pod.Spec.Containers {
			prefix := fmt.Sprintf("Pod %s/%s container %q", pod.Namespace, pod.Name, container.Name)

			// Privileged container check
			if container.SecurityContext != nil {
				if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
					failures = append(failures, fmt.Sprintf(
						"%s runs in privileged mode", prefix))
				}
				if container.SecurityContext.AllowPrivilegeEscalation != nil && *container.SecurityContext.AllowPrivilegeEscalation {
					failures = append(failures, fmt.Sprintf(
						"%s allows privilege escalation", prefix))
				}
				// Check capabilities
				if container.SecurityContext.Capabilities != nil {
					for _, cap := range container.SecurityContext.Capabilities.Add {
						if cap == "ALL" || cap == "SYS_ADMIN" || cap == "NET_ADMIN" {
							failures = append(failures, fmt.Sprintf(
								"%s adds dangerous capability: %s", prefix, cap))
						}
					}
				}
			} else {
				failures = append(failures, fmt.Sprintf(
					"%s has no SecurityContext defined", prefix))
			}

			// Check for missing resource limits
			if container.Resources.Limits == nil {
				failures = append(failures, fmt.Sprintf(
					"%s has no resource limits set", prefix))
			}

			// Check for latest tag or no tag
			image := container.Image
			if image == "" {
				continue
			}
			if hasLatestOrNoTag(image) {
				failures = append(failures, fmt.Sprintf(
					"%s uses image with 'latest' or no tag: %s", prefix, image))
			}
		}

		if len(failures) > 0 {
			parent, hasParent := client.GetParent(pod.ObjectMeta)
			result := AnalyzeResult{
				Kind:  "Security",
				Name:  fmt.Sprintf("%s/%s", pod.Namespace, pod.Name),
				Error: failures,
			}
			if hasParent {
				result.ParentObj = parent
			}
			results = append(results, result)
		}
	}

	// 2. Check ClusterRoleBindings for wildcard permissions
	crbList, err := client.Clientset.RbacV1().ClusterRoleBindings().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, crb := range crbList.Items {
			// Only check non-system bindings
			if isSystemNamespace(crb.RoleRef.Name) {
				continue
			}
			cr, err := client.Clientset.RbacV1().ClusterRoles().Get(ctx, crb.RoleRef.Name, metav1.GetOptions{})
			if err != nil {
				continue
			}
			for _, rule := range cr.Rules {
				for _, verb := range rule.Verbs {
					if verb == "*" {
						for _, res := range rule.Resources {
							if res == "*" {
								var failures []string
								failures = append(failures, fmt.Sprintf(
									"ClusterRoleBinding %q grants wildcard permissions (*.* via ClusterRole %q)",
									crb.Name, cr.Name))
								results = append(results, AnalyzeResult{
									Kind:  "Security",
									Name:  crb.Name,
									Error: failures,
								})
							}
						}
					}
				}
			}
		}
	}

	return results, nil
}

// hasLatestOrNoTag checks if an image reference uses 'latest' tag or no tag.
func hasLatestOrNoTag(image string) bool {
	// Check for explicit :latest
	if len(image) > 7 && image[len(image)-7:] == ":latest" {
		return true
	}
	// Check for no tag at all (no colon after the last slash)
	lastSlash := -1
	for i := len(image) - 1; i >= 0; i-- {
		if image[i] == '/' {
			lastSlash = i
			break
		}
	}
	afterSlash := image[lastSlash+1:]
	for _, c := range afterSlash {
		if c == ':' {
			return false
		}
	}
	return true
}

// isSystemNamespace checks if a name appears to be a Kubernetes system resource.
func isSystemNamespace(name string) bool {
	systemPrefixes := []string{"system:", "kube-", "calico-", "flannel-"}
	for _, prefix := range systemPrefixes {
		if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}
