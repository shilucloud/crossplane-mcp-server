package tools

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	operationsGVR = schema.GroupVersionResource{
		Group:    "ops.crossplane.io",
		Version:  "v1alpha1",
		Resource: "operations",
	}
	cronOperationsGVR = schema.GroupVersionResource{
		Group:    "ops.crossplane.io",
		Version:  "v1alpha1",
		Resource: "cronoperations",
	}
	watchOperationsGVR = schema.GroupVersionResource{
		Group:    "ops.crossplane.io",
		Version:  "v1alpha1",
		Resource: "watchoperations",
	}
)

// OperationResult holds a one-off operation
type OperationResult struct {
	Name           string
	Phase          string
	StartTime      string
	CompletionTime string
	Duration       string
	Message        string
}

// CronOperationResult holds a scheduled operation
type CronOperationResult struct {
	Name          string
	Schedule      string
	LastRunTime   string
	LastRunStatus string
	TotalRuns     int64
}

// WatchOperationResult holds an event-driven operation
type WatchOperationResult struct {
	Name          string
	WatchingKind  string
	WatchingName  string
	LastTriggered string
	TriggerCount  int64
	Status        string
}

// OperationsSummary is the full result returned to the agent
type OperationsSummary struct {
	Operations      []OperationResult
	CronOperations  []CronOperationResult
	WatchOperations []WatchOperationResult
}

func ListOperations(ctx context.Context, client dynamic.Interface) (*OperationsSummary, error) {
	summary := &OperationsSummary{}

	// Operations
	ops, err := client.Resource(operationsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing operations: %w", err)
	}
	for _, op := range ops.Items {
		start := getNestedString(op.Object, "status", "startTime")
		completion := getNestedString(op.Object, "status", "completionTime")

		duration := ""
		if start != "unknown" && completion != "unknown" {
			s, err1 := time.Parse(time.RFC3339, start)
			c, err2 := time.Parse(time.RFC3339, completion)
			if err1 == nil && err2 == nil {
				duration = c.Sub(s).String()
			}
		}

		summary.Operations = append(summary.Operations, OperationResult{
			Name:           op.GetName(),
			Phase:          getNestedString(op.Object, "status", "phase"),
			StartTime:      start,
			CompletionTime: completion,
			Duration:       duration,
			Message:        getNestedString(op.Object, "status", "message"),
		})
	}

	// CronOperations
	cronOps, err := client.Resource(cronOperationsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing cronoperations: %w", err)
	}
	for _, op := range cronOps.Items {
		summary.CronOperations = append(summary.CronOperations, CronOperationResult{
			Name:          op.GetName(),
			Schedule:      getNestedString(op.Object, "spec", "schedule"),
			LastRunTime:   getNestedString(op.Object, "status", "lastRunTime"),
			LastRunStatus: getNestedString(op.Object, "status", "lastRunStatus"),
			TotalRuns:     getNestedInt64(op.Object, "status", "totalRuns"),
		})
	}

	// WatchOperations
	watchOps, err := client.Resource(watchOperationsGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing watchoperations: %w", err)
	}
	for _, op := range watchOps.Items {
		summary.WatchOperations = append(summary.WatchOperations, WatchOperationResult{
			Name:          op.GetName(),
			WatchingKind:  getNestedString(op.Object, "spec", "watch", "kind"),
			WatchingName:  getNestedString(op.Object, "spec", "watch", "resourceRef", "name"),
			LastTriggered: getNestedString(op.Object, "status", "lastTriggeredTime"),
			TriggerCount:  getNestedInt64(op.Object, "status", "triggerCount"),
			Status:        getNestedString(op.Object, "status", "phase"),
		})
	}

	return summary, nil
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return "unknown"
		}
		if i == len(fields)-1 {
			str, ok := val.(string)
			if !ok {
				return "unknown"
			}
			return str
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return "unknown"
		}
	}
	return "unknown"
}

func getNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return 0
		}
		if i == len(fields)-1 {
			switch v := val.(type) {
			case int64:
				return v
			case float64:
				return int64(v)
			}
			return 0
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return 0
		}
	}
	return 0
}
