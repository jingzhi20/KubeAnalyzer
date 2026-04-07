package main

import (
	"fmt"
	"strings"
)

// k8sDocBaseURL is the base URL for Kubernetes official documentation.
const k8sDocBaseURL = "https://kubernetes.io/docs"

// k8sAPIRefBaseURL is the base URL for Kubernetes API reference.
const k8sAPIRefBaseURL = "https://kubernetes.io/docs/reference/kubernetes-api"

// docRefMap maps resource kinds to their Kubernetes documentation paths.
var docRefMap = map[string]string{
	"Pod":                             "/concepts/workloads/pods/",
	"Deployment":                      "/concepts/workloads/controllers/deployment/",
	"StatefulSet":                     "/concepts/workloads/controllers/statefulset/",
	"DaemonSet":                      "/concepts/workloads/controllers/daemonset/",
	"ReplicaSet":                     "/concepts/workloads/controllers/replicaset/",
	"Job":                            "/concepts/workloads/controllers/job/",
	"CronJob":                        "/concepts/workloads/controllers/cron-jobs/",
	"Service":                        "/concepts/services-networking/service/",
	"Ingress":                        "/concepts/services-networking/ingress/",
	"NetworkPolicy":                  "/concepts/services-networking/network-policies/",
	"PersistentVolumeClaim":          "/concepts/storage/persistent-volumes/",
	"ConfigMap":                      "/concepts/configuration/configmap/",
	"Node":                           "/concepts/architecture/nodes/",
	"HorizontalPodAutoscaler":       "/tasks/run-application/horizontal-pod-autoscale/",
	"PodDisruptionBudget":           "/concepts/workloads/pods/disruptions/",
	"MutatingWebhookConfiguration":  "/reference/access-authn-authz/extensible-admission-controllers/",
	"ValidatingWebhookConfiguration": "/reference/access-authn-authz/extensible-admission-controllers/",
	"GatewayClass":                   "/concepts/services-networking/gateway/",
	"Gateway":                        "/concepts/services-networking/gateway/",
	"HTTPRoute":                      "/concepts/services-networking/gateway/",
	"Security":                       "/concepts/security/pod-security-standards/",
	"Log":                            "/concepts/cluster-administration/logging/",
	"StorageClass":                   "/concepts/storage/storage-classes/",
	"IngressRoute":                   "https://doc.traefik.io/traefik/routing/providers/kubernetes-crd/",
	"VirtualService":                 "https://istio.io/latest/docs/reference/config/networking/virtual-service/",
	"DestinationRule":                "https://istio.io/latest/docs/reference/config/networking/destination-rule/",
	"IstioGateway":                   "https://istio.io/latest/docs/reference/config/networking/gateway/",
	"PeerAuthentication":             "https://istio.io/latest/docs/reference/config/security/peer_authentication/",
	"AuthorizationPolicy":            "https://istio.io/latest/docs/reference/config/security/authorization-policy/",
	"Secret":                         "/concepts/configuration/secret/",
	"PersistentVolume":               "/concepts/storage/persistent-volumes/",
}

// fieldDocRefMap maps kind+field patterns to API reference paths for field-level docs.
var fieldDocRefMap = map[string]string{
	// Pod fields
	"Pod/spec.containers":                      "/workload-resources/pod-v1/#Container",
	"Pod/spec.securityContext":                  "/workload-resources/pod-v1/#PodSecurityContext",
	"Pod/status.containerStatuses":              "/workload-resources/pod-v1/#ContainerStatus",
	"Pod/status.conditions":                     "/workload-resources/pod-v1/#PodCondition",
	// Deployment fields
	"Deployment/spec.replicas":                  "/workload-resources/deployment-v1/#DeploymentSpec",
	"Deployment/spec.strategy":                  "/workload-resources/deployment-v1/#DeploymentStrategy",
	// StatefulSet fields
	"StatefulSet/spec.serviceName":              "/workload-resources/stateful-set-v1/#StatefulSetSpec",
	"StatefulSet/spec.volumeClaimTemplates":     "/workload-resources/stateful-set-v1/#StatefulSetSpec",
	// Service fields
	"Service/spec.selector":                     "/service-resources/service-v1/#ServiceSpec",
	"Service/spec.ports":                        "/service-resources/service-v1/#ServicePort",
	// Ingress fields
	"Ingress/spec.ingressClassName":             "/service-resources/ingress-v1/#IngressSpec",
	"Ingress/spec.rules":                        "/service-resources/ingress-v1/#IngressRule",
	// HPA fields
	"HorizontalPodAutoscaler/spec.scaleTargetRef": "/workload-resources/horizontal-pod-autoscaler-v2/#HorizontalPodAutoscalerSpec",
	"HorizontalPodAutoscaler/spec.metrics":      "/workload-resources/horizontal-pod-autoscaler-v2/#MetricSpec",
	// Node fields
	"Node/status.conditions":                    "/cluster-resources/node-v1/#NodeCondition",
	// CronJob fields
	"CronJob/spec.schedule":                     "/workload-resources/cron-job-v1/#CronJobSpec",
	"CronJob/spec.concurrencyPolicy":            "/workload-resources/cron-job-v1/#CronJobSpec",
}

