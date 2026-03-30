package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	crdGVR = schema.GroupVersionResource{
		Group:    "apiextensions.k8s.io",
		Version:  "v1",
		Resource: "customresourcedefinitions",
	}
	mrdGVR2 = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "managedresourcedefinitions",
	}
)

// ListManagedResources lists all MRs that have actual instances
func ListManagedResources(ctx context.Context, client dynamic.Interface) ([]ManagedResourceInfo, error) {
	result := []ManagedResourceInfo{}

	// get active MRD names for fast lookup
	activeMRDs := map[string]bool{}
	mrds, err := client.Resource(mrdGVR2).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, mrd := range mrds.Items {
			state := getNestedString(mrd.Object, "spec", "state")
			if state == "Active" {
				activeMRDs[mrd.GetName()] = true
			}
		}
	}

	// list all CRDs
	crds, err := client.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing CRDs: %w", err)
	}

	for _, crd := range crds.Items {
		// Using OwnerReference to find only the Managed Resources.
		ownerRefs := crd.GetOwnerReferences()

		providerRevision := ""
		isManagedResource := false

		for _, ref := range ownerRefs {
			if ref.Kind == "ProviderRevision" {
				isManagedResource = true
				providerRevision = ref.Name
				break
			}
		}

		if !isManagedResource {
			continue
		}

		// if MRDs exist, skip inactive ones
		if len(activeMRDs) > 0 && !activeMRDs[crd.GetName()] {
			continue
		}

		group := getNestedString(crd.Object, "spec", "group")
		plural := getNestedString(crd.Object, "spec", "names", "plural")
		kind := getNestedString(crd.Object, "spec", "names", "kind")
		scope := getNestedString(crd.Object, "spec", "scope")

		// only use latest non-deprecated version
		version := getLatestNonDeprecatedVersion(crd.Object)
		if version == "" {
			continue
		}

		mrGVR := schema.GroupVersionResource{
			Group:    group,
			Version:  version,
			Resource: plural,
		}

		// Finding all resources of the Managed Resource.
		var items []map[string]interface{}
		if scope == "Namespaced" {
			list, err := client.Resource(mrGVR).Namespace(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range list.Items {
				items = append(items, item.Object)
			}
		} else {
			list, err := client.Resource(mrGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}
			for _, item := range list.Items {
				items = append(items, item.Object)
			}
		}

		// skip CRDs with no instances
		if len(items) == 0 {
			continue
		}

		for _, obj := range items {
			result = append(result, ManagedResourceInfo{
				UID:       getNestedString(obj, "metadata", "uid"),
				Name:      getNestedString(obj, "metadata", "name"),
				Namespace: getNestedString(obj, "metadata", "namespace"),
				Kind:      kind,
				Group:     group,
				Ready:     resolveConditionStatus(obj, "Ready"),
				Synced:    resolveConditionStatus(obj, "Synced"),
				Age:       getNestedString(obj, "metadata", "creationTimestamp"),
				Provider:  providerRevision,
			})
		}
	}

	return result, nil
}

// GetManagedResource deep inspects a single MR by kind and name
func GetManagedResource(ctx context.Context, client dynamic.Interface, group string, version string, kind string, name string, namespace string) (*ManagedResourceDetail, error) {
	// Getting all the CRD's first
	crds, err := client.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing CRDs: %w", err)
	}

	var mrGVR schema.GroupVersionResource
	var scope string
	found := false

	// Finding Whether there is CRD exist for specified Group and Version
	for _, crd := range crds.Items {
		crdKind := getNestedString(crd.Object, "spec", "names", "kind")
		crdGroup := getNestedString(crd.Object, "spec", "group")

		if crdKind == kind && crdGroup == group {
			plural := getNestedString(crd.Object, "spec", "names", "plural")
			scope = getNestedString(crd.Object, "spec", "scope")
			v := version
			if v == "" {
				v = getLatestNonDeprecatedVersion(crd.Object)
			}
			mrGVR = schema.GroupVersionResource{
				Group:    group,
				Version:  v,
				Resource: plural,
			}
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("CRD not found for kind: %s group: %s", kind, group)
	}

	// fetch the actual MR
	var obj map[string]interface{}
	if scope == "Namespaced" && namespace != "" {
		mr, err := client.Resource(mrGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting MR: %w", err)
		}
		obj = mr.Object
	} else {
		mr, err := client.Resource(mrGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("error getting MR: %w", err)
		}
		obj = mr.Object
	}

	// extract conditions
	var conditions []ConditionInfo
	statusObj, _ := obj["status"].(map[string]interface{})
	if statusObj != nil {
		conds, _ := statusObj["conditions"].([]interface{})
		for _, c := range conds {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			conditions = append(conditions, ConditionInfo{
				Type:    getString(cond, "type"),
				Status:  getString(cond, "status"),
				Reason:  getString(cond, "reason"),
				Message: getString(cond, "message"),
				Age:     getString(cond, "lastTransitionTime"),
			})
		}
	}

	// extract annotations
	annotations := map[string]string{}
	if ann, ok := obj["metadata"].(map[string]interface{})["annotations"].(map[string]interface{}); ok {
		for k, v := range ann {
			if s, ok := v.(string); ok {
				annotations[k] = s
			}
		}
	}

	return &ManagedResourceDetail{
		ManagedResourceInfo: ManagedResourceInfo{
			Name:      getNestedString(obj, "metadata", "name"),
			Namespace: getNestedString(obj, "metadata", "namespace"),
			Kind:      kind,
			Group:     group,
			Ready:     resolveConditionStatus(obj, "Ready"),
			Synced:    resolveConditionStatus(obj, "Synced"),
			Age:       getNestedString(obj, "metadata", "creationTimestamp"),
		},
		Spec:         obj["spec"].(map[string]interface{}),
		Status:       statusObj,
		Conditions:   conditions,
		Annotations:  annotations,
		CompositeRef: getNestedString(obj, "metadata", "annotations", "crossplane.io/composite"),
	}, nil
}
