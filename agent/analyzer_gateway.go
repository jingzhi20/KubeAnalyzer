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
	gatewayClassGVR = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gatewayclasses"}
	gatewayGVR      = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "gateways"}
	httpRouteGVR    = schema.GroupVersionResource{Group: "gateway.networking.k8s.io", Version: "v1", Resource: "httproutes"}
)

// GatewayClassAnalyzer checks for GatewayClass issues.
type GatewayClassAnalyzer struct{}

func (g *GatewayClassAnalyzer) Name() string { return "GatewayClass" }

func (g *GatewayClassAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(gatewayClassGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Gateway API CRDs not installed, skip silently
		return nil, nil
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()

		// Check conditions for Accepted status
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			if condType == "Accepted" && condStatus != "True" {
				failures = append(failures, fmt.Sprintf(
					"GatewayClass %s is not Accepted: %s - %s", name, condReason, condMsg))
			}
		}

		// Check if controllerName is set
		controllerName, _, _ := unstructured.NestedString(item.Object, "spec", "controllerName")
		if controllerName == "" {
			failures = append(failures, fmt.Sprintf(
				"GatewayClass %s has no controllerName specified", name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "GatewayClass",
				Name:  name,
				Error: failures,
			})
		}
	}

	return results, nil
}

// GatewayAnalyzer checks for Gateway issues.
type GatewayAnalyzer struct{}

func (g *GatewayAnalyzer) Name() string { return "Gateway" }

