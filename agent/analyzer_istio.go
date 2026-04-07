package main

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	virtualServiceGVR      = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "virtualservices"}
	destinationRuleGVR     = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "destinationrules"}
	istioGatewayGVR        = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "gateways"}
	serviceEntryGVR        = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "serviceentries"}
	sidecarGVR             = schema.GroupVersionResource{Group: "networking.istio.io", Version: "v1beta1", Resource: "sidecars"}
	peerAuthenticationGVR  = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1beta1", Resource: "peerauthentications"}
	authorizationPolicyGVR = schema.GroupVersionResource{Group: "security.istio.io", Version: "v1beta1", Resource: "authorizationpolicies"}
)

// VirtualServiceAnalyzer checks Istio VirtualService CRDs.
type VirtualServiceAnalyzer struct{}

func (a *VirtualServiceAnalyzer) Name() string { return "VirtualService" }

func (a *VirtualServiceAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(virtualServiceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())

		// Check hosts
		hosts, _, _ := unstructured.NestedSlice(item.Object, "spec", "hosts")
		if len(hosts) == 0 {
			failures = append(failures, fmt.Sprintf("VirtualService %s has no hosts defined", fullName))
		}

		// Check gateway references
		gateways, _, _ := unstructured.NestedSlice(item.Object, "spec", "gateways")
		for _, gw := range gateways {
			gwStr, _ := gw.(string)
			if gwStr == "" || gwStr == "mesh" {
				continue
			}
			gwNs, gwName := ns, gwStr
			if parts := strings.SplitN(gwStr, "/", 2); len(parts) == 2 {
				gwNs, gwName = parts[0], parts[1]
			}
			_, err := client.DynamicClient.Resource(istioGatewayGVR).Namespace(gwNs).Get(ctx, gwName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf("VirtualService %s references Gateway %s/%s which does not exist", fullName, gwNs, gwName))
			}
		}

		// Check HTTP route destinations
		httpRoutes, _, _ := unstructured.NestedSlice(item.Object, "spec", "http")
		for i, route := range httpRoutes {
			rMap, _ := route.(map[string]interface{})
			dests, _ := rMap["route"].([]interface{})
			for _, dest := range dests {
				dMap, _ := dest.(map[string]interface{})
				destination, _ := dMap["destination"].(map[string]interface{})
				host, _ := destination["host"].(string)
				if host != "" && !strings.Contains(host, ".") {
					_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, host, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("VirtualService %s http[%d] destination host %s not found as service in %s", fullName, i, host, ns))
					}
				}
			}
		}

		// Check TCP route destinations
		tcpRoutes, _, _ := unstructured.NestedSlice(item.Object, "spec", "tcp")
		for i, route := range tcpRoutes {
			rMap, _ := route.(map[string]interface{})
			dests, _ := rMap["route"].([]interface{})
			for _, dest := range dests {
				dMap, _ := dest.(map[string]interface{})
				destination, _ := dMap["destination"].(map[string]interface{})
				host, _ := destination["host"].(string)
				if host != "" && !strings.Contains(host, ".") {
					_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, host, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("VirtualService %s tcp[%d] destination host %s not found as service in %s", fullName, i, host, ns))
					}
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "VirtualService", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// DestinationRuleAnalyzer checks Istio DestinationRule CRDs.
type DestinationRuleAnalyzer struct{}

func (a *DestinationRuleAnalyzer) Name() string { return "DestinationRule" }

func (a *DestinationRuleAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(destinationRuleGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())

		host, _, _ := unstructured.NestedString(item.Object, "spec", "host")
		if host == "" {
			failures = append(failures, fmt.Sprintf("DestinationRule %s has no host defined", fullName))
		}

		// Check subsets
		subsets, _, _ := unstructured.NestedSlice(item.Object, "spec", "subsets")
		for _, subset := range subsets {
			sMap, _ := subset.(map[string]interface{})
			sName, _ := sMap["name"].(string)
			if sName == "" {
				failures = append(failures, fmt.Sprintf("DestinationRule %s has a subset without name", fullName))
			}
			labels, _ := sMap["labels"].(map[string]interface{})
			if len(labels) == 0 {
				failures = append(failures, fmt.Sprintf("DestinationRule %s subset %s has no labels defined", fullName, sName))
			}
		}

		// Check TLS mode
		tlsMode, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "mode")
		if tlsMode != "" {
			validModes := map[string]bool{"DISABLE": true, "SIMPLE": true, "MUTUAL": true, "ISTIO_MUTUAL": true}
			if !validModes[tlsMode] {
				failures = append(failures, fmt.Sprintf("DestinationRule %s has invalid TLS mode: %s", fullName, tlsMode))
			}
			if tlsMode == "MUTUAL" {
				clientCert, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "clientCertificate")
				privateKey, _, _ := unstructured.NestedString(item.Object, "spec", "trafficPolicy", "tls", "privateKey")
				if clientCert == "" {
					failures = append(failures, fmt.Sprintf("DestinationRule %s MUTUAL TLS mode requires clientCertificate", fullName))
				}
				if privateKey == "" {
					failures = append(failures, fmt.Sprintf("DestinationRule %s MUTUAL TLS mode requires privateKey", fullName))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "DestinationRule", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// IstioGatewayAnalyzer checks Istio Gateway CRDs.
type IstioGatewayAnalyzer struct{}

func (a *IstioGatewayAnalyzer) Name() string { return "IstioGateway" }

func (a *IstioGatewayAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(istioGatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())

		servers, _, _ := unstructured.NestedSlice(item.Object, "spec", "servers")
		if len(servers) == 0 {
			failures = append(failures, fmt.Sprintf("Gateway %s has no servers defined", fullName))
		} else {
			for i, server := range servers {
				sMap, _ := server.(map[string]interface{})
				port, _ := sMap["port"].(map[string]interface{})
				if port == nil {
					failures = append(failures, fmt.Sprintf("Gateway %s server[%d] has no port defined", fullName, i))
				}
				hosts, _ := sMap["hosts"].([]interface{})
				if len(hosts) == 0 {
					failures = append(failures, fmt.Sprintf("Gateway %s server[%d] has no hosts defined", fullName, i))
				}
				// Check TLS credential
				if tls, ok := sMap["tls"].(map[string]interface{}); ok {
					mode, _ := tls["mode"].(string)
					if mode == "SIMPLE" || mode == "MUTUAL" {
						credName, _ := tls["credentialName"].(string)
						if credName != "" {
							_, err1 := client.Clientset.CoreV1().Secrets("istio-system").Get(ctx, credName, metav1.GetOptions{})
							if err1 != nil {
								_, err2 := client.Clientset.CoreV1().Secrets(ns).Get(ctx, credName, metav1.GetOptions{})
								if err2 != nil {
									failures = append(failures, fmt.Sprintf("Gateway %s server[%d] TLS credentialName %s secret not found", fullName, i, credName))
								}
							}
						}
					}
				}
			}
		}

		selector, _, _ := unstructured.NestedMap(item.Object, "spec", "selector")
		if len(selector) == 0 {
			failures = append(failures, fmt.Sprintf("Gateway %s has no selector (cannot bind to ingress gateway)", fullName))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "IstioGateway", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// ServiceEntryAnalyzer checks Istio ServiceEntry CRDs.
type ServiceEntryAnalyzer struct{}

func (a *ServiceEntryAnalyzer) Name() string { return "ServiceEntry" }

func (a *ServiceEntryAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(serviceEntryGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())

		hosts, _, _ := unstructured.NestedSlice(item.Object, "spec", "hosts")
		if len(hosts) == 0 {
			failures = append(failures, fmt.Sprintf("ServiceEntry %s has no hosts defined", fullName))
		}
		resolution, _, _ := unstructured.NestedString(item.Object, "spec", "resolution")
		validRes := map[string]bool{"NONE": true, "STATIC": true, "DNS": true, "DNS_ROUND_ROBIN": true}
		if resolution != "" && !validRes[resolution] {
			failures = append(failures, fmt.Sprintf("ServiceEntry %s has invalid resolution: %s", fullName, resolution))
		}
		if resolution == "STATIC" {
			endpoints, _, _ := unstructured.NestedSlice(item.Object, "spec", "endpoints")
			if len(endpoints) == 0 {
				failures = append(failures, fmt.Sprintf("ServiceEntry %s uses STATIC resolution but has no endpoints", fullName))
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "ServiceEntry", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// SidecarAnalyzer checks Istio Sidecar CRDs.
type SidecarAnalyzer struct{}

func (a *SidecarAnalyzer) Name() string { return "Sidecar" }

func (a *SidecarAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(sidecarGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())
		egress, _, _ := unstructured.NestedSlice(item.Object, "spec", "egress")
		if len(egress) == 0 {
			results = append(results, AnalyzeResult{Kind: "Sidecar", Name: fullName, Error: []string{fmt.Sprintf("Sidecar %s has no egress configuration", fullName)}})
		}
	}
	return results, nil
}

// PeerAuthenticationAnalyzer checks Istio PeerAuthentication CRDs.
type PeerAuthenticationAnalyzer struct{}

func (a *PeerAuthenticationAnalyzer) Name() string { return "PeerAuthentication" }

func (a *PeerAuthenticationAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(peerAuthenticationGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())

		mode, _, _ := unstructured.NestedString(item.Object, "spec", "mtls", "mode")
		if mode != "" {
			validModes := map[string]bool{"UNSET": true, "DISABLE": true, "PERMISSIVE": true, "STRICT": true}
			if !validModes[mode] {
				failures = append(failures, fmt.Sprintf("PeerAuthentication %s has invalid mTLS mode: %s", fullName, mode))
			}
			if mode == "DISABLE" {
				failures = append(failures, fmt.Sprintf("PeerAuthentication %s has mTLS DISABLED — traffic is unencrypted", fullName))
			}
		}
		// Check portLevelMtls
		portMtls, _, _ := unstructured.NestedMap(item.Object, "spec", "portLevelMtls")
		for port, conf := range portMtls {
			confMap, _ := conf.(map[string]interface{})
			pMode, _ := confMap["mode"].(string)
			if pMode == "DISABLE" {
				failures = append(failures, fmt.Sprintf("PeerAuthentication %s has mTLS DISABLED on port %s", fullName, port))
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "PeerAuthentication", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// AuthorizationPolicyAnalyzer checks Istio AuthorizationPolicy CRDs.
type AuthorizationPolicyAnalyzer struct{}

func (a *AuthorizationPolicyAnalyzer) Name() string { return "AuthorizationPolicy" }

func (a *AuthorizationPolicyAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(authorizationPolicyGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())

		action, _, _ := unstructured.NestedString(item.Object, "spec", "action")
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")

		if action == "DENY" && len(rules) == 0 {
			failures = append(failures, fmt.Sprintf("AuthorizationPolicy %s action is DENY but no rules defined (denies all traffic)", fullName))
		}
		if action == "ALLOW" && len(rules) == 0 {
			failures = append(failures, fmt.Sprintf("AuthorizationPolicy %s action is ALLOW but no rules defined (denies all traffic)", fullName))
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "AuthorizationPolicy", Name: fullName, Error: failures})
		}
	}
	return results, nil
}
