package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	xrdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v2",
		Resource: "compositeresourcedefinitions",
	}
	mrdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "managedresourcedefinitions",
	}
	providerGVR = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "providers",
	}
)

type CrossplaneInfo struct {
	// In crossplane v2 claims are removed, and XR's made namespaced, to
	// remove another layer of abstraction.
	// core version
	Version      string // e.g. "v2.0.1"
	MajorVersion int    // 1 or 2

	// feature flags based on version
	HasMRDs          bool // v2 only
	HasOperations    bool // v2 only
	HasNamespacedXRs bool // v2 only
	HasNamespacedMRs bool // v2 only

	// XRD summary
	TotalXRDs      int
	NamespacedXRDs int
	ClusterXRDs    int

	// total provider
	NumberOfProvider int

	// provider summary
	Providers []ProviderInfo
}

type ProviderInfo struct {
	Name      string
	Version   string
	Health    bool
	Installed bool
	State     string // Healthy, Unhealthy, Installing, Unknown
}

func GetCrossplaneInfo(ctx context.Context, dynamicClient dynamic.Interface, clientset kubernetes.Interface) (*CrossplaneInfo, error) {
	result := &CrossplaneInfo{}

	// Step 1: detect version from crossplane pod image tag
	pods, err := clientset.CoreV1().Pods("crossplane-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=crossplane",
	})
	if err == nil && len(pods.Items) > 0 {
		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				if container.Name == "crossplane" {
					result.Version = extractImageTag(container.Image)
					result.MajorVersion = extractMajorVersion(result.Version)
					break
				}
			}
		}
	}

	// Step 2: detect v2 features by checking if MRD CRD exists
	_, err = dynamicClient.Resource(mrdGVR).List(ctx, metav1.ListOptions{})
	result.HasMRDs = err == nil

	// Step 3: detect operations support
	_, err = dynamicClient.Resource(operationsGVR).List(ctx, metav1.ListOptions{})
	result.HasOperations = err == nil

	// Step 4: scan XRDs and check their scope
	xrds, err := dynamicClient.Resource(xrdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing XRDs: %w", err)
	}
	result.TotalXRDs = len(xrds.Items)
	for _, xrd := range xrds.Items {
		scope := getNestedString(xrd.Object, "spec", "scope")
		if scope == "Namespaced" {
			result.NamespacedXRDs++
		} else {
			result.ClusterXRDs++
		}
	}
	result.HasNamespacedXRs = result.NamespacedXRDs > 0

	// Step 5: list providers and their health
	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing providers: %w", err)
	}
	for _, p := range providers.Items {
		state := resolveProviderState(p.Object)
		version := getNestedString(p.Object, "status", "package")
		result.Providers = append(result.Providers, ProviderInfo{
			Name:      p.GetName(),
			Version:   version,
			Health:    state == "Healthy",
			Installed: true,
			State:     state,
		})
	}

	result.NumberOfProvider = len(providers.Items)

	return result, nil
}

func resolveProviderState(obj map[string]interface{}) string {
	conditions, ok := obj["status"].(map[string]interface{})["conditions"].([]interface{})
	if !ok {
		return "Unknown"
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condition["type"] == "Healthy" && condition["status"] == "True" {
			return "Healthy"
		}
		if condition["type"] == "Healthy" && condition["status"] == "False" {
			return "Unhealthy"
		}
	}
	return "Installing"
}

func extractImageTag(image string) string {
	for i := len(image) - 1; i >= 0; i-- {
		if image[i] == ':' {
			return image[i+1:]
		}
	}
	return "unknown"
}

func extractMajorVersion(version string) int {
	// e.g. "v2.0.1" -> 2, "v1.20.0" -> 1
	if len(version) < 2 {
		return 0
	}
	v := version
	if v[0] == 'v' {
		v = v[1:]
	}
	if len(v) > 0 && v[0] == '2' {
		return 2
	}
	return 1
}
