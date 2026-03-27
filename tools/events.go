package tools

import (
	"context"
	"fmt"
	"sort"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type EventInfo struct {
	LastSeen string
	Type     string // Warning or Normal
	Reason   string
	Object   string
	Message  string
	Count    int32
}

func GetEvents(ctx context.Context, clientset kubernetes.Interface, name string, namespace string) ([]EventInfo, error) {
	// field selector to filter events for specific object
	fieldSelector := fmt.Sprintf("involvedObject.name=%s", name)

	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting events: %w", err)
	}

	var result []EventInfo
	for _, e := range events.Items {
		result = append(result, EventInfo{
			LastSeen: e.LastTimestamp.String(),
			Type:     e.Type,
			Reason:   e.Reason,
			Object:   fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Message:  e.Message,
			Count:    e.Count,
		})
	}

	// sort by most recent first
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen > result[j].LastSeen
	})

	return result, nil
}
