package tools

import (
	"context"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type MRTreeInfo struct {
	UID                string
	Name               string
	Kind               string
	Group              string
	Version            string
	Ready              string
	Synced             string
	ProviderConfigInfo ProviderConfigInfo
	ProviderConfigName string
}

type XRTreeInfo struct {
	UID             string
	XRName          string
	XRNamespace     string
	XRReady         string
	XRSynced        string
	XRKind          string
	XRFields        map[string]any
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

	result.UID = getNestedString(xrObj, "metadata", "uid")
	result.XRFields = getNestedMap(xrObj, "spec", "parameters")
	result.XRName = getNestedString(xrObj, "metadata", "name")
	result.XRNamespace = getNestedString(xrObj, "metadata", "namespace")
	result.XRReady = resolveConditionStatus(xrObj, "Ready")
	result.XRSynced = resolveConditionStatus(xrObj, "Synced")
	result.XRKind = getNestedString(xrObj, "spec", "names", "kind")

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

		apiVersion := getString(mrMap, "apiVersion")
		kind := getString(mrMap, "kind")
		mrName := getString(mrMap, "name")

		mrGroup, mrVersion := splitAPIVersion(apiVersion)
		plural := kindToPlural(kind)

		mrGVR := schema.GroupVersionResource{
			Group:    mrGroup,
			Version:  mrVersion,
			Resource: plural,
		}

		// fetch actual MR — declare err here so it's accessible below
		var mrObj *unstructured.Unstructured
		var err error

		if namespace != "" {
			mrObj, err = dynamicClient.Resource(mrGVR).Namespace(namespace).Get(ctx, mrName, metav1.GetOptions{})
		} else {
			mrObj, err = dynamicClient.Resource(mrGVR).Get(ctx, mrName, metav1.GetOptions{})
		}

		mrInfo := MRTreeInfo{
			Name:    mrName,
			Kind:    kind,
			Group:   mrGroup,
			Version: mrVersion,
		}

		if err == nil {
			mrInfo.UID = getNestedString(mrObj.Object, "metadata", "uid")
			mrInfo.ProviderConfigName = getNestedString(mrObj.Object, "spec", "providerConfigRef", "name")
			mrInfo.Ready = resolveConditionStatus(mrObj.Object, "Ready")
			mrInfo.Synced = resolveConditionStatus(mrObj.Object, "Synced")

			pcGroup := mrGroupToProviderConfigGroup(mrInfo.Group)
			pcGVR := schema.GroupVersionResource{
				Group:    pcGroup,
				Version:  "v1beta1",
				Resource: "providerconfigs",
			}

			pcObj, pcErr := dynamicClient.Resource(pcGVR).Get(ctx, mrInfo.ProviderConfigName, metav1.GetOptions{})
			if pcErr == nil {
				mrInfo.ProviderConfigInfo = ProviderConfigInfo{
					Group:  pcGroup,
					Ready:  resolveConditionStatus(pcObj.Object, "Ready"),
					Synced: resolveConditionStatus(pcObj.Object, "Synced"),
				}
			}
		} else {
			mrInfo.Ready = "NotFound"
			mrInfo.Synced = "NotFound"
		}

		result.MRs = append(result.MRs, mrInfo)
	}

	return result, nil
}

func splitAPIVersion(apiVersion string) (string, string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return apiVersion, ""
}

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

func mrGroupToProviderConfigGroup(mrGroup string) string {
	parts := strings.SplitN(mrGroup, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return mrGroup
}
