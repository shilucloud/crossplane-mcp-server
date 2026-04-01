package tools

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestListProviders_Healthy(t *testing.T) {
	scheme := runtime.NewScheme()

	provider := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-aws",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-aws:v1.0.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Installed",
						"status": "True",
						"reason": "HealthyPackage",
					},
					map[string]interface{}{
						"type":   "Healthy",
						"status": "True",
						"reason": "ProviderHealthy",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, provider)
	ctx := context.Background()

	result, err := ListProviders(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "provider-aws", result[0].Name)
	assert.Equal(t, "v1.0.0", result[0].Version)
	assert.True(t, result[0].Installed)
	assert.True(t, result[0].Health)
	assert.Equal(t, "Healthy", result[0].State)
}

func TestListProviders_Unhealthy(t *testing.T) {
	scheme := runtime.NewScheme()

	provider := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-gcp",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-gcp:v0.5.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Installed",
						"status": "True",
						"reason": "HealthyPackage",
					},
					map[string]interface{}{
						"type":   "Healthy",
						"status": "False",
						"reason": "ProviderNotHealthy",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, provider)
	ctx := context.Background()

	result, err := ListProviders(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "provider-gcp", result[0].Name)
	assert.True(t, result[0].Installed)
	assert.False(t, result[0].Health)
	assert.Equal(t, "Unhealthy", result[0].State)
}

func TestListProviders_NotInstalled(t *testing.T) {
	scheme := runtime.NewScheme()

	provider := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-azure",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-azure:v2.0.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Installed",
						"status": "False",
						"reason": "PackageNotInstalled",
					},
					map[string]interface{}{
						"type":   "Healthy",
						"status": "False",
						"reason": "ProviderNotInstalled",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, provider)
	ctx := context.Background()

	result, err := ListProviders(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "provider-azure", result[0].Name)
	assert.False(t, result[0].Installed)
	assert.False(t, result[0].Health)
	assert.Equal(t, "Unhealthy", result[0].State)
}

func TestListProviders_UnknownState(t *testing.T) {
	scheme := runtime.NewScheme()

	provider := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-unknown",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-unknown:v1.0.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, provider)
	ctx := context.Background()

	result, err := ListProviders(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Installing", result[0].State)
}

func TestListProviders_Multiple(t *testing.T) {
	scheme := runtime.NewScheme()

	provider1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-aws",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-aws:v1.0.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Installed",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Healthy",
						"status": "True",
					},
				},
			},
		},
	}

	provider2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "pkg.crossplane.io/v1",
			"kind":       "Provider",
			"metadata": map[string]interface{}{
				"name": "provider-gcp",
			},
			"spec": map[string]interface{}{
				"package": "xpkg.upbound.io/crossplane/provider-gcp:v0.5.0",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Installed",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Healthy",
						"status": "False",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, provider1, provider2)
	ctx := context.Background()

	result, err := ListProviders(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 2)

	for _, p := range result {
		if p.Name == "provider-aws" {
			assert.True(t, p.Health)
			assert.Equal(t, "Healthy", p.State)
		}
		if p.Name == "provider-gcp" {
			assert.False(t, p.Health)
			assert.Equal(t, "Unhealthy", p.State)
		}
	}
}

func TestExtractProviderConditions(t *testing.T) {
	tests := []struct {
		name              string
		conditions        []interface{}
		expectedInstalled bool
		expectedHealthy   bool
	}{
		{
			name: "both true",
			conditions: []interface{}{
				map[string]interface{}{"type": "Installed", "status": "True"},
				map[string]interface{}{"type": "Healthy", "status": "True"},
			},
			expectedInstalled: true,
			expectedHealthy:   true,
		},
		{
			name: "both false",
			conditions: []interface{}{
				map[string]interface{}{"type": "Installed", "status": "False"},
				map[string]interface{}{"type": "Healthy", "status": "False"},
			},
			expectedInstalled: false,
			expectedHealthy:   false,
		},
		{
			name: "installed true, healthy false",
			conditions: []interface{}{
				map[string]interface{}{"type": "Installed", "status": "True"},
				map[string]interface{}{"type": "Healthy", "status": "False"},
			},
			expectedInstalled: true,
			expectedHealthy:   false,
		},
		{
			name:              "no conditions",
			conditions:        []interface{}{},
			expectedInstalled: false,
			expectedHealthy:   false,
		},
		{
			name:              "empty conditions",
			conditions:        nil,
			expectedInstalled: false,
			expectedHealthy:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := map[string]interface{}{
				"status": map[string]interface{}{
					"conditions": tt.conditions,
				},
			}

			installed, healthy := extractProviderConditions(obj)
			assert.Equal(t, tt.expectedInstalled, installed)
			assert.Equal(t, tt.expectedHealthy, healthy)
		})
	}
}

func TestExtractVersion(t *testing.T) {
	tests := []struct {
		name     string
		pkg      string
		expected string
	}{
		{"with version tag", "xpkg.upbound.io/crossplane/provider-aws:v1.0.0", "v1.0.0"},
		{"with v prefix", "xpkg.upbound.io/crossplane/provider-aws:v0.5.0-beta", "v0.5.0-beta"},
		{"no version tag", "xpkg.upbound.io/crossplane/provider-aws", "xpkg.upbound.io/crossplane/provider-aws"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractVersion(tt.pkg)
			assert.Equal(t, tt.expected, result)
		})
	}
}
