package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type ProviderDebugResult struct {
	ProviderName    string
	Healthy         bool
	Installed       bool
	State           string
	Diagnosis       Diagnosis
	Conditions      []Condition
	Events          []EventInfo
	ProviderConfigs []ProviderConfigHealth
	AffectedMRs     int
}

func DebugProvider(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	clientset kubernetes.Interface,
	providerName string,
) (*ProviderDebugResult, error) {
	result := &ProviderDebugResult{
		ProviderName: providerName,
	}

	// Step 1: get provider
	p, err := dynamicClient.Resource(providerGVR).Get(ctx, providerName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("provider '%s' not found: %w", providerName, err)
	}

	result.Healthy = resolveConditionStatus(p.Object, "Healthy") == "True"
	result.Installed = resolveConditionStatus(p.Object, "Installed") == "True"
	result.State = resolveProviderState(p.Object)

	// Step 2: extract conditions
	if status, ok := p.Object["status"].(map[string]interface{}); ok {
		if conditions, ok := status["conditions"].([]interface{}); ok {
			for _, c := range conditions {
				cond, ok := c.(map[string]interface{})
				if !ok {
					continue
				}
				result.Conditions = append(result.Conditions, Condition{
					Type:               getString(cond, "type"),
					Status:             getString(cond, "status"),
					Reason:             getString(cond, "reason"),
					Message:            getString(cond, "message"),
					LastTransitionTime: getString(cond, "lastTransitionTime"),
				})
			}
		}
	}

	// Step 3: get provider events
	events, err := GetEventsByUID(ctx, clientset, string(p.GetUID()))
	if err == nil {
		result.Events = events
	}

	// Step 4: count affected MRs
	mrs, err := ListManagedResources(ctx, dynamicClient)
	if err == nil {
		for _, mr := range mrs {
			// check if MR belongs to this provider
			if strings.Contains(mr.Provider, providerName) {
				if strings.HasPrefix(mr.Ready, "False") ||
					strings.HasPrefix(mr.Synced, "False") {
					result.AffectedMRs++
				}
			}
		}
	}

	// Step 5: diagnose
	result.Diagnosis = diagnoseProvider(result)

	return result, nil
}

func diagnoseProvider(result *ProviderDebugResult) Diagnosis {

	// Case 1: not installed
	if !result.Installed {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Provider '%s' is not installed", result.ProviderName),
			Severity:     "Critical",
			AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
			SuggestedFix: "Check provider revision: kubectl get providerrevisions. Check if the package can be pulled from the registry.",
		}
	}

	// Case 2: installed but not healthy
	if !result.Healthy {
		// check conditions for specific reason
		for _, c := range result.Conditions {
			if c.Type == "Healthy" && c.Status == "False" {
				// dependency conflict
				if strings.Contains(c.Message, "incompatible") ||
					strings.Contains(c.Message, "dependency") {
					return Diagnosis{
						RootCause:    fmt.Sprintf("Provider '%s' has incompatible dependencies: %s", result.ProviderName, c.Message),
						Severity:     "Critical",
						AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
						SuggestedFix: "Install matching versions of all provider dependencies. Check: kubectl get providers.",
					}
				}

				// image pull error
				if strings.Contains(c.Message, "pull") ||
					strings.Contains(c.Message, "image") {
					return Diagnosis{
						RootCause:    fmt.Sprintf("Provider '%s' cannot pull package image: %s", result.ProviderName, c.Message),
						Severity:     "Critical",
						AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
						SuggestedFix: "Check network connectivity to the registry. If using a private registry, ensure ImageConfig is configured correctly.",
					}
				}

				// generic unhealthy
				return Diagnosis{
					RootCause:    fmt.Sprintf("Provider '%s' is unhealthy: %s", result.ProviderName, c.Message),
					Severity:     "Critical",
					AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
					SuggestedFix: fmt.Sprintf("Check provider pod: kubectl get pods -n crossplane-system -l pkg.crossplane.io/provider=%s", result.ProviderName),
				}
			}
		}
	}

	// Case 3: healthy but has affected MRs
	if result.Healthy && result.AffectedMRs > 0 {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Provider '%s' is healthy but %d managed resource(s) are failing", result.ProviderName, result.AffectedMRs),
			Severity:     "Warning",
			AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
			SuggestedFix: "Check ProviderConfig credentials and run debug_mr on individual failing resources.",
		}
	}

	// All good
	return Diagnosis{
		RootCause:    fmt.Sprintf("Provider '%s' is healthy with no issues detected", result.ProviderName),
		Severity:     "Info",
		AffectedPath: fmt.Sprintf("Provider/%s", result.ProviderName),
		SuggestedFix: "",
	}
}
