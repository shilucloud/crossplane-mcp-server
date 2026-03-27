package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	compositionGVRForSuggest = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v1",
		Resource: "compositions",
	}
)

type CompositionInfoBrief struct {
	Name            string
	Namespace       string
	Mode            string
	PipelineLength  int
	EnvironmentRefs []string
	ResourcesCount  int
	Functions       []string
}

type CompositionSuggestion struct {
	Composition *CompositionInfoBrief
	Score       int
	Reasons     []string
}

type ListCompatibleCompositionsResult struct {
	XRDKind      string
	Compositions []CompositionInfoBrief
	DefaultFound bool
	DefaultName  string
}

func ListCompatibleCompositions(ctx context.Context, client dynamic.Interface, xrdKind string) (*ListCompatibleCompositionsResult, error) {
	xrdSchema, err := GetXRDSchema(ctx, client, xrdKind)
	if err != nil {
		return nil, fmt.Errorf("failed to get XRD schema: %w", err)
	}

	result := &ListCompatibleCompositionsResult{
		XRDKind:      xrdKind,
		Compositions: []CompositionInfoBrief{},
	}

	if xrdSchema.DefaultCompositionRef != nil {
		result.DefaultFound = true
		result.DefaultName = *xrdSchema.DefaultCompositionRef
	}

	compositions, err := client.Resource(compositionGVRForSuggest).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list compositions: %w", err)
	}

	for _, comp := range compositions.Items {
		compName := comp.GetName()
		compNamespace := comp.GetNamespace()

		compositeType := getNestedString(comp.Object, "spec", "compositeTypeRef", "kind")
		if compositeType != xrdKind && compositeType != "" {
			continue
		}

		pipeline := getSlice(comp.Object, "spec", "pipeline")
		functions := []string{}
		for _, p := range pipeline {
			if fn, ok := p.(map[string]interface{}); ok {
				if ref := getStringFromPath(fn, "ref", "name"); ref != "" {
					functions = append(functions, ref)
				}
			}
		}

		environmentRefs := getStringSliceFromPath(comp.Object, "spec", "environment", "policy", "allowedSources", "environmentSourceRefs")
		if environmentRefs == nil {
			environmentRefs = []string{}
		}

		resources := getSlice(comp.Object, "spec", "resources")
		functionsFromBase := getSlice(comp.Object, "spec", "base", "resources")

		result.Compositions = append(result.Compositions, CompositionInfoBrief{
			Name:            compName,
			Namespace:       compNamespace,
			Mode:            getNestedString(comp.Object, "spec", "mode"),
			PipelineLength:  len(pipeline),
			EnvironmentRefs: environmentRefs,
			ResourcesCount:  len(resources) + len(functionsFromBase),
			Functions:       functions,
		})
	}

	return result, nil
}

func SuggestComposition(ctx context.Context, client dynamic.Interface, xrdKind string, requirements map[string]interface{}) (*CompositionSuggestion, error) {
	result, err := ListCompatibleCompositions(ctx, client, xrdKind)
	if err != nil {
		return nil, err
	}

	if len(result.Compositions) == 0 {
		return nil, fmt.Errorf("no compositions found for XRD kind: %s", xrdKind)
	}

	if result.DefaultFound {
		for i, comp := range result.Compositions {
			if comp.Name == result.DefaultName {
				return &CompositionSuggestion{
					Composition: &result.Compositions[i],
					Score:       100,
					Reasons:     []string{"Default composition for this XRD"},
				}, nil
			}
		}
	}

	var best *CompositionSuggestion
	bestScore := 0

	for i := range result.Compositions {
		comp := &result.Compositions[i]
		score := 0
		reasons := []string{}

		score += comp.PipelineLength * 5
		if comp.PipelineLength > 0 {
			reasons = append(reasons, fmt.Sprintf("Has %d function(s) in pipeline", comp.PipelineLength))
		}

		if comp.Mode == "Pipeline" {
			score += 10
			reasons = append(reasons, "Uses Pipeline mode (recommended)")
		}

		if len(requirements) > 0 {
			requirementsStr := fmt.Sprintf("%v", requirements)
			for _, fn := range comp.Functions {
				if strings.Contains(requirementsStr, fn) {
					score += 20
					reasons = append(reasons, fmt.Sprintf("Contains required function: %s", fn))
				}
			}

			if envReq, ok := requirements["environment"].(string); ok {
				for _, envRef := range comp.EnvironmentRefs {
					if envRef == envReq {
						score += 15
						reasons = append(reasons, fmt.Sprintf("Supports environment: %s", envReq))
					}
				}
			}
		}

		if score > bestScore {
			bestScore = score
			best = &CompositionSuggestion{
				Composition: comp,
				Score:       score,
				Reasons:     reasons,
			}
		}
	}

	if best == nil && len(result.Compositions) > 0 {
		best = &CompositionSuggestion{
			Composition: &result.Compositions[0],
			Score:       50,
			Reasons:     []string{"First available composition"},
		}
	}

	return best, nil
}

