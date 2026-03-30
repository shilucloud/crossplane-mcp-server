package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func GetConditions(ctx context.Context, client dynamic.Interface, group string, version string, resource string, name string, namespace string) ([]Condition, error) {
	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// fetch the resource
	var obj map[string]interface{}
	if namespace != "" {
		fmt.Println("--in--")
		o, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting resource: %w", err)
		}
		obj = o.Object
	} else {
		o, err := client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting resource: %w", err)
		}
		obj = o.Object
	}

	// extract conditions
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no status found on resource %s", name)
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no conditions found on resource %s", name)
	}

	var result []Condition
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		result = append(result, Condition{
			Type:               getString(cond, "type"),
			Status:             getString(cond, "status"),
			Reason:             getString(cond, "reason"),
			Message:            getString(cond, "message"),
			LastTransitionTime: getString(cond, "lastTransitionTime"),
			ObservedGeneration: getNestedInt64(cond, "observedGeneration"),
		})
	}

	return result, nil
}
