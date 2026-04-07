package k8sgpt

import (
	"aiops-backend/internal/cluster"
	"aiops-backend/internal/database"
	"aiops-backend/internal/llmclient"
	"aiops-backend/internal/model"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AnalyzeResult represents a single analysis result (k8sgpt-compatible).
type AnalyzeResult struct {
	Kind      string   `json:"kind"`
	Name      string   `json:"name"`
	Error     []string `json:"error"`
	Details   string   `json:"details"`
	ParentObj string   `json:"parentObject"`
}

// AnalyzeStats records per-analyzer execution time.
type AnalyzeStats struct {
	Analyzer string `json:"analyzer"`
	Duration string `json:"duration"`
}

// AnalyzeResponse represents the full analysis output.
type AnalyzeResponse struct {
	Provider string          `json:"provider"`
	Errors   int             `json:"errors"`
	Status   string          `json:"status"`
	Problems int             `json:"problems"`
	Results  []AnalyzeResult `json:"results"`
	Stats    []AnalyzeStats  `json:"stats,omitempty"`
	RawJSON  string          `json:"raw_json,omitempty"`
}

// defaultFilters mirrors k8sgpt's default enabled analyzers.
var defaultFilters = []string{
	"Pod", "Deployment", "Service", "Node", "StatefulSet",
	"ReplicaSet", "Ingress", "PersistentVolumeClaim",
	"CronJob", "Job", "ConfigMap", "DaemonSet",
}

// allFilters includes optional analyzers (standard K8s + Gateway API + Traefik CRDs + Istio CRDs + OLM + Network Diagnosis + Security + Storage).
var allFilters = []string{
	// Standard Kubernetes resources
	"Pod", "Deployment", "Service", "Node", "StatefulSet",
	"ReplicaSet", "Ingress", "PersistentVolumeClaim",
	"CronJob", "Job", "DaemonSet", "NetworkPolicy", "ConfigMap",
	"HPA", "Endpoints", "Secret", "PersistentVolume",
	"PodDisruptionBudget",
	"MutatingWebhookConfiguration", "ValidatingWebhookConfiguration",
	// Gateway API (k8sgpt additionalAnalyzerMap)
	"GatewayClass", "Gateway", "HTTPRoute",
	// Traefik CRDs
	"IngressRoute", "IngressRouteTCP", "IngressRouteUDP",
	"Middleware", "MiddlewareTCP",
	"TraefikService", "TLSOption", "TLSStore",
	// Istio CRDs
	"VirtualService", "DestinationRule", "IstioGateway",
	"ServiceEntry", "Sidecar", "PeerAuthentication", "AuthorizationPolicy",
	// OLM v1 (k8sgpt additionalAnalyzerMap)
	"ClusterCatalog", "ClusterExtension",
	// OLM v1alpha1 (operators.coreos.com)
	"ClusterServiceVersion", "Subscription", "CatalogSource", "OperatorGroup",
	// Network Diagnosis (composite analyzers)
	"NetworkComponentPods", "IngressAccessLog", "WarningEvents",
	// Security & Storage (composite analyzers — referenced from k8sgpt)
	"Log", "Security", "Storage",
}

// resourceMapping maps filter names to kubectl resource types.
var resourceMapping = map[string]string{
	// Standard Kubernetes resources
	"Pod":                   "pods",
	"Deployment":            "deployments",
	"Service":               "services",
	"Node":                  "nodes",
	"StatefulSet":           "statefulsets",
	"ReplicaSet":            "replicasets",
	"Ingress":               "ingresses",
	"PersistentVolumeClaim": "persistentvolumeclaims",
	"CronJob":               "cronjobs",
	"DaemonSet":             "daemonsets",
	"Job":                   "jobs",
	"NetworkPolicy":         "networkpolicies",
	"ConfigMap":             "configmaps",
	"HPA":                   "horizontalpodautoscalers.autoscaling",
	"Endpoints":             "endpoints",
	"Secret":                "secrets",
	"PersistentVolume":                    "persistentvolumes",
	"PodDisruptionBudget":                 "poddisruptionbudgets.policy",
	"MutatingWebhookConfiguration":        "mutatingwebhookconfigurations.admissionregistration.k8s.io",
	"ValidatingWebhookConfiguration":      "validatingwebhookconfigurations.admissionregistration.k8s.io",
	// Traefik CRDs
	"IngressRoute":     "ingressroutes.traefik.io",
	"IngressRouteTCP":  "ingressroutetcps.traefik.io",
	"IngressRouteUDP":  "ingressrouteudps.traefik.io",
	"Middleware":        "middlewares.traefik.io",
	"MiddlewareTCP":     "middlewaretcps.traefik.io",
	"TraefikService":    "traefikservices.traefik.io",
	"TLSOption":         "tlsoptions.traefik.io",
	"TLSStore":          "tlsstores.traefik.io",
	// Istio CRDs
	"VirtualService":      "virtualservices.networking.istio.io",
	"DestinationRule":     "destinationrules.networking.istio.io",
	"IstioGateway":        "gateways.networking.istio.io",
	"ServiceEntry":        "serviceentries.networking.istio.io",
	"Sidecar":             "sidecars.networking.istio.io",
	"PeerAuthentication":  "peerauthentications.security.istio.io",
	"AuthorizationPolicy": "authorizationpolicies.security.istio.io",
	// Gateway API CRDs
	"GatewayClass": "gatewayclasses.gateway.networking.k8s.io",
	"Gateway":      "gateways.gateway.networking.k8s.io",
	"HTTPRoute":    "httproutes.gateway.networking.k8s.io",
	// OLM v1 CRDs
	"ClusterCatalog":   "clustercatalogs.olm.operatorframework.io",
	"ClusterExtension": "clusterextensions.olm.operatorframework.io",
	// OLM v1alpha1 CRDs (operators.coreos.com)
	"ClusterServiceVersion": "clusterserviceversions.operators.coreos.com",
	"Subscription":          "subscriptions.operators.coreos.com",
	"CatalogSource":         "catalogsources.operators.coreos.com",
	"OperatorGroup":         "operatorgroups.operators.coreos.com",
}

// Failure error reasons list — referenced from k8sgpt official code.
var containerWaitingErrorReasons = map[string]bool{
	"CrashLoopBackOff":           true,
	"ImagePullBackOff":           true,
	"CreateContainerConfigError": true,
	"PreCreateHookError":         true,
	"CreateContainerError":       true,
	"PreStartHookError":          true,
	"RunContainerError":          true,
	"ImageInspectError":          true,
	"ErrImagePull":               true,
	"ErrImageNeverPull":          true,
	"InvalidImageName":           true,
}

// Executor handles built-in cluster analysis (replaces k8sgpt CLI).
type Executor struct {
	llmClient llmclient.LLMClient
}

func NewExecutor() *Executor {
	return &Executor{llmClient: llmclient.New()}
}

func getConfig() model.K8sGPTConfig {
	var config model.K8sGPTConfig
	database.DB.FirstOrCreate(&config)
	return config
}

// getLLMConfig returns the LLM config for AI explanations.
func (e *Executor) getLLMConfig() (*llmclient.LLMConfig, error) {
	config := getConfig()
	if config.UseBuiltinLLM {
		var llmConf model.LLMConfig
		if err := database.DB.Where("is_default = ?", true).First(&llmConf).Error; err != nil {
			return nil, fmt.Errorf("未配置默认 LLM，请先在 LLM 配置中设置默认模型")
		}
		return &llmclient.LLMConfig{
			APIURL: llmConf.APIURL, APIKey: llmConf.APIKey, ModelName: llmConf.ModelName,
		}, nil
	}
	if config.BaseURL == "" {
		return nil, fmt.Errorf("未配置 AI Backend URL")
	}
	return &llmclient.LLMConfig{
		APIURL: config.BaseURL, APIKey: "", ModelName: config.Model,
	}, nil
}

// kubectlGet executes kubectl get and returns parsed JSON items.
func kubectlGet(ctx context.Context, executor cluster.KubeExecutor, resource, namespace, labelSelector string) ([]json.RawMessage, error) {
	args := []string{"get", resource, "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else if resource != "nodes" && resource != "persistentvolumes" && resource != "storageclasses" {
		args = append(args, "--all-namespaces")
	}
	if labelSelector != "" {
		args = append(args, "-l", labelSelector)
	}
	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s failed: %s", resource, string(output))
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}
	return list.Items, nil
}

