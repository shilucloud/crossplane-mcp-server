package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type ProviderHealth struct {
	ProviderName string
	Health       bool
}

func ListProviders(ctx context.Context, dynamicClient dynamic.Interface) ([]ProviderInfo, error) {
	result := []ProviderInfo{}

	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing providers: %w", err)
	}
	for _, p := range providers.Items {
		state := resolveProviderState(p.Object)
		version := getNestedString(p.Object, "status", "currentRevision")
		result = append(result, ProviderInfo{
			Name:      p.GetName(),
			Version:   version,
			State:     state,
			Installed: true,
		})
	}

	return result, nil
}

func CheckAllProviderHealth(ctx context.Context, dynamicClient dynamic.Interface) ([]ProviderHealth, error) {
	result := []ProviderHealth{}

	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("Error listing provider: %w", err)
	}

	for _, p := range providers.Items {
		state := resolveProviderState(p.Object)

		result = append(result, ProviderHealth{
			ProviderName: p.GetName(),
			Health:       state == "Healthy",
		})
	}
	return result, nil
}
