package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type CreateResult struct {
	Name            string
	Kind            string
	APIVersion      string
	Namespace       string
	Created         bool
	Message         string
	ResourceVersion string
}

type ApplyResult struct {
	Name            string
	Kind            string
	APIVersion      string
	Namespace       string
	Created         bool
	Updated         bool
	Message         string
	ResourceVersion string
}

type DeleteResult struct {
	Name      string
	Kind      string
	Namespace string
	Deleted   bool
	Message   string
}

type PatchResult struct {
	Name       string
	Kind       string
	Namespace  string
	Patched    bool
	Message    string
	WasCreated bool
}

func kindToPluralForCreate(kind string) string {
	lower := strings.ToLower(kind)
	if strings.HasSuffix(lower, "s") {
		return lower
	}
	if strings.HasSuffix(lower, "y") {
		return lower[:len(lower)-1] + "ies"
	}
	return lower + "s"
}

func resolveGVRForCreate(apiVersion string, kind string) (schema.GroupVersionResource, error) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) != 2 {
		return schema.GroupVersionResource{}, fmt.Errorf("invalid apiVersion: %s (expected group/version)", apiVersion)
	}
	group, version := parts[0], parts[1]
	plural := kindToPluralForCreate(kind)
	return schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: plural,
	}, nil
}

func CreateXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, spec map[string]interface{}) (*CreateResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	xrdSchema, err := GetXRDSchema(ctx, client, kind)
	if err == nil && xrdSchema != nil {
		if xrdSchema.DefaultCompositionRef != nil || xrdSchema.EnforcedCompositionRef != nil {
			spec = addCompositionRefIfMissing(spec, xrdSchema)
		}
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvr.GroupVersion().WithKind(kind))
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.Object["spec"] = spec

	var created *unstructured.Unstructured
	if namespace != "" {
		created, err = client.Resource(gvr).Namespace(namespace).Create(ctx, u, metav1.CreateOptions{})
	} else {
		created, err = client.Resource(gvr).Create(ctx, u, metav1.CreateOptions{})
	}

	if err != nil {
		return &CreateResult{
			Name:    name,
			Kind:    kind,
			Message: fmt.Sprintf("Failed to create: %v", err),
		}, nil
	}

	return &CreateResult{
		Name:            created.GetName(),
		Kind:            kind,
		APIVersion:      apiVersion,
		Namespace:       namespace,
		Created:         true,
		Message:         "Resource created successfully",
		ResourceVersion: created.GetResourceVersion(),
	}, nil
}

func ApplyXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, spec map[string]interface{}) (*ApplyResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvr.GroupVersion().WithKind(kind))
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.Object["spec"] = spec

	created := false
	updated := false
	var applied *unstructured.Unstructured

	if namespace != "" {
		existing, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			applied, err = client.Resource(gvr).Namespace(namespace).Create(ctx, u, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create: %w", err)
			}
			created = true
		} else {
			u.SetResourceVersion(existing.GetResourceVersion())
			applied, err = client.Resource(gvr).Namespace(namespace).Update(ctx, u, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update: %w", err)
			}
			updated = true
		}
	} else {
		existing, err := client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			applied, err = client.Resource(gvr).Create(ctx, u, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create: %w", err)
			}
			created = true
		} else {
			u.SetResourceVersion(existing.GetResourceVersion())
			applied, err = client.Resource(gvr).Update(ctx, u, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update: %w", err)
			}
			updated = true
		}
	}

	return &ApplyResult{
		Name:            applied.GetName(),
		Kind:            kind,
		APIVersion:      apiVersion,
		Namespace:       namespace,
		Created:         created,
		Updated:         updated,
		Message:         getStatusMessageFromUnstructured(applied.Object),
		ResourceVersion: applied.GetResourceVersion(),
	}, nil
}

func DeleteXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string) (*DeleteResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	if namespace != "" {
		err = client.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	} else {
		err = client.Resource(gvr).Delete(ctx, name, metav1.DeleteOptions{})
	}

	if err != nil {
		return &DeleteResult{
			Name:      name,
			Kind:      kind,
			Namespace: namespace,
			Deleted:   false,
			Message:   fmt.Sprintf("Failed to delete: %v", err),
		}, nil
	}

	return &DeleteResult{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		Deleted:   true,
		Message:   "Resource deleted successfully",
	}, nil
}

func PatchXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, patchData map[string]interface{}) (*PatchResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	patchBytes, err := runtime.Encode(nil, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	var patched *unstructured.Unstructured
	if namespace != "" {
		patched, err = client.Resource(gvr).Namespace(namespace).Patch(ctx, name, "application/apply-patch", patchBytes, metav1.PatchOptions{})
	} else {
		patched, err = client.Resource(gvr).Patch(ctx, name, "application/apply-patch", patchBytes, metav1.PatchOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to patch: %w", err)
	}

	return &PatchResult{
		Name:       name,
		Kind:       kind,
		Namespace:  namespace,
		Patched:    true,
		Message:    getStatusMessageFromUnstructured(patched.Object),
		WasCreated: false,
	}, nil
}

func DryRunCreateXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, spec map[string]interface{}) (*CreateResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvr.GroupVersion().WithKind(kind))
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.Object["spec"] = spec

	var created *unstructured.Unstructured
	if namespace != "" {
		created, err = client.Resource(gvr).Namespace(namespace).Create(ctx, u, metav1.CreateOptions{DryRun: []string{"All"}})
	} else {
		created, err = client.Resource(gvr).Create(ctx, u, metav1.CreateOptions{DryRun: []string{"All"}})
	}

	if err != nil {
		return &CreateResult{
			Name:    name,
			Kind:    kind,
			Message: fmt.Sprintf("Dry-run failed: %v", err),
		}, nil
	}

	return &CreateResult{
		Name:            created.GetName(),
		Kind:            kind,
		APIVersion:      apiVersion,
		Namespace:       namespace,
		Created:         true,
		Message:         "Dry-run: Resource would be created successfully",
		ResourceVersion: created.GetResourceVersion(),
	}, nil
}

func DryRunApplyXR(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, spec map[string]interface{}) (*ApplyResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	created := false
	updated := false

	if namespace != "" {
		_, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			created = true
		} else {
			updated = true
		}
	} else {
		_, err := client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			created = true
		} else {
			updated = true
		}
	}

	message := "Dry-run: No changes needed (resource unchanged)"
	if created {
		message = "Dry-run: Resource would be created"
	} else if updated {
		message = "Dry-run: Resource would be updated"
	}

	return &ApplyResult{
		Name:            name,
		Kind:            kind,
		APIVersion:      apiVersion,
		Namespace:       namespace,
		Created:         created,
		Updated:         updated,
		Message:         message,
		ResourceVersion: "",
	}, nil
}

func ApplyManagedResource(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, spec map[string]interface{}) (*ApplyResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvr.GroupVersion().WithKind(kind))
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	u.Object["spec"] = spec

	created := false
	updated := false
	var applied *unstructured.Unstructured

	if namespace != "" {
		existing, err := client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			applied, err = client.Resource(gvr).Namespace(namespace).Create(ctx, u, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create managed resource: %w", err)
			}
			created = true
		} else {
			u.SetResourceVersion(existing.GetResourceVersion())
			applied, err = client.Resource(gvr).Namespace(namespace).Update(ctx, u, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update managed resource: %w", err)
			}
			updated = true
		}
	} else {
		existing, err := client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			applied, err = client.Resource(gvr).Create(ctx, u, metav1.CreateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to create managed resource: %w", err)
			}
			created = true
		} else {
			u.SetResourceVersion(existing.GetResourceVersion())
			applied, err = client.Resource(gvr).Update(ctx, u, metav1.UpdateOptions{})
			if err != nil {
				return nil, fmt.Errorf("failed to update managed resource: %w", err)
			}
			updated = true
		}
	}

	return &ApplyResult{
		Name:            name,
		Kind:            kind,
		APIVersion:      apiVersion,
		Namespace:       namespace,
		Created:         created,
		Updated:         updated,
		Message:         getMRStatusMessage(applied.Object),
		ResourceVersion: applied.GetResourceVersion(),
	}, nil
}

