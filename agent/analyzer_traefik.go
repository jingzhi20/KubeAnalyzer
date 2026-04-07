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
	ingressRouteGVR    = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "ingressroutes"}
	ingressRouteTCPGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "ingressroutetcps"}
	ingressRouteUDPGVR = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "ingressrouteudps"}
	middlewareGVR      = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewares"}
	middlewareTCPGVR   = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "middlewaretcps"}
	traefikServiceGVR  = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "traefikservices"}
	tlsOptionGVR       = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "tlsoptions"}
	tlsStoreGVR        = schema.GroupVersionResource{Group: "traefik.io", Version: "v1alpha1", Resource: "tlsstores"}
)

// IngressRouteAnalyzer checks Traefik IngressRoute CRDs.
type IngressRouteAnalyzer struct{}

func (a *IngressRouteAnalyzer) Name() string { return "IngressRoute" }

func (a *IngressRouteAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(ingressRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, name)

		// Check entryPoints
		entryPoints, _, _ := unstructured.NestedSlice(item.Object, "spec", "entryPoints")
		if len(entryPoints) == 0 {
			failures = append(failures, fmt.Sprintf("IngressRoute %s has no entryPoints defined", fullName))
		}

		// Check routes
		routes, _, _ := unstructured.NestedSlice(item.Object, "spec", "routes")
		if len(routes) == 0 {
			failures = append(failures, fmt.Sprintf("IngressRoute %s has no routes defined", fullName))
		} else {
			for i, route := range routes {
				rMap, ok := route.(map[string]interface{})
				if !ok {
					continue
				}
				match, _ := rMap["match"].(string)
				if match == "" {
					failures = append(failures, fmt.Sprintf("IngressRoute %s route[%d] has no match rule", fullName, i))
				}
				// Check services
				services, _ := rMap["services"].([]interface{})
				for _, svc := range services {
					svcMap, _ := svc.(map[string]interface{})
					svcName, _ := svcMap["name"].(string)
					svcKind, _ := svcMap["kind"].(string)
					if svcName == "" {
						continue
					}
					if svcKind == "TraefikService" {
						_, err := client.DynamicClient.Resource(traefikServiceGVR).Namespace(ns).Get(ctx, svcName, metav1.GetOptions{})
						if err != nil {
							failures = append(failures, fmt.Sprintf("IngressRoute %s references TraefikService %s/%s which does not exist", fullName, ns, svcName))
						}
					} else {
						_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
						if err != nil {
							failures = append(failures, fmt.Sprintf("IngressRoute %s references Service %s/%s which does not exist", fullName, ns, svcName))
						}
					}
				}
				// Check middlewares
				middlewares, _ := rMap["middlewares"].([]interface{})
				for _, mw := range middlewares {
					mwMap, _ := mw.(map[string]interface{})
					mwName, _ := mwMap["name"].(string)
					if mwName == "" {
						continue
					}
					mwNameClean := mwName
					if idx := strings.Index(mwName, "@"); idx > 0 {
						mwNameClean = mwName[:idx]
					}
					mwNs := ns
					if mwNamespace, ok := mwMap["namespace"].(string); ok && mwNamespace != "" {
						mwNs = mwNamespace
					}
					_, err := client.DynamicClient.Resource(middlewareGVR).Namespace(mwNs).Get(ctx, mwNameClean, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("IngressRoute %s references Middleware %s/%s which does not exist", fullName, mwNs, mwNameClean))
					}
				}
			}
		}

		// Check TLS secret
		tlsMap, _, _ := unstructured.NestedMap(item.Object, "spec", "tls")
		if tlsMap != nil {
			secretName, _ := tlsMap["secretName"].(string)
			if secretName != "" {
				_, err := client.Clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf("IngressRoute %s references TLS secret %s/%s which does not exist", fullName, ns, secretName))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "IngressRoute", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// IngressRouteTCPAnalyzer checks Traefik IngressRouteTCP CRDs.
type IngressRouteTCPAnalyzer struct{}

func (a *IngressRouteTCPAnalyzer) Name() string { return "IngressRouteTCP" }

func (a *IngressRouteTCPAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(ingressRouteTCPGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())

		entryPoints, _, _ := unstructured.NestedSlice(item.Object, "spec", "entryPoints")
		if len(entryPoints) == 0 {
			failures = append(failures, fmt.Sprintf("IngressRouteTCP %s has no entryPoints defined", fullName))
		}
		routes, _, _ := unstructured.NestedSlice(item.Object, "spec", "routes")
		if len(routes) == 0 {
			failures = append(failures, fmt.Sprintf("IngressRouteTCP %s has no routes defined", fullName))
		} else {
			for _, route := range routes {
				rMap, _ := route.(map[string]interface{})
				services, _ := rMap["services"].([]interface{})
				for _, svc := range services {
					svcMap, _ := svc.(map[string]interface{})
					svcName, _ := svcMap["name"].(string)
					if svcName != "" {
						_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
						if err != nil {
							failures = append(failures, fmt.Sprintf("IngressRouteTCP %s references Service %s/%s which does not exist", fullName, ns, svcName))
						}
					}
				}
			}
		}
		tlsMap, _, _ := unstructured.NestedMap(item.Object, "spec", "tls")
		if tlsMap != nil {
			if secretName, _ := tlsMap["secretName"].(string); secretName != "" {
				_, err := client.Clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf("IngressRouteTCP %s references TLS secret %s/%s which does not exist", fullName, ns, secretName))
				}
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "IngressRouteTCP", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// IngressRouteUDPAnalyzer checks Traefik IngressRouteUDP CRDs.
type IngressRouteUDPAnalyzer struct{}

func (a *IngressRouteUDPAnalyzer) Name() string { return "IngressRouteUDP" }

func (a *IngressRouteUDPAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(ingressRouteUDPGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())

		entryPoints, _, _ := unstructured.NestedSlice(item.Object, "spec", "entryPoints")
		if len(entryPoints) == 0 {
			failures = append(failures, fmt.Sprintf("IngressRouteUDP %s has no entryPoints defined", fullName))
		}
		routes, _, _ := unstructured.NestedSlice(item.Object, "spec", "routes")
		for _, route := range routes {
			rMap, _ := route.(map[string]interface{})
			services, _ := rMap["services"].([]interface{})
			for _, svc := range services {
				svcMap, _ := svc.(map[string]interface{})
				svcName, _ := svcMap["name"].(string)
				if svcName != "" {
					_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("IngressRouteUDP %s references Service %s/%s which does not exist", fullName, ns, svcName))
					}
				}
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "IngressRouteUDP", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// MiddlewareAnalyzer checks Traefik Middleware CRDs.
type MiddlewareAnalyzer struct{}

func (a *MiddlewareAnalyzer) Name() string { return "Middleware" }

func (a *MiddlewareAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(middlewareGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		if len(spec) == 0 {
			failures = append(failures, fmt.Sprintf("Middleware %s has empty spec (no middleware type configured)", fullName))
		} else {
			// rateLimit check
			if rl, ok := spec["rateLimit"].(map[string]interface{}); ok {
				avg, _ := rl["average"].(int64)
				if avg <= 0 {
					failures = append(failures, fmt.Sprintf("Middleware %s rateLimit has invalid average", fullName))
				}
			}
			// retry check
			if retry, ok := spec["retry"].(map[string]interface{}); ok {
				attempts, _ := retry["attempts"].(int64)
				if attempts <= 0 {
					failures = append(failures, fmt.Sprintf("Middleware %s retry has invalid attempts", fullName))
				}
			}
			// circuitBreaker check
			if cb, ok := spec["circuitBreaker"].(map[string]interface{}); ok {
				expr, _ := cb["expression"].(string)
				if expr == "" {
					failures = append(failures, fmt.Sprintf("Middleware %s circuitBreaker has no expression defined", fullName))
				}
			}
			// ipAllowList / ipWhiteList check
			for _, key := range []string{"ipAllowList", "ipWhiteList"} {
				if ipList, ok := spec[key].(map[string]interface{}); ok {
					sourceRange, _ := ipList["sourceRange"].([]interface{})
					if len(sourceRange) == 0 {
						failures = append(failures, fmt.Sprintf("Middleware %s %s has empty sourceRange", fullName, key))
					}
				}
			}
			// CORS wildcard check
			if headers, ok := spec["headers"].(map[string]interface{}); ok {
				if origins, ok := headers["accessControlAllowOriginList"].([]interface{}); ok {
					for _, o := range origins {
						if s, _ := o.(string); s == "*" {
							failures = append(failures, fmt.Sprintf("Middleware %s allows all CORS origins (*) which may be insecure", fullName))
						}
					}
				}
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "Middleware", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// MiddlewareTCPAnalyzer checks Traefik MiddlewareTCP CRDs.
type MiddlewareTCPAnalyzer struct{}

func (a *MiddlewareTCPAnalyzer) Name() string { return "MiddlewareTCP" }

func (a *MiddlewareTCPAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(middlewareTCPGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())
		spec, _, _ := unstructured.NestedMap(item.Object, "spec")
		if len(spec) == 0 {
			results = append(results, AnalyzeResult{Kind: "MiddlewareTCP", Name: fullName, Error: []string{fmt.Sprintf("MiddlewareTCP %s has empty spec", fullName)}})
		}
	}
	return results, nil
}

// TraefikServiceAnalyzer checks Traefik TraefikService CRDs.
type TraefikServiceAnalyzer struct{}

func (a *TraefikServiceAnalyzer) Name() string { return "TraefikService" }

func (a *TraefikServiceAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(traefikServiceGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())

		// Check weighted services
		weightedSvcs, _, _ := unstructured.NestedSlice(item.Object, "spec", "weighted", "services")
		if len(weightedSvcs) == 0 {
			// Check mirroring
			mirrorName, _, _ := unstructured.NestedString(item.Object, "spec", "mirroring", "name")
			if mirrorName != "" {
				_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, mirrorName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf("TraefikService %s mirroring references Service %s/%s which does not exist", fullName, ns, mirrorName))
				}
			}
		} else {
			for _, svc := range weightedSvcs {
				svcMap, _ := svc.(map[string]interface{})
				svcName, _ := svcMap["name"].(string)
				kind, _ := svcMap["kind"].(string)
				if svcName != "" && kind != "TraefikService" {
					_, err := client.Clientset.CoreV1().Services(ns).Get(ctx, svcName, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf("TraefikService %s references Service %s/%s which does not exist", fullName, ns, svcName))
					}
				}
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "TraefikService", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// TLSOptionAnalyzer checks Traefik TLSOption CRDs.
type TLSOptionAnalyzer struct{}

func (a *TLSOptionAnalyzer) Name() string { return "TLSOption" }

func (a *TLSOptionAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(tlsOptionGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		fullName := fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName())
		minVersion, _, _ := unstructured.NestedString(item.Object, "spec", "minVersion")
		if minVersion != "" {
			valid := map[string]bool{"VersionTLS10": true, "VersionTLS11": true, "VersionTLS12": true, "VersionTLS13": true}
			if !valid[minVersion] {
				failures = append(failures, fmt.Sprintf("TLSOption %s has invalid minVersion: %s", fullName, minVersion))
			}
			if minVersion == "VersionTLS10" || minVersion == "VersionTLS11" {
				failures = append(failures, fmt.Sprintf("TLSOption %s uses deprecated TLS version: %s", fullName, minVersion))
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "TLSOption", Name: fullName, Error: failures})
		}
	}
	return results, nil
}

// TLSStoreAnalyzer checks Traefik TLSStore CRDs.
type TLSStoreAnalyzer struct{}

func (a *TLSStoreAnalyzer) Name() string { return "TLSStore" }

func (a *TLSStoreAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(tlsStoreGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, item := range list.Items {
		var failures []string
		ns := item.GetNamespace()
		fullName := fmt.Sprintf("%s/%s", ns, item.GetName())
		secretName, _, _ := unstructured.NestedString(item.Object, "spec", "defaultCertificate", "secretName")
		if secretName != "" {
			_, err := client.Clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				failures = append(failures, fmt.Sprintf("TLSStore %s references default certificate secret %s/%s which does not exist", fullName, ns, secretName))
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "TLSStore", Name: fullName, Error: failures})
		}
	}
	return results, nil
}