// kubectlGetSingle gets a single resource by name.
func kubectlGetSingle(ctx context.Context, executor cluster.KubeExecutor, resource, namespace, name string) (map[string]interface{}, error) {
	args := []string{"get", resource, name, "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("not found")
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(output, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

// Analyze runs cluster analysis using kubectl + LLM (k8sgpt-compatible).
func (e *Executor) Analyze(ctx context.Context, filters []string, namespace string, explain bool) (*AnalyzeResponse, error) {
	return e.AnalyzeWithOptions(ctx, filters, namespace, "", explain, false)
}

// AnalyzeWithOptions runs cluster analysis with full options (label selector, stats).
func (e *Executor) AnalyzeWithOptions(ctx context.Context, filters []string, namespace, labelSelector string, explain, withStats bool) (*AnalyzeResponse, error) {
	return e.AnalyzeWithCluster(ctx, 0, filters, namespace, labelSelector, explain, withStats)
}

// AnalyzeWithCluster runs cluster analysis targeting a specific cluster (0 = active cluster).
func (e *Executor) AnalyzeWithCluster(ctx context.Context, clusterID uint, filters []string, namespace, labelSelector string, explain, withStats bool) (*AnalyzeResponse, error) {
	var ac *model.ClusterConfig
	var err error
	if clusterID > 0 {
		var c model.ClusterConfig
		if err := database.DB.First(&c, clusterID).Error; err != nil {
			return nil, fmt.Errorf("cluster not found: %w", err)
		}
		ac = &c
	} else {
		ac, err = cluster.GetActiveCluster()
		if err != nil {
			return nil, fmt.Errorf("no active cluster: %w", err)
		}
	}

	if ac.ConnMode == "agent" {
		return e.analyzeViaAgent(ctx, ac, filters, namespace, labelSelector, explain)
	}

	// Direct mode: use kubectl-based analysis
	exec, err := cluster.GetExecutorForCluster(ac)
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}
	defer exec.Close()

	resourcesToCheck := filters
	if len(resourcesToCheck) == 0 {
		resourcesToCheck = defaultFilters
	}

	var allResults []AnalyzeResult
	var stats []AnalyzeStats
	for _, filter := range resourcesToCheck {
		start := time.Now()
		results, err := e.analyzeFilter(ctx, exec, filter, namespace, labelSelector)
		elapsed := time.Since(start)
		if withStats {
			stats = append(stats, AnalyzeStats{Analyzer: filter, Duration: elapsed.String()})
		}
		if err != nil {
			continue // skip failures on individual analyzers
		}
		allResults = append(allResults, results...)
	}

	// Deduplicate by kind+name
	allResults = deduplicateResults(allResults)

	// Anonymize sensitive data before sending to LLM
	config := getConfig()
	if config.Anonymize {
		anonymizeResults(allResults)
	}

	// Enrich with LLM explanations
	if explain && len(allResults) > 0 {
		lang := config.Language
		if lang == "" {
			lang = "chinese"
		}
		e.enrichWithLLM(ctx, allResults, lang)
	}

	return &AnalyzeResponse{
		Provider: "builtin",
		Status:   "completed",
		Problems: len(allResults),
		Results:  allResults,
		Stats:    stats,
	}, nil
}

// analyzeViaAgent sends an analyze request to a remote agent and returns the structured response.
func (e *Executor) analyzeViaAgent(ctx context.Context, ac *model.ClusterConfig, filters []string, namespace, labelSelector string, explain bool) (*AnalyzeResponse, error) {
	agentExec, err := cluster.NewAgentExecutor(ac.ID)
	if err != nil {
		return nil, fmt.Errorf("agent not available: %w", err)
	}
	defer agentExec.Close()

	// Send analyze request to agent via WebSocket
	respData, err := agentExec.ExecAnalyze(ctx, filters, namespace, labelSelector)
	if err != nil {
		return nil, fmt.Errorf("agent analysis failed: %w", err)
	}

	// Parse the structured response from agent
	var agentResp AnalyzeResponse
	if err := json.Unmarshal(respData, &agentResp); err != nil {
		return nil, fmt.Errorf("failed to parse agent analysis response: %w", err)
	}

	// Anonymize sensitive data if configured
	config := getConfig()
	if config.Anonymize {
		anonymizeResults(agentResp.Results)
	}

	// Enrich with LLM explanations (server-side LLM)
	if explain && len(agentResp.Results) > 0 {
		lang := config.Language
		if lang == "" {
			lang = "chinese"
		}
		e.enrichWithLLM(ctx, agentResp.Results, lang)
	}

	agentResp.Provider = "agent-native"
	agentResp.Status = "completed"

	return &agentResp, nil
}

// analyzeFilter dispatches to the appropriate analyzer based on filter name.
func (e *Executor) analyzeFilter(ctx context.Context, executor cluster.KubeExecutor, filter, namespace, labelSelector string) ([]AnalyzeResult, error) {
	// Composite analyzers (not simple kubectl get)
	switch filter {
	case "NetworkComponentPods":
		return e.analyzeNetworkComponentPods(ctx, executor, namespace)
	case "IngressAccessLog":
		return e.analyzeIngressAccessLog(ctx, executor, namespace)
	case "WarningEvents":
		return e.analyzeWarningEvents(ctx, executor, namespace)
	case "Log":
		return e.analyzeLog(ctx, executor, namespace)
	case "Security":
		return e.analyzeSecurity(ctx, executor, namespace)
	case "Storage":
		return e.analyzeStorage(ctx, executor, namespace)
	case "OperatorGroup":
		return e.analyzeOperatorGroups(ctx, executor, namespace)
	}

	resType, ok := resourceMapping[filter]
	if !ok {
		return nil, fmt.Errorf("unknown filter: %s", filter)
	}

	items, err := kubectlGet(ctx, executor, resType, namespace, labelSelector)
	if err != nil {
		return nil, err
	}

	var results []AnalyzeResult
	for _, item := range items {
		var resource map[string]interface{}
		if err := json.Unmarshal(item, &resource); err != nil {
			continue
		}
		problems := e.detectProblems(ctx, executor, filter, resource)
		results = append(results, problems...)
	}
	return results, nil
}

func getResourceFullName(metadata map[string]interface{}) string {
	name, _ := metadata["name"].(string)
	ns, _ := metadata["namespace"].(string)
	if ns != "" {
		return ns + "/" + name
	}
	return name
}

func getOwnerParent(metadata map[string]interface{}) string {
	if owners, ok := metadata["ownerReferences"].([]interface{}); ok && len(owners) > 0 {
		if owner, ok := owners[0].(map[string]interface{}); ok {
			return fmt.Sprintf("%s/%s", owner["kind"], owner["name"])
		}
	}
	return ""
}

// detectProblems identifies issues in a resource — logic referenced from k8sgpt official analyzers.
func (e *Executor) detectProblems(ctx context.Context, executor cluster.KubeExecutor, kind string, resource map[string]interface{}) []AnalyzeResult {
	metadata, _ := resource["metadata"].(map[string]interface{})
	fullName := getResourceFullName(metadata)

	var errors []string
	parentObj := ""

	switch kind {
	case "Pod":
		errors = analyzePod(resource)
		parentObj = getOwnerParent(metadata)
	case "Deployment":
		errors = analyzeDeployment(resource)
	case "Node":
		errors = analyzeNode(resource)
	case "Service":
		errors = e.analyzeService(ctx, executor, resource)
	case "PersistentVolumeClaim":
		errors = analyzePVC(resource)
	case "StatefulSet":
		errors = e.analyzeStatefulSet(ctx, executor, resource)
	case "ReplicaSet":
		errors = analyzeReplicaSet(resource)
		parentObj = getOwnerParent(metadata)
	case "Job":
		errors = analyzeJob(resource)
	case "CronJob":
		errors = analyzeCronJob(resource)
	case "DaemonSet":
		errors = analyzeDaemonSet(resource)
	case "Ingress":
		errors = e.analyzeIngress(ctx, executor, resource)
	case "ConfigMap":
		errors = analyzeConfigMap(resource)
	case "HPA":
		errors = analyzeHPA(resource)
	case "Endpoints":
		errors = analyzeEndpoints(resource)
	case "Secret":
		errors = analyzeSecret(resource)
	case "PersistentVolume":
		errors = analyzePV(resource)
	case "IngressRoute":
		errors = e.analyzeIngressRoute(ctx, executor, resource)
	case "IngressRouteTCP":
		errors = e.analyzeIngressRouteTCP(ctx, executor, resource)
	case "IngressRouteUDP":
		errors = e.analyzeIngressRouteUDP(ctx, executor, resource)
	case "Middleware":
		errors = analyzeMiddleware(resource)
	case "MiddlewareTCP":
		errors = analyzeMiddlewareTCP(resource)
	case "TraefikService":
		errors = e.analyzeTraefikService(ctx, executor, resource)
	case "TLSOption":
		errors = analyzeTLSOption(resource)
	case "TLSStore":
		errors = e.analyzeTLSStore(ctx, executor, resource)
	case "VirtualService":
		errors = e.analyzeVirtualService(ctx, executor, resource)
	case "DestinationRule":
		errors = analyzeDestinationRule(resource)
	case "IstioGateway":
		errors = e.analyzeIstioGateway(ctx, executor, resource)
	case "ServiceEntry":
		errors = analyzeServiceEntry(resource)
	case "Sidecar":
		errors = analyzeSidecar(resource)
	case "PeerAuthentication":
		errors = analyzePeerAuthentication(resource)
	case "AuthorizationPolicy":
		errors = analyzeAuthorizationPolicy(resource)
	case "PodDisruptionBudget":
		errors = analyzePDB(resource)
	case "NetworkPolicy":
		errors = e.analyzeNetworkPolicy(ctx, executor, resource)
	case "MutatingWebhookConfiguration":
		errors = e.analyzeMutatingWebhook(ctx, executor, resource)
	case "ValidatingWebhookConfiguration":
		errors = e.analyzeValidatingWebhook(ctx, executor, resource)
	// Gateway API analyzers (referenced from k8sgpt gateway.go, gatewayclass.go, httproute.go)
	case "GatewayClass":
		errors = analyzeGatewayClass(resource)
	case "Gateway":
		errors = e.analyzeGatewayAPI(ctx, executor, resource)
	case "HTTPRoute":
		errors = e.analyzeHTTPRoute(ctx, executor, resource)
	// OLM v1 analyzers (referenced from k8sgpt clustercatalog.go, clusterextension.go)
	case "ClusterCatalog":
		errors = analyzeClusterCatalog(resource)
	case "ClusterExtension":
		errors = analyzeClusterExtension(resource)
	// OLM v1alpha1 analyzers (referenced from k8sgpt clusterserviceversion.go, subscription.go, catalogsource.go)
	case "ClusterServiceVersion":
		errors = analyzeCSV(resource)
	case "Subscription":
		errors = analyzeSubscription(resource)
	case "CatalogSource":
		errors = analyzeCatalogSource(resource)
	}

	if len(errors) == 0 {
		return nil
	}
	return []AnalyzeResult{{Kind: kind, Name: fullName, Error: errors, ParentObj: parentObj}}
}

// ==================== Pod Analyzer (referenced from k8sgpt pod.go) ====================

func analyzePod(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	phase, _ := status["phase"].(string)

	// Check pending pods with conditions
	if phase == "Pending" {
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cond, _ := c.(map[string]interface{})
				condType, _ := cond["type"].(string)
				reason, _ := cond["reason"].(string)
				message, _ := cond["message"].(string)
				if condType == "PodScheduled" && reason == "Unschedulable" && message != "" {
					errs = append(errs, message)
				}
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "Pod is in Pending state")
		}
	}

	if phase == "Failed" || phase == "Unknown" {
		reason, _ := status["reason"].(string)
		msg := fmt.Sprintf("Pod is in %s state", phase)
		if reason != "" {
			msg += ": " + reason
		}
		errs = append(errs, msg)
	}

	// Check init container statuses (k8sgpt checks these too)
	if initStatuses, ok := status["initContainerStatuses"].([]interface{}); ok {
		errs = append(errs, analyzeContainerStatuses(initStatuses, phase)...)
	}

	// Check container statuses
	if containerStatuses, ok := status["containerStatuses"].([]interface{}); ok {
		errs = append(errs, analyzeContainerStatuses(containerStatuses, phase)...)
	}

	return errs
}

func analyzeContainerStatuses(statuses []interface{}, podPhase string) []string {
	var errs []string
	for _, cs := range statuses {
		csMap, _ := cs.(map[string]interface{})
		ready, _ := csMap["ready"].(bool)
		restartCount, _ := csMap["restartCount"].(float64)
		cName, _ := csMap["name"].(string)

		if waiting, ok := csMap["waiting"].(map[string]interface{}); ok {
			reason, _ := waiting["reason"].(string)
			message, _ := waiting["message"].(string)

			if reason == "CrashLoopBackOff" {
				// k8sgpt: report last termination reason
				if lastTerm, ok := csMap["lastTerminationState"].(map[string]interface{}); ok {
					if terminated, ok := lastTerm["terminated"].(map[string]interface{}); ok {
						termReason, _ := terminated["reason"].(string)
						errs = append(errs, fmt.Sprintf("the last termination reason is %s container=%s (restarts: %d)", termReason, cName, int(restartCount)))
						continue
					}
				}
				errs = append(errs, fmt.Sprintf("container %s is in CrashLoopBackOff (restarts: %d)", cName, int(restartCount)))
			} else if containerWaitingErrorReasons[reason] && message != "" {
				errs = append(errs, message)
			} else if reason == "ContainerCreating" && podPhase == "Pending" {
				// k8sgpt: would check events here; we note the state
				errs = append(errs, fmt.Sprintf("container %s is stuck in ContainerCreating", cName))
			}
		} else if terminated, ok := csMap["terminated"].(map[string]interface{}); ok {
			exitCode, _ := terminated["exitCode"].(float64)
			reason, _ := terminated["reason"].(string)
			if exitCode != 0 {
				if reason == "" {
					reason = "Unknown"
				}
				errs = append(errs, fmt.Sprintf("the termination reason is %s exitCode=%d container=%s", reason, int(exitCode), cName))
			}
		} else {
			// Running but not ready — k8sgpt checks for Unhealthy events
			if !ready && podPhase == "Running" {
				errs = append(errs, fmt.Sprintf("container %s is running but not ready (possible readiness probe failure)", cName))
			}
		}
	}
	return errs
}

// ==================== Deployment Analyzer (referenced from k8sgpt deployment.go) ====================

func analyzeDeployment(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	replicas, _ := spec["replicas"].(float64)
	readyReplicas, _ := status["readyReplicas"].(float64)
	statusReplicas, _ := status["replicas"].(float64)

	if replicas > 0 && readyReplicas != replicas {
		if statusReplicas > replicas {
			errs = append(errs, fmt.Sprintf("Deployment %s has %d replicas in spec but %d replicas in status (scaling in progress), %d ready",
				name, int(replicas), int(statusReplicas), int(readyReplicas)))
		} else {
			errs = append(errs, fmt.Sprintf("Deployment %s has %d replicas but %d are available with status running",
				name, int(replicas), int(readyReplicas)))
		}
	}
	return errs
}

// ==================== Node Analyzer (referenced from k8sgpt node.go) ====================

func analyzeNode(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	nodeName, _ := metadata["name"].(string)

	knownTypes := map[string]bool{
		"Ready": true, "MemoryPressure": true, "DiskPressure": true,
		"PIDPressure": true, "NetworkUnavailable": true,
	}

	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			cond, _ := c.(map[string]interface{})
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			reason, _ := cond["reason"].(string)
			message, _ := cond["message"].(string)

			// k3s EtcdIsVoter should be skipped
			if condType == "EtcdIsVoter" {
				continue
			}

			if condType == "Ready" {
				if condStatus != "True" {
					errs = append(errs, fmt.Sprintf("%s has condition of type %s, reason %s: %s", nodeName, condType, reason, message))
				}
			} else if knownTypes[condType] {
				if condStatus == "True" || condStatus == "Unknown" {
					errs = append(errs, fmt.Sprintf("%s has condition of type %s, reason %s: %s", nodeName, condType, reason, message))
				}
			} else {
				// Unknown condition type — report as potential issue
				if condStatus == "True" || condStatus == "Unknown" {
					errs = append(errs, fmt.Sprintf("%s has condition of type %s, reason %s: %s", nodeName, condType, reason, message))
				}
			}
		}
	}
	return errs
}

// ==================== Service Analyzer (referenced from k8sgpt service.go) ====================
// k8sgpt checks Endpoints, not just Service spec

func (e *Executor) analyzeService(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	ns, _ := metadata["namespace"].(string)
	spec, _ := resource["spec"].(map[string]interface{})
	svcType, _ := spec["type"].(string)

	if svcType == "ExternalName" {
		return nil
	}

	// Check endpoints (k8sgpt approach)
	ep, err := kubectlGetSingle(ctx, executor, "endpoints", ns, name)
	if err != nil {
		// No endpoints object — skip silently
		return nil
	}

	subsets, _ := ep["subsets"].([]interface{})
	if len(subsets) == 0 {
		// Empty endpoints — service has no matching pods
		if selector, ok := spec["selector"].(map[string]interface{}); ok {
			for k, v := range selector {
				errs = append(errs, fmt.Sprintf("Service has no endpoints, expected label %s=%s", k, v))
			}
		}
		if len(errs) == 0 {
			errs = append(errs, "Service has no endpoints")
		}
	} else {
		// Check for NotReady addresses
		notReadyCount := 0
		for _, subset := range subsets {
			s, _ := subset.(map[string]interface{})
			if notReady, ok := s["notReadyAddresses"].([]interface{}); ok {
				notReadyCount += len(notReady)
			}
		}
		if notReadyCount > 0 {
			errs = append(errs, fmt.Sprintf("Service has %d not ready endpoints", notReadyCount))
		}
	}

	return errs
}

// ==================== PVC Analyzer (referenced from k8sgpt pvc.go) ====================

func analyzePVC(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	phase, _ := status["phase"].(string)

	if phase == "Pending" {
		errs = append(errs, "PersistentVolumeClaim is in Pending state (possible provisioning failure or no matching PV)")
	} else if phase == "Lost" {
		errs = append(errs, "PersistentVolumeClaim is in Lost state, bound PV may be deleted")
	}
	return errs
}

// ==================== StatefulSet Analyzer (referenced from k8sgpt statefulset.go) ====================

func (e *Executor) analyzeStatefulSet(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)

	// Check headless service existence (k8sgpt check)
	serviceName, _ := spec["serviceName"].(string)
	if serviceName != "" {
		if _, err := kubectlGetSingle(ctx, executor, "service", ns, serviceName); err != nil {
			errs = append(errs, fmt.Sprintf("StatefulSet uses the service %s/%s which does not exist", ns, serviceName))
		}
	}

	// Check StorageClass in volumeClaimTemplates (k8sgpt check)
	if vcts, ok := spec["volumeClaimTemplates"].([]interface{}); ok {
		for _, vct := range vcts {
			vctMap, _ := vct.(map[string]interface{})
			vctSpec, _ := vctMap["spec"].(map[string]interface{})
			if scName, ok := vctSpec["storageClassName"].(string); ok && scName != "" {
				if _, err := kubectlGetSingle(ctx, executor, "storageclass", "", scName); err != nil {
					errs = append(errs, fmt.Sprintf("StatefulSet uses the storage class %s which does not exist", scName))
				}
			}
		}
	}

	// Check replica availability
	replicas, _ := spec["replicas"].(float64)
	availableReplicas, _ := status["availableReplicas"].(float64)
	if replicas > 0 && availableReplicas != replicas {
		errs = append(errs, fmt.Sprintf("StatefulSet has %d replicas but only %d available", int(replicas), int(availableReplicas)))
	}

	return errs
}

