package main

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	catalogSourceGVR       = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "catalogsources"}
	subscriptionGVR        = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions"}
	installPlanGVR         = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans"}
	clusterServiceVersionGVR = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"}
	clusterExtensionGVR    = schema.GroupVersionResource{Group: "olm.operatorframework.io", Version: "v1", Resource: "clusterextensions"}
	// OLM v1 CRDs
	clusterCatalogGVR      = schema.GroupVersionResource{Group: "olm.operatorframework.io", Version: "v1", Resource: "clustercatalogs"}
	operatorGroupGVR       = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups"}
)

// CatalogSourceAnalyzer checks for OLM CatalogSource issues.
type CatalogSourceAnalyzer struct{}

func (c *CatalogSourceAnalyzer) Name() string { return "CatalogSource" }

func (c *CatalogSourceAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(catalogSourceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil // OLM not installed
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// Check connection state
		connState, _, _ := unstructured.NestedString(item.Object, "status", "connectionState", "lastObservedState")
		if connState != "" && connState != "READY" {
			failures = append(failures, fmt.Sprintf(
				"CatalogSource %s/%s connection state is %s (expected READY)", ns, name, connState))
		}

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)

			if condStatus == "False" {
				failures = append(failures, fmt.Sprintf(
					"CatalogSource %s/%s condition %s is False: %s", ns, name, condType, condMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "CatalogSource",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// SubscriptionAnalyzer checks for OLM Subscription issues.
type SubscriptionAnalyzer struct{}

func (s *SubscriptionAnalyzer) Name() string { return "Subscription" }

func (s *SubscriptionAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(subscriptionGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// Check state
		state, _, _ := unstructured.NestedString(item.Object, "status", "state")
		if state != "" && state != "AtLatestKnown" {
			failures = append(failures, fmt.Sprintf(
				"Subscription %s/%s state is %q (expected AtLatestKnown)", ns, name, state))
		}

		// Check CatalogSource reference
		catalogSrc, _, _ := unstructured.NestedString(item.Object, "spec", "source")
		catalogNs, _, _ := unstructured.NestedString(item.Object, "spec", "sourceNamespace")
		if catalogSrc != "" && catalogNs != "" {
			_, err := client.DynamicClient.Resource(catalogSourceGVR).Namespace(catalogNs).Get(ctx, catalogSrc, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"Subscription %s/%s references non-existent CatalogSource %s/%s",
					ns, name, catalogNs, catalogSrc))
			}
		}

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			if condType == "CatalogSourcesUnhealthy" && condStatus == "True" {
				failures = append(failures, fmt.Sprintf(
					"Subscription %s/%s CatalogSourcesUnhealthy: %s - %s",
					ns, name, condReason, condMsg))
			}
			if condType == "InstallPlanFailed" && condStatus == "True" {
				failures = append(failures, fmt.Sprintf(
					"Subscription %s/%s InstallPlan failed: %s - %s",
					ns, name, condReason, condMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Subscription",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// InstallPlanAnalyzer checks for OLM InstallPlan issues.
type InstallPlanAnalyzer struct{}

func (i *InstallPlanAnalyzer) Name() string { return "InstallPlan" }

func (i *InstallPlanAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(installPlanGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// Check phase
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		if phase == "Failed" {
			failures = append(failures, fmt.Sprintf(
				"InstallPlan %s/%s is in Failed phase", ns, name))
		}

		// Check if requiring approval
		approved, _, _ := unstructured.NestedBool(item.Object, "spec", "approved")
		if !approved && phase == "RequiresApproval" {
			failures = append(failures, fmt.Sprintf(
				"InstallPlan %s/%s requires manual approval", ns, name))
		}

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)

			if condType == "Installed" && condStatus == "False" {
				failures = append(failures, fmt.Sprintf(
					"InstallPlan %s/%s not Installed: %s", ns, name, condMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "InstallPlan",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// CSVAnalyzer checks for OLM ClusterServiceVersion issues.
type CSVAnalyzer struct{}

func (c *CSVAnalyzer) Name() string { return "ClusterServiceVersion" }

func (c *CSVAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(clusterServiceVersionGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// Check phase
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		reason, _, _ := unstructured.NestedString(item.Object, "status", "reason")
		message, _, _ := unstructured.NestedString(item.Object, "status", "message")

		if phase != "" && phase != "Succeeded" {
			failures = append(failures, fmt.Sprintf(
				"CSV %s/%s phase is %q: %s - %s", ns, name, phase, reason, message))
		}

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			if condStatus == "False" && condType != "" {
				failures = append(failures, fmt.Sprintf(
					"CSV %s/%s condition %s is False: %s - %s",
					ns, name, condType, condReason, condMsg))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ClusterServiceVersion",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// ClusterExtensionAnalyzer checks for OLMv1 ClusterExtension issues.
type ClusterExtensionAnalyzer struct{}

func (c *ClusterExtensionAnalyzer) Name() string { return "ClusterExtension" }

func (c *ClusterExtensionAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(clusterExtensionGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil // OLMv1 not installed
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()

		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			switch condType {
			case "Installed":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"ClusterExtension %s not Installed: %s - %s", name, condReason, condMsg))
				}
			case "Progressing":
				if condStatus == "True" && condReason == "Retrying" {
					failures = append(failures, fmt.Sprintf(
						"ClusterExtension %s is retrying: %s", name, condMsg))
				}
			case "Resolved":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"ClusterExtension %s not Resolved: %s - %s", name, condReason, condMsg))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ClusterExtension",
				Name:  name,
				Error: failures,
			})
		}
	}

	return results, nil
}

// ClusterCatalogAnalyzer checks for OLM v1 ClusterCatalog issues.
type ClusterCatalogAnalyzer struct{}

func (c *ClusterCatalogAnalyzer) Name() string { return "ClusterCatalog" }

func (c *ClusterCatalogAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(clusterCatalogGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil // OLM v1 not installed
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			switch condType {
			case "Unpacked":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"ClusterCatalog %s not Unpacked: %s - %s", name, condReason, condMsg))
				}
			case "Valid":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"ClusterCatalog %s is not Valid: %s - %s", name, condReason, condMsg))
				}
			case "Progressing":
				if condStatus == "True" {
					failures = append(failures, fmt.Sprintf(
						"ClusterCatalog %s is still Progressing: %s", name, condMsg))
				}
			}
		}

		// Check sourceRef
		sourceType, _, _ := unstructured.NestedString(item.Object, "spec", "source", "type")
		if sourceType == "" {
			failures = append(failures, fmt.Sprintf(
				"ClusterCatalog %s has no source type specified", name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "ClusterCatalog",
				Name:  name,
				Error: failures,
			})
		}
	}

	return results, nil
}

