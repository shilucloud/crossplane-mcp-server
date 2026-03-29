package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	functionGVR = schema.GroupVersionResource{
		Group:    "pkg.crossplane.io",
		Version:  "v1",
		Resource: "functions",
	}
	mrdGVR3 = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1alpha1",
		Resource: "managedresourcedefinitions",
	}
	usageGVR = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1beta1",
		Resource: "usages",
	}
)

type Diagnosis struct {
	RootCause    string
	Severity     string
	AffectedPath string
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
	var allEvents []EventInfo

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

	// Step 2: collect events from XR
	xrEvents, err := GetEventsByUID(ctx, clientset, tree.UID)
	if err == nil {
		allEvents = append(allEvents, xrEvents...)
	}

	// collect events from MRs
	for _, mr := range tree.MRs {
		mrEvents, err := GetEventsByUID(ctx, clientset, mr.UID)
		if err == nil {
			allEvents = append(allEvents, mrEvents...)
		}
	}

	// collect events from providers
	seenProviders := map[string]bool{}
	for _, mr := range tree.MRs {
		pcGroup := mrGroupToProviderConfigGroup(mr.Group)
		if seenProviders[pcGroup] {
			continue
		}
		seenProviders[pcGroup] = true
		providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, p := range providers.Items {
				providerEvents, err := GetEventsByUID(ctx, clientset, string(p.GetUID()))
				if err == nil {
					allEvents = append(allEvents, providerEvents...)
				}
			}
		}
	}

	result.Events = allEvents
	result.Diagnosis = diagnoseWithContext(ctx, dynamicClient, clientset, tree, allEvents, namespace)

	return result, nil
}