// ==================== ReplicaSet Analyzer (referenced from k8sgpt rs.go) ====================

func analyzeReplicaSet(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})

	statusReplicas, _ := status["replicas"].(float64)
	if statusReplicas == 0 {
		// Check for ReplicaFailure condition (k8sgpt approach)
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cond, _ := c.(map[string]interface{})
				condType, _ := cond["type"].(string)
				reason, _ := cond["reason"].(string)
				message, _ := cond["message"].(string)
				if condType == "ReplicaFailure" && reason == "FailedCreate" && message != "" {
					errs = append(errs, message)
				}
			}
		}
	}
	return errs
}

// ==================== Job Analyzer (referenced from k8sgpt job.go) ====================

func analyzeJob(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	// Check suspended
	if suspend, ok := spec["suspend"].(bool); ok && suspend {
		errs = append(errs, fmt.Sprintf("Job %s is suspended", name))
	}

	// Check failed
	failed, _ := status["failed"].(float64)
	if failed > 0 {
		errs = append(errs, fmt.Sprintf("Job %s has failed", name))
	}

	return errs
}

// ==================== CronJob Analyzer (referenced from k8sgpt cronjob.go) ====================

func analyzeCronJob(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	// Check suspended
	if suspend, ok := spec["suspend"].(bool); ok && suspend {
		errs = append(errs, fmt.Sprintf("CronJob %s is suspended", name))
		return errs
	}

	// Check schedule format validity
	schedule, _ := spec["schedule"].(string)
	if schedule != "" {
		if !isValidCronSchedule(schedule) {
			errs = append(errs, fmt.Sprintf("CronJob %s has an invalid schedule: %s", name, schedule))
		}
	}

	// Check negative starting deadline
	if deadline, ok := spec["startingDeadlineSeconds"].(float64); ok && deadline < 0 {
		errs = append(errs, fmt.Sprintf("CronJob %s has a negative starting deadline", name))
	}

	return errs
}

func isValidCronSchedule(schedule string) bool {
	// Basic validation: should have 5 fields (minute hour day month weekday)
	fields := strings.Fields(schedule)
	return len(fields) == 5 || len(fields) == 6
}

// ==================== DaemonSet Analyzer ====================

func analyzeDaemonSet(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})

	desired, _ := status["desiredNumberScheduled"].(float64)
	ready, _ := status["numberReady"].(float64)
	misscheduled, _ := status["numberMisscheduled"].(float64)

	if desired > 0 && ready < desired {
		errs = append(errs, fmt.Sprintf("DaemonSet has %d desired but only %d ready", int(desired), int(ready)))
	}
	if misscheduled > 0 {
		errs = append(errs, fmt.Sprintf("DaemonSet has %d misscheduled pods", int(misscheduled)))
	}
	return errs
}

// ==================== Ingress Analyzer (referenced from k8sgpt ingress.go) ====================

