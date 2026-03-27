package tools

import (
	"context"
	"fmt"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type EventInfo struct {
	LastSeenTime time.Time
	LastSeen     string
	Type         string // Warning or Normal
	Reason       string
	Object       string
	Message      string
	Count        int32
}

func GetEvents(
	ctx context.Context,
	clientset kubernetes.Interface,
	name, namespace, kind string,
) ([]EventInfo, error) {

	if namespace == "" {
		namespace = metav1.NamespaceAll
	}

	fieldSelector := fmt.Sprintf(
		"involvedObject.name=%s,involvedObject.kind=%s",
		name, kind,
	)

	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		Limit:         100,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting events: %w", err)
	}

	var result []EventInfo

	for _, e := range events.Items {
		t := e.EventTime.Time
		if t.IsZero() {
			t = e.LastTimestamp.Time
		}

		result = append(result, EventInfo{
			LastSeenTime: t,
			LastSeen:     t.Format(time.RFC3339),
			Type:         e.Type,
			Reason:       e.Reason,
			Object:       fmt.Sprintf("%s/%s", e.InvolvedObject.Kind, e.InvolvedObject.Name),
			Message:      e.Message,
			Count:        e.Count,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeenTime.After(result[j].LastSeenTime)
	})

	return result, nil
}
