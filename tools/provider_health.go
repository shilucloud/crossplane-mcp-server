package tools

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

var (
	providerConfigKind = "ProviderConfig"
)

// Discover ALL ProviderConfig GVRs
func FindProviderConfigGVRs(discoveryClient discovery.DiscoveryInterface) ([]schema.GroupVersionResource, error) {
	var result []schema.GroupVersionResource

	resources, err := discoveryClient.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	for _, list := range resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			if r.Kind == providerConfigKind {
				result = append(result, schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: r.Name,
				})
			}
		}
	}

	return result, nil
}

// Build Provider → Groups mapping using CRDs
func BuildProviderGroupMap(ctx context.Context, dynamicClient dynamic.Interface) (map[string][]string, error) {
	result := make(map[string][]string)

	crds, err := dynamicClient.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, crd := range crds.Items {

		group := getNestedString(crd.Object, "spec", "group")
		if group == "" {
			continue
		}

		for _, o := range crd.GetOwnerReferences() {
			if o.Kind == "Provider" {
				result[o.Name] = append(result[o.Name], group)
			}
		}
	}

	return result, nil
}

// MAIN FUNCTION
func GetProviderHealth(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	discoveryClient discovery.DiscoveryInterface,
) (*ProviderHealthSummary, error) {

	summary := &ProviderHealthSummary{}

	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing providers: %w", err)
	}

	summary.TotalProviders = len(providers.Items)

	//discover providerconfigs
	pcGVRs, err := FindProviderConfigGVRs(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("error discovering providerconfigs: %w", err)
	}

	// build Provider → group mapping
	providerToGroups, err := BuildProviderGroupMap(ctx, dynamicClient)
	if err != nil {
		return nil, fmt.Errorf("error building provider group map: %w", err)
	}

	for _, p := range providers.Items {

		name := p.GetName()

		healthy := resolveConditionStatus(p.Object, "Healthy") == "True"
		installed := resolveConditionStatus(p.Object, "Installed") == "True"
		state := resolveProviderState(p.Object)

		ph := ProviderHealth{
			ProviderName: name,
			Healthy:      healthy,
			Installed:    installed,
			State:        state,
			Version:      getNestedString(p.Object, "status", "currentRevision"),
			Package:      getNestedString(p.Object, "spec", "package"),
		}

		// extract conditions
		if status, ok := p.Object["status"].(map[string]interface{}); ok {
			if conditions, ok := status["conditions"].([]interface{}); ok {
				for _, c := range conditions {
					cond, ok := c.(map[string]interface{})
					if !ok {
						continue
					}
					ph.Conditions = append(ph.Conditions, Condition{
						Type:               getString(cond, "type"),
						Status:             getString(cond, "status"),
						Reason:             getString(cond, "reason"),
						Message:            getString(cond, "message"),
						LastTransitionTime: getString(cond, "lastTransitionTime"),
					})
				}
			}
		}

		// get groups owned by this provider
		groups := providerToGroups[name]

		// match ProviderConfig GVRs using CRD ownership
		for _, pcGVR := range pcGVRs {

			if !contains(groups, pcGVR.Group) {
				continue
			}

			pcs, err := dynamicClient.Resource(pcGVR).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

			for _, pc := range pcs.Items {
				ph.ProviderConfigs = append(ph.ProviderConfigs, ProviderConfigHealth{
					Name:      pc.GetName(),
					Namespace: pc.GetNamespace(),
					Ready:     resolveConditionStatus(pc.Object, "Ready"),
					Synced:    resolveConditionStatus(pc.Object, "Synced"),
					SecretRef: getNestedString(pc.Object, "spec", "credentials", "secretRef", "name"),
				})
			}
		}

		if healthy {
			summary.HealthyProviders++
		} else {
			summary.UnhealthyProviders++
		}

		summary.Providers = append(summary.Providers, ph)
	}

	return summary, nil
}