func (e *Executor) analyzeIngress(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)
	annotations, _ := metadata["annotations"].(map[string]interface{})

	// Check IngressClass (k8sgpt check)
	var ingressClassName string
	if icn, ok := spec["ingressClassName"].(string); ok {
		ingressClassName = icn
	} else if annotations != nil {
		if icn, ok := annotations["kubernetes.io/ingress.class"].(string); ok {
			ingressClassName = icn
		}
	}
	if ingressClassName == "" {
		errs = append(errs, fmt.Sprintf("Ingress %s does not specify an Ingress class", name))
	} else {
		// Verify IngressClass exists
		if _, err := kubectlGetSingle(ctx, executor, "ingressclass", "", ingressClassName); err != nil {
			errs = append(errs, fmt.Sprintf("Ingress uses the ingress class %s which does not exist", ingressClassName))
		}
	}

	// Check backend services exist (k8sgpt check)
	if rules, ok := spec["rules"].([]interface{}); ok {
		for _, rule := range rules {
			ruleMap, _ := rule.(map[string]interface{})
			if http, ok := ruleMap["http"].(map[string]interface{}); ok {
				if paths, ok := http["paths"].([]interface{}); ok {
					for _, path := range paths {
						pathMap, _ := path.(map[string]interface{})
						if backend, ok := pathMap["backend"].(map[string]interface{}); ok {
							if svc, ok := backend["service"].(map[string]interface{}); ok {
								svcName, _ := svc["name"].(string)
								if svcName != "" {
									if _, err := kubectlGetSingle(ctx, executor, "service", ns, svcName); err != nil {
										errs = append(errs, fmt.Sprintf("Ingress uses the service %s/%s which does not exist", ns, svcName))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Check TLS secrets exist (k8sgpt check)
	if tlsList, ok := spec["tls"].([]interface{}); ok {
		for _, tls := range tlsList {
			tlsMap, _ := tls.(map[string]interface{})
			secretName, _ := tlsMap["secretName"].(string)
			if secretName != "" {
				if _, err := kubectlGetSingle(ctx, executor, "secret", ns, secretName); err != nil {
					errs = append(errs, fmt.Sprintf("Ingress uses the secret %s/%s as a TLS certificate which does not exist", ns, secretName))
				}
			}
		}
	}

	return errs
}

// ==================== ConfigMap Analyzer (referenced from k8sgpt configmap.go) ====================

func analyzeConfigMap(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)

	// Skip system configmaps
	if strings.HasPrefix(name, "kube-") || name == "extension-apiserver-authentication" {
		return nil
	}

	data, _ := resource["data"].(map[string]interface{})
	binaryData, _ := resource["binaryData"].(map[string]interface{})

	// Check for empty configmaps
	if len(data) == 0 && len(binaryData) == 0 {
		errs = append(errs, fmt.Sprintf("ConfigMap %s is empty", name))
	}

	// Check for oversized (> 1MB)
	totalSize := 0
	for _, v := range data {
		if s, ok := v.(string); ok {
			totalSize += len(s)
		}
	}
	if totalSize > 1024*1024 {
		errs = append(errs, fmt.Sprintf("ConfigMap %s is larger than 1MB (%d bytes)", name, totalSize))
	}

	return errs
}

// ==================== Helpers ====================

// ListNamespaces returns all namespaces in the active cluster.
func (e *Executor) ListNamespaces(ctx context.Context) ([]string, error) {
	return e.ListNamespacesForCluster(ctx, 0)
}

// ListNamespacesForCluster returns namespaces for a specific cluster (0 = active cluster).
func (e *Executor) ListNamespacesForCluster(ctx context.Context, clusterID uint) ([]string, error) {
	var exec *cluster.CloseableExecutor
	var err error
	if clusterID > 0 {
		var c model.ClusterConfig
		if err := database.DB.First(&c, clusterID).Error; err != nil {
			return nil, fmt.Errorf("cluster not found: %w", err)
		}
		exec, err = cluster.GetExecutorForCluster(&c)
	} else {
		exec, err = cluster.GetActiveExecutor()
	}
	if err != nil {
		return nil, fmt.Errorf("no active cluster: %w", err)
	}
	defer exec.Close()

	output, err := exec.ExecKubectl(ctx, []string{"get", "namespaces", "-o", "json"})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %s", string(output))
	}
	var list struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}
	var namespaces []string
	for _, item := range list.Items {
		namespaces = append(namespaces, item.Metadata.Name)
	}
	return namespaces, nil
}

// ==================== HPA Analyzer ====================

func analyzeHPA(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	currentReplicas, _ := status["currentReplicas"].(float64)
	desiredReplicas, _ := status["desiredReplicas"].(float64)
	maxReplicas, _ := spec["maxReplicas"].(float64)
	minReplicas := float64(1)
	if m, ok := spec["minReplicas"].(float64); ok {
		minReplicas = m
	}

	// Check if at max
	if currentReplicas >= maxReplicas {
		errs = append(errs, fmt.Sprintf("HPA %s is at max replicas (%d/%d), may need to increase maxReplicas", name, int(currentReplicas), int(maxReplicas)))
	}

	// Check if desired > current (scaling issues)
	if desiredReplicas > 0 && currentReplicas < desiredReplicas {
		errs = append(errs, fmt.Sprintf("HPA %s desired %d replicas but only %d are current", name, int(desiredReplicas), int(currentReplicas)))
	}

	// Check conditions
	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			cond, _ := c.(map[string]interface{})
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			message, _ := cond["message"].(string)
			if condType == "ScalingLimited" && condStatus == "True" {
				errs = append(errs, fmt.Sprintf("HPA %s scaling limited: %s", name, message))
			}
			if condType == "AbleToScale" && condStatus == "False" {
				errs = append(errs, fmt.Sprintf("HPA %s unable to scale: %s", name, message))
			}
		}
	}

	_ = minReplicas
	return errs
}

// ==================== Endpoints Analyzer ====================

func analyzeEndpoints(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	subsets, _ := resource["subsets"].([]interface{})
	if len(subsets) == 0 {
		errs = append(errs, fmt.Sprintf("Endpoints %s has no subsets (no backing pods)", name))
		return errs
	}

	notReadyTotal := 0
	readyTotal := 0
	for _, subset := range subsets {
		s, _ := subset.(map[string]interface{})
		if addrs, ok := s["addresses"].([]interface{}); ok {
			readyTotal += len(addrs)
		}
		if notReady, ok := s["notReadyAddresses"].([]interface{}); ok {
			notReadyTotal += len(notReady)
		}
	}
	if readyTotal == 0 && notReadyTotal > 0 {
		errs = append(errs, fmt.Sprintf("Endpoints %s has %d not-ready addresses and 0 ready", name, notReadyTotal))
	}
	return errs
}

// ==================== Secret Analyzer ====================

func analyzeSecret(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	ns, _ := metadata["namespace"].(string)
	secretType, _ := resource["type"].(string)

	// Skip system secrets
	if strings.HasPrefix(name, "default-token-") || strings.HasPrefix(name, "sh.helm.") {
		return nil
	}

	// Check TLS secrets for required fields
	if secretType == "kubernetes.io/tls" {
		data, _ := resource["data"].(map[string]interface{})
		if data != nil {
			if _, ok := data["tls.crt"]; !ok {
				errs = append(errs, fmt.Sprintf("TLS Secret %s/%s missing tls.crt", ns, name))
			}
			if _, ok := data["tls.key"]; !ok {
				errs = append(errs, fmt.Sprintf("TLS Secret %s/%s missing tls.key", ns, name))
			}
		}
	}

	return errs
}

// ==================== PersistentVolume Analyzer ====================

func analyzePV(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	phase, _ := status["phase"].(string)

	if phase == "Failed" {
		reason, _ := status["reason"].(string)
		msg := fmt.Sprintf("PersistentVolume %s is in Failed state", name)
		if reason != "" {
			msg += ": " + reason
		}
		errs = append(errs, msg)
	} else if phase == "Released" {
		errs = append(errs, fmt.Sprintf("PersistentVolume %s is Released but not reclaimed", name))
	}
	return errs
}

// ==================== Traefik IngressRoute Analyzer ====================

func (e *Executor) analyzeIngressRoute(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	// Check entryPoints
	entryPoints, _ := spec["entryPoints"].([]interface{})
	if len(entryPoints) == 0 {
		errs = append(errs, fmt.Sprintf("IngressRoute %s has no entryPoints defined", name))
	}

	// Check routes
	routes, _ := spec["routes"].([]interface{})
	if len(routes) == 0 {
		errs = append(errs, fmt.Sprintf("IngressRoute %s has no routes defined", name))
		return errs
	}

	for i, route := range routes {
		routeMap, _ := route.(map[string]interface{})
		match, _ := routeMap["match"].(string)
		if match == "" {
			errs = append(errs, fmt.Sprintf("IngressRoute %s route[%d] has no match rule", name, i))
		}

		// Check services in the route
		services, _ := routeMap["services"].([]interface{})
		for _, svc := range services {
			svcMap, _ := svc.(map[string]interface{})
			svcName, _ := svcMap["name"].(string)
			svcKind, _ := svcMap["kind"].(string)

			if svcName == "" {
				continue
			}

			// TraefikService kind references traefik service CRDs
			if svcKind == "TraefikService" {
				if _, err := kubectlGetSingle(ctx, executor, "traefikservices.traefik.io", ns, svcName); err != nil {
					errs = append(errs, fmt.Sprintf("IngressRoute %s references TraefikService %s/%s which does not exist", name, ns, svcName))
				}
			} else {
				// Default: Kubernetes Service
				if _, err := kubectlGetSingle(ctx, executor, "service", ns, svcName); err != nil {
					errs = append(errs, fmt.Sprintf("IngressRoute %s references Service %s/%s which does not exist", name, ns, svcName))
				}
			}
		}

		// Check middlewares in the route
		middlewares, _ := routeMap["middlewares"].([]interface{})
		for _, mw := range middlewares {
			mwMap, _ := mw.(map[string]interface{})
			mwName, _ := mwMap["name"].(string)
			if mwName == "" {
				continue
			}
			// Middleware can be namespace-qualified: ns-mwname@kubernetescrd
			mwNameClean := mwName
			if idx := strings.Index(mwName, "@"); idx > 0 {
				mwNameClean = mwName[:idx]
			}
			mwNs := ns
			if mwNamespace, ok := mwMap["namespace"].(string); ok && mwNamespace != "" {
				mwNs = mwNamespace
			}
			if _, err := kubectlGetSingle(ctx, executor, "middlewares.traefik.io", mwNs, mwNameClean); err != nil {
				errs = append(errs, fmt.Sprintf("IngressRoute %s references Middleware %s/%s which does not exist", name, mwNs, mwNameClean))
			}
		}
	}

	// Check TLS secret if defined
	if tls, ok := spec["tls"].(map[string]interface{}); ok {
		if secretName, ok := tls["secretName"].(string); ok && secretName != "" {
			if _, err := kubectlGetSingle(ctx, executor, "secret", ns, secretName); err != nil {
				errs = append(errs, fmt.Sprintf("IngressRoute %s references TLS secret %s/%s which does not exist", name, ns, secretName))
			}
		}
	}

	return errs
}

// ==================== Traefik IngressRouteTCP Analyzer ====================

func (e *Executor) analyzeIngressRouteTCP(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	entryPoints, _ := spec["entryPoints"].([]interface{})
	if len(entryPoints) == 0 {
		errs = append(errs, fmt.Sprintf("IngressRouteTCP %s has no entryPoints defined", name))
	}

	routes, _ := spec["routes"].([]interface{})
	if len(routes) == 0 {
		errs = append(errs, fmt.Sprintf("IngressRouteTCP %s has no routes defined", name))
		return errs
	}

	for _, route := range routes {
		routeMap, _ := route.(map[string]interface{})
		services, _ := routeMap["services"].([]interface{})
		for _, svc := range services {
			svcMap, _ := svc.(map[string]interface{})
			svcName, _ := svcMap["name"].(string)
			if svcName != "" {
				if _, err := kubectlGetSingle(ctx, executor, "service", ns, svcName); err != nil {
					errs = append(errs, fmt.Sprintf("IngressRouteTCP %s references Service %s/%s which does not exist", name, ns, svcName))
				}
			}
		}
	}

	if tls, ok := spec["tls"].(map[string]interface{}); ok {
		if secretName, ok := tls["secretName"].(string); ok && secretName != "" {
			if _, err := kubectlGetSingle(ctx, executor, "secret", ns, secretName); err != nil {
				errs = append(errs, fmt.Sprintf("IngressRouteTCP %s references TLS secret %s/%s which does not exist", name, ns, secretName))
			}
		}
	}

	return errs
}

// ==================== Traefik IngressRouteUDP Analyzer ====================

func (e *Executor) analyzeIngressRouteUDP(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	entryPoints, _ := spec["entryPoints"].([]interface{})
	if len(entryPoints) == 0 {
		errs = append(errs, fmt.Sprintf("IngressRouteUDP %s has no entryPoints defined", name))
	}

	routes, _ := spec["routes"].([]interface{})
	for _, route := range routes {
		routeMap, _ := route.(map[string]interface{})
		services, _ := routeMap["services"].([]interface{})
		for _, svc := range services {
			svcMap, _ := svc.(map[string]interface{})
			svcName, _ := svcMap["name"].(string)
			if svcName != "" {
				if _, err := kubectlGetSingle(ctx, executor, "service", ns, svcName); err != nil {
					errs = append(errs, fmt.Sprintf("IngressRouteUDP %s references Service %s/%s which does not exist", name, ns, svcName))
				}
			}
		}
	}

	return errs
}

// ==================== Traefik Middleware Analyzer ====================

func analyzeMiddleware(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	if len(spec) == 0 {
		errs = append(errs, fmt.Sprintf("Middleware %s has empty spec (no middleware type configured)", name))
		return errs
	}

	// Check common middleware configs for issues
	if rateLimit, ok := spec["rateLimit"].(map[string]interface{}); ok {
		avg, _ := rateLimit["average"].(float64)
		if avg <= 0 {
			errs = append(errs, fmt.Sprintf("Middleware %s rateLimit has invalid average: %v", name, avg))
		}
	}

	if retry, ok := spec["retry"].(map[string]interface{}); ok {
		attempts, _ := retry["attempts"].(float64)
		if attempts <= 0 {
			errs = append(errs, fmt.Sprintf("Middleware %s retry has invalid attempts: %v", name, attempts))
		}
	}

	if circuitBreaker, ok := spec["circuitBreaker"].(map[string]interface{}); ok {
		expr, _ := circuitBreaker["expression"].(string)
		if expr == "" {
			errs = append(errs, fmt.Sprintf("Middleware %s circuitBreaker has no expression defined", name))
		}
	}

	if ipWhiteList, ok := spec["ipWhiteList"].(map[string]interface{}); ok {
		sourceRange, _ := ipWhiteList["sourceRange"].([]interface{})
		if len(sourceRange) == 0 {
			errs = append(errs, fmt.Sprintf("Middleware %s ipWhiteList has empty sourceRange", name))
		}
	}

	// Also check ipAllowList (newer Traefik v3 naming)
	if ipAllowList, ok := spec["ipAllowList"].(map[string]interface{}); ok {
		sourceRange, _ := ipAllowList["sourceRange"].([]interface{})
		if len(sourceRange) == 0 {
			errs = append(errs, fmt.Sprintf("Middleware %s ipAllowList has empty sourceRange", name))
		}
	}

	if headers, ok := spec["headers"].(map[string]interface{}); ok {
		// Check for potentially insecure CORS config
		if accessControl, ok := headers["accessControlAllowOriginList"].([]interface{}); ok {
			for _, origin := range accessControl {
				if o, ok := origin.(string); ok && o == "*" {
					errs = append(errs, fmt.Sprintf("Middleware %s allows all CORS origins (*) which may be insecure", name))
				}
			}
		}
	}

	return errs
}

// ==================== Traefik MiddlewareTCP Analyzer ====================

func analyzeMiddlewareTCP(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	if len(spec) == 0 {
		errs = append(errs, fmt.Sprintf("MiddlewareTCP %s has empty spec", name))
	}
	return errs
}

// ==================== Traefik TraefikService Analyzer ====================

func (e *Executor) analyzeTraefikService(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	// Check weighted services
	if weighted, ok := spec["weighted"].(map[string]interface{}); ok {
		services, _ := weighted["services"].([]interface{})
		if len(services) == 0 {
			errs = append(errs, fmt.Sprintf("TraefikService %s weighted has no backend services", name))
		}
		totalWeight := 0
		for _, svc := range services {
			svcMap, _ := svc.(map[string]interface{})
			svcName, _ := svcMap["name"].(string)
			weight, _ := svcMap["weight"].(float64)
			totalWeight += int(weight)
			if svcName != "" {
				kind, _ := svcMap["kind"].(string)
				if kind != "TraefikService" {
					if _, err := kubectlGetSingle(ctx, executor, "service", ns, svcName); err != nil {
						errs = append(errs, fmt.Sprintf("TraefikService %s references Service %s/%s which does not exist", name, ns, svcName))
					}
				}
			}
		}
	}

	// Check mirroring
	if mirroring, ok := spec["mirroring"].(map[string]interface{}); ok {
		mirrorName, _ := mirroring["name"].(string)
		if mirrorName != "" {
			if _, err := kubectlGetSingle(ctx, executor, "service", ns, mirrorName); err != nil {
				errs = append(errs, fmt.Sprintf("TraefikService %s mirroring references Service %s/%s which does not exist", name, ns, mirrorName))
			}
		}
	}

	return errs
}

// ==================== Traefik TLSOption Analyzer ====================

func analyzeTLSOption(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	minVersion, _ := spec["minVersion"].(string)
	if minVersion != "" {
		validVersions := map[string]bool{"VersionTLS10": true, "VersionTLS11": true, "VersionTLS12": true, "VersionTLS13": true}
		if !validVersions[minVersion] {
			errs = append(errs, fmt.Sprintf("TLSOption %s has invalid minVersion: %s", name, minVersion))
		}
		if minVersion == "VersionTLS10" || minVersion == "VersionTLS11" {
			errs = append(errs, fmt.Sprintf("TLSOption %s uses deprecated TLS version: %s", name, minVersion))
		}
	}

	return errs
}

// ==================== Traefik TLSStore Analyzer ====================

func (e *Executor) analyzeTLSStore(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	if defaultCert, ok := spec["defaultCertificate"].(map[string]interface{}); ok {
		secretName, _ := defaultCert["secretName"].(string)
		if secretName != "" {
			if _, err := kubectlGetSingle(ctx, executor, "secret", ns, secretName); err != nil {
				errs = append(errs, fmt.Sprintf("TLSStore %s references default certificate secret %s/%s which does not exist", name, ns, secretName))
			}
		}
	}

	return errs
}

// ==================== Istio VirtualService Analyzer ====================

func (e *Executor) analyzeVirtualService(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	// Check hosts
	hosts, _ := spec["hosts"].([]interface{})
	if len(hosts) == 0 {
		errs = append(errs, fmt.Sprintf("VirtualService %s has no hosts defined", name))
	}

	// Check gateways reference
	gateways, _ := spec["gateways"].([]interface{})
	for _, gw := range gateways {
		gwStr, _ := gw.(string)
		if gwStr == "" || gwStr == "mesh" {
			continue
		}
		// Gateway can be namespace/name format
		gwNs := ns
		gwName := gwStr
		if parts := strings.SplitN(gwStr, "/", 2); len(parts) == 2 {
			gwNs = parts[0]
			gwName = parts[1]
		}
		if _, err := kubectlGetSingle(ctx, executor, "gateways.networking.istio.io", gwNs, gwName); err != nil {
			errs = append(errs, fmt.Sprintf("VirtualService %s references Gateway %s/%s which does not exist", name, gwNs, gwName))
		}
	}

	// Check HTTP routes destination services
	if httpRoutes, ok := spec["http"].([]interface{}); ok {
		for i, route := range httpRoutes {
			routeMap, _ := route.(map[string]interface{})
			if dests, ok := routeMap["route"].([]interface{}); ok {
				for _, dest := range dests {
					destMap, _ := dest.(map[string]interface{})
					if destination, ok := destMap["destination"].(map[string]interface{}); ok {
						host, _ := destination["host"].(string)
						if host != "" && !strings.Contains(host, ".") {
							// Short name — check if service exists in same namespace
							if _, err := kubectlGetSingle(ctx, executor, "service", ns, host); err != nil {
								errs = append(errs, fmt.Sprintf("VirtualService %s http[%d] destination host %s not found as service in %s", name, i, host, ns))
							}
						}
					}
				}
			}
		}
	}

	// Check TCP routes
	if tcpRoutes, ok := spec["tcp"].([]interface{}); ok {
		for i, route := range tcpRoutes {
			routeMap, _ := route.(map[string]interface{})
			if dests, ok := routeMap["route"].([]interface{}); ok {
				for _, dest := range dests {
					destMap, _ := dest.(map[string]interface{})
					if destination, ok := destMap["destination"].(map[string]interface{}); ok {
						host, _ := destination["host"].(string)
						if host != "" && !strings.Contains(host, ".") {
							if _, err := kubectlGetSingle(ctx, executor, "service", ns, host); err != nil {
								errs = append(errs, fmt.Sprintf("VirtualService %s tcp[%d] destination host %s not found as service in %s", name, i, host, ns))
							}
						}
					}
				}
			}
		}
	}

	return errs
}

// ==================== Istio DestinationRule Analyzer ====================

func analyzeDestinationRule(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	host, _ := spec["host"].(string)
	if host == "" {
		errs = append(errs, fmt.Sprintf("DestinationRule %s has no host defined", name))
	}

	// Check subsets for empty labels
	if subsets, ok := spec["subsets"].([]interface{}); ok {
		for _, subset := range subsets {
			subsetMap, _ := subset.(map[string]interface{})
			subsetName, _ := subsetMap["name"].(string)
			if subsetName == "" {
				errs = append(errs, fmt.Sprintf("DestinationRule %s has a subset without name", name))
			}
			labels, _ := subsetMap["labels"].(map[string]interface{})
			if len(labels) == 0 {
				errs = append(errs, fmt.Sprintf("DestinationRule %s subset %s has no labels defined", name, subsetName))
			}
		}
	}

	// Check trafficPolicy outlierDetection
	if tp, ok := spec["trafficPolicy"].(map[string]interface{}); ok {
		if tls, ok := tp["tls"].(map[string]interface{}); ok {
			mode, _ := tls["mode"].(string)
			validModes := map[string]bool{"DISABLE": true, "SIMPLE": true, "MUTUAL": true, "ISTIO_MUTUAL": true}
			if mode != "" && !validModes[mode] {
				errs = append(errs, fmt.Sprintf("DestinationRule %s has invalid TLS mode: %s", name, mode))
			}
			if mode == "MUTUAL" {
				if _, ok := tls["clientCertificate"]; !ok {
					errs = append(errs, fmt.Sprintf("DestinationRule %s MUTUAL TLS mode requires clientCertificate", name))
				}
				if _, ok := tls["privateKey"]; !ok {
					errs = append(errs, fmt.Sprintf("DestinationRule %s MUTUAL TLS mode requires privateKey", name))
				}
			}
		}
	}

	return errs
}

// ==================== Istio Gateway Analyzer ====================

func (e *Executor) analyzeIstioGateway(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)

	servers, _ := spec["servers"].([]interface{})
	if len(servers) == 0 {
		errs = append(errs, fmt.Sprintf("Gateway %s has no servers defined", name))
		return errs
	}

	for i, server := range servers {
		serverMap, _ := server.(map[string]interface{})
		port, _ := serverMap["port"].(map[string]interface{})
		if port == nil {
			errs = append(errs, fmt.Sprintf("Gateway %s server[%d] has no port defined", name, i))
		}

		hosts, _ := serverMap["hosts"].([]interface{})
		if len(hosts) == 0 {
			errs = append(errs, fmt.Sprintf("Gateway %s server[%d] has no hosts defined", name, i))
		}

		// Check TLS config for HTTPS
		if tls, ok := serverMap["tls"].(map[string]interface{}); ok {
			mode, _ := tls["mode"].(string)
			if mode == "SIMPLE" || mode == "MUTUAL" {
				credName, _ := tls["credentialName"].(string)
				if credName != "" {
					// Check if the secret exists in istio-system (default) or same ns
					if _, err := kubectlGetSingle(ctx, executor, "secret", "istio-system", credName); err != nil {
						if _, err2 := kubectlGetSingle(ctx, executor, "secret", ns, credName); err2 != nil {
							errs = append(errs, fmt.Sprintf("Gateway %s server[%d] TLS credentialName %s secret not found", name, i, credName))
						}
					}
				}
			}
		}
	}

	// Check selector
	selector, _ := spec["selector"].(map[string]interface{})
	if len(selector) == 0 {
		errs = append(errs, fmt.Sprintf("Gateway %s has no selector (cannot bind to ingress gateway)", name))
	}

	return errs
}

// ==================== Istio ServiceEntry Analyzer ====================

func analyzeServiceEntry(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	hosts, _ := spec["hosts"].([]interface{})
	if len(hosts) == 0 {
		errs = append(errs, fmt.Sprintf("ServiceEntry %s has no hosts defined", name))
	}

	resolution, _ := spec["resolution"].(string)
	location, _ := spec["location"].(string)
	validResolutions := map[string]bool{"NONE": true, "STATIC": true, "DNS": true, "DNS_ROUND_ROBIN": true}
	if resolution != "" && !validResolutions[resolution] {
		errs = append(errs, fmt.Sprintf("ServiceEntry %s has invalid resolution: %s", name, resolution))
	}

	// STATIC resolution needs endpoints
	if resolution == "STATIC" {
		endpoints, _ := spec["endpoints"].([]interface{})
		if len(endpoints) == 0 {
			errs = append(errs, fmt.Sprintf("ServiceEntry %s uses STATIC resolution but has no endpoints", name))
		}
	}

	_ = location
	return errs
}

// ==================== Istio Sidecar Analyzer ====================

func analyzeSidecar(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	// Check egress
	egress, _ := spec["egress"].([]interface{})
	if len(egress) == 0 {
		errs = append(errs, fmt.Sprintf("Sidecar %s has no egress configuration", name))
	}

	return errs
}

// ==================== Istio PeerAuthentication Analyzer ====================

func analyzePeerAuthentication(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	mtls, _ := spec["mtls"].(map[string]interface{})
	if mtls != nil {
		mode, _ := mtls["mode"].(string)
		validModes := map[string]bool{"UNSET": true, "DISABLE": true, "PERMISSIVE": true, "STRICT": true}
		if mode != "" && !validModes[mode] {
			errs = append(errs, fmt.Sprintf("PeerAuthentication %s has invalid mTLS mode: %s", name, mode))
		}
		if mode == "DISABLE" {
			errs = append(errs, fmt.Sprintf("PeerAuthentication %s has mTLS DISABLED — traffic is unencrypted", name))
		}
	}

	// Check portLevelMtls for inconsistencies
	if portMtls, ok := spec["portLevelMtls"].(map[string]interface{}); ok {
		for port, conf := range portMtls {
			confMap, _ := conf.(map[string]interface{})
			mode, _ := confMap["mode"].(string)
			if mode == "DISABLE" {
				errs = append(errs, fmt.Sprintf("PeerAuthentication %s has mTLS DISABLED on port %s", name, port))
			}
		}
	}

	return errs
}

// ==================== Istio AuthorizationPolicy Analyzer ====================

func analyzeAuthorizationPolicy(resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	action, _ := spec["action"].(string)
	rules, _ := spec["rules"].([]interface{})

	// DENY/ALLOW without rules
	if action == "DENY" && len(rules) == 0 {
		errs = append(errs, fmt.Sprintf("AuthorizationPolicy %s action is DENY but no rules defined (denies all traffic)", name))
	}
	if action == "ALLOW" && len(rules) == 0 {
		errs = append(errs, fmt.Sprintf("AuthorizationPolicy %s action is ALLOW but no rules defined (denies all traffic — ALLOW with no rules means deny-all)", name))
	}

	return errs
}

// ==================== Network Component Pods Analyzer ====================
// Scans for controller Pods of network components (traefik, istio, nginx-ingress, coredns)
// and reports their errors — the most critical info for network troubleshooting.

var networkComponentLabels = []struct {
	Description string
	LabelSelector string
}{
	{"Traefik", "app.kubernetes.io/name=traefik"},
	{"Traefik (legacy)", "app=traefik"},
	{"Istio Ingress Gateway", "istio=ingressgateway"},
	{"Istio Egress Gateway", "istio=egressgateway"},
	{"Istiod", "app=istiod"},
	{"Istiod (alt)", "istio=pilot"},
	{"Nginx Ingress", "app.kubernetes.io/name=ingress-nginx"},
	{"Nginx Ingress (alt)", "app=nginx-ingress"},
	{"CoreDNS", "k8s-app=kube-dns"},
	{"Calico", "k8s-app=calico-node"},
	{"Cilium", "k8s-app=cilium"},
}

func (e *Executor) analyzeNetworkComponentPods(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	for _, comp := range networkComponentLabels {
		args := []string{"get", "pods", "-l", comp.LabelSelector, "-o", "json"}
		if namespace != "" {
			args = append(args, "-n", namespace)
		} else {
			args = append(args, "--all-namespaces")
		}

		output, err := executor.ExecKubectl(ctx, args)
		if err != nil {
			continue
		}

		var list struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(output, &list); err != nil || len(list.Items) == 0 {
			continue
		}

		for _, item := range list.Items {
			var pod map[string]interface{}
			if err := json.Unmarshal(item, &pod); err != nil {
				continue
			}

			metadata, _ := pod["metadata"].(map[string]interface{})
			podName := getResourceFullName(metadata)
			parentObj := getOwnerParent(metadata)

			// Use existing Pod analyzer
			podErrors := analyzePod(pod)

			// Also check recent events for this pod
			podNs, _ := metadata["namespace"].(string)
			podSimpleName, _ := metadata["name"].(string)
			eventErrors := getRecentPodWarningEvents(ctx, executor, podNs, podSimpleName)
			podErrors = append(podErrors, eventErrors...)

			if len(podErrors) > 0 {
				results = append(results, AnalyzeResult{
					Kind:      fmt.Sprintf("NetworkPod[%s]", comp.Description),
					Name:      podName,
					Error:     podErrors,
					ParentObj: parentObj,
				})
			}
		}
	}

	return results, nil
}

// ==================== Ingress Access Log Analyzer ====================
// Parses Nginx Ingress and Traefik access logs to detect non-2xx HTTP responses (3xx/4xx/5xx).

type ingressLogTarget struct {
	Description   string
	LabelSelector string
	Container     string
}

var ingressLogTargets = []ingressLogTarget{
	{"Nginx Ingress", "app.kubernetes.io/name=ingress-nginx", "controller"},
	{"Nginx Ingress (alt)", "app=nginx-ingress", ""},
	{"Traefik", "app.kubernetes.io/name=traefik", "traefik"},
	{"Traefik (legacy)", "app=traefik", ""},
}

const (
	ingressLogTailLines = 200
	maxIngressErrorGroups = 20
)

func (e *Executor) analyzeIngressAccessLog(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	for _, target := range ingressLogTargets {
		args := []string{"get", "pods", "-l", target.LabelSelector, "-o", "json"}
		if namespace != "" {
			args = append(args, "-n", namespace)
		} else {
			args = append(args, "--all-namespaces")
		}

		output, err := executor.ExecKubectl(ctx, args)
		if err != nil {
			continue
		}

		var list struct {
			Items []json.RawMessage `json:"items"`
		}
		if err := json.Unmarshal(output, &list); err != nil || len(list.Items) == 0 {
			continue
		}

		for _, item := range list.Items {
			var pod map[string]interface{}
			if err := json.Unmarshal(item, &pod); err != nil {
				continue
			}

			metadata, _ := pod["metadata"].(map[string]interface{})
			status, _ := pod["status"].(map[string]interface{})
			podPhase, _ := status["phase"].(string)
			
			// Only analyze running pods
			if podPhase != "Running" {
				continue
			}

			podNs, _ := metadata["namespace"].(string)
			podName, _ := metadata["name"].(string)

			// Determine container name
			container := target.Container
			if container == "" {
				if spec, ok := pod["spec"].(map[string]interface{}); ok {
					if containers, ok := spec["containers"].([]interface{}); ok && len(containers) > 0 {
						if firstContainer, ok := containers[0].(map[string]interface{}); ok {
							container, _ = firstContainer["name"].(string)
						}
					}
				}
			}

			// Fetch recent logs
			logArgs := []string{"logs", podName, "-n", podNs, "--tail", fmt.Sprintf("%d", ingressLogTailLines)}
			if container != "" {
				logArgs = append(logArgs, "-c", container)
			}

			logOutput, err := executor.ExecKubectl(ctx, logArgs)
			if err != nil {
				continue
			}

			// Parse logs for non-2xx status codes
			summaries := parseIngressLogs(string(logOutput))
			if len(summaries) == 0 {
				continue
			}

			var failures []string
			for _, s := range summaries {
				line := fmt.Sprintf("HTTP %d", s.StatusCode)
				if s.Method != "" {
					line += " " + s.Method
				}
				if s.Path != "" {
					line += " " + s.Path
				}
				if s.Upstream != "" {
					line += " -> " + s.Upstream
				}
				line += fmt.Sprintf(" (%d 次)", s.Count)
				failures = append(failures, line)
			}

			parentObj := getOwnerParent(metadata)
			results = append(results, AnalyzeResult{
				Kind:      fmt.Sprintf("IngressLog[%s]", target.Description),
				Name:      fmt.Sprintf("%s/%s", podNs, podName),
				Error:     failures,
				ParentObj: parentObj,
			})
		}
	}

	return results, nil
}

// ingressLogEntry represents a parsed access log entry
type ingressLogEntry struct {
	StatusCode int
	Method     string
	Path       string
	Upstream   string
}

// ingressErrorSummary represents aggregated error information
type ingressErrorSummary struct {
	StatusCode int
	Method     string
	Path       string
	Upstream   string
	Count      int
}

// parseIngressLogs parses log lines and extracts non-2xx HTTP status codes
func parseIngressLogs(logContent string) []ingressErrorSummary {
	lines := strings.Split(logContent, "\n")
	counts := make(map[string]*ingressErrorSummary)

	for _, line := range lines {
		if entry := parseLogLine(line); entry != nil {
			// Only report 3xx, 4xx, 5xx
			if entry.StatusCode < 300 {
				continue
			}

			// Normalize path for grouping
			path := normalizeIngressPath(entry.Path)
			key := fmt.Sprintf("%d|%s|%s", entry.StatusCode, entry.Method, path)

			if existing, ok := counts[key]; ok {
				existing.Count++
				if existing.Upstream == "" && entry.Upstream != "" {
					existing.Upstream = entry.Upstream
				}
			} else {
				counts[key] = &ingressErrorSummary{
					StatusCode: entry.StatusCode,
					Method:     entry.Method,
					Path:       path,
					Upstream:   entry.Upstream,
					Count:      1,
				}
			}
		}
	}

	// Sort by count desc, then status code desc
	sorted := make([]ingressErrorSummary, 0, len(counts))
	for _, s := range counts {
		sorted = append(sorted, *s)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Count != sorted[j].Count {
			return sorted[i].Count > sorted[j].Count
		}
		return sorted[i].StatusCode > sorted[j].StatusCode
	})

	if len(sorted) > maxIngressErrorGroups {
		sorted = sorted[:maxIngressErrorGroups]
	}
	return sorted
}

// parseLogLine tries to parse a log line using multiple patterns
func parseLogLine(line string) *ingressLogEntry {
	// Try Nginx/Traefik CLF format: "METHOD PATH PROTO" STATUS
	clfRe := regexp.MustCompile(`"(\w+)\s+(\S+)\s+[^"]*"\s+(\d{3})`)
	if m := clfRe.FindStringSubmatch(line); m != nil {
		code, _ := strconv.Atoi(m[3])
		entry := &ingressLogEntry{StatusCode: code, Method: m[1], Path: m[2]}
		
		// Try to extract upstream info
		upstreamRe := regexp.MustCompile(`upstream:\s*"([^"]*)"`)
		if um := upstreamRe.FindStringSubmatch(line); um != nil {
			entry.Upstream = um[1]
		}
		return entry
	}

	// Try Traefik JSON format
	jsonStatusRe := regexp.MustCompile(`"(?:OriginStatus|DownstreamStatus|status)":\s*(\d{3})`)
	if sm := jsonStatusRe.FindStringSubmatch(line); sm != nil {
		code, _ := strconv.Atoi(sm[1])
		entry := &ingressLogEntry{StatusCode: code}
		
		if mm := regexp.MustCompile(`"(?:RequestMethod|method)":\s*"(\w+)"`).FindStringSubmatch(line); mm != nil {
			entry.Method = mm[1]
		}
		if pm := regexp.MustCompile(`"(?:RequestPath|request)":\s*"([^"]+)"`).FindStringSubmatch(line); pm != nil {
			entry.Path = pm[1]
		}
		if um := regexp.MustCompile(`"(?:ServiceURL|ServiceAddr|serviceUrl)":\s*"([^"]+)"`).FindStringSubmatch(line); um != nil {
			entry.Upstream = um[1]
		}
		return entry
	}

	return nil
}

// normalizeIngressPath strips query strings and collapses IDs for grouping
func normalizeIngressPath(path string) string {
	// Strip query string
	if idx := strings.IndexByte(path, '?'); idx >= 0 {
		path = path[:idx]
	}
	
	// Collapse UUIDs
	uuidRe := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	path = uuidRe.ReplaceAllString(path, ":id")
	
	// Collapse numeric segments
	numericRe := regexp.MustCompile(`/\d+(/|$)`)
	path = numericRe.ReplaceAllString(path, "/:id$1")
	
	return path
}

// getRecentPodWarningEvents fetches recent Warning events for a specific pod.
func getRecentPodWarningEvents(ctx context.Context, executor cluster.KubeExecutor, namespace, podName string) []string {
	var errs []string
	args := []string{"get", "events", "--field-selector",
		fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,type=Warning", podName),
		"-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}

	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil
	}

	var list struct {
		Items []struct {
			Reason  string `json:"reason"`
			Message string `json:"message"`
			Count   int    `json:"count"`
			Type    string `json:"type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil
	}

	for _, evt := range list.Items {
		if evt.Type == "Warning" && evt.Message != "" {
			msg := fmt.Sprintf("[Event] %s: %s", evt.Reason, evt.Message)
			if evt.Count > 1 {
				msg += fmt.Sprintf(" (x%d)", evt.Count)
			}
			errs = append(errs, msg)
		}
	}
	return errs
}

// ==================== Warning Events Analyzer ====================
// Aggregates all Warning events in the cluster/namespace — great for network issue triage.

func (e *Executor) analyzeWarningEvents(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	args := []string{"get", "events", "--field-selector", "type=Warning", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}

	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get events: %s", string(output))
	}

	var list struct {
		Items []struct {
			Metadata struct {
				Namespace string `json:"namespace"`
			} `json:"metadata"`
			InvolvedObject struct {
				Kind string `json:"kind"`
				Name string `json:"name"`
			} `json:"involvedObject"`
			Reason  string `json:"reason"`
			Message string `json:"message"`
			Count   int    `json:"count"`
		} `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}

	// Group events by involved object
	type eventGroup struct {
		Kind   string
		Name   string
		Errors []string
	}
	grouped := make(map[string]*eventGroup)
	var order []string

	for _, evt := range list.Items {
		if evt.Message == "" {
			continue
		}
		key := fmt.Sprintf("%s/%s/%s", evt.Metadata.Namespace, evt.InvolvedObject.Kind, evt.InvolvedObject.Name)
		if _, ok := grouped[key]; !ok {
			resName := evt.InvolvedObject.Name
			if evt.Metadata.Namespace != "" {
				resName = evt.Metadata.Namespace + "/" + resName
			}
			grouped[key] = &eventGroup{
				Kind: fmt.Sprintf("Event[%s]", evt.InvolvedObject.Kind),
				Name: resName,
			}
			order = append(order, key)
		}
		msg := fmt.Sprintf("%s: %s", evt.Reason, evt.Message)
		if evt.Count > 1 {
			msg += fmt.Sprintf(" (x%d)", evt.Count)
		}
		grouped[key].Errors = append(grouped[key].Errors, msg)
	}

	var results []AnalyzeResult
	for _, key := range order {
		g := grouped[key]
		results = append(results, AnalyzeResult{
			Kind:  g.Kind,
			Name:  g.Name,
			Error: g.Errors,
		})
	}

	return results, nil
}

func deduplicateResults(results []AnalyzeResult) []AnalyzeResult {
	seen := make(map[string]*AnalyzeResult)
	var order []string
	for _, r := range results {
		key := r.Kind + "|" + r.Name
		if existing, ok := seen[key]; ok {
			existing.Error = append(existing.Error, r.Error...)
			if r.ParentObj != "" && existing.ParentObj == "" {
				existing.ParentObj = r.ParentObj
			}
		} else {
			copy := r
			seen[key] = &copy
			order = append(order, key)
		}
	}
	var out []AnalyzeResult
	for _, key := range order {
		out = append(out, *seen[key])
	}
	return out
}

// enrichWithLLM adds AI explanations to results.
func (e *Executor) enrichWithLLM(ctx context.Context, results []AnalyzeResult, language string) {
	llmConf, err := e.getLLMConfig()
	if err != nil {
		return
	}

	langPrompt := "请用中文回复。"
	if language == "english" {
		langPrompt = "Please respond in English."
	} else if language == "japanese" {
		langPrompt = "日本語で回答してください。"
	}

	for i := range results {
		r := &results[i]
		if len(r.Error) == 0 {
			continue
		}
		problemDesc := fmt.Sprintf("Resource: %s %s\nProblems:\n", r.Kind, r.Name)
		for _, e := range r.Error {
			problemDesc += "- " + e + "\n"
		}

		messages := []llmclient.Message{
			{Role: "system", Content: fmt.Sprintf(
				"You are a Kubernetes expert. Analyze the following Kubernetes resource issue, "+
					"provide a concise root cause analysis and actionable fix suggestions. %s", langPrompt)},
			{Role: "user", Content: problemDesc},
		}

		llmCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		resp, llmErr := e.llmClient.ChatCompletion(llmCtx, *llmConf, messages)
		cancel()
		if llmErr == nil {
			r.Details = resp
		}
	}
}

// ListFilters returns supported resource types for analysis.
func (e *Executor) ListFilters(ctx context.Context) ([]string, error) {
	return allFilters, nil
}

// GetConfig returns the current configuration.
func (e *Executor) GetConfig() (*model.K8sGPTConfig, error) {
	var config model.K8sGPTConfig
	if err := database.DB.FirstOrCreate(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateConfig updates the configuration.
func (e *Executor) UpdateConfig(backend, modelName, baseURL, language string, anonymize, useBuiltinLLM bool) (*model.K8sGPTConfig, error) {
	var config model.K8sGPTConfig
	database.DB.FirstOrCreate(&config)
	config.Backend = backend
	config.Model = modelName
	config.BaseURL = baseURL
	config.Language = language
	config.Anonymize = anonymize
	config.UseBuiltinLLM = useBuiltinLLM
	config.UpdatedAt = time.Now()
	if err := database.DB.Save(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// TestConnection tests kubectl cluster connectivity.
func (e *Executor) TestConnection(ctx context.Context) (string, error) {
	return e.TestConnectionForCluster(ctx, 0)
}

// TestConnectionForCluster tests connectivity for a specific cluster (0 = active cluster).
func (e *Executor) TestConnectionForCluster(ctx context.Context, clusterID uint) (string, error) {
	var exec *cluster.CloseableExecutor
	var err error
	if clusterID > 0 {
		var c model.ClusterConfig
		if err := database.DB.First(&c, clusterID).Error; err != nil {
			return "", fmt.Errorf("cluster not found: %w", err)
		}
		exec, err = cluster.GetExecutorForCluster(&c)
	} else {
		exec, err = cluster.GetActiveExecutor()
	}
	if err != nil {
		return "", fmt.Errorf("未配置活跃集群: %w", err)
	}
	defer exec.Close()

	output, err := exec.ExecKubectl(ctx, []string{"cluster-info"})
	if err != nil {
		return "", fmt.Errorf("集群连接失败: %s", strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// ==================== Log Analyzer (referenced from k8sgpt log.go) ====================
// Scans Pod logs for error/exception/fail patterns.

var logErrorPattern = regexp.MustCompile(`(?i)(error|exception|fail)`)

func (e *Executor) analyzeLog(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// Get all pods
	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}
	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, err
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		var pod map[string]interface{}
		if err := json.Unmarshal(item, &pod); err != nil {
			continue
		}
		metadata, _ := pod["metadata"].(map[string]interface{})
		podName, _ := metadata["name"].(string)
		podNs, _ := metadata["namespace"].(string)
		parentObj := getOwnerParent(metadata)

		spec, _ := pod["spec"].(map[string]interface{})
		containers, _ := spec["containers"].([]interface{})

		for _, c := range containers {
			cMap, _ := c.(map[string]interface{})
			cName, _ := cMap["name"].(string)

			// Get last 100 lines of logs
			logArgs := []string{"logs", podName, "-c", cName, "--tail=100", "-n", podNs}
			logOutput, err := executor.ExecKubectl(ctx, logArgs)
			var errs []string

			if err != nil {
				errs = append(errs, fmt.Sprintf("Error getting logs from Pod %s container %s: %s", podName, cName, string(logOutput)))
			} else {
				rawLogs := string(logOutput)
				if logErrorPattern.MatchString(rawLogs) {
					// Find first matching error line
					for _, line := range strings.Split(rawLogs, "\n") {
						if logErrorPattern.MatchString(line) {
							errs = append(errs, line)
							break
						}
					}
				}
			}

			if len(errs) > 0 {
				results = append(results, AnalyzeResult{
					Kind:      "Log",
					Name:      fmt.Sprintf("%s/%s/%s", podNs, podName, cName),
					Error:     errs,
					ParentObj: parentObj,
				})
			}
		}
	}
	return results, nil
}

// ==================== Security Analyzer (referenced from k8sgpt security.go) ====================
// Three-layer security audit: ServiceAccount, RoleBinding RBAC, Pod SecurityContext.

func (e *Executor) analyzeSecurity(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// 1. Check default ServiceAccount usage
	saResults, err := e.analyzeDefaultSA(ctx, executor, namespace)
	if err == nil {
		results = append(results, saResults...)
	}

	// 2. Check RoleBindings for wildcard permissions
	rbResults, err := e.analyzeRoleBindingWildcards(ctx, executor, namespace)
	if err == nil {
		results = append(results, rbResults...)
	}

	// 3. Check Pod SecurityContext
	podResults, err := e.analyzePodSecurityContext(ctx, executor, namespace)
	if err == nil {
		results = append(results, podResults...)
	}

	return results, nil
}

func (e *Executor) analyzeDefaultSA(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// Get pods to check default SA usage
	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}
	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, err
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}

	// Group pods using default SA by namespace
	nsPods := make(map[string][]string)
	for _, item := range list.Items {
		var pod map[string]interface{}
		if err := json.Unmarshal(item, &pod); err != nil {
			continue
		}
		metadata, _ := pod["metadata"].(map[string]interface{})
		spec, _ := pod["spec"].(map[string]interface{})
		podName, _ := metadata["name"].(string)
		podNs, _ := metadata["namespace"].(string)
		sa, _ := spec["serviceAccountName"].(string)
		if sa == "" || sa == "default" {
			nsPods[podNs] = append(nsPods[podNs], podName)
		}
	}

	for ns, pods := range nsPods {
		if len(pods) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Security/ServiceAccount",
				Name:  ns + "/default",
				Error: []string{fmt.Sprintf("Default service account is being used by pods: %v", pods)},
			})
		}
	}
	return results, nil
}

func (e *Executor) analyzeRoleBindingWildcards(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// Get rolebindings
	rbArgs := []string{"get", "rolebindings", "-o", "json"}
	if namespace != "" {
		rbArgs = append(rbArgs, "-n", namespace)
	} else {
		rbArgs = append(rbArgs, "--all-namespaces")
	}
	rbOutput, err := executor.ExecKubectl(ctx, rbArgs)
	if err != nil {
		return nil, err
	}
	var rbList struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rbOutput, &rbList); err != nil {
		return nil, err
	}

	for _, item := range rbList.Items {
		var rb map[string]interface{}
		if err := json.Unmarshal(item, &rb); err != nil {
			continue
		}
		metadata, _ := rb["metadata"].(map[string]interface{})
		rbName, _ := metadata["name"].(string)
		rbNs, _ := metadata["namespace"].(string)
		roleRef, _ := rb["roleRef"].(map[string]interface{})
		roleName, _ := roleRef["name"].(string)

		// Get the referenced role
		roleObj, err := kubectlGetSingle(ctx, executor, "role", rbNs, roleName)
		if err != nil {
			continue
		}

		rules, _ := roleObj["rules"].([]interface{})
		for _, rule := range rules {
			ruleMap, _ := rule.(map[string]interface{})
			verbs, _ := ruleMap["verbs"].([]interface{})
			resources, _ := ruleMap["resources"].([]interface{})
			if sliceContainsWildcard(verbs) || sliceContainsWildcard(resources) {
				results = append(results, AnalyzeResult{
					Kind: "Security/RoleBinding",
					Name: fmt.Sprintf("%s/%s", rbNs, rbName),
					Error: []string{fmt.Sprintf("RoleBinding %s references Role %s which contains wildcard permissions - not recommended for security best practices", rbName, roleName)},
				})
				break
			}
		}
	}
	return results, nil
}

func sliceContainsWildcard(items []interface{}) bool {
	for _, item := range items {
		if s, ok := item.(string); ok && s == "*" {
			return true
		}
	}
	return false
}

func (e *Executor) analyzePodSecurityContext(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	args := []string{"get", "pods", "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	} else {
		args = append(args, "--all-namespaces")
	}
	output, err := executor.ExecKubectl(ctx, args)
	if err != nil {
		return nil, err
	}
	var list struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, err
	}

	for _, item := range list.Items {
		var pod map[string]interface{}
		if err := json.Unmarshal(item, &pod); err != nil {
			continue
		}
		metadata, _ := pod["metadata"].(map[string]interface{})
		podName, _ := metadata["name"].(string)
		podNs, _ := metadata["namespace"].(string)
		spec, _ := pod["spec"].(map[string]interface{})

		// Check for privileged containers (most critical)
		hasPrivileged := false
		containers, _ := spec["containers"].([]interface{})
		for _, c := range containers {
			cMap, _ := c.(map[string]interface{})
			cName, _ := cMap["name"].(string)
			if sc, ok := cMap["securityContext"].(map[string]interface{}); ok {
				if priv, ok := sc["privileged"].(bool); ok && priv {
					results = append(results, AnalyzeResult{
						Kind:  "Security/Pod",
						Name:  fmt.Sprintf("%s/%s", podNs, podName),
						Error: []string{fmt.Sprintf("Container %s in pod %s is running as privileged which poses security risks", cName, podName)},
					})
					hasPrivileged = true
					break
				}
			}
		}

		// Only check for missing security context if no privileged containers found
		if !hasPrivileged {
			if _, ok := spec["securityContext"].(map[string]interface{}); !ok {
				results = append(results, AnalyzeResult{
					Kind:  "Security/Pod",
					Name:  fmt.Sprintf("%s/%s", podNs, podName),
					Error: []string{fmt.Sprintf("Pod %s does not have a security context defined which may pose security risks", podName)},
				})
			}
		}
	}
	return results, nil
}

