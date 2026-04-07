package main

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClassAnalyzer checks for StorageClass and storage provisioner issues.
type StorageClassAnalyzer struct{}

func (s *StorageClassAnalyzer) Name() string { return "StorageClass" }

func (s *StorageClassAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// 1. Check StorageClasses
	scList, err := client.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	defaultSCCount := 0
	for _, sc := range scList.Items {
		var failures []string

		// Check for default StorageClass annotation
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" ||
			sc.Annotations["storageclass.beta.kubernetes.io/is-default-class"] == "true" {
			defaultSCCount++
		}

		// Check provisioner is set
		if sc.Provisioner == "" {
			failures = append(failures, fmt.Sprintf(
				"StorageClass %s has no provisioner specified", sc.Name))
		}

		// Check for deprecated provisioner
		deprecatedProvisioners := map[string]string{
			"kubernetes.io/gce-pd":       "Use pd.csi.storage.gke.io instead",
			"kubernetes.io/aws-ebs":      "Use ebs.csi.aws.com instead",
			"kubernetes.io/azure-disk":   "Use disk.csi.azure.com instead",
			"kubernetes.io/azure-file":   "Use file.csi.azure.com instead",
			"kubernetes.io/cinder":       "Use cinder.csi.openstack.org instead",
			"kubernetes.io/vsphere-volume": "Use csi.vsphere.vmware.com instead",
		}
		if suggestion, ok := deprecatedProvisioners[sc.Provisioner]; ok {
			failures = append(failures, fmt.Sprintf(
				"StorageClass %s uses deprecated in-tree provisioner %q. %s",
				sc.Name, sc.Provisioner, suggestion))
		}

		// Check reclaimPolicy
		if sc.ReclaimPolicy != nil {
			policy := string(*sc.ReclaimPolicy)
			if policy == "Delete" {
				// Not an error per se, but worth noting for data-critical workloads
				// Only flag if it's the default SC
				if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
					failures = append(failures, fmt.Sprintf(
						"StorageClass %s is the default and has ReclaimPolicy=Delete (data will be lost on PV release)",
						sc.Name))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "StorageClass",
				Name:  sc.Name,
				Error: failures,
			})
		}
	}

	// Check for multiple default StorageClasses
	if defaultSCCount > 1 {
		results = append(results, AnalyzeResult{
			Kind:  "StorageClass",
			Name:  "(cluster)",
			Error: []string{fmt.Sprintf("Cluster has %d default StorageClasses (should be exactly 1)", defaultSCCount)},
		})
	}
	if defaultSCCount == 0 && len(scList.Items) > 0 {
		results = append(results, AnalyzeResult{
			Kind:  "StorageClass",
			Name:  "(cluster)",
			Error: []string{"No default StorageClass is configured"},
		})
	}

	// 2. Check CSIDrivers
	csiDrivers, err := client.Clientset.StorageV1().CSIDrivers().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, driver := range csiDrivers.Items {
			var failures []string

			// Check if CSIDriver has volumeLifecycleModes set
			if len(driver.Spec.VolumeLifecycleModes) == 0 {
				failures = append(failures, fmt.Sprintf(
					"CSIDriver %s has no volumeLifecycleModes specified", driver.Name))
			}

			if len(failures) > 0 {
				results = append(results, AnalyzeResult{
					Kind:  "StorageClass",
					Name:  fmt.Sprintf("CSIDriver/%s", driver.Name),
					Error: failures,
				})
			}
		}
	}

	// 3. Check CSINodes for driver registration
	csiNodes, err := client.Clientset.StorageV1().CSINodes().List(ctx, metav1.ListOptions{})
	if err == nil {
		// Collect all expected drivers from StorageClasses
		expectedDrivers := make(map[string]bool)
		for _, sc := range scList.Items {
			expectedDrivers[sc.Provisioner] = true
		}

		// Check if all nodes have the expected CSI drivers
		for _, csiNode := range csiNodes.Items {
			nodeDrivers := make(map[string]bool)
			for _, d := range csiNode.Spec.Drivers {
				nodeDrivers[d.Name] = true
			}
			for driver := range expectedDrivers {
				if !nodeDrivers[driver] {
					// Only flag CSI drivers (skip in-tree provisioners)
					if len(driver) > 0 && driver[0] != 'k' { // Skip kubernetes.io/* in-tree
						results = append(results, AnalyzeResult{
							Kind:  "StorageClass",
							Name:  fmt.Sprintf("CSINode/%s", csiNode.Name),
							Error: []string{fmt.Sprintf("Node %s missing CSI driver %q required by StorageClass", csiNode.Name, driver)},
						})
					}
				}
			}
		}
	}

	return results, nil
}