func GetCompositionDetails(ctx context.Context, client dynamic.Interface, name string) (map[string]interface{}, error) {
	comp, err := client.Resource(compositionGVRForSuggest).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get composition: %w", err)
	}

	details := map[string]interface{}{
		"name":           comp.GetName(),
		"namespace":      comp.GetNamespace(),
		"mode":           getNestedString(comp.Object, "spec", "mode"),
		"compositeKind":  getNestedString(comp.Object, "spec", "compositeTypeRef", "kind"),
		"compositeGroup": getNestedString(comp.Object, "spec", "compositeTypeRef", "apiVersion"),
	}

	pipeline := getSlice(comp.Object, "spec", "pipeline")
	functions := []map[string]interface{}{}
	for _, p := range pipeline {
		if fn, ok := p.(map[string]interface{}); ok {
			fnInfo := map[string]interface{}{
				"name": getStringFromPath(fn, "ref", "name"),
			}
			if img := getStringFromPath(fn, "functionRef", "name"); img != "" {
				fnInfo["name"] = img
			}
			functions = append(functions, fnInfo)
		}
	}
	details["functions"] = functions

	resources := getSlice(comp.Object, "spec", "resources")
	details["resourcesCount"] = len(resources)

	connectionDetails := getSlice(comp.Object, "spec", "publishConnectionDetailsTo")
	details["publishesConnectionSecrets"] = len(connectionDetails) > 0

	return details, nil
}

func ListAllCompositions(ctx context.Context, client dynamic.Interface) ([]CompositionInfoBrief, error) {
	compositions, err := client.Resource(compositionGVRForSuggest).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list compositions: %w", err)
	}

	var result []CompositionInfoBrief
	for _, comp := range compositions.Items {
		compName := comp.GetName()
		compNamespace := comp.GetNamespace()

		pipeline := getSlice(comp.Object, "spec", "pipeline")
		functions := []string{}
		for _, p := range pipeline {
			if fn, ok := p.(map[string]interface{}); ok {
				if ref := getStringFromPath(fn, "ref", "name"); ref != "" {
					functions = append(functions, ref)
				}
			}
		}

		resources := getSlice(comp.Object, "spec", "resources")
		functionsFromBase := getSlice(comp.Object, "spec", "base", "resources")

		result = append(result, CompositionInfoBrief{
			Name:            compName,
			Namespace:       compNamespace,
			Mode:            getNestedString(comp.Object, "spec", "mode"),
			PipelineLength:  len(pipeline),
			EnvironmentRefs: []string{},
			ResourcesCount:  len(resources) + len(functionsFromBase),
			Functions:       functions,
		})
	}

	return result, nil
}

func ValidateCompositionExists(ctx context.Context, client dynamic.Interface, name string) (bool, error) {
	_, err := client.Resource(compositionGVRForSuggest).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return false, nil
	}
	return true, nil
}

func getStringSliceFromPath(obj map[string]interface{}, keys ...string) []string {
	val := obj
	for _, key := range keys {
		if val == nil {
			return nil
		}
		next, ok := val[key]
		if !ok {
			return nil
		}
		mapVal, ok := next.(map[string]interface{})
		if !ok {
			if slice, ok := next.([]interface{}); ok {
				var result []string
				for _, item := range slice {
					if m, ok := item.(map[string]interface{}); ok {
						if name := getStringFromPath(m, "name"); name != "" {
							result = append(result, name)
						}
					}
				}
				return result
			}
			return nil
		}
		val = mapVal
	}
	return nil
}

func getStringFromPath(obj map[string]interface{}, keys ...string) string {
	if len(keys) == 0 {
		return ""
	}
	val := obj
	for i, key := range keys {
		if val == nil {
			return ""
		}
		next, ok := val[key]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			if s, ok := next.(string); ok {
				return s
			}
			return ""
		}
		mapVal, ok := next.(map[string]interface{})
		if !ok {
			return ""
		}
		val = mapVal
	}
	return ""
}
