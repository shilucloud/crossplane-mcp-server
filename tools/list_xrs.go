package tools

import (
	"context"
	"fmt"

	"github.com/shilucloud/crossplane-mcp-server/internal/logging"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

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
func ListXrs(ctx context.Context, dynamicClient dynamic.Interface) (*XRListResult, error) {
	result := []XRInfo{}
	warnings := []string{}

	xrObjects, err := GetXRInfo(ctx, dynamicClient)
	if err != nil {
		return nil, err
	}

	logging.Info("listing composite resources", "count", len(xrObjects))

	for _, xrObject := range xrObjects {
		xrGVR := schema.GroupVersionResource{
			Group:    xrObject.Group,
			Version:  xrObject.Version,
			Resource: xrObject.Resource,
		}

		var items []interface{}

		if xrObject.Scope == "Namespaced" {
			list, err := dynamicClient.Resource(xrGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				warning := fmt.Sprintf("could not list %s: %v", xrObject.Resource, err)
				warnings = append(warnings, warning)
				logging.Warn("failed to list namespaced resources", "resource", xrObject.Resource, "error", err.Error())
				continue
			}
			for _, item := range list.Items {
				items = append(items, item.Object)
			}
		} else {
			list, err := dynamicClient.Resource(xrGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				warning := fmt.Sprintf("could not list %s: %v", xrObject.Resource, err)
				warnings = append(warnings, warning)
				logging.Warn("failed to list cluster-scoped resources", "resource", xrObject.Resource, "error", err.Error())
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
			ns := getNestedString(obj, "metadata", "namespace")
			if ns != "" && ns != "unknown" {
				xrInfo = XRInfo{
					Name:           getNestedString(obj, "metadata", "name"),
					Namespace:      getNestedString(obj, "metadata", "namespace"),
					Kind:           xrObject.Kind,
					Ready:          resolveConditionStatus(obj, "Ready"),
					Synced:         resolveConditionStatus(obj, "Synced"),
					Age:            getNestedString(obj, "metadata", "creationTimestamp"),
					CompositionRef: getNestedString(obj, "spec", "crossplane", "compositionRef", "name"),
					Scope:          "Namespaced",
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

	logging.Info("completed listing composite resources", "total_xrs", len(result), "warnings", len(warnings))
	return &XRListResult{XRs: result, Warnings: warnings}, nil
}