// OperatorGroupAnalyzer checks for OLM v1alpha1 OperatorGroup issues.
type OperatorGroupAnalyzer struct{}

func (o *OperatorGroupAnalyzer) Name() string { return "OperatorGroup" }

func (o *OperatorGroupAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(operatorGroupGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil // OLM not installed
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)

			if condStatus == "False" && condType != "" {
				failures = append(failures, fmt.Sprintf(
					"OperatorGroup %s/%s condition %s is False: %s", ns, name, condType, condMsg))
			}
		}

		// Check for ServiceAccount reference
		saName, _, _ := unstructured.NestedString(item.Object, "spec", "serviceAccountName")
		if saName != "" {
			// Verify ServiceAccount exists
			_, err := client.Clientset.CoreV1().ServiceAccounts(ns).Get(ctx, saName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf(
					"OperatorGroup %s/%s references non-existent ServiceAccount %s", ns, name, saName))
			}
		}

		// Check status.namespaces matches targetNamespaces
		targetNamespaces, _, _ := unstructured.NestedSlice(item.Object, "spec", "targetNamespaces")
		statusNamespaces, _, _ := unstructured.NestedSlice(item.Object, "status", "namespaces")
		if len(targetNamespaces) > 0 && len(statusNamespaces) == 0 {
			failures = append(failures, fmt.Sprintf(
				"OperatorGroup %s/%s has targetNamespaces but status.namespaces is empty", ns, name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "OperatorGroup",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}
