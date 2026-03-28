package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

type TransformInfo struct {
	Type    string
	Map     map[string]interface{}
	Convert string
	Math    map[string]interface{}
}

type CombineVariable struct {
	FromFieldPath string
}

type CombineInfo struct {
	Strategy  string
	Format    string
	Variables []CombineVariable
}

type PatchInfo struct {
	Type          string
	FromFieldPath string
	ToFieldPath   string
	PatchSetName  string
	Policy        string
	Transforms    []TransformInfo
	Combine       *CombineInfo
}

type PatchSet struct {
	Name    string
	Patches []PatchInfo
}

type ResourceBlockInfo struct {
	Name           string
	APIVersion     string
	Kind           string
	ProviderConfig string
	DefaultValues  map[string]string
	Patches        []PatchInfo
}

type PipelineStep struct {
	StepName     string
	FunctionName string
	Resources    []ResourceBlockInfo
	PatchSets    []PatchSet
}

type CompositionExplanation struct {
	Name           string
	Mode           string
	ForKind        string
	ForAPIVersion  string
	Pipeline       []PipelineStep
	Summary        string
	TotalResources int
	TotalFunctions int
}

func ExplainComposition(ctx context.Context, dynamicClient dynamic.Interface, name string) (*CompositionExplanation, error) {
	comp, err := dynamicClient.Resource(compositionGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error getting composition: %w", err)
	}

	result := &CompositionExplanation{
		Name:          name,
		Mode:          getNestedString(comp.Object, "spec", "mode"),
		ForKind:       getNestedString(comp.Object, "spec", "compositeTypeRef", "kind"),
		ForAPIVersion: getNestedString(comp.Object, "spec", "compositeTypeRef", "apiVersion"),
	}

	// parse pipeline steps
	pipeline := getNestedSlice(comp.Object, "spec", "pipeline")
	for _, step := range pipeline {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			continue
		}

		ps := PipelineStep{
			StepName:     getString(stepMap, "step"),
			FunctionName: getNestedString(stepMap, "functionRef", "name"),
		}
		result.TotalFunctions++

		// parse input
		input, ok := stepMap["input"].(map[string]interface{})
		if !ok {
			result.Pipeline = append(result.Pipeline, ps)
			continue
		}

		// parse patchSets from input
		if patchSets, ok := input["patchSets"].([]interface{}); ok {
			for _, psi := range patchSets {
				psMap, ok := psi.(map[string]interface{})
				if !ok {
					continue
				}
				patchSet := PatchSet{
					Name: getString(psMap, "name"),
				}
				if patches, ok := psMap["patches"].([]interface{}); ok {
					for _, p := range patches {
						pMap, ok := p.(map[string]interface{})
						if !ok {
							continue
						}
						patchSet.Patches = append(patchSet.Patches, parsePatch(pMap))
					}
				}
				ps.PatchSets = append(ps.PatchSets, patchSet)
			}
		}

		// parse resources from input
		resources, ok := input["resources"].([]interface{})
		if !ok {
			result.Pipeline = append(result.Pipeline, ps)
			continue
		}

		for _, r := range resources {
			rMap, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			ri := ResourceBlockInfo{
				Name:          getString(rMap, "name"),
				DefaultValues: map[string]string{},
			}

			// extract base
			base, ok := rMap["base"].(map[string]interface{})
			if ok {
				ri.APIVersion = getString(base, "apiVersion")
				ri.Kind = getString(base, "kind")

				spec, ok := base["spec"].(map[string]interface{})
				if ok {
					ri.ProviderConfig = getNestedString(spec, "providerConfigRef", "name")

					forProvider, ok := spec["forProvider"].(map[string]interface{})
					if ok {
						for k, v := range forProvider {
							if s, ok := v.(string); ok {
								ri.DefaultValues[k] = s
							}
						}
					}
				}
			}

			// extract patches
			patches, ok := rMap["patches"].([]interface{})
			if ok {
				for _, p := range patches {
					pMap, ok := p.(map[string]interface{})
					if !ok {
						continue
					}
					ri.Patches = append(ri.Patches, parsePatch(pMap))
				}
			}

			ps.Resources = append(ps.Resources, ri)
			result.TotalResources++
		}

		result.Pipeline = append(result.Pipeline, ps)
	}

	result.Summary = buildSummary(result)

	return result, nil
}

