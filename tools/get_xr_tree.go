package tools

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type CompositionInfo struct {
	Name     string
	Mode     string
	Pipeline []interface{}
}

type MRTreeInfo struct {
	Name           string
	Kind           string
	Group          string
	Version        string
	Ready          string
	Synced         string
	ProviderConfig string
}

type XRTreeInfo struct {
	XRName          string
	XRNamespace     string
	XRReady         string
	XRSynced        string
	CompositionInfo CompositionInfo
	MRs             []MRTreeInfo
}

var (
	compositionGVR = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositions",
	}
)

func GetXRTree(ctx context.Context, dynamicClient dynamic.Interface, group, version, resource, name, namespace string) (*XRTreeInfo, error) {
	result := &XRTreeInfo{}

	xrgvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	// fetch XR
	var xrObj map[string]interface{}
	if namespace == "" {
		o, err := dynamicClient.Resource(xrgvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		xrObj = o.Object
	} else {
		o, err := dynamicClient.Resource(xrgvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		xrObj = o.Object
	}

	result.XRName = getNestedString(xrObj, "metadata", "name")
	result.XRNamespace = getNestedString(xrObj, "metadata", "namespace")
	result.XRReady = resolveConditionStatus(xrObj, "Ready")
	result.XRSynced = resolveConditionStatus(xrObj, "Synced")

	// fetch composition — not fatal if missing
	compositionUsed := getNestedString(xrObj, "spec", "crossplane", "compositionRef", "name")
	if compositionUsed != "unknown" && compositionUsed != "" {
		compositionObj, err := dynamicClient.Resource(compositionGVR).Get(ctx, compositionUsed, metav1.GetOptions{})
		if err == nil {
			result.CompositionInfo = CompositionInfo{
				Name:     compositionUsed,
				Pipeline: getNestedSlice(compositionObj.Object, "spec", "pipeline"),
				Mode:     getNestedString(compositionObj.Object, "spec", "mode"),
			}
		}
	} else {
		result.CompositionInfo = CompositionInfo{
			Name: "none selected",
		}
	}

	// fetch MRs from resourceRefs
	mrs := getNestedSlice(xrObj, "spec", "crossplane", "resourceRefs")
	for _, mr := range mrs {
		mrMap, ok := mr.(map[string]interface{})
		if !ok {
			continue
		}

		apiVersion := getString(mrMap, "apiVersion") // e.g. "s3.aws.upbound.io/v1beta1"
		kind := getString(mrMap, "kind")
		mrName := getString(mrMap, "name")

		// split apiVersion into group and version
		group, version := splitAPIVersion(apiVersion)

		// find plural for this kind to build GVR
		plural := kindToPlural(kind)

		mrGVR := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: plural,
		}

		// fetch actual MR to get status
		mrObj, err := dynamicClient.Resource(mrGVR).Get(ctx, mrName, metav1.GetOptions{})

		mrInfo := MRTreeInfo{
			Name:    mrName,
			Kind:    kind,
			Group:   group,
			Version: version,
		}

		if err == nil {
			mrInfo.Ready = resolveConditionStatus(mrObj.Object, "Ready")
			mrInfo.Synced = resolveConditionStatus(mrObj.Object, "Synced")
			mrInfo.ProviderConfig = getNestedString(mrObj.Object, "spec", "providerConfigRef", "name")
		}

		result.MRs = append(result.MRs, mrInfo)
	}

	return result, nil
}

// splitAPIVersion splits "s3.aws.upbound.io/v1beta1" into group and version
func splitAPIVersion(apiVersion string) (string, string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return apiVersion, ""
}

// kindToPlural is a simple lowercase + s pluralization
// good enough for most Crossplane resources
func kindToPlural(kind string) string {
	return strings.ToLower(kind) + "s"
}

func getNestedSlice(obj map[string]interface{}, fields ...string) []interface{} {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return nil
		}
		if i == len(fields)-1 {
			slice, ok := val.([]interface{})
			if !ok {
				return nil
			}
			return slice
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return nil
		}
	}
	return nil
}
