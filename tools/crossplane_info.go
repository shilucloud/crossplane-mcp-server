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