// ==================== Storage Analyzer (referenced from k8sgpt storage.go) ====================
// Three-layer storage check: StorageClass, PV, PVC.

func (e *Executor) analyzeStorage(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	// 1. Analyze StorageClasses
	scResults, err := e.analyzeStorageClasses(ctx, executor)
	if err == nil {
		results = append(results, scResults...)
	}

	// 2. Enhanced PV analysis (small capacity check)
	pvResults, err := e.analyzeStoragePVs(ctx, executor)
	if err == nil {
		results = append(results, pvResults...)
	}

	// 3. Enhanced PVC analysis
	pvcResults, err := e.analyzeStoragePVCs(ctx, executor, namespace)
	if err == nil {
		results = append(results, pvcResults...)
	}

	return results, nil
}

func (e *Executor) analyzeStorageClasses(ctx context.Context, executor cluster.KubeExecutor) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	items, err := kubectlGet(ctx, executor, "storageclasses", "", "")
	if err != nil {
		return nil, err
	}

	// Count default storage classes
	defaultCount := 0
	for _, item := range items {
		var sc map[string]interface{}
		if err := json.Unmarshal(item, &sc); err != nil {
			continue
		}
		metadata, _ := sc["metadata"].(map[string]interface{})
		annotations, _ := metadata["annotations"].(map[string]interface{})
		if annotations != nil {
			if v, ok := annotations["storageclass.kubernetes.io/is-default-class"].(string); ok && v == "true" {
				defaultCount++
			}
		}
	}

	for _, item := range items {
		var sc map[string]interface{}
		if err := json.Unmarshal(item, &sc); err != nil {
			continue
		}
		metadata, _ := sc["metadata"].(map[string]interface{})
		scName, _ := metadata["name"].(string)
		provisioner, _ := sc["provisioner"].(string)

		var errs []string

		// Check for deprecated provisioner
		if provisioner == "kubernetes.io/no-provisioner" {
			errs = append(errs, fmt.Sprintf("StorageClass %s uses deprecated provisioner 'kubernetes.io/no-provisioner'", scName))
		}

		// Check for multiple default storage classes
		if defaultCount > 1 {
			annotations, _ := metadata["annotations"].(map[string]interface{})
			if annotations != nil {
				if v, ok := annotations["storageclass.kubernetes.io/is-default-class"].(string); ok && v == "true" {
					errs = append(errs, fmt.Sprintf("Multiple default StorageClasses found (%d), which can cause confusion", defaultCount))
				}
			}
		}

		if len(errs) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/StorageClass",
				Name:  scName,
				Error: errs,
			})
		}
	}
	return results, nil
}

