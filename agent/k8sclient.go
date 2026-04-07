package main

import (
	"context"
	"fmt"
	"os"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps client-go clients for Kubernetes API access.
type K8sClient struct {
	Clientset     kubernetes.Interface
	DynamicClient dynamic.Interface
	Config        *rest.Config
}

// NewK8sClient creates a new K8sClient.
// It tries InClusterConfig first, then falls back to KUBECONFIG env or default kubeconfig path.
func NewK8sClient() (*K8sClient, error) {
	var config *rest.Config

	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			home, _ := os.UserHomeDir()
			kubeconfig = home + "/.kube/config"
		}

		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = kubeconfig

		kubeContext := os.Getenv("AIOPS_KUBE_CONTEXT")

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			&clientcmd.ConfigOverrides{
				CurrentContext: kubeContext,
			})

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
		}
	}

	// Create standard clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &K8sClient{
		Clientset:     clientset,
		DynamicClient: dynamicClient,
		Config:        config,
	}, nil
}

// TestConnection verifies the cluster is reachable.
func (c *K8sClient) TestConnection() error {
	_, err := c.Clientset.Discovery().ServerVersion()
	return err
}

// GetNamespaces returns all namespace names.
func (c *K8sClient) GetNamespaces(ctx context.Context) ([]string, error) {
	nsList, err := c.Clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ns := range nsList.Items {
		names = append(names, ns.Name)
	}
	return names, nil
}

// GetParent traces OwnerReferences to find the top-level parent object.
func (c *K8sClient) GetParent(meta metav1.ObjectMeta) (string, bool) {
	if meta.OwnerReferences == nil {
		return "", false
	}
	for _, owner := range meta.OwnerReferences {
		switch owner.Kind {
		case "ReplicaSet":
			rs, err := c.Clientset.AppsV1().ReplicaSets(meta.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", false
			}
			if rs.OwnerReferences != nil {
				return c.GetParent(rs.ObjectMeta)
			}
			return "ReplicaSet/" + rs.Name, true
		case "Deployment":
			dep, err := c.Clientset.AppsV1().Deployments(meta.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", false
			}
			if dep.OwnerReferences != nil {
				return c.GetParent(dep.ObjectMeta)
			}
			return "Deployment/" + dep.Name, true
		case "StatefulSet":
			sts, err := c.Clientset.AppsV1().StatefulSets(meta.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", false
			}
			if sts.OwnerReferences != nil {
				return c.GetParent(sts.ObjectMeta)
			}
			return "StatefulSet/" + sts.Name, true
		case "DaemonSet":
			ds, err := c.Clientset.AppsV1().DaemonSets(meta.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", false
			}
			if ds.OwnerReferences != nil {
				return c.GetParent(ds.ObjectMeta)
			}
			return "DaemonSet/" + ds.Name, true
		case "Job":
			job, err := c.Clientset.BatchV1().Jobs(meta.Namespace).Get(context.Background(), owner.Name, metav1.GetOptions{})
			if err != nil {
				return "", false
			}
			if job.OwnerReferences != nil {
				return c.GetParent(job.ObjectMeta)
			}
			return "Job/" + job.Name, true
		}
	}
	return "", false
}

// FetchLatestEvent returns the most recent event for a given resource.
func (c *K8sClient) FetchLatestEvent(ctx context.Context, namespace, name string) (string, string, error) {
	events, err := c.Clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "involvedObject.name=" + name,
	})
	if err != nil {
		return "", "", err
	}
	if len(events.Items) == 0 {
		return "", "", nil
	}
	// Find most recent event
	latest := events.Items[0]
	for _, evt := range events.Items[1:] {
		if evt.LastTimestamp.After(latest.LastTimestamp.Time) {
			latest = evt
		}
	}
	return latest.Reason, latest.Message, nil
}
