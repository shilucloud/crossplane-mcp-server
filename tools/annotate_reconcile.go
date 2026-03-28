package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

type ReconcileResult struct {
	Name      string
	Namespace string
	Kind      string
	Triggered bool
	Message   string
}

func AnnotateReconcile(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	group, version, resource, name, namespace string,
) (*ReconcileResult, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// crossplane reconcile annotation
	patch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"crossplane.io/paused":                   "false",
				"reconcile.crossplane.io/last-triggered": time.Now().UTC().Format(time.RFC3339),
			},
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("error marshaling patch: %w", err)
	}

	if namespace != "" {
		_, err = dynamicClient.Resource(gvr).Namespace(namespace).Patch(
			ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	} else {
		_, err = dynamicClient.Resource(gvr).Patch(
			ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	}

	if err != nil {
		return &ReconcileResult{
			Name:      name,
			Namespace: namespace,
			Kind:      resource,
			Triggered: false,
			Message:   fmt.Sprintf("failed to trigger reconcile: %v", err),
		}, nil
	}

	return &ReconcileResult{
		Name:      name,
		Namespace: namespace,
		Kind:      resource,
		Triggered: true,
		Message:   "reconcile triggered successfully",
	}, nil
}
