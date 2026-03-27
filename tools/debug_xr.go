package tools

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Diagnosis struct {
	RootCause    string
	Severity     string // Critical, Warning, Info
	AffectedPath string // e.g. "new-bucket → Bucket/new-bucket-xyz → ProviderConfig/aws-provider"
	SuggestedFix string
	Details      []string
}

type DebugResult struct {
	XRName      string
	XRNamespace string
	XRReady     string
	XRSynced    string
	Diagnosis   Diagnosis
	Tree        *XRTreeInfo
	Events      []EventInfo
}

func DebugXR(ctx context.Context, dynamicClient dynamic.Interface, clientset kubernetes.Interface, group, version, resource, name, namespace string) (*DebugResult, error) {
	result := &DebugResult{
		XRName:      name,
		XRNamespace: namespace,
	}

	// Step 1: get full tree
	tree, err := GetXRTree(ctx, dynamicClient, group, version, resource, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting XR tree: %w", err)
	}
	result.Tree = tree
	result.XRReady = tree.XRReady
	result.XRSynced = tree.XRSynced

	// Step 2: get events
	events, err := GetEvents(ctx, clientset, name, namespace)
	if err == nil {
		result.Events = events
	}

	// Step 3: diagnose
	result.Diagnosis = diagnose(tree, result.Events)

	return result, nil
}

func diagnose(tree *XRTreeInfo, events []EventInfo) Diagnosis {
	// Case 1: no composition selected
	if tree.CompositionInfo.Name == "none selected" || tree.CompositionInfo.Name == "" {
		return Diagnosis{
			RootCause:    "No compatible Composition found for this XR",
			Severity:     "Critical",
			AffectedPath: tree.XRName,
			SuggestedFix: "Check that a Composition exists with compositeTypeRef matching this XR's group/version/kind. Also check compositionRef or compositionSelector on the XR.",
			Details:      extractEventMessages(events),
		}
	}

	// Case 1.1 Checking Composition whether mixed Namespaced and cluster-scoped resources
	if len(tree.XRNamespace) > 0 {
		// we get composition with the name and check what provider it haved used.
		// and check whether the provider is namespaced or managed.

	}

	// Case 2: no MRs created yet
	if len(tree.MRs) == 0 {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Composition '%s' selected but no managed resources created yet", tree.CompositionInfo.Name),
			Severity:     "Warning",
			AffectedPath: fmt.Sprintf("%s → %s", tree.XRName, tree.CompositionInfo.Name),
			SuggestedFix: "Check the Composition pipeline for errors. Ensure all functions are installed and healthy.",
			Details:      extractEventMessages(events),
		}
	}

	// Case 3: walk MRs and find broken one
	for _, mr := range tree.MRs {
		if mr.Ready == "NotFound" || mr.Synced == "NotFound" {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Managed resource '%s' of kind '%s' was referenced but does not exist", mr.Name, mr.Kind),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s → %s/%s", tree.XRName, mr.Kind, mr.Name),
				SuggestedFix: "The provider may not have created the resource yet. Check provider pod logs and ProviderConfig credentials.",
				Details:      extractEventMessages(events),
			}
		}

		if mr.Synced == "False" || strings.HasPrefix(mr.Synced, "False") {
			rootCause, fix := analyzeMRError(mr)
			return Diagnosis{
				RootCause:    rootCause,
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s → %s/%s → ProviderConfig/%s", tree.XRName, mr.Kind, mr.Name, mr.ProviderConfigName),
				SuggestedFix: fix,
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 4: MRs exist but XR not ready
	if tree.XRReady != "True" {
		return Diagnosis{
			RootCause:    "All managed resources exist but XR is not yet ready",
			Severity:     "Warning",
			AffectedPath: tree.XRName,
			SuggestedFix: "Resources are being provisioned. Wait for cloud provider to complete creation.",
			Details:      extractEventMessages(events),
		}
	}

	// All good
	return Diagnosis{
		RootCause:    "No issues found",
		Severity:     "Info",
		AffectedPath: tree.XRName,
		SuggestedFix: "",
	}
}

func analyzeMRError(mr MRTreeInfo) (string, string) {
	synced := mr.Synced

	// connect failed — credentials issue
	if strings.Contains(synced, "connect failed") || strings.Contains(synced, "cannot initialize") {
		return fmt.Sprintf("Provider cannot connect to cloud API. ProviderConfig '%s' credentials may be invalid or missing", mr.ProviderConfigName),
			fmt.Sprintf("Check the credentials secret referenced by ProviderConfig '%s'. Ensure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are valid.", mr.ProviderConfigName)
	}

	// providerconfig not found
	if strings.Contains(synced, "ProviderConfig") && strings.Contains(synced, "not found") {
		return fmt.Sprintf("ProviderConfig '%s' referenced by managed resource '%s' does not exist", mr.ProviderConfigName, mr.Name),
			fmt.Sprintf("Create a ProviderConfig named '%s' in the correct namespace with valid cloud credentials.", mr.ProviderConfigName)
	}

	// quota/limit errors
	if strings.Contains(synced, "LimitExceeded") || strings.Contains(synced, "quota") {
		return fmt.Sprintf("Cloud provider quota or limit exceeded while creating '%s'", mr.Name),
			"Check your cloud provider quotas and request limit increases if needed."
	}

	// access denied
	if strings.Contains(synced, "AccessDenied") || strings.Contains(synced, "Forbidden") || strings.Contains(synced, "unauthorized") {
		return fmt.Sprintf("Access denied while creating '%s'. IAM permissions may be insufficient.", mr.Name),
			fmt.Sprintf("Check IAM permissions for the credentials in ProviderConfig '%s'. Ensure the role has required permissions.", mr.ProviderConfigName)
	}

	// already exists
	if strings.Contains(synced, "AlreadyExists") || strings.Contains(synced, "already exists") {
		return fmt.Sprintf("Resource '%s' already exists in the cloud provider", mr.Name),
			"Use crossplane.io/external-name annotation to import the existing resource, or delete it from the cloud provider first."
	}

	// generic
	return fmt.Sprintf("Managed resource '%s' failed to sync: %s", mr.Name, synced),
		"Check provider pod logs for more details: kubectl logs -n crossplane-system -l pkg.crossplane.io/revision"
}

func extractEventMessages(events []EventInfo) []string {
	var messages []string
	for _, e := range events {
		if e.Type == "Warning" {
			messages = append(messages, fmt.Sprintf("[%s] %s", e.Reason, e.Message))
		}
	}
	return messages
}
