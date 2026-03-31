package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func ListProviders(ctx context.Context, dynamicClient dynamic.Interface) ([]ProviderInfo, error) {
	result := []ProviderInfo{}

	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing providers: %w", err)
	}

	for _, p := range providers.Items {

		pkg := getNestedString(p.Object, "spec", "package")
		version := extractVersion(pkg)

		installed, healthy := extractProviderConditions(p.Object)

		result = append(result, ProviderInfo{
			Name:      p.GetName(),
			Version:   version,
			Installed: installed,
			Health:    healthy,
			State:     resolveProviderState(p.Object),
		})
	}

	return result, nil
}

func extractProviderConditions(obj map[string]interface{}) (installed bool, healthy bool) {
	conditions := getNestedSlice(obj, "status", "conditions")

	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		condType := getNestedString(cond, "type")
		status := getNestedString(cond, "status")

		if condType == "Installed" {
			installed = (status == "True")
		}

		if condType == "Healthy" {
			healthy = (status == "True")
		}
	}

	return
}

func extractVersion(pkg string) string {
	parts := strings.Split(pkg, ":")
	if len(parts) == 2 {
		return parts[1]
	}
	return pkg
}