func UpdateCompositionRef(ctx context.Context, client dynamic.Interface, xrAPIVersion string, xrKind string, xrName string, xrNamespace string, compositionName string) (*ApplyResult, error) {
	gvr, err := resolveGVRForCreate(xrAPIVersion, xrKind)
	if err != nil {
		return nil, err
	}

	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"compositionRef": map[string]interface{}{
				"name": compositionName,
			},
		},
	}

	patchBytes, err := runtime.Encode(nil, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	var patched *unstructured.Unstructured
	if xrNamespace != "" {
		patched, err = client.Resource(gvr).Namespace(xrNamespace).Patch(ctx, xrName, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	} else {
		patched, err = client.Resource(gvr).Patch(ctx, xrName, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to update composition ref: %w", err)
	}

	return &ApplyResult{
		Name:            xrName,
		Kind:            xrKind,
		APIVersion:      xrAPIVersion,
		Namespace:       xrNamespace,
		Created:         false,
		Updated:         true,
		Message:         fmt.Sprintf("CompositionRef updated to: %s", compositionName),
		ResourceVersion: patched.GetResourceVersion(),
	}, nil
}

func SetProviderConfig(ctx context.Context, client dynamic.Interface, mrAPIVersion string, mrKind string, mrName string, mrNamespace string, providerConfigName string) (*ApplyResult, error) {
	gvr, err := resolveGVRForCreate(mrAPIVersion, mrKind)
	if err != nil {
		return nil, err
	}

	patchData := map[string]interface{}{
		"spec": map[string]interface{}{
			"providerConfigRef": map[string]interface{}{
				"name": providerConfigName,
			},
		},
	}

	patchBytes, err := runtime.Encode(nil, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	var patched *unstructured.Unstructured
	if mrNamespace != "" {
		patched, err = client.Resource(gvr).Namespace(mrNamespace).Patch(ctx, mrName, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	} else {
		patched, err = client.Resource(gvr).Patch(ctx, mrName, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to set provider config: %w", err)
	}

	return &ApplyResult{
		Name:            mrName,
		Kind:            mrKind,
		APIVersion:      mrAPIVersion,
		Namespace:       mrNamespace,
		Created:         false,
		Updated:         true,
		Message:         fmt.Sprintf("ProviderConfigRef updated to: %s", providerConfigName),
		ResourceVersion: patched.GetResourceVersion(),
	}, nil
}

func AddXRAnnotation(ctx context.Context, client dynamic.Interface, apiVersion string, kind string, name string, namespace string, annotations map[string]string) (*PatchResult, error) {
	gvr, err := resolveGVRForCreate(apiVersion, kind)
	if err != nil {
		return nil, err
	}

	var current *unstructured.Unstructured
	if namespace != "" {
		current, err = client.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		current, err = client.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get resource: %w", err)
	}

	currentAnnotations := current.GetAnnotations()
	if currentAnnotations == nil {
		currentAnnotations = make(map[string]string)
	}

	for k, v := range annotations {
		currentAnnotations[k] = v
	}

	patchData := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": currentAnnotations,
		},
	}

	patchBytes, err := runtime.Encode(nil, &unstructured.Unstructured{Object: patchData})
	if err != nil {
		return nil, fmt.Errorf("failed to encode patch: %w", err)
	}

	if namespace != "" {
		_, err = client.Resource(gvr).Namespace(namespace).Patch(ctx, name, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	} else {
		_, err = client.Resource(gvr).Patch(ctx, name, "application/merge-patch+json", patchBytes, metav1.PatchOptions{})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to add annotation: %w", err)
	}

	return &PatchResult{
		Name:      name,
		Kind:      kind,
		Namespace: namespace,
		Patched:   true,
		Message:   "Annotations added successfully",
	}, nil
}

func getStatusMessageFromUnstructured(obj map[string]interface{}) string {
	if obj == nil {
		return "Unknown status"
	}

	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return "No status available"
	}

	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return "No conditions"
	}

	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		if condType == "Ready" {
			condStatus, _ := cond["status"].(string)
			reason, _ := cond["reason"].(string)
			message, _ := cond["message"].(string)
			if message != "" {
				return fmt.Sprintf("Ready: %s (%s) - %s", condStatus, reason, message)
			}
			return fmt.Sprintf("Ready: %s (%s)", condStatus, reason)
		}
	}

	return "Status available"
}

func getMRStatusMessage(obj map[string]interface{}) string {
	if obj == nil {
		return "Unknown status"
	}

	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return "No status available"
	}

	var readyStatus, syncedStatus string
	var readyReason, syncedReason string

	conditions, ok := status["conditions"].([]interface{})
	if ok {
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			if condType == "Ready" {
				readyStatus, _ = cond["status"].(string)
				readyReason, _ = cond["reason"].(string)
			}
			if condType == "Synced" {
				syncedStatus, _ = cond["status"].(string)
				syncedReason, _ = cond["reason"].(string)
			}
		}
	}

	if readyStatus != "" || syncedStatus != "" {
		return fmt.Sprintf("Ready: %s (%s) | Synced: %s (%s)", readyStatus, readyReason, syncedStatus, syncedReason)
	}

	return "Status available"
}

func addCompositionRefIfMissing(spec map[string]interface{}, xrdSchema *XRDSchema) map[string]interface{} {
	if spec == nil {
		spec = map[string]interface{}{}
	}

	if compRef, ok := spec["compositionRef"].(map[string]interface{}); ok {
		if name, ok := compRef["name"].(string); ok && name != "" {
			return spec
		}
	}

	if xrdSchema.EnforcedCompositionRef != nil {
		if _, ok := spec["compositionRef"]; !ok {
			spec["compositionRef"] = map[string]interface{}{
				"name": *xrdSchema.EnforcedCompositionRef,
			}
		}
	} else if xrdSchema.DefaultCompositionRef != nil {
		if _, ok := spec["compositionRef"]; !ok {
			spec["compositionRef"] = map[string]interface{}{
				"name": *xrdSchema.DefaultCompositionRef,
			}
		}
	}

	return spec
}
