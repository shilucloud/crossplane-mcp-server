package tools

import (
	"context"
	"testing"

	"github.com/shilucloud/crossplane-agent/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func init() {
	logging.Init("error")
}

func TestGetConditions_Namespaced(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "ResourcesAvailable",
						"message":            "Ready to use",
						"lastTransitionTime": "2024-01-01T00:00:00Z",
						"observedGeneration": float64(1),
					},
					map[string]interface{}{
						"type":               "Synced",
						"status":             "True",
						"reason":             "ResourceSynced",
						"message":            "Synced successfully",
						"lastTransitionTime": "2024-01-01T00:00:00Z",
						"observedGeneration": float64(1),
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	result, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.NoError(t, err)
	assert.Len(t, result, 2)

	assert.Equal(t, "Ready", result[0].Type)
	assert.Equal(t, "True", result[0].Status)
	assert.Equal(t, "ResourcesAvailable", result[0].Reason)
	assert.Equal(t, "Ready to use", result[0].Message)
	assert.Equal(t, int64(1), result[0].ObservedGeneration)

	assert.Equal(t, "Synced", result[1].Type)
	assert.Equal(t, "True", result[1].Status)
}

func TestGetConditions_ClusterScoped(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XCluster",
			"metadata": map[string]interface{}{
				"name": "test-cluster",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":               "Ready",
						"status":             "True",
						"reason":             "Ready",
						"lastTransitionTime": "2024-01-01T00:00:00Z",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	result, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xclusters", "test-cluster", "")

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Ready", result[0].Type)
	assert.Equal(t, "True", result[0].Status)
}

func TestGetConditions_EmptyConditions(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	result, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestGetConditions_NoStatus(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	_, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no status found")
}

func TestGetConditions_NoConditions(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"someOtherField": "value",
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	_, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no conditions found")
}

func TestGetConditions_MixedConditionTypes(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
						"reason": "Available",
					},
					map[string]interface{}{
						"type":   "Synced",
						"status": "False",
						"reason": "SyncFailed",
					},
					map[string]interface{}{
						"type":   "Claimed",
						"status": "True",
						"reason": "ClaimedSuccessfully",
					},
					map[string]interface{}{
						"type":   "InvalidType",
						"status": "True",
						"reason": "Valid",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	result, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.NoError(t, err)
	assert.Len(t, result, 4)

	readyCond := result[0]
	assert.Equal(t, "Ready", readyCond.Type)
	assert.Equal(t, "True", readyCond.Status)

	syncedCond := result[1]
	assert.Equal(t, "Synced", syncedCond.Type)
	assert.Equal(t, "False", syncedCond.Status)
}

func TestGetConditions_PartialConditionData(t *testing.T) {
	scheme := runtime.NewScheme()

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":      "test-resource",
				"namespace": "default",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type": "Ready",
					},
					map[string]interface{}{
						"type":   "Synced",
						"status": "True",
						"reason": "Synced",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, resource)
	ctx := context.Background()

	result, err := GetConditions(ctx, fakeClient, "platform.example.com", "v1alpha1", "xnetworks", "test-resource", "default")

	require.NoError(t, err)
	assert.Len(t, result, 2)

	assert.Equal(t, "Ready", result[0].Type)
	assert.Empty(t, result[0].Status)

	assert.Equal(t, "Synced", result[1].Type)
	assert.Equal(t, "True", result[1].Status)
}
