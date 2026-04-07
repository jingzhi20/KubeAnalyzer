package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// AnalyzeResult represents a single analysis finding (compatible with server-side k8sgpt.AnalyzeResult).
type AnalyzeResult struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Error     []string `json:"error"`
	Details   string   `json:"details"`
	ParentObj string   `json:"parentObject"`
}

// AnalyzeResponse represents the full analysis output.
type AnalyzeResponse struct {
	Provider string          `json:"provider"`
	Errors   int             `json:"errors"`
	Status   string          `json:"status"`
	Problems int             `json:"problems"`
	Results  []AnalyzeResult `json:"results"`
}

// IAnalyzer is the interface all analyzers must implement.
type IAnalyzer interface {
	Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error)
	Name() string
}

// defaultFilters mirrors the server-side k8sgpt/executor.go defaultFilters.
var defaultFilters = []string{
	"Pod", "Deployment", "Service", "Node", "StatefulSet",
	"ReplicaSet", "Ingress", "PersistentVolumeClaim",
	"CronJob", "Job", "ConfigMap", "DaemonSet",
	"HorizontalPodAutoscaler", "PodDisruptionBudget",
	"NetworkPolicy", "Security", "Log",
	"GatewayClass", "Gateway", "HTTPRoute",
	"CatalogSource", "Subscription", "InstallPlan",
	"ClusterServiceVersion", "ClusterExtension",
	"StorageClass", "Storage",
	// Traefik CRDs
	"IngressRoute", "IngressRouteTCP", "IngressRouteUDP",
	"Middleware", "MiddlewareTCP", "TraefikService",
	"TLSOption", "TLSStore",
	// Istio CRDs
	"VirtualService", "DestinationRule", "IstioGateway",
	"ServiceEntry", "Sidecar", "PeerAuthentication", "AuthorizationPolicy",
	// OLM v1
	"ClusterCatalog", "OperatorGroup",
	// Composite analyzers
	"NetworkComponentPods", "IngressAccessLog", "WarningEvents",
	"Secret", "PersistentVolume",
}

// analyzerRegistry maps filter names to analyzer implementations.
var analyzerRegistry = map[string]IAnalyzer{}

// registerAnalyzer adds an analyzer to the registry.
func registerAnalyzer(name string, a IAnalyzer) {
	analyzerRegistry[name] = a
}

func init() {
	registerAnalyzer("Pod", &PodAnalyzer{})
	registerAnalyzer("Deployment", &DeploymentAnalyzer{})
	registerAnalyzer("Service", &ServiceAnalyzer{})
	registerAnalyzer("Node", &NodeAnalyzer{})
	registerAnalyzer("StatefulSet", &StatefulSetAnalyzer{})
	registerAnalyzer("ReplicaSet", &ReplicaSetAnalyzer{})
	registerAnalyzer("Ingress", &IngressAnalyzer{})
	registerAnalyzer("PersistentVolumeClaim", &PVCAnalyzer{})
	registerAnalyzer("Job", &JobAnalyzer{})
	registerAnalyzer("CronJob", &CronJobAnalyzer{})
	registerAnalyzer("ConfigMap", &ConfigMapAnalyzer{})
	registerAnalyzer("DaemonSet", &DaemonSetAnalyzer{})
	registerAnalyzer("MutatingWebhookConfiguration", &MutatingWebhookAnalyzer{})
	registerAnalyzer("ValidatingWebhookConfiguration", &ValidatingWebhookAnalyzer{})
	registerAnalyzer("HorizontalPodAutoscaler", &HPAAnalyzer{})
	registerAnalyzer("PodDisruptionBudget", &PDBAnalyzer{})
	registerAnalyzer("NetworkPolicy", &NetworkPolicyAnalyzer{})
	registerAnalyzer("Security", &SecurityAnalyzer{})
	registerAnalyzer("Log", &LogAnalyzer{})
	// Gateway API
	registerAnalyzer("GatewayClass", &GatewayClassAnalyzer{})
	registerAnalyzer("Gateway", &GatewayAnalyzer{})
	registerAnalyzer("HTTPRoute", &HTTPRouteAnalyzer{})
	// OLM
	registerAnalyzer("CatalogSource", &CatalogSourceAnalyzer{})
	registerAnalyzer("Subscription", &SubscriptionAnalyzer{})
	registerAnalyzer("InstallPlan", &InstallPlanAnalyzer{})
	registerAnalyzer("ClusterServiceVersion", &CSVAnalyzer{})
	registerAnalyzer("ClusterExtension", &ClusterExtensionAnalyzer{})
	// Storage
	registerAnalyzer("StorageClass", &StorageClassAnalyzer{})
	// Traefik CRDs
	registerAnalyzer("IngressRoute", &IngressRouteAnalyzer{})
	registerAnalyzer("IngressRouteTCP", &IngressRouteTCPAnalyzer{})
	registerAnalyzer("IngressRouteUDP", &IngressRouteUDPAnalyzer{})
	registerAnalyzer("Middleware", &MiddlewareAnalyzer{})
	registerAnalyzer("MiddlewareTCP", &MiddlewareTCPAnalyzer{})
	registerAnalyzer("TraefikService", &TraefikServiceAnalyzer{})
	registerAnalyzer("TLSOption", &TLSOptionAnalyzer{})
	registerAnalyzer("TLSStore", &TLSStoreAnalyzer{})
	// Istio CRDs
	registerAnalyzer("VirtualService", &VirtualServiceAnalyzer{})
	registerAnalyzer("DestinationRule", &DestinationRuleAnalyzer{})
	registerAnalyzer("IstioGateway", &IstioGatewayAnalyzer{})
	registerAnalyzer("ServiceEntry", &ServiceEntryAnalyzer{})
	registerAnalyzer("Sidecar", &SidecarAnalyzer{})
	registerAnalyzer("PeerAuthentication", &PeerAuthenticationAnalyzer{})
	registerAnalyzer("AuthorizationPolicy", &AuthorizationPolicyAnalyzer{})
	// OLM v1
	registerAnalyzer("ClusterCatalog", &ClusterCatalogAnalyzer{})
	registerAnalyzer("OperatorGroup", &OperatorGroupAnalyzer{})
	// Ingress access log analysis
	registerAnalyzer("IngressAccessLog", &IngressAccessLogAnalyzer{})
	// Composite analyzers
	registerAnalyzer("NetworkComponentPods", &NetworkComponentPodsAnalyzer{})
	registerAnalyzer("WarningEvents", &WarningEventsAnalyzer{})
	registerAnalyzer("Secret", &SecretAnalyzer{})
	registerAnalyzer("PersistentVolume", &PVAnalyzer{})
	// Storage (comprehensive)
	registerAnalyzer("Storage", &StorageAnalyzer{})
}

