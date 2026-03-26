package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type XRObjectInfo struct {
	Group    string
	Version  string
	Resource string // plural name
	Kind     string
	Scope    string // Namespaced or Cluster
}

type XRInfo struct {
	Name           string
	Namespace      string
	Kind           string
	Ready          string
	Synced         string
	Message        string
	Age            string
	CompositionRef string
	Scope          string
	Group          string
}

var (
	XRDGVR = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v2",
		Resource: "compositeresourcedefinitions",
	}
)

func GetXRInfo(ctx context.Context, dynamicClient dynamic.Interface) ([]XRObjectInfo, error) {
	xrdObjs, err := dynamicClient.Resource(XRDGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing XRDs: %w", err)
	}

	var result []XRObjectInfo

	for _, xrdObj := range xrdObjs.Items {
		group := getNestedString(xrdObj.Object, "spec", "group")
		plural := getNestedString(xrdObj.Object, "spec", "names", "plural")
		kind := getNestedString(xrdObj.Object, "spec", "names", "kind")
		scope := getNestedString(xrdObj.Object, "spec", "scope")
		version := getServedVersion(xrdObj.Object)

		if group == "unknown" || plural == "unknown" || version == "" {
			continue
		}

		result = append(result, XRObjectInfo{
			Group:    group,
			Version:  version,
			Resource: plural,
			Kind:     kind,
			Scope:    scope,
		})
	}

	return result, nil
}
func ListXrs(ctx context.Context, dynamicClient dynamic.Interface) ([]XRInfo, error) {
	result := []XRInfo{}

	xrObjects, err := GetXRInfo(ctx, dynamicClient)
	if err != nil {
		return nil, err
	}

	for _, xrObject := range xrObjects {
		xrGVR := schema.GroupVersionResource{
			Group:    xrObject.Group,
			Version:  xrObject.Version,
			Resource: xrObject.Resource,
		}

		var items []interface{}

		if xrObject.Scope == "Namespaced" {
			list, err := dynamicClient.Resource(xrGVR).Namespace("").List(ctx, metav1.ListOptions{})
			if err != nil {
				fmt.Printf("warning: could not list %s: %v\n", xrObject.Resource, err)
				continue
			}
			for _, item := range list.Items {
				items = append(items, item.Object)
			}
		} else {
			list, err := dynamicClient.Resource(xrGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				fmt.Printf("warning: could not list %s: %v\n", xrObject.Resource, err)
				continue
			}
			for _, item := range list.Items {
				items = append(items, item.Object)
			}
		}

		for _, item := range items {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}

			var xrInfo XRInfo
			if getNestedString(obj, "metadata", "namespace") != "unknown" {
				xrInfo = XRInfo{
					Name:           getNestedString(obj, "metadata", "name"),
					Namespace:      getNestedString(obj, "metadata", "namespace"),
					Kind:           xrObject.Kind,
					Ready:          resolveConditionStatus(obj, "Ready"),
					Synced:         resolveConditionStatus(obj, "Synced"),
					Age:            getNestedString(obj, "metadata", "creationTimestamp"),
					CompositionRef: getNestedString(obj, "spec", "compositionRef", "name"),
					Scope:          "NameSpaced",
					Group:          xrObject.Group,
				}
			} else {
				xrInfo = XRInfo{
					Name:           getNestedString(obj, "metadata", "name"),
					Kind:           xrObject.Kind,
					Ready:          resolveConditionStatus(obj, "Ready"),
					Synced:         resolveConditionStatus(obj, "Synced"),
					Age:            getNestedString(obj, "metadata", "creationTimestamp"),
					CompositionRef: getNestedString(obj, "spec", "compositionRef", "name"),
					Scope:          "Cluster",
					Group:          xrObject.Group,
				}

			}
			result = append(result, xrInfo)

		}
	}

	return result, nil
}

func getServedVersion(obj map[string]interface{}) string {
	versions, ok := obj["spec"].(map[string]interface{})["versions"].([]interface{})
	if !ok {
		return ""
	}
	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _ := ver["served"].(bool)
		if served {
			name, _ := ver["name"].(string)
			return name
		}
	}
	return ""
}

func resolveConditionStatus(obj map[string]interface{}, condType string) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return "Unknown"
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return "Unknown"
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condition["type"] == condType {
			s, _ := condition["status"].(string)
			msg, _ := condition["message"].(string)
			if msg != "" && s == "False" {
				return fmt.Sprintf("False (%s)", truncate(msg, 60))
			}
			return s
		}
	}
	return "Unknown"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