func parsePatch(pMap map[string]interface{}) PatchInfo {
	pi := PatchInfo{
		Type:          getString(pMap, "type"),
		FromFieldPath: getString(pMap, "fromFieldPath"),
		ToFieldPath:   getString(pMap, "toFieldPath"),
		PatchSetName:  getString(pMap, "patchSetName"),
	}

	// parse policy
	if policy, ok := pMap["policy"].(map[string]interface{}); ok {
		pi.Policy = getString(policy, "fromFieldPath")
	}

	// parse transforms
	if transforms, ok := pMap["transforms"].([]interface{}); ok {
		for _, t := range transforms {
			tMap, ok := t.(map[string]interface{})
			if !ok {
				continue
			}
			ti := TransformInfo{
				Type: getString(tMap, "type"),
			}
			if m, ok := tMap["map"].(map[string]interface{}); ok {
				ti.Map = m
			}
			if convert, ok := tMap["convert"].(map[string]interface{}); ok {
				ti.Convert = getString(convert, "toType")
			}
			if math, ok := tMap["math"].(map[string]interface{}); ok {
				ti.Math = math
			}
			pi.Transforms = append(pi.Transforms, ti)
		}
	}

	// parse combine
	if combine, ok := pMap["combine"].(map[string]interface{}); ok {
		ci := &CombineInfo{
			Strategy: getString(combine, "strategy"),
		}
		if str, ok := combine["string"].(map[string]interface{}); ok {
			ci.Format = getString(str, "fmt")
		}
		if vars, ok := combine["variables"].([]interface{}); ok {
			for _, v := range vars {
				vMap, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				ci.Variables = append(ci.Variables, CombineVariable{
					FromFieldPath: getString(vMap, "fromFieldPath"),
				})
			}
		}
		pi.Combine = ci
	}

	return pi
}

func buildSummary(c *CompositionExplanation) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Composition '%s' handles '%s' resources.\n", c.Name, c.ForKind))
	sb.WriteString(fmt.Sprintf("Mode: %s | Functions: %d | Total Resources: %d\n\n",
		c.Mode, c.TotalFunctions, c.TotalResources))

	for _, step := range c.Pipeline {
		sb.WriteString(fmt.Sprintf("Step '%s' uses function '%s':\n", step.StepName, step.FunctionName))

		if len(step.PatchSets) > 0 {
			sb.WriteString("  PatchSets:\n")
			for _, ps := range step.PatchSets {
				sb.WriteString(fmt.Sprintf("    [%s]\n", ps.Name))
				for _, p := range ps.Patches {
					sb.WriteString(fmt.Sprintf("      %s → %s\n", p.FromFieldPath, p.ToFieldPath))
				}
			}
		}

		for _, r := range step.Resources {
			sb.WriteString(fmt.Sprintf("  Creates: %s (%s)\n", r.Kind, r.APIVersion))
			sb.WriteString(fmt.Sprintf("  ProviderConfig: %s\n", r.ProviderConfig))

			if len(r.DefaultValues) > 0 {
				sb.WriteString("  Default values:\n")
				for k, v := range r.DefaultValues {
					sb.WriteString(fmt.Sprintf("    %s: %s\n", k, v))
				}
			}

			if len(r.Patches) > 0 {
				sb.WriteString("  Field mappings:\n")
				for _, p := range r.Patches {
					switch p.Type {
					case "FromCompositeFieldPath":
						sb.WriteString(fmt.Sprintf("    %s → %s", p.FromFieldPath, p.ToFieldPath))
						if len(p.Transforms) > 0 {
							sb.WriteString(fmt.Sprintf(" (with %d transform(s))", len(p.Transforms)))
						}
						if p.Policy != "" {
							sb.WriteString(fmt.Sprintf(" [policy: %s]", p.Policy))
						}
						sb.WriteString("\n")
					case "CombineFromComposite":
						if p.Combine != nil {
							vars := []string{}
							for _, v := range p.Combine.Variables {
								vars = append(vars, v.FromFieldPath)
							}
							sb.WriteString(fmt.Sprintf("    combine(%s) → %s [strategy: %s",
								strings.Join(vars, " + "), p.ToFieldPath, p.Combine.Strategy))
							if p.Combine.Format != "" {
								sb.WriteString(fmt.Sprintf(", fmt: %s", p.Combine.Format))
							}
							sb.WriteString("]\n")
						}
					case "PatchSet":
						sb.WriteString(fmt.Sprintf("    patchSet: %s\n", p.PatchSetName))
					case "ToCompositeFieldPath":
						sb.WriteString(fmt.Sprintf("    %s ← %s (write back to XR)\n",
							p.ToFieldPath, p.FromFieldPath))
					default:
						sb.WriteString(fmt.Sprintf("    %s: %s → %s\n",
							p.Type, p.FromFieldPath, p.ToFieldPath))
					}
				}
			}
		}
	}

	return sb.String()
}