func (g *GatewayAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(gatewayGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	// Pre-fetch GatewayClasses for existence check
	gcList, _ := client.DynamicClient.Resource(gatewayClassGVR).List(ctx, metav1.ListOptions{})
	gcNames := make(map[string]bool)
	if gcList != nil {
		for _, gc := range gcList.Items {
			gcNames[gc.GetName()] = true
		}
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// 1. Check GatewayClassName reference
		gcName, _, _ := unstructured.NestedString(item.Object, "spec", "gatewayClassName")
		if gcName == "" {
			failures = append(failures, fmt.Sprintf(
				"Gateway %s/%s has no gatewayClassName specified", ns, name))
		} else if !gcNames[gcName] {
			failures = append(failures, fmt.Sprintf(
				"Gateway %s/%s references non-existent GatewayClass %q", ns, name, gcName))
		}

		// 2. Check conditions
		conditions := getConditions(item)
		for _, cond := range conditions {
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			condMsg, _ := cond["message"].(string)
			condReason, _ := cond["reason"].(string)

			switch condType {
			case "Accepted":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"Gateway %s/%s not Accepted: %s - %s", ns, name, condReason, condMsg))
				}
			case "Programmed":
				if condStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"Gateway %s/%s not Programmed: %s - %s", ns, name, condReason, condMsg))
				}
			}
		}

		// 3. Check listeners for conflicts
		listeners, _, _ := unstructured.NestedSlice(item.Object, "spec", "listeners")
		listenerPorts := make(map[int64]string)
		for _, l := range listeners {
			lMap, ok := l.(map[string]interface{})
			if !ok {
				continue
			}
			lName, _ := lMap["name"].(string)
			port, _, _ := unstructured.NestedInt64(lMap, "port")
			if existing, ok := listenerPorts[port]; ok {
				failures = append(failures, fmt.Sprintf(
					"Gateway %s/%s has duplicate port %d on listeners %q and %q",
					ns, name, port, existing, lName))
			}
			listenerPorts[port] = lName
		}

		// 4. Check addresses assigned
		addresses, _, _ := unstructured.NestedSlice(item.Object, "status", "addresses")
		if len(addresses) == 0 {
			failures = append(failures, fmt.Sprintf(
				"Gateway %s/%s has no addresses assigned", ns, name))
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "Gateway",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// HTTPRouteAnalyzer checks for HTTPRoute issues.
type HTTPRouteAnalyzer struct{}

func (h *HTTPRouteAnalyzer) Name() string { return "HTTPRoute" }

func (h *HTTPRouteAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	list, err := client.DynamicClient.Resource(httpRouteGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	var results []AnalyzeResult

	for _, item := range list.Items {
		var failures []string
		name := item.GetName()
		ns := item.GetNamespace()

		// 1. Check parentRefs (Gateway references) and namespace policy
		parentRefs, _, _ := unstructured.NestedSlice(item.Object, "spec", "parentRefs")
		if len(parentRefs) == 0 {
			failures = append(failures, fmt.Sprintf(
				"HTTPRoute %s/%s has no parentRefs (not attached to any Gateway)", ns, name))
		} else {
			for i, ref := range parentRefs {
				refMap, ok := ref.(map[string]interface{})
				if !ok {
					continue
				}
				gwName, _ := refMap["name"].(string)
				gwNs := ns
				if nsVal, ok := refMap["namespace"].(string); ok && nsVal != "" {
					gwNs = nsVal
				}
				gwObj, err := client.DynamicClient.Resource(gatewayGVR).Namespace(gwNs).Get(ctx, gwName, metav1.GetOptions{})
				if err != nil {
					failures = append(failures, fmt.Sprintf(
						"HTTPRoute %s/%s parentRef[%d] references non-existent Gateway %s/%s",
						ns, name, i, gwNs, gwName))
					continue
				}

				// Check AllowedRoutes.Namespaces policy on Gateway listeners
				sectionName, _ := refMap["sectionName"].(string)
				listeners, _, _ := unstructured.NestedSlice(gwObj.Object, "spec", "listeners")
				for _, l := range listeners {
					lMap, ok := l.(map[string]interface{})
					if !ok {
						continue
					}
					lName, _ := lMap["name"].(string)
					// If sectionName specified, only check matching listener
					if sectionName != "" && lName != sectionName {
						continue
					}

					allowedFrom, _, _ := unstructured.NestedString(lMap, "allowedRoutes", "namespaces", "from")
					switch allowedFrom {
					case "Same":
						if ns != gwNs {
							failures = append(failures, fmt.Sprintf(
								"HTTPRoute %s/%s parentRef[%d]: Gateway %s/%s listener %q only allows routes from Same namespace",
								ns, name, i, gwNs, gwName, lName))
						}
					case "Selector":
						// Check if route namespace matches the selector
						selectorMap, _, _ := unstructured.NestedMap(lMap, "allowedRoutes", "namespaces", "selector", "matchLabels")
						if len(selectorMap) > 0 {
							routeNsObj, err := client.Clientset.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
							if err == nil {
								matches := true
								for k, v := range selectorMap {
									vStr, _ := v.(string)
									if routeNsObj.Labels[k] != vStr {
										matches = false
										break
									}
								}
								if !matches {
									failures = append(failures, fmt.Sprintf(
										"HTTPRoute %s/%s parentRef[%d]: namespace %q does not match Gateway %s/%s listener %q selector",
										ns, name, i, ns, gwNs, gwName, lName))
								}
							}
						}
					}
					// "All" or empty: no restriction
				}
			}
		}

		// 2. Check backend refs with port matching
		rules, _, _ := unstructured.NestedSlice(item.Object, "spec", "rules")
		for ri, rule := range rules {
			ruleMap, ok := rule.(map[string]interface{})
			if !ok {
				continue
			}
			backendRefs, _, _ := unstructured.NestedSlice(ruleMap, "backendRefs")
			for bi, backend := range backendRefs {
				bMap, ok := backend.(map[string]interface{})
				if !ok {
					continue
				}
				svcName, _ := bMap["name"].(string)
				svcNs := ns
				if nsVal, ok := bMap["namespace"].(string); ok && nsVal != "" {
					svcNs = nsVal
				}
				kind := "Service"
				if k, ok := bMap["kind"].(string); ok && k != "" {
					kind = k
				}
				if kind == "Service" && svcName != "" {
					svc, err := client.Clientset.CoreV1().Services(svcNs).Get(ctx, svcName, metav1.GetOptions{})
					if err != nil {
						failures = append(failures, fmt.Sprintf(
							"HTTPRoute %s/%s rules[%d].backendRefs[%d] references non-existent Service %s/%s",
							ns, name, ri, bi, svcNs, svcName))
					} else {
						// Port matching: check backend port exists on Service
						backendPort, _, _ := unstructured.NestedInt64(bMap, "port")
						if backendPort > 0 {
							portFound := false
							for _, sp := range svc.Spec.Ports {
								if int64(sp.Port) == backendPort {
									portFound = true
									break
								}
							}
							if !portFound {
								var availablePorts []string
								for _, sp := range svc.Spec.Ports {
									availablePorts = append(availablePorts, fmt.Sprintf("%d", sp.Port))
								}
								failures = append(failures, fmt.Sprintf(
									"HTTPRoute %s/%s rules[%d].backendRefs[%d] port %d not found on Service %s/%s (available: %s)",
									ns, name, ri, bi, backendPort, svcNs, svcName, strings.Join(availablePorts, ",")))
							}
						}
					}
				}
			}
		}

		// 3. Check route status conditions
		parents, _, _ := unstructured.NestedSlice(item.Object, "status", "parents")
		for _, parent := range parents {
			pMap, ok := parent.(map[string]interface{})
			if !ok {
				continue
			}
			condSlice, _, _ := unstructured.NestedSlice(pMap, "conditions")
			for _, c := range condSlice {
				cMap, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				cType, _ := cMap["type"].(string)
				cStatus, _ := cMap["status"].(string)
				cMsg, _ := cMap["message"].(string)
				cReason, _ := cMap["reason"].(string)

				if cType == "Accepted" && cStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"HTTPRoute %s/%s not Accepted by parent: %s - %s",
						ns, name, cReason, cMsg))
				}
				if cType == "ResolvedRefs" && cStatus != "True" {
					failures = append(failures, fmt.Sprintf(
						"HTTPRoute %s/%s has unresolved refs: %s - %s",
						ns, name, cReason, cMsg))
				}
			}
		}

		if len(failures) > 0 {
			results = append(results, AnalyzeResult{
				Kind:  "HTTPRoute",
				Name:  fmt.Sprintf("%s/%s", ns, name),
				Error: failures,
			})
		}
	}

	return results, nil
}

// getConditions extracts status.conditions from an unstructured object.
func getConditions(item unstructured.Unstructured) []map[string]interface{} {
	conditions, _, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
	var result []map[string]interface{}
	for _, c := range conditions {
		if cMap, ok := c.(map[string]interface{}); ok {
			result = append(result, cMap)
		}
	}
	return result
}