func (e *Executor) analyzeStoragePVs(ctx context.Context, executor cluster.KubeExecutor) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	items, err := kubectlGet(ctx, executor, "persistentvolumes", "", "")
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		var pv map[string]interface{}
		if err := json.Unmarshal(item, &pv); err != nil {
			continue
		}
		metadata, _ := pv["metadata"].(map[string]interface{})
		pvName, _ := metadata["name"].(string)
		status, _ := pv["status"].(map[string]interface{})
		phase, _ := status["phase"].(string)
		spec, _ := pv["spec"].(map[string]interface{})

		var errs []string

		if phase == "Released" {
			errs = append(errs, fmt.Sprintf("PersistentVolume %s is in Released state and should be cleaned up", pvName))
		}
		if phase == "Failed" {
			errs = append(errs, fmt.Sprintf("PersistentVolume %s is in Failed state", pvName))
		}

		// Check for small PVs (< 1Gi)
		if capacity, ok := spec["capacity"].(map[string]interface{}); ok {
			if storage, ok := capacity["storage"].(string); ok {
				if isSmallCapacity(storage) {
					errs = append(errs, fmt.Sprintf("PersistentVolume %s has small capacity (%s)", pvName, storage))
				}
			}
		}

		if len(errs) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/PersistentVolume",
				Name:  pvName,
				Error: errs,
			})
		}
	}
	return results, nil
}