func diagnoseWithContext(ctx context.Context, dynamicClient dynamic.Interface, clientset kubernetes.Interface, tree *XRTreeInfo, events []EventInfo, namespace string) Diagnosis {

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

	// Case 1.1: cluster-scoped MR in namespaced XR
	for _, e := range events {
		if strings.Contains(e.Message, "cluster scoped composed resource") &&
			strings.Contains(e.Message, "namespaced composite resource") {
			return Diagnosis{
				RootCause:    "Composition is using cluster-scoped managed resources but XR is namespaced",
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s → %s", tree.XRName, tree.CompositionInfo.Name),
				SuggestedFix: "Update the Composition to use namespaced provider resources (e.g. s3.aws.m.upbound.io instead of s3.aws.upbound.io), or change the XRD scope to Cluster.",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 1.2: function pipeline error
	for _, e := range events {
		if strings.Contains(e.Message, "cannot run pipeline") ||
			(strings.Contains(e.Message, "function") && strings.Contains(e.Message, "failed")) {
			funcName := extractFunctionName(e.Message)
			return Diagnosis{
				RootCause:    fmt.Sprintf("Composition pipeline function '%s' failed", funcName),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s → %s → function/%s", tree.XRName, tree.CompositionInfo.Name, funcName),
				SuggestedFix: fmt.Sprintf("Check function '%s' pod logs: kubectl logs -n crossplane-system -l pkg.crossplane.io/function=%s", funcName, funcName),
				Details:      extractEventMessages(events),
			}
		}
	}

	// Case 2: composition selected but no MRs
	if len(tree.MRs) == 0 {
		// Case 2.1: check if function is not installed
		funcDiag := checkFunctionsInstalled(ctx, dynamicClient, tree.CompositionInfo)
		if funcDiag != nil {
			funcDiag.Details = extractEventMessages(events)
			return *funcDiag
		}

		// Case 2.2: check if MRD is inactive
		mrdDiag := checkMRDActive(ctx, dynamicClient, events)
		if mrdDiag != nil {
			mrdDiag.Details = extractEventMessages(events)
			return *mrdDiag
		}

		return Diagnosis{
			RootCause:    fmt.Sprintf("Composition '%s' selected but no managed resources created yet", tree.CompositionInfo.Name),
			Severity:     "Warning",
			AffectedPath: fmt.Sprintf("%s → %s", tree.XRName, tree.CompositionInfo.Name),
			SuggestedFix: "Check the Composition pipeline for errors. Ensure all functions are installed and healthy.",
			Details:      extractEventMessages(events),
		}
	}

	// Case 3: XR stuck in terminating — check usages
	if strings.Contains(tree.XRSynced, "Terminating") {
		usageDiag := checkUsagesBlocking(ctx, dynamicClient, tree.XRName, namespace)
		if usageDiag != nil {
			usageDiag.Details = extractEventMessages(events)
			return *usageDiag
		}
	}

	// Case 4: walk MRs
	for _, mr := range tree.MRs {
		// Case 4.1: MR NotFound
		if mr.Ready == "NotFound" || mr.Synced == "NotFound" {
			mrdDiag := checkMRDActiveForKind(ctx, dynamicClient, mr.Kind, mr.Group)
			if mrdDiag != nil {
				mrdDiag.Details = extractEventMessages(events)
				return *mrdDiag
			}

			providerDiag := checkProviderHealthForMR(ctx, dynamicClient, mr.Group)
			if providerDiag != nil {
				providerDiag.Details = extractEventMessages(events)
				return *providerDiag
			}

			return Diagnosis{
				RootCause:    fmt.Sprintf("Managed resource '%s' of kind '%s' was referenced but does not exist", mr.Name, mr.Kind),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("%s → %s/%s", tree.XRName, mr.Kind, mr.Name),
				SuggestedFix: "The provider may not have created the resource yet. Check provider pod logs and ProviderConfig credentials.",
				Details:      extractEventMessages(events),
			}
		}

		// Case 4.2: MR Synced=False
		if strings.HasPrefix(mr.Synced, "False") {
			credDiag := checkCredentialsSecret(ctx, clientset, dynamicClient, mr, namespace)
			if credDiag != nil {
				credDiag.Details = extractEventMessages(events)
				return *credDiag
			}

			providerDiag := checkProviderHealthForMR(ctx, dynamicClient, mr.Group)
			if providerDiag != nil {
				providerDiag.Details = extractEventMessages(events)
				return *providerDiag
			}

			if strings.Contains(mr.Synced, "not found") && !strings.Contains(mr.Synced, "ProviderConfig") {
				return Diagnosis{
					RootCause:    fmt.Sprintf("External resource for '%s/%s' may have been deleted manually in the cloud provider", mr.Kind, mr.Name),
					Severity:     "Critical",
					AffectedPath: fmt.Sprintf("%s → %s/%s", tree.XRName, mr.Kind, mr.Name),
					SuggestedFix: "The cloud resource may have been deleted outside Crossplane. Trigger reconcile: kubectl annotate managed " + mr.Name + " crossplane.io/paused=false",
					Details:      extractEventMessages(events),
				}
			}

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

	// Case 5: MRs exist but XR not ready
	if tree.XRReady != "True" {
		return Diagnosis{
			RootCause:    "All managed resources exist but XR is not yet ready",
			Severity:     "Warning",
			AffectedPath: tree.XRName,
			SuggestedFix: "Resources are being provisioned. Wait for cloud provider to complete creation.",
			Details:      extractEventMessages(events),
		}
	}

	// rbac forbidden
	for _, e := range events {
		if strings.Contains(e.Message, "forbidden") &&
			strings.Contains(e.Message, "cannot patch resource") {
			return Diagnosis{
				RootCause:    "Crossplane service account lacks RBAC permissions to manage namespaced MRs",
				Severity:     "Critical",
				AffectedPath: tree.XRName,
				SuggestedFix: "Grant permissions: kubectl create clusterrolebinding crossplane-admin --clusterrole=cluster-admin --serviceaccount=crossplane-system:crossplane",
				Details:      extractEventMessages(events),
			}
		}
	}

	// Resource not found in API group (MRD inactive or provider not installed)
	for _, e := range events {
		if strings.Contains(e.Message, "no resource found for") &&
			strings.Contains(e.Message, "Kind=") {
			// extract the missing kind from message
			kind := extractKindFromMessage(e.Message)
			return Diagnosis{
				RootCause:    fmt.Sprintf("Resource type '%s' not found — provider may not be installed or MRD is inactive", kind),
				Severity:     "Critical",
				AffectedPath: tree.XRName,
				SuggestedFix: "Check if the provider is installed and healthy: kubectl get providers. If using v2 MRDs, check MRD state is Active.",
				Details:      extractEventMessages(events),
			}
		}
	}

	return Diagnosis{
		RootCause:    "No issues found",
		Severity:     "Info",
		AffectedPath: tree.XRName,
		SuggestedFix: "",
	}
}

func checkFunctionsInstalled(ctx context.Context, dynamicClient dynamic.Interface, compInfo CompositionInfo) *Diagnosis {
	for _, step := range compInfo.Pipeline {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			continue
		}
		funcName := getNestedString(stepMap, "functionRef", "name")
		if funcName == "unknown" || funcName == "" {
			continue
		}

		fn, err := dynamicClient.Resource(functionGVR).Get(ctx, funcName, metav1.GetOptions{})
		if err != nil {
			return &Diagnosis{
				RootCause:    fmt.Sprintf("Function '%s' required by Composition is not installed", funcName),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("Composition → function/%s", funcName),
				SuggestedFix: fmt.Sprintf("Install the function: kubectl apply -f https://raw.githubusercontent.com/crossplane-contrib/%s/main/package/crds/...", funcName),
			}
		}

		healthy := resolveConditionStatus(fn.Object, "Healthy")
		if healthy != "True" {
			return &Diagnosis{
				RootCause:    fmt.Sprintf("Function '%s' is installed but not healthy: %s", funcName, healthy),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("Composition → function/%s", funcName),
				SuggestedFix: fmt.Sprintf("Check function pod: kubectl get pods -n crossplane-system -l pkg.crossplane.io/function=%s", funcName),
			}
		}
	}
	return nil
}

func checkMRDActive(ctx context.Context, dynamicClient dynamic.Interface, events []EventInfo) *Diagnosis {
	for _, e := range events {
		if strings.Contains(e.Message, "no kind") || strings.Contains(e.Message, "resource type not found") {
			return &Diagnosis{
				RootCause:    "Managed resource type may not be activated (MRD inactive)",
				Severity:     "Critical",
				AffectedPath: "ManagedResourceDefinition",
				SuggestedFix: "Apply a ManagedResourceActivationPolicy to activate the required resource type, or set the MRD state to Active.",
			}
		}
	}
	return nil
}

func checkMRDActiveForKind(ctx context.Context, dynamicClient dynamic.Interface, kind, group string) *Diagnosis {
	mrds, err := dynamicClient.Resource(mrdGVR3).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, mrd := range mrds.Items {
		mrdKind := getNestedString(mrd.Object, "spec", "names", "kind")
		mrdGroup := getNestedString(mrd.Object, "spec", "group")
		if mrdKind == kind && mrdGroup == group {
			state := getNestedString(mrd.Object, "spec", "state")
			if state == "Inactive" {
				return &Diagnosis{
					RootCause:    fmt.Sprintf("ManagedResourceDefinition for '%s.%s' exists but is Inactive — CRD is not installed", kind, group),
					Severity:     "Critical",
					AffectedPath: fmt.Sprintf("MRD/%s.%s", kind, group),
					SuggestedFix: fmt.Sprintf("Activate the MRD: kubectl patch mrd %s.%s --type=merge -p '{\"spec\":{\"state\":\"Active\"}}'", strings.ToLower(kind), group),
				}
			}
		}
	}
	return nil
}

func checkProviderHealthForMR(ctx context.Context, dynamicClient dynamic.Interface, mrGroup string) *Diagnosis {
	providers, err := dynamicClient.Resource(providerGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, p := range providers.Items {
		healthy := resolveConditionStatus(p.Object, "Healthy")
		installed := resolveConditionStatus(p.Object, "Installed")
		if healthy != "True" || installed != "True" {
			return &Diagnosis{
				RootCause:    fmt.Sprintf("Provider '%s' is not healthy (Healthy=%s, Installed=%s)", p.GetName(), healthy, installed),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("Provider/%s", p.GetName()),
				SuggestedFix: fmt.Sprintf("Check provider pod: kubectl get pods -n crossplane-system -l pkg.crossplane.io/revision=%s", p.GetName()),
			}
		}
	}
	return nil
}

func checkCredentialsSecret(ctx context.Context, clientset kubernetes.Interface, dynamicClient dynamic.Interface, mr MRTreeInfo, namespace string) *Diagnosis {
	if mr.ProviderConfigName == "" || mr.ProviderConfigName == "unknown" {
		return nil
	}

	pcGroup := mrGroupToProviderConfigGroup(mr.Group)
	pcGVR := schema.GroupVersionResource{
		Group:    pcGroup,
		Version:  "v1beta1",
		Resource: "providerconfigs",
	}

	var pcObject map[string]interface{}
	var err error

	if namespace != "" {
		o, e := dynamicClient.Resource(pcGVR).Namespace(namespace).Get(ctx, mr.ProviderConfigName, metav1.GetOptions{})
		err = e
		if o != nil {
			pcObject = o.Object
		}
	} else {
		o, e := dynamicClient.Resource(pcGVR).Get(ctx, mr.ProviderConfigName, metav1.GetOptions{})
		err = e
		if o != nil {
			pcObject = o.Object
		}
	}

	if err != nil {
		return &Diagnosis{
			RootCause:    fmt.Sprintf("ProviderConfig '%s' not found in group '%s'", mr.ProviderConfigName, pcGroup),
			Severity:     "Critical",
			AffectedPath: fmt.Sprintf("ProviderConfig/%s", mr.ProviderConfigName),
			SuggestedFix: fmt.Sprintf("Create ProviderConfig '%s' in group '%s' with valid credentials.", mr.ProviderConfigName, pcGroup),
		}
	}

	if pcObject == nil {
		return nil
	}

	secretName := getNestedString(pcObject, "spec", "credentials", "secretRef", "name")
	secretNS := getNestedString(pcObject, "spec", "credentials", "secretRef", "namespace")

	if secretName != "unknown" && secretName != "" {
		_, err := clientset.CoreV1().Secrets(secretNS).Get(ctx, secretName, metav1.GetOptions{})
		if err != nil {
			return &Diagnosis{
				RootCause:    fmt.Sprintf("Credentials secret '%s' in namespace '%s' referenced by ProviderConfig '%s' does not exist", secretName, secretNS, mr.ProviderConfigName),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("ProviderConfig/%s → Secret/%s", mr.ProviderConfigName, secretName),
				SuggestedFix: fmt.Sprintf("Create the credentials secret: kubectl create secret generic %s -n %s --from-file=credentials=./aws-creds.txt", secretName, secretNS),
			}
		}
	}

	return nil
}

func checkUsagesBlocking(ctx context.Context, dynamicClient dynamic.Interface, name, namespace string) *Diagnosis {
	usages, err := dynamicClient.Resource(usageGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}
	for _, u := range usages.Items {
		ofName := getNestedString(u.Object, "spec", "of", "resourceRef", "name")
		if ofName == name {
			byKind := getNestedString(u.Object, "spec", "by", "apiVersion")
			byName := getNestedString(u.Object, "spec", "by", "resourceRef", "name")
			return &Diagnosis{
				RootCause:    fmt.Sprintf("Resource '%s' is protected by a Usage resource — deletion is blocked", name),
				Severity:     "Warning",
				AffectedPath: fmt.Sprintf("%s → Usage/%s", name, u.GetName()),
				SuggestedFix: fmt.Sprintf("Delete the dependent resource first: %s/%s, then retry deleting '%s'.", byKind, byName, name),
			}
		}
	}
	return nil
}

func extractFunctionName(message string) string {
	if idx := strings.Index(message, "\""); idx != -1 {
		end := strings.Index(message[idx+1:], "\"")
		if end != -1 {
			return message[idx+1 : idx+1+end]
		}
	}
	return "unknown"
}

func analyzeMRError(mr MRTreeInfo) (string, string) {
	synced := mr.Synced

	if strings.Contains(synced, "referenced field was empty") ||
		strings.Contains(synced, "cannot resolve references") {
		return fmt.Sprintf("Cross-resource reference not resolved yet for '%s' — a dependent resource may not be ready", mr.Name),
			"Check if the referenced resource (e.g. VPC) is Ready=True. If credentials are invalid, the referenced resource may appear created but have no external ID."
	}

	if strings.Contains(synced, "connect failed") || strings.Contains(synced, "cannot initialize") {
		return fmt.Sprintf("Provider cannot connect to cloud API. ProviderConfig '%s' credentials may be invalid or missing", mr.ProviderConfigName),
			fmt.Sprintf("Check the credentials secret referenced by ProviderConfig '%s'. Ensure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY are valid.", mr.ProviderConfigName)
	}

	if strings.Contains(synced, "ProviderConfig") && strings.Contains(synced, "not found") {
		return fmt.Sprintf("ProviderConfig '%s' referenced by managed resource '%s' does not exist", mr.ProviderConfigName, mr.Name),
			fmt.Sprintf("Create a ProviderConfig named '%s' in the correct namespace with valid cloud credentials.", mr.ProviderConfigName)
	}

	if strings.Contains(synced, "LimitExceeded") || strings.Contains(synced, "quota") {
		return fmt.Sprintf("Cloud provider quota or limit exceeded while creating '%s'", mr.Name),
			"Check your cloud provider quotas and request limit increases if needed."
	}

	if strings.Contains(synced, "AccessDenied") || strings.Contains(synced, "Forbidden") || strings.Contains(synced, "unauthorized") {
		return fmt.Sprintf("Access denied while creating '%s'. IAM permissions may be insufficient.", mr.Name),
			fmt.Sprintf("Check IAM permissions for the credentials in ProviderConfig '%s'. Ensure the role has required permissions.", mr.ProviderConfigName)
	}

	if strings.Contains(synced, "AlreadyExists") || strings.Contains(synced, "already exists") {
		return fmt.Sprintf("Resource '%s' already exists in the cloud provider", mr.Name),
			"Use crossplane.io/external-name annotation to import the existing resource, or delete it from the cloud provider first."
	}

	return fmt.Sprintf("Managed resource '%s' failed to sync: %s", mr.Name, synced),
		"Check provider pod logs for more details: kubectl logs -n crossplane-system -l pkg.crossplane.io/revision"
}

func extractEventMessages(events []EventInfo) []string {
	var messages []string
	for _, e := range events {
		if e.Type == "Warning" {
			msg := fmt.Sprintf("[%s] %s (%s)", e.Reason, e.Message, e.Object)
			messages = append(messages, msg)
		}
	}
	return messages
}

func extractKindFromMessage(message string) string {
	// "no resource found for ec2.aws.m.upbound.io/v1beta1, Kind=VPC"
	if idx := strings.Index(message, "Kind="); idx != -1 {
		kind := message[idx+5:]
		// trim anything after space or quote
		if end := strings.IndexAny(kind, " \"'"); end != -1 {
			kind = kind[:end]
		}
		return kind
	}
	return "unknown"
}
