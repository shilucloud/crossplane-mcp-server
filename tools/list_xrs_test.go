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

func TestGetXRInfo(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd)
	ctx := context.Background()

	result, err := GetXRInfo(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "platform.example.com", result[0].Group)
	assert.Equal(t, "v1alpha1", result[0].Version)
	assert.Equal(t, "xnetworks", result[0].Resource)
	assert.Equal(t, "XNetwork", result[0].Kind)
	assert.Equal(t, "Namespaced", result[0].Scope)
}

func TestGetXRInfo_ClusterScope(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xclusters.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XCluster",
					"plural": "xclusters",
				},
				"scope": "Cluster",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd)
	ctx := context.Background()

	result, err := GetXRInfo(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Cluster", result[0].Scope)
}

func TestGetXRInfo_NoServedVersion(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xinvalids.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XInvalid",
					"plural": "xinvalids",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  false,
						"storage": true,
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd)
	ctx := context.Background()

	result, err := GetXRInfo(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 0)
}

func TestGetXRInfo_MultipleXRDs(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	xrd2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xdatabases.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XDatabase",
					"plural": "xdatabases",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd1, xrd2)
	ctx := context.Background()

	result, err := GetXRInfo(ctx, fakeClient)

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListXrs_Namespaced(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	xr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":              "test-xr",
				"namespace":         "default",
				"creationTimestamp": "2024-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"crossplane": map[string]interface{}{
					"compositionRef": map[string]interface{}{
						"name": "test-composition",
					},
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "ResourcesAvailable",
						"message": "Ready",
					},
					map[string]interface{}{
						"type":    "Synced",
						"status":  "True",
						"reason":  "ResourceSynced",
						"message": "Synced",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd, xr)
	ctx := context.Background()

	result, err := ListXrs(ctx, fakeClient)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.XRs, 1)
	assert.Len(t, result.Warnings, 0)
	assert.Equal(t, "test-xr", result.XRs[0].Name)
	assert.Equal(t, "default", result.XRs[0].Namespace)
	assert.Equal(t, "XNetwork", result.XRs[0].Kind)
	assert.Equal(t, "True", result.XRs[0].Ready)
	assert.Equal(t, "True", result.XRs[0].Synced)
	assert.Equal(t, "Namespaced", result.XRs[0].Scope)
	assert.Equal(t, "test-composition", result.XRs[0].CompositionRef)
}

func TestListXrs_ClusterScope(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xclusters.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XCluster",
					"plural": "xclusters",
				},
				"scope": "Cluster",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	xr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XCluster",
			"metadata": map[string]interface{}{
				"name":              "test-cluster",
				"creationTimestamp": "2024-01-01T00:00:00Z",
			},
			"spec": map[string]interface{}{
				"compositionRef": map[string]interface{}{
					"name": "cluster-composition",
				},
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
						"reason": "ResourcesAvailable",
					},
					map[string]interface{}{
						"type":   "Synced",
						"status": "True",
						"reason": "ResourceSynced",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd, xr)
	ctx := context.Background()

	result, err := ListXrs(ctx, fakeClient)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.XRs, 1)
	assert.Equal(t, "test-cluster", result.XRs[0].Name)
	assert.Equal(t, "Cluster", result.XRs[0].Scope)
	assert.Empty(t, result.XRs[0].Namespace)
}

func TestListXrs_Empty(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  false,
						"storage": true,
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd)
	ctx := context.Background()

	result, err := ListXrs(ctx, fakeClient)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.XRs, 0)
}

func TestListXrs_WarningsOnError(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	xr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":              "test-xr",
				"namespace":         "default",
				"creationTimestamp": "2024-01-01T00:00:00Z",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd, xr)
	ctx := context.Background()

	result, err := ListXrs(ctx, fakeClient)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.XRs, 1)
}

func TestListXrs_MultipleXRs(t *testing.T) {
	scheme := runtime.NewScheme()

	xrd := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.crossplane.io/v2",
			"kind":       "CompositeResourceDefinition",
			"metadata": map[string]interface{}{
				"name": "xnetworks.platform.example.com",
			},
			"spec": map[string]interface{}{
				"group": "platform.example.com",
				"names": map[string]interface{}{
					"kind":   "XNetwork",
					"plural": "xnetworks",
				},
				"scope": "Namespaced",
				"versions": []interface{}{
					map[string]interface{}{
						"name":    "v1alpha1",
						"served":  true,
						"storage": true,
					},
				},
			},
		},
	}

	xr1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":              "xr-1",
				"namespace":         "default",
				"creationTimestamp": "2024-01-01T00:00:00Z",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
					map[string]interface{}{
						"type":   "Synced",
						"status": "True",
					},
				},
			},
		},
	}

	xr2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "platform.example.com/v1alpha1",
			"kind":       "XNetwork",
			"metadata": map[string]interface{}{
				"name":              "xr-2",
				"namespace":         "production",
				"creationTimestamp": "2024-01-02T00:00:00Z",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "False",
						"reason":  "ResourcesUnavailable",
						"message": "Resources not available",
					},
					map[string]interface{}{
						"type":   "Synced",
						"status": "True",
					},
				},
			},
		},
	}

	fakeClient := dynamicfake.NewSimpleDynamicClient(scheme, xrd, xr1, xr2)
	ctx := context.Background()

	result, err := ListXrs(ctx, fakeClient)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.XRs, 2)

	assert.Equal(t, "xr-1", result.XRs[0].Name)
	assert.Equal(t, "True", result.XRs[0].Ready)

	assert.Equal(t, "xr-2", result.XRs[1].Name)
	assert.Contains(t, result.XRs[1].Ready, "False")
}