// RunAnalysis executes analyzers concurrently and collects results.
func RunAnalysis(ctx context.Context, client *K8sClient, filters []string, namespace, labelSelector string) *AnalyzeResponse {
	if len(filters) == 0 {
		filters = defaultFilters
	}

	var (
		allResults []AnalyzeResult
		mu         sync.Mutex
		wg         sync.WaitGroup
	)

	// Concurrency limiter (max 10 goroutines)
	semaphore := make(chan struct{}, 10)

	for _, filter := range filters {
		analyzer, ok := analyzerRegistry[filter]
		if !ok {
			log.Printf("[Analyzer] Unknown filter: %s, skipping", filter)
			continue
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(a IAnalyzer, name string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			start := time.Now()
			results, err := a.Analyze(ctx, client, namespace, labelSelector)
			elapsed := time.Since(start)

			// Record per-analyzer metrics
			RecordAnalyzerRun(name, elapsed.Seconds(), err)

			if err != nil {
				log.Printf("[Analyzer] %s failed (%s): %v", name, elapsed, err)
				return
			}

			mu.Lock()
			allResults = append(allResults, results...)
			mu.Unlock()

			if len(results) > 0 {
				log.Printf("[Analyzer] %s found %d issues (%s)", name, len(results), elapsed)
			}
		}(analyzer, filter)
	}

	wg.Wait()

	// Deduplicate by kind+name
	allResults = deduplicateResults(allResults)

	// Enrich results with K8s doc references and mask sensitive data
	for i := range allResults {
		EnrichWithDocRef(&allResults[i])
		for j, e := range allResults[i].Error {
			allResults[i].Error[j] = MaskSensitiveValues(e)
		}
	}

	// Record metrics
	RecordAnalyzerErrors(allResults)

	status := "OK"
	if len(allResults) > 0 {
		status = "ProblemDetected"
	}

	return &AnalyzeResponse{
		Provider: "agent-native",
		Status:   status,
		Problems: len(allResults),
		Results:  allResults,
	}
}

// deduplicateResults removes duplicate results by kind+name.
func deduplicateResults(results []AnalyzeResult) []AnalyzeResult {
	seen := make(map[string]int) // key -> index in deduped
	var deduped []AnalyzeResult

	for _, r := range results {
		key := r.Kind + "/" + r.Name
		if idx, exists := seen[key]; exists {
			// Merge errors
			deduped[idx].Error = append(deduped[idx].Error, r.Error...)
		} else {
			seen[key] = len(deduped)
			deduped = append(deduped, r)
		}
	}
	return deduped
}

// ListAvailableFilters returns all registered analyzer names.
func ListAvailableFilters() []string {
	var filters []string
	for name := range analyzerRegistry {
		filters = append(filters, name)
	}
	return filters
}