// StorageAnalyzer checks for comprehensive storage issues (StorageClass, PV, PVC).
type StorageAnalyzer struct{}

func (s *StorageAnalyzer) Name() string { return "Storage" }

func (s *StorageAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// 1. StorageClass analysis
	scResults, _ := s.analyzeStorageClasses(ctx, client)
	results = append(results, scResults...)

	// 2. PV analysis
	pvResults, _ := s.analyzePVs(ctx, client)
	results = append(results, pvResults...)

	// 3. PVC analysis
	pvcResults, _ := s.analyzePVCs(ctx, client, namespace)
	results = append(results, pvcResults...)

	return results, nil
}

func (s *StorageAnalyzer) analyzeStorageClasses(ctx context.Context, client *K8sClient) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	scList, err := client.Clientset.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	defaultCount := 0
	for _, sc := range scList.Items {
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultCount++
		}
	}

	for _, sc := range scList.Items {
		var failures []string

		// Check for deprecated provisioner
		if sc.Provisioner == "kubernetes.io/no-provisioner" {
			failures = append(failures, fmt.Sprintf(
				"StorageClass %s uses deprecated provisioner 'kubernetes.io/no-provisioner'", sc.Name))
		}

		// Check for multiple defaults
		if defaultCount > 1 && sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			failures = append(failures, fmt.Sprintf(
				"Multiple default StorageClasses found (%d), which can cause confusion", defaultCount))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/StorageClass",
				Name:  sc.Name,
				Error: failures,
			})
		}
	}

	return results, nil
}

func (s *StorageAnalyzer) analyzePVs(ctx context.Context, client *K8sClient) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	pvList, err := client.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	for _, pv := range pvList.Items {
		var failures []string

		phase := string(pv.Status.Phase)

		if phase == "Released" {
			failures = append(failures, fmt.Sprintf(
				"PersistentVolume %s is in Released state and should be cleaned up", pv.Name))
		}
		if phase == "Failed" {
			failures = append(failures, fmt.Sprintf(
				"PersistentVolume %s is in Failed state", pv.Name))
		}

		// Check for small capacity (< 1Gi)
		if capacity, ok := pv.Spec.Capacity["storage"]; ok {
			if isSmallCapacityStr(capacity.String()) {
				failures = append(failures, fmt.Sprintf(
					"PersistentVolume %s has small capacity (%s)", pv.Name, capacity.String()))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/PersistentVolume",
				Name:  pv.Name,
				Error: failures,
			})
		}
	}

	return results, nil
}

func (s *StorageAnalyzer) analyzePVCs(ctx context.Context, client *K8sClient, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	var pvcList *v1.PersistentVolumeClaimList
	var err error
	if namespace != "" {
		pvcList, err = client.Clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	} else {
		pvcList, err = client.Clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		return nil, nil
	}

	for _, pvc := range pvcList.Items {
		var failures []string

		phase := string(pvc.Status.Phase)
		fullName := fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)

		switch phase {
		case "Pending":
			failures = append(failures, fmt.Sprintf(
				"PersistentVolumeClaim %s is in Pending state", fullName))
		case "Lost":
			failures = append(failures, fmt.Sprintf(
				"PersistentVolumeClaim %s is in Lost state", fullName))
		default:
			// Check for small capacity
			if storage, ok := pvc.Spec.Resources.Requests["storage"]; ok {
				if isSmallCapacityStr(storage.String()) {
					failures = append(failures, fmt.Sprintf(
						"PersistentVolumeClaim %s has small capacity (%s)", fullName, storage.String()))
				}
			}
			// Check for missing storage class
			if pvc.Spec.StorageClassName == nil && pvc.Spec.VolumeName == "" {
				failures = append(failures, fmt.Sprintf(
					"PersistentVolumeClaim %s has no StorageClass specified", fullName))
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/PersistentVolumeClaim",
				Name:  fullName,
				Error: []string{failures[0]}, // Only report the first (most critical)
			})
		}
	}

	return results, nil
}

// isSmallCapacityStr checks if a K8s resource quantity string is less than 1Gi.
func isSmallCapacityStr(s string) bool {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "Gi") || strings.HasSuffix(s, "Ti") {
		return false // >= 1Gi
	}
	if strings.HasSuffix(s, "G") || strings.HasSuffix(s, "T") {
		return false
	}
	// Mi, Ki, or bare numbers are likely < 1Gi
	return true
}
