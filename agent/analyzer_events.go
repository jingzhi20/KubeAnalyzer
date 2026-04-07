package main

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// networkComponentLabels defines label selectors for critical network component pods.
var networkComponentLabels = []struct {
	Description   string
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

// NetworkComponentPodsAnalyzer scans network component pods for errors.
type NetworkComponentPodsAnalyzer struct{}

func (a *NetworkComponentPodsAnalyzer) Name() string { return "NetworkComponentPods" }

func (a *NetworkComponentPodsAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	var results []AnalyzeResult

	for _, comp := range networkComponentLabels {
		pods, err := client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: comp.LabelSelector,
		})
		if err != nil || len(pods.Items) == 0 {
			continue
		}

		for _, pod := range pods.Items {
			var failures []string
			podName := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

			// Check pod phase
			if pod.Status.Phase == corev1.PodPending {
				failures = append(failures, fmt.Sprintf("Pod %s is Pending", podName))
			}
			if pod.Status.Phase == corev1.PodFailed {
				failures = append(failures, fmt.Sprintf("Pod %s is Failed", podName))
			}

			// Check container statuses
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
					failures = append(failures, fmt.Sprintf("container %s is %s", cs.Name, cs.State.Waiting.Reason))
				}
				if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
					failures = append(failures, fmt.Sprintf("container %s terminated with exit code %d", cs.Name, cs.State.Terminated.ExitCode))
				}
				if !cs.Ready && cs.State.Running != nil {
					failures = append(failures, fmt.Sprintf("container %s running but not ready", cs.Name))
				}
			}

			// Check warning events
			events, err := client.Clientset.CoreV1().Events(pod.Namespace).List(ctx, metav1.ListOptions{
				FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,type=Warning", pod.Name),
			})
			if err == nil {
				for _, evt := range events.Items {
					if evt.Message != "" {
						msg := fmt.Sprintf("[Event] %s: %s", evt.Reason, evt.Message)
						if evt.Count > 1 {
							msg += fmt.Sprintf(" (x%d)", evt.Count)
						}
						failures = append(failures, msg)
					}
				}
			}

			if len(failures) > 0 {
				parent, _ := client.GetParent(pod.ObjectMeta)
				results = append(results, AnalyzeResult{
					Kind:      fmt.Sprintf("NetworkPod[%s]", comp.Description),
					Name:      podName,
					Error:     failures,
					ParentObj: parent,
				})
			}
		}
	}
	return results, nil
}

// WarningEventsAnalyzer aggregates Warning events and Normal events with error-like reasons.
// Kubernetes events only have type=Normal or type=Warning, but some Normal events
// may still indicate problems through their Reason field (e.g., OOMKilled, Evicted).
type WarningEventsAnalyzer struct{}

func (a *WarningEventsAnalyzer) Name() string { return "WarningEvents" }

// errorReasonPatterns defines Reason patterns that indicate errors even in Normal events.
var errorReasonPatterns = []string{
	"Failed", "Error", "BackOff", "Evicted", "OOMKilled",
	"DeadlineExceeded", "Insufficient", "Unhealthy", "NodeNotReady",
	"ContainerStatusUnknown", "CrashLoopBackOff", "ImagePullBackOff",
}

// isProblemEvent returns true if the event should be reported as a problem.
func isProblemEvent(evt corev1.Event) bool {
	// Always include Warning type events
	if evt.Type == "Warning" {
		return true
	}
	// Check if Normal event has error-like Reason
	for _, pattern := range errorReasonPatterns {
		if strings.Contains(evt.Reason, pattern) {
			return true
		}
	}
	return false
}

func (a *WarningEventsAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	// Fetch all events (both Warning and Normal), filter in code
	events, err := client.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}

	// Group by involved object
	type eventGroup struct {
		kind   string
		name   string
		errors []string
	}
	grouped := make(map[string]*eventGroup)
	var order []string

	for _, evt := range events.Items {
		if evt.Message == "" {
			continue
		}
		// Filter: Warning OR Normal with error-like Reason
		if !isProblemEvent(evt) {
			continue
		}
		key := fmt.Sprintf("%s/%s/%s", evt.Namespace, evt.InvolvedObject.Kind, evt.InvolvedObject.Name)
		if _, ok := grouped[key]; !ok {
			resName := evt.InvolvedObject.Name
			if evt.Namespace != "" {
				resName = evt.Namespace + "/" + resName
			}
			grouped[key] = &eventGroup{
				kind: fmt.Sprintf("Event[%s]", evt.InvolvedObject.Kind),
				name: resName,
			}
			order = append(order, key)
		}
		// Prefix with event type to distinguish Warning vs Normal
		typePrefix := "[Warning]"
		if evt.Type == "Normal" {
			typePrefix = "[Normal]"
		}
		msg := fmt.Sprintf("%s %s: %s", typePrefix, evt.Reason, evt.Message)
		if evt.Count > 1 {
			msg += fmt.Sprintf(" (x%d)", evt.Count)
		}
		grouped[key].errors = append(grouped[key].errors, msg)
	}

	var results []AnalyzeResult
	for _, key := range order {
		g := grouped[key]
		results = append(results, AnalyzeResult{Kind: g.kind, Name: g.name, Error: g.errors})
	}
	return results, nil
}

// SecretAnalyzer checks for Secret issues (TLS field completeness).
type SecretAnalyzer struct{}

func (a *SecretAnalyzer) Name() string { return "Secret" }

func (a *SecretAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	secrets, err := client.Clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, err
	}
	var results []AnalyzeResult
	for _, secret := range secrets.Items {
		name := secret.Name
		ns := secret.Namespace
		// Skip system secrets
		if strings.HasPrefix(name, "default-token-") || strings.HasPrefix(name, "sh.helm.") {
			continue
		}
		var failures []string
		if secret.Type == corev1.SecretTypeTLS {
			if _, ok := secret.Data["tls.crt"]; !ok {
				failures = append(failures, fmt.Sprintf("TLS Secret %s/%s missing tls.crt", ns, name))
			}
			if _, ok := secret.Data["tls.key"]; !ok {
				failures = append(failures, fmt.Sprintf("TLS Secret %s/%s missing tls.key", ns, name))
			}
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "Secret", Name: fmt.Sprintf("%s/%s", ns, name), Error: failures})
		}
	}
	return results, nil
}

// PVAnalyzer checks for PersistentVolume issues (Released/Failed state).
type PVAnalyzer struct{}

func (a *PVAnalyzer) Name() string { return "PersistentVolume" }

func (a *PVAnalyzer) Analyze(ctx context.Context, client *K8sClient, namespace, labelSelector string) ([]AnalyzeResult, error) {
	pvs, err := client.Clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil
	}
	var results []AnalyzeResult
	for _, pv := range pvs.Items {
		var failures []string
		if pv.Status.Phase == corev1.VolumeReleased {
			failures = append(failures, fmt.Sprintf("PersistentVolume %s is in Released state and should be cleaned up", pv.Name))
		}
		if pv.Status.Phase == corev1.VolumeFailed {
			reason := pv.Status.Reason
			msg := fmt.Sprintf("PersistentVolume %s is in Failed state", pv.Name)
			if reason != "" {
				msg += ": " + reason
			}
			failures = append(failures, msg)
		}
		if len(failures) > 0 {
			results = append(results, AnalyzeResult{Kind: "PersistentVolume", Name: pv.Name, Error: failures})
		}
	}
	return results, nil
}