// GetDocRef returns the Kubernetes documentation URL for a given resource kind.
func GetDocRef(kind string) string {
	if path, ok := docRefMap[kind]; ok {
		return k8sDocBaseURL + path
	}
	return ""
}

// EnrichWithDocRef appends a documentation link to the error messages of an AnalyzeResult.
// It uses field-level references when possible, falling back to resource-level docs.
func EnrichWithDocRef(result *AnalyzeResult) {
	docURL := GetDocRef(result.Kind)
	if docURL == "" {
		return
	}

	// Try to find field-level doc reference from error messages
	fieldRef := GetFieldDocRef(result.Kind, result.Error)
	if fieldRef != "" {
		result.Details = fmt.Sprintf("Kubernetes docs: %s | API ref: %s", docURL, fieldRef)
	} else {
		result.Details = fmt.Sprintf("Kubernetes docs: %s", docURL)
	}
}

// GetFieldDocRef tries to match error messages to field-level API reference docs.
func GetFieldDocRef(kind string, errors []string) string {
	// Map of error keywords to field paths
	keywordFieldMap := map[string]map[string]string{
		"Pod": {
			"CrashLoopBackOff": "Pod/status.containerStatuses",
			"ImagePullBackOff": "Pod/status.containerStatuses",
			"SecurityContext":  "Pod/spec.securityContext",
			"container":        "Pod/spec.containers",
			"condition":        "Pod/status.conditions",
		},
		"Deployment": {
			"replicas": "Deployment/spec.replicas",
			"strategy": "Deployment/spec.strategy",
		},
		"StatefulSet": {
			"serviceName":          "StatefulSet/spec.serviceName",
			"headless":             "StatefulSet/spec.serviceName",
			"VolumeClaimTemplate":  "StatefulSet/spec.volumeClaimTemplates",
			"StorageClass":         "StatefulSet/spec.volumeClaimTemplates",
		},
		"Service": {
			"endpoints":    "Service/spec.selector",
			"selector":     "Service/spec.selector",
			"port":         "Service/spec.ports",
		},
		"Ingress": {
			"IngressClass":    "Ingress/spec.ingressClassName",
			"ingressClassName": "Ingress/spec.ingressClassName",
			"rule":            "Ingress/spec.rules",
		},
		"HorizontalPodAutoscaler": {
			"ScaleTargetRef": "HorizontalPodAutoscaler/spec.scaleTargetRef",
			"metric":         "HorizontalPodAutoscaler/spec.metrics",
			"resource request": "HorizontalPodAutoscaler/spec.metrics",
		},
		"Node": {
			"condition": "Node/status.conditions",
			"NotReady":  "Node/status.conditions",
			"Pressure":  "Node/status.conditions",
		},
		"CronJob": {
			"cron":        "CronJob/spec.schedule",
			"schedule":    "CronJob/spec.schedule",
			"concurrency": "CronJob/spec.concurrencyPolicy",
		},
	}

	if kindMap, ok := keywordFieldMap[kind]; ok {
		for _, errMsg := range errors {
			errLower := strings.ToLower(errMsg)
			for keyword, fieldKey := range kindMap {
				if strings.Contains(errLower, strings.ToLower(keyword)) {
					if path, ok := fieldDocRefMap[fieldKey]; ok {
						return k8sAPIRefBaseURL + path
					}
				}
			}
		}
	}
	return ""
}

// MaskString masks sensitive data in a string, preserving the first and last characters.
// Example: "my-secret-password" -> "m*****************d"
func MaskString(s string) string {
	if len(s) <= 2 {
		return strings.Repeat("*", len(s))
	}
	return string(s[0]) + strings.Repeat("*", len(s)-2) + string(s[len(s)-1])
}

// MaskSensitiveValues masks values that appear to be sensitive (passwords, tokens, keys).
func MaskSensitiveValues(input string) string {
	// List of sensitive key patterns
	sensitivePatterns := []string{
		"password", "passwd", "secret", "token", "apikey", "api_key",
		"api-key", "access_key", "secret_key", "private_key",
		"credential", "auth", "bearer",
	}

	result := input
	for _, pattern := range sensitivePatterns {
		lowerInput := strings.ToLower(result)
		idx := strings.Index(lowerInput, pattern)
		if idx != -1 {
			// Find the value part after = or : following the key
			afterKey := result[idx+len(pattern):]
			for i, c := range afterKey {
				if c == '=' || c == ':' {
					// Extract the value (until next space or end)
					valueStart := idx + len(pattern) + i + 1
					valueEnd := valueStart
					for valueEnd < len(result) && result[valueEnd] != ' ' && result[valueEnd] != '\n' && result[valueEnd] != ',' && result[valueEnd] != ';' {
						valueEnd++
					}
					if valueEnd > valueStart {
						value := result[valueStart:valueEnd]
						masked := MaskString(strings.TrimSpace(value))
						result = result[:valueStart] + masked + result[valueEnd:]
					}
					break
				}
				if c == ' ' || c == '\n' {
					break
				}
			}
		}
	}
	return result
}
