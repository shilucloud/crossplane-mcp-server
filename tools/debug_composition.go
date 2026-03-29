package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type CompositionDebugResult struct {
	Name      string
	Mode      string
	ForKind   string
	Diagnosis Diagnosis
	Functions []FunctionStatus
	XRsUsing  []string // XRs currently using this composition
	Issues    []string
}

type FunctionStatus struct {
	Name      string
	Healthy   bool
	Installed bool
	Message   string
}

func DebugComposition(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	name string,
) (*CompositionDebugResult, error) {
	result := &CompositionDebugResult{
		Name: name,
	}

	// Step 1: get composition
	comp, err := dynamicClient.Resource(compositionGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("composition '%s' not found: %w", name, err)
	}

	result.Mode = getNestedString(comp.Object, "spec", "mode")
	result.ForKind = getNestedString(comp.Object, "spec", "compositeTypeRef", "kind")
	forAPIVersion := getNestedString(comp.Object, "spec", "compositeTypeRef", "apiVersion")

	// Step 2: check all functions in pipeline
	pipeline := getNestedSlice(comp.Object, "spec", "pipeline")
	for _, step := range pipeline {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			continue
		}

		funcName := getNestedString(stepMap, "functionRef", "name")
		if funcName == "unknown" || funcName == "" {
			continue
		}

		fs := FunctionStatus{Name: funcName}

		fn, err := dynamicClient.Resource(functionGVR).Get(ctx, funcName, metav1.GetOptions{})
		if err != nil {
			fs.Healthy = false
			fs.Installed = false
			fs.Message = "function not installed"
			result.Issues = append(result.Issues,
				fmt.Sprintf("Function '%s' is not installed", funcName))
		} else {
			fs.Installed = true
			fs.Healthy = resolveConditionStatus(fn.Object, "Healthy") == "True"
			if !fs.Healthy {
				msg := resolveConditionStatus(fn.Object, "Healthy")
				fs.Message = msg
				result.Issues = append(result.Issues,
					fmt.Sprintf("Function '%s' is not healthy: %s", funcName, msg))
			}
		}

		result.Functions = append(result.Functions, fs)
	}

	// Step 3: find XRs using this composition
	// parse group and version from compositeTypeRef apiVersion
	xrGroup, xrVersion := splitAPIVersion(forAPIVersion)
	xrResource := strings.ToLower(result.ForKind) + "s"

	xrGVR := schemaGVR(xrGroup, xrVersion, xrResource)
	xrs, err := dynamicClient.Resource(xrGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, xr := range xrs.Items {
			compRef := getNestedString(xr.Object, "spec", "crossplane", "compositionRef", "name")
			if compRef == name {
				ns := xr.GetNamespace()
				xrName := xr.GetName()
				if ns != "" {
					result.XRsUsing = append(result.XRsUsing, fmt.Sprintf("%s/%s", ns, xrName))
				} else {
					result.XRsUsing = append(result.XRsUsing, xrName)
				}
			}
		}
	}

	// also check cluster-scoped XRs
	xrs, err = dynamicClient.Resource(xrGVR).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, xr := range xrs.Items {
			compRef := getNestedString(xr.Object, "spec", "crossplane", "compositionRef", "name")
			if compRef == name {
				result.XRsUsing = append(result.XRsUsing, xr.GetName())
			}
		}
	}

	// Step 4: check mode
	if result.Mode != "Pipeline" {
		result.Issues = append(result.Issues,
			fmt.Sprintf("Composition mode is '%s' — Pipeline mode is recommended in v2", result.Mode))
	}

	// Step 5: diagnose
	result.Diagnosis = diagnoseComposition(result)

	return result, nil
}

func diagnoseComposition(result *CompositionDebugResult) Diagnosis {

	// Case 1: function not installed
	for _, f := range result.Functions {
		if !f.Installed {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Function '%s' required by Composition is not installed", f.Name),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("Composition/%s → Function/%s", result.Name, f.Name),
				SuggestedFix: fmt.Sprintf("Install the function: kubectl apply -f - <<EOF\napiVersion: pkg.crossplane.io/v1\nkind: Function\nmetadata:\n  name: %s\nspec:\n  package: xpkg.upbound.io/crossplane-contrib/%s:latest\nEOF", f.Name, f.Name),
			}
		}
	}

	// Case 2: function not healthy
	for _, f := range result.Functions {
		if f.Installed && !f.Healthy {
			return Diagnosis{
				RootCause:    fmt.Sprintf("Function '%s' is installed but not healthy", f.Name),
				Severity:     "Critical",
				AffectedPath: fmt.Sprintf("Composition/%s → Function/%s", result.Name, f.Name),
				SuggestedFix: fmt.Sprintf("Check function pod: kubectl get pods -n crossplane-system -l pkg.crossplane.io/function=%s", f.Name),
			}
		}
	}

	// Case 3: not pipeline mode
	if result.Mode != "Pipeline" {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Composition '%s' uses deprecated mode '%s'", result.Name, result.Mode),
			Severity:     "Warning",
			AffectedPath: fmt.Sprintf("Composition/%s", result.Name),
			SuggestedFix: "Migrate to Pipeline mode — the Resources mode is deprecated in Crossplane v2.",
		}
	}

	// Case 4: no XRs using it
	if len(result.XRsUsing) == 0 {
		return Diagnosis{
			RootCause:    fmt.Sprintf("Composition '%s' exists but no XRs are using it", result.Name),
			Severity:     "Info",
			AffectedPath: fmt.Sprintf("Composition/%s", result.Name),
			SuggestedFix: "This may be intentional. Ensure XRs reference this composition via compositionRef or compositionSelector.",
		}
	}

	// All good
	return Diagnosis{
		RootCause:    fmt.Sprintf("Composition '%s' is healthy with %d XR(s) using it", result.Name, len(result.XRsUsing)),
		Severity:     "Info",
		AffectedPath: fmt.Sprintf("Composition/%s", result.Name),
		SuggestedFix: "",
	}
}