func (e *Executor) analyzeStoragePVCs(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	items, err := kubectlGet(ctx, executor, "persistentvolumeclaims", namespace, "")
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		var pvc map[string]interface{}
		if err := json.Unmarshal(item, &pvc); err != nil {
			continue
		}
		metadata, _ := pvc["metadata"].(map[string]interface{})
		pvcName := getResourceFullName(metadata)
		status, _ := pvc["status"].(map[string]interface{})
		phase, _ := status["phase"].(string)
		spec, _ := pvc["spec"].(map[string]interface{})

		var errs []string

		switch phase {
		case "Pending":
			errs = append(errs, fmt.Sprintf("PersistentVolumeClaim %s is in Pending state", pvcName))
		case "Lost":
			errs = append(errs, fmt.Sprintf("PersistentVolumeClaim %s is in Lost state", pvcName))
		default:
			// Check for small capacity
			if resources, ok := spec["resources"].(map[string]interface{}); ok {
				if requests, ok := resources["requests"].(map[string]interface{}); ok {
					if storage, ok := requests["storage"].(string); ok {
						if isSmallCapacity(storage) {
							errs = append(errs, fmt.Sprintf("PersistentVolumeClaim %s has small capacity (%s)", pvcName, storage))
						}
					}
				}
			}
			// Check for missing storage class
			_, hasSC := spec["storageClassName"]
			volumeName, _ := spec["volumeName"].(string)
			if !hasSC && volumeName == "" {
				errs = append(errs, fmt.Sprintf("PersistentVolumeClaim %s has no StorageClass specified", pvcName))
			}
		}

		if len(errs) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Storage/PersistentVolumeClaim",
				Name:  pvcName,
				Error: errs[:1], // Only report the first failure (most critical)
			})
		}
	}
	return results, nil
}

// isSmallCapacity checks if a K8s resource quantity string is less than 1Gi.
func isSmallCapacity(s string) bool {
	s = strings.TrimSpace(s)
	// Handle common suffixes: Mi, Gi, Ti, Ki, m, k, M, G, T
	if strings.HasSuffix(s, "Gi") || strings.HasSuffix(s, "Ti") {
		return false // >= 1Gi
	}
	if strings.HasSuffix(s, "G") || strings.HasSuffix(s, "T") {
		return false
	}
	// Mi, Ki, or bare numbers are likely < 1Gi
	return true
}

// ==================== PDB Analyzer (referenced from k8sgpt pdb.go) ====================

func analyzePDB(resource map[string]interface{}) []string {
	var errs []string
	status, _ := resource["status"].(map[string]interface{})
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)

	conditions, _ := status["conditions"].([]interface{})
	if len(conditions) == 0 {
		return nil
	}

	firstCond, _ := conditions[0].(map[string]interface{})
	condType, _ := firstCond["type"].(string)
	condStatus, _ := firstCond["status"].(string)
	reason, _ := firstCond["reason"].(string)

	if condType == "DisruptionAllowed" && condStatus == "False" {
		// Extract expected pod labels from selector
		if selector, ok := spec["selector"].(map[string]interface{}); ok {
			if matchLabels, ok := selector["matchLabels"].(map[string]interface{}); ok {
				for k, v := range matchLabels {
					vStr, _ := v.(string)
					errs = append(errs, fmt.Sprintf("%s, expected pdb pod label %s=%s", reason, k, vStr))
				}
			}
		}
		if len(errs) == 0 {
			errs = append(errs, fmt.Sprintf("PodDisruptionBudget %s disruption not allowed: %s", name, reason))
		}
	}

	return errs
}

// ==================== NetworkPolicy Analyzer (referenced from k8sgpt netpol.go) ====================

func (e *Executor) analyzeNetworkPolicy(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	spec, _ := resource["spec"].(map[string]interface{})
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	ns, _ := metadata["namespace"].(string)

	podSelector, _ := spec["podSelector"].(map[string]interface{})
	matchLabels, _ := podSelector["matchLabels"].(map[string]interface{})

	if len(matchLabels) == 0 {
		// Empty selector — allows traffic to all pods in the namespace
		errs = append(errs, fmt.Sprintf("Network policy allows traffic to all pods: %s", name))
	} else {
		// Check if the policy actually matches any pods
		var selectorParts []string
		for k, v := range matchLabels {
			vStr, _ := v.(string)
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, vStr))
		}
		labelSelector := strings.Join(selectorParts, ",")

		podsArgs := []string{"get", "pods", "-l", labelSelector, "-n", ns, "-o", "json"}
		podsOutput, err := executor.ExecKubectl(ctx, podsArgs)
		if err == nil {
			var podList struct {
				Items []json.RawMessage `json:"items"`
			}
			if json.Unmarshal(podsOutput, &podList) == nil && len(podList.Items) == 0 {
				errs = append(errs, fmt.Sprintf("Network policy is not applied to any pods: %s", name))
			}
		}
	}

	return errs
}

// ==================== MutatingWebhook Analyzer (referenced from k8sgpt mutating_webhook.go) ====================

func (e *Executor) analyzeMutatingWebhook(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})

	webhooks, _ := resource["webhooks"].([]interface{})
	for _, wh := range webhooks {
		whMap, _ := wh.(map[string]interface{})
		whName, _ := whMap["name"].(string)
		clientConfig, _ := whMap["clientConfig"].(map[string]interface{})
		if clientConfig == nil {
			continue
		}

		svcRef, _ := clientConfig["service"].(map[string]interface{})
		if svcRef == nil {
			continue
		}
		svcName, _ := svcRef["name"].(string)
		svcNs, _ := svcRef["namespace"].(string)

		// Check if service exists
		svcObj, err := kubectlGetSingle(ctx, executor, "service", svcNs, svcName)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Service %s not found as mapped to by Mutating Webhook %s", svcName, whName))
			continue
		}

		// Check if service has selector and matching pods
		svcSpec, _ := svcObj["spec"].(map[string]interface{})
		selector, _ := svcSpec["selector"].(map[string]interface{})
		if len(selector) == 0 {
			continue
		}

		var selectorParts []string
		for k, v := range selector {
			vStr, _ := v.(string)
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, vStr))
		}
		labelSelector := strings.Join(selectorParts, ",")

		podsArgs := []string{"get", "pods", "-l", labelSelector, "-n", svcNs, "-o", "json"}
		podsOutput, podErr := executor.ExecKubectl(ctx, podsArgs)
		if podErr != nil {
			continue
		}

		var podList struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(podsOutput, &podList) != nil {
			continue
		}

		if len(podList.Items) == 0 {
			errs = append(errs, fmt.Sprintf("No active pods found within service %s as mapped to by Mutating Webhook %s", svcName, whName))
			continue
		}

		for _, podItem := range podList.Items {
			var pod map[string]interface{}
			if json.Unmarshal(podItem, &pod) != nil {
				continue
			}
			podStatus, _ := pod["status"].(map[string]interface{})
			phase, _ := podStatus["phase"].(string)
			podMeta, _ := pod["metadata"].(map[string]interface{})
			podName, _ := podMeta["name"].(string)
			if phase != "Running" {
				errs = append(errs, fmt.Sprintf("Mutating Webhook (%s) is pointing to an inactive receiver pod (%s)", whName, podName))
			}
		}
	}

	_ = metadata
	return errs
}

// ==================== ValidatingWebhook Analyzer (referenced from k8sgpt validating_webhook.go) ====================

func (e *Executor) analyzeValidatingWebhook(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})

	webhooks, _ := resource["webhooks"].([]interface{})
	for _, wh := range webhooks {
		whMap, _ := wh.(map[string]interface{})
		whName, _ := whMap["name"].(string)
		clientConfig, _ := whMap["clientConfig"].(map[string]interface{})
		if clientConfig == nil {
			continue
		}

		svcRef, _ := clientConfig["service"].(map[string]interface{})
		if svcRef == nil {
			continue
		}
		svcName, _ := svcRef["name"].(string)
		svcNs, _ := svcRef["namespace"].(string)

		// Check if service exists
		svcObj, err := kubectlGetSingle(ctx, executor, "service", svcNs, svcName)
		if err != nil {
			errs = append(errs, fmt.Sprintf("Service %s not found as mapped to by Validating Webhook %s", svcName, whName))
			continue
		}

		// Check if service has selector and matching pods
		svcSpec, _ := svcObj["spec"].(map[string]interface{})
		selector, _ := svcSpec["selector"].(map[string]interface{})
		if len(selector) == 0 {
			continue
		}

		var selectorParts []string
		for k, v := range selector {
			vStr, _ := v.(string)
			selectorParts = append(selectorParts, fmt.Sprintf("%s=%s", k, vStr))
		}
		labelSelector := strings.Join(selectorParts, ",")

		podsArgs := []string{"get", "pods", "-l", labelSelector, "-n", svcNs, "-o", "json"}
		podsOutput, podErr := executor.ExecKubectl(ctx, podsArgs)
		if podErr != nil {
			continue
		}

		var podList struct {
			Items []json.RawMessage `json:"items"`
		}
		if json.Unmarshal(podsOutput, &podList) != nil {
			continue
		}

		if len(podList.Items) == 0 {
			errs = append(errs, fmt.Sprintf("No active pods found within service %s as mapped to by Validating Webhook %s", svcName, whName))
			continue
		}

		for _, podItem := range podList.Items {
			var pod map[string]interface{}
			if json.Unmarshal(podItem, &pod) != nil {
				continue
			}
			podStatus, _ := pod["status"].(map[string]interface{})
			phase, _ := podStatus["phase"].(string)
			podMeta, _ := pod["metadata"].(map[string]interface{})
			podName, _ := podMeta["name"].(string)
			if phase != "Running" {
				errs = append(errs, fmt.Sprintf("Validating Webhook (%s) is pointing to an inactive receiver pod (%s)", whName, podName))
			}
		}
	}

	_ = metadata
	return errs
}

// ==================== Gateway API: GatewayClass Analyzer (referenced from k8sgpt gatewayclass.go) ====================

func analyzeGatewayClass(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})

	controllerName, _ := spec["controllerName"].(string)

	conditions, _ := status["conditions"].([]interface{})
	if len(conditions) > 0 {
		first, _ := conditions[0].(map[string]interface{})
		condStatus, _ := first["status"].(string)
		message, _ := first["message"].(string)
		if condStatus != "True" {
			errs = append(errs, fmt.Sprintf("GatewayClass '%s' with controller '%s' is not accepted. Message: '%s'", name, controllerName, message))
		}
	}
	return errs
}

