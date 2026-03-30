package tools

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

func DebugMR(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	clientset kubernetes.Interface,
	restMapper meta.RESTMapper,
	group, version, kind, name, namespace string,
) (*MRDebugResult, error) {
	result := &MRDebugResult{
		Name:      name,
		Namespace: namespace,
		Kind:      kind,
		Group:     group,
	}

	// Step 1: get MR details
	//plural := kindToPlural(kind)
	plural, err := restMapper.RESTMapping(schema.GroupKind{
		Group: group,
		Kind:  kind,
	})

	if err != nil {
		return nil, err
	}

	mr, err := GetManagedResource(ctx, dynamicClient, group, version, kind, name, namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting MR: %w", err)
	}
	result.Ready = mr.Ready
	result.Synced = mr.Synced

	// Step 2: get conditions
	conditions, err := GetConditions(ctx, dynamicClient, group, version, plural.Resource.Resource, name, namespace)
	if err == nil {
		result.Conditions = conditions
	}

	// Step 3: get events by name
	events, err := GetEventsByUID(ctx, clientset, mr.UID)
	if err == nil {
		result.Events = events
	}

	// Step 4: check providerconfig
	pcGroup := mrGroupToProviderConfigGroup(group)
	pcName := mr.Provider
	if pcName != "" && pcName != "unknown" {
		pc, err := CheckProviderConfig(ctx, dynamicClient, clientset, restMapper,
			"ProviderConfig", pcGroup, pcName, namespace)
		if err == nil {
			result.ProviderConfig = pc
		}
	}

	// Step 5: diagnose
	result.Diagnosis = diagnoseMR(mr, result.Conditions, result.Events, result.ProviderConfig)

	return result, nil
}

func diagnoseMR(mr *ManagedResourceDetail, conditions []Condition, events []EventInfo, pc *ProviderConfig) Diagnosis {

	// Case 1: credentials secret missing
	if pc != nil && !pc.SecretExists {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Credentials secret '%s' referenced by ProviderConfig '%s' does not exist", pc.SecretName, pc.Name),
			Severity:     "Critical",
			AffectedPath: fmt.Sprintf("%s/%s → ProviderConfig/%s → Secret/%s", mr.Kind, mr.Name, pc.Name, pc.SecretName),
			SuggestedFix: fmt.Sprintf("Create secret: kubectl create secret generic %s -n %s --from-file=credentials=./aws-creds.txt", pc.SecretName, pc.CredentialNamespace),
			Details:      extractEventMessages(events),
		}
	}

	// Case 2: cannot connect to provider
	for _, e := range events {
		if strings.Contains(e.Message, "connect failed") ||
			strings.Contains(e.Message, "cannot initialize") ||
			strings.Contains(e.Message, "InvalidClientTokenId") {
			return Diagnosis{
				RootCause:    "Provider cannot connect to cloud API — credentials are invalid or expired",
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s/%s → ProviderConfig/%s", mr.Kind, mr.Name, pc.Name),
				SuggestedFix: "Update the credentials secret with valid AWS access key and secret key.",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 3: cross-resource reference not resolved
	for _, e := range events {
		if strings.Contains(e.Message, "referenced field was empty") ||
			strings.Contains(e.Message, "cannot resolve references") {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Cross-resource reference not resolved for '%s/%s' — a dependent resource may not be ready yet", mr.Kind, mr.Name),
				Severity:     "Warning",
				AffectedPath: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
				SuggestedFix: "Wait for the referenced resource to become Ready=True. Check if the referenced resource has valid cloud credentials.",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 4: missing required parameter
	for _, e := range events {
		if strings.Contains(e.Message, "MissingParameter") {
			param := extractMissingParameter(e.Message)
			return Diagnosis{
				RootCause:    fmt.Sprintf("Cloud API rejected resource creation — missing required parameter: %s", param),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
				SuggestedFix: fmt.Sprintf("Add a cross-resource reference patch for '%s' in the Composition.", param),
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 5: access denied
	for _, e := range events {
		if strings.Contains(e.Message, "AccessDenied") ||
			strings.Contains(e.Message, "Forbidden") {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Access denied creating '%s/%s' — IAM permissions insufficient", mr.Kind, mr.Name),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s/%s → ProviderConfig/%s", mr.Kind, mr.Name, pc.Name),
				SuggestedFix: "Check IAM policy attached to the credentials in the ProviderConfig secret.",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 6: resource already exists
	for _, e := range events {
		if strings.Contains(e.Message, "AlreadyExists") ||
			strings.Contains(e.Message, "already exists") {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Resource '%s/%s' already exists in the cloud provider", mr.Kind, mr.Name),
				Severity:     "Warning",
				AffectedPath: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
				SuggestedFix: "Use crossplane.io/external-name annotation to import the existing resource.",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 7: synced=false generic
	if strings.HasPrefix(mr.Synced, "False") {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Managed resource '%s/%s' failed to sync", mr.Kind, mr.Name),
			Severity:     "Critical",
			AffectedPath: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
			SuggestedFix: "Check provider pod logs for more details.",
			Details:      extractEventMessages(events),
		}
	}

	// All good
	return Diagnosis{
		RootCause:    "No issues found",
		Severity:     "Info",
		AffectedPath: fmt.Sprintf("%s/%s", mr.Kind, mr.Name),
		SuggestedFix: "",
	}
}

func extractMissingParameter(message string) string {
	needle := "The request must contain the parameter "
	if idx := strings.Index(message, needle); idx != -1 {
		param := message[idx+len(needle):]
		if end := strings.IndexAny(param, " \n\t"); end != -1 {
			param = param[:end]
		}
		return param
	}
	return "unknown"
}