// ==================== Gateway API: Gateway Analyzer (referenced from k8sgpt gateway.go) ====================
// Note: This is the Gateway API Gateway, NOT the Istio Gateway (which is handled by analyzeGateway above).

func (e *Executor) analyzeGatewayAPI(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})

	// Check if GatewayClass exists
	gatewayClassName, _ := spec["gatewayClassName"].(string)
	if gatewayClassName != "" {
		if _, err := kubectlGetSingle(ctx, executor, "gatewayclasses.gateway.networking.k8s.io", "", gatewayClassName); err != nil {
			errs = append(errs, fmt.Sprintf("Gateway uses the GatewayClass %s which does not exist", gatewayClassName))
		}
	}

	// Check status conditions
	conditions, _ := status["conditions"].([]interface{})
	if len(conditions) > 0 {
		first, _ := conditions[0].(map[string]interface{})
		condStatus, _ := first["status"].(string)
		message, _ := first["message"].(string)
		if condStatus != "True" {
			errs = append(errs, fmt.Sprintf("Gateway '%s' is not accepted. Message: '%s'", name, message))
		}
	}

	return errs
}

// ==================== Gateway API: HTTPRoute Analyzer (referenced from k8sgpt httproute.go) ====================

func (e *Executor) analyzeHTTPRoute(ctx context.Context, executor cluster.KubeExecutor, resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	ns, _ := metadata["namespace"].(string)
	name := getResourceFullName(metadata)
	spec, _ := resource["spec"].(map[string]interface{})

	// Check parentRefs (Gateways)
	parentRefs, _ := spec["parentRefs"].([]interface{})
	for _, ref := range parentRefs {
		refMap, _ := ref.(map[string]interface{})
		gwName, _ := refMap["name"].(string)
		gwNs := ns
		if refNs, ok := refMap["namespace"].(string); ok && refNs != "" {
			gwNs = refNs
		}
		if gwName != "" {
			if _, err := kubectlGetSingle(ctx, executor, "gateways.gateway.networking.k8s.io", gwNs, gwName); err != nil {
				errs = append(errs, fmt.Sprintf("HTTPRoute '%s' uses Gateway '%s/%s' which does not exist", name, gwNs, gwName))
			}
		}
	}

	// Check backend service references
	rules, _ := spec["rules"].([]interface{})
	for _, rule := range rules {
		ruleMap, _ := rule.(map[string]interface{})
		backendRefs, _ := ruleMap["backendRefs"].([]interface{})
		for _, backend := range backendRefs {
			bMap, _ := backend.(map[string]interface{})
			svcName, _ := bMap["name"].(string)
			port, _ := bMap["port"].(float64)
			if svcName == "" {
				continue
			}
			svcObj, err := kubectlGetSingle(ctx, executor, "service", ns, svcName)
			if err != nil {
				errs = append(errs, fmt.Sprintf("HTTPRoute '%s' uses Service '%s/%s' which does not exist", name, ns, svcName))
				continue
			}
			// Check port match
			if port > 0 {
				svcSpec, _ := svcObj["spec"].(map[string]interface{})
				ports, _ := svcSpec["ports"].([]interface{})
				portMatch := false
				for _, p := range ports {
					pMap, _ := p.(map[string]interface{})
					svcPort, _ := pMap["port"].(float64)
					if svcPort == port {
						portMatch = true
						break
					}
				}
				if !portMatch {
					errs = append(errs, fmt.Sprintf("HTTPRoute backend service '%s' uses port %d but service '%s/%s' doesn't have that port", svcName, int(port), ns, svcName))
				}
			}
		}
	}

	return errs
}

// ==================== OLM v1: ClusterCatalog Analyzer (referenced from k8sgpt clustercatalog.go) ====================

var imageRefPattern = regexp.MustCompile(`^([a-zA-Z0-9\-\.]+(?::[0-9]+)?/)?([a-z0-9]+(?:[._\-/][a-z0-9]+)*)(:[\w][\w.\-]{0,127})?(?:@sha256:[a-f0-9]{64})?$`)
var sha256DigestPattern = regexp.MustCompile(`@sha256:[a-f0-9]{64}$`)

func analyzeClusterCatalog(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})

	// Validate image ref
	if source, ok := spec["source"].(map[string]interface{}); ok {
		if image, ok := source["image"].(map[string]interface{}); ok {
			ref, _ := image["ref"].(string)
			if ref != "" && !imageRefPattern.MatchString(ref) {
				errs = append(errs, fmt.Sprintf("ClusterCatalog %s has invalid image ref: %s", name, ref))
			}
		}
	}

	// Check resolved source
	if resolvedSource, ok := status["resolvedSource"].(map[string]interface{}); ok {
		if image, ok := resolvedSource["image"].(map[string]interface{}); ok {
			ref, _ := image["ref"].(string)
			if ref == "" {
				errs = append(errs, fmt.Sprintf("ClusterCatalog %s missing status.resolvedSource.image.ref", name))
			} else if !sha256DigestPattern.MatchString(ref) {
				errs = append(errs, fmt.Sprintf("ClusterCatalog %s status.resolvedSource.image.ref must end with @sha256:<digest>", name))
			}
		}
	}

	// Check conditions
	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			cond, _ := c.(map[string]interface{})
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			reason, _ := cond["reason"].(string)
			message, _ := cond["message"].(string)
			if condType == "Serving" && condStatus != "True" {
				errs = append(errs, fmt.Sprintf("ClusterCatalog %s has condition Serving=%s, reason %s: %s", name, condStatus, reason, message))
			}
			if condType == "Progressing" && reason != "Succeeded" {
				errs = append(errs, fmt.Sprintf("ClusterCatalog %s has condition Progressing reason=%s: %s", name, reason, message))
			}
		}
	}

	return errs
}

// ==================== OLM v1: ClusterExtension Analyzer (referenced from k8sgpt clusterextension.go) ====================

func analyzeClusterExtension(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name, _ := metadata["name"].(string)
	spec, _ := resource["spec"].(map[string]interface{})
	status, _ := resource["status"].(map[string]interface{})

	// Validate source type
	if source, ok := spec["source"].(map[string]interface{}); ok {
		sourceType, _ := source["sourceType"].(string)
		if sourceType != "" && sourceType != "Catalog" {
			errs = append(errs, fmt.Sprintf("ClusterExtension %s has invalid spec.source.sourceType '%s' (expecting 'Catalog')", name, sourceType))
		}
		if catalog, ok := source["catalog"].(map[string]interface{}); ok {
			policy, _ := catalog["upgradeConstraintPolicy"].(string)
			if policy != "" && policy != "CatalogProvided" && policy != "SelfCertified" {
				errs = append(errs, fmt.Sprintf("ClusterExtension %s has invalid upgradeConstraintPolicy '%s'", name, policy))
			}
		}
	}

	// Check conditions
	if conditions, ok := status["conditions"].([]interface{}); ok {
		for _, c := range conditions {
			cond, _ := c.(map[string]interface{})
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			reason, _ := cond["reason"].(string)
			message, _ := cond["message"].(string)
			if condType == "Installed" && condStatus != "True" {
				errs = append(errs, fmt.Sprintf("ClusterExtension %s not installed: reason=%s: %s", name, reason, message))
			}
			if condType == "Progressing" && reason != "Succeeded" {
				errs = append(errs, fmt.Sprintf("ClusterExtension %s progressing: reason=%s: %s", name, reason, message))
			}
		}
	}

	return errs
}

// ==================== OLM v1alpha1: ClusterServiceVersion Analyzer (referenced from k8sgpt clusterserviceversion.go) ====================

func analyzeCSV(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)
	status, _ := resource["status"].(map[string]interface{})

	phase, _ := status["phase"].(string)
	if phase != "" && phase != "Succeeded" {
		conditions, _ := status["conditions"].([]interface{})
		msg := pickWorstOLMCondition(conditions)
		if msg != "" {
			errs = append(errs, fmt.Sprintf("CSV %s phase=%s: %s", name, phase, msg))
		} else {
			errs = append(errs, fmt.Sprintf("CSV %s phase=%s (see status.conditions)", name, phase))
		}
	}
	return errs
}

// pickWorstOLMCondition finds the first non-True condition with a message.
func pickWorstOLMCondition(conditions []interface{}) string {
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condStatus, _ := cond["status"].(string)
		if condStatus == "True" {
			continue
		}
		reason, _ := cond["reason"].(string)
		message, _ := cond["message"].(string)
		if reason == "" && message == "" {
			continue
		}
		if reason != "" && message != "" {
			return reason + ": " + message
		}
		return reason + message
	}
	return ""
}

// ==================== OLM v1alpha1: Subscription Analyzer (referenced from k8sgpt subscription.go) ====================

func analyzeSubscription(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)
	status, _ := resource["status"].(map[string]interface{})

	state, _ := status["state"].(string)
	if state == "" || state == "UpgradePending" || state == "UpgradeAvailable" {
		conditions, _ := status["conditions"].([]interface{})
		msg := "subscription not at latest"
		if c := pickWorstOLMCondition(conditions); c != "" {
			msg += "; " + c
		}
		errs = append(errs, fmt.Sprintf("Subscription %s state=%s: %s", name, state, msg))
	}
	return errs
}

// ==================== OLM v1alpha1: CatalogSource Analyzer (referenced from k8sgpt catalogsource.go) ====================

func analyzeCatalogSource(resource map[string]interface{}) []string {
	var errs []string
	metadata, _ := resource["metadata"].(map[string]interface{})
	name := getResourceFullName(metadata)
	status, _ := resource["status"].(map[string]interface{})

	if connState, ok := status["connectionState"].(map[string]interface{}); ok {
		state, _ := connState["lastObservedState"].(string)
		addr, _ := connState["address"].(string)
		if state != "" && strings.ToUpper(state) != "READY" {
			errs = append(errs, fmt.Sprintf("CatalogSource %s connectionState=%s (address=%s)", name, state, addr))
		}
	}
	return errs
}

// ==================== OLM v1alpha1: OperatorGroup Analyzer (referenced from k8sgpt operatorgroup.go) ====================

func (e *Executor) analyzeOperatorGroups(ctx context.Context, executor cluster.KubeExecutor, namespace string) ([]AnalyzeResult, error) {
	items, err := kubectlGet(ctx, executor, "operatorgroups.operators.coreos.com", namespace, "")
	if err != nil {
		return nil, err
	}

	// Count OperatorGroups per namespace
	countByNS := make(map[string]int)
	for _, item := range items {
		var og map[string]interface{}
		if err := json.Unmarshal(item, &og); err != nil {
			continue
		}
		metadata, _ := og["metadata"].(map[string]interface{})
		ns, _ := metadata["namespace"].(string)
		countByNS[ns]++
	}

	var results []AnalyzeResult
	for ns, count := range countByNS {
		if count > 1 {
			results = append(results, AnalyzeResult{
				Kind:  "OperatorGroup",
				Name:  ns,
				Error: []string{fmt.Sprintf("%d OperatorGroups in namespace %s; this can break CSV resolution", count, ns)},
			})
		}
	}
	return results, nil
}

// ==================== Anonymization (referenced from k8sgpt anonymize feature) ====================

var anonymizePatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),                                     // IPv4
	regexp.MustCompile(`\b[0-9a-fA-F]{1,4}(:[0-9a-fA-F]{1,4}){7}\b`),                                  // IPv6
	regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`),                        // email
	regexp.MustCompile(`\b[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*\.svc\b`),   // K8s service DNS
}

func anonymizeResults(results []AnalyzeResult) {
	for i := range results {
		for j, errMsg := range results[i].Error {
			for _, pattern := range anonymizePatterns {
				errMsg = pattern.ReplaceAllStringFunc(errMsg, maskString)
			}
			results[i].Error[j] = errMsg
		}
	}
}

func maskString(s string) string {
	if len(s) <= 3 {
		return "***"
	}
	return s[:2] + strings.Repeat("*", len(s)-3) + s[len(s)-1:]
}

// ==================== Analysis Cache ====================

type cacheEntry struct {
	response  *AnalyzeResponse
	expiresAt time.Time
}

var (
	analysisCache     = make(map[string]*cacheEntry)
	analysisCacheLock sync.Mutex
	cacheTTL          = 2 * time.Minute
)

// AnalyzeWithCache wraps AnalyzeWithOptions with a simple in-memory cache.
func (e *Executor) AnalyzeWithCache(ctx context.Context, filters []string, namespace, labelSelector string, explain, withStats bool) (*AnalyzeResponse, error) {
	return e.AnalyzeWithCacheForCluster(ctx, 0, filters, namespace, labelSelector, explain, withStats)
}

// AnalyzeWithCacheForCluster wraps AnalyzeWithCluster with a simple in-memory cache.
func (e *Executor) AnalyzeWithCacheForCluster(ctx context.Context, clusterID uint, filters []string, namespace, labelSelector string, explain, withStats bool) (*AnalyzeResponse, error) {
	cacheKey := fmt.Sprintf("%d|%v|%s|%s|%v|%v", clusterID, filters, namespace, labelSelector, explain, withStats)

	analysisCacheLock.Lock()
	if entry, ok := analysisCache[cacheKey]; ok && time.Now().Before(entry.expiresAt) {
		analysisCacheLock.Unlock()
		return entry.response, nil
	}
	analysisCacheLock.Unlock()

	resp, err := e.AnalyzeWithCluster(ctx, clusterID, filters, namespace, labelSelector, explain, withStats)
	if err != nil {
		return nil, err
	}

	analysisCacheLock.Lock()
	analysisCache[cacheKey] = &cacheEntry{response: resp, expiresAt: time.Now().Add(cacheTTL)}
	analysisCacheLock.Unlock()

	return resp, nil
}

// InvalidateCache clears the analysis cache.
func InvalidateCache() {
	analysisCacheLock.Lock()
	analysisCache = make(map[string]*cacheEntry)
	analysisCacheLock.Unlock()
}
