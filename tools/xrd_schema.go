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
	xrdGVRForSchema = schema.GroupVersionResource{
		Group:    "apiextensions.crossplane.io",
		Version:  "v2",
		Resource: "compositeresourcedefinitions",
	}
)

type XRDSchema struct {
	Name                   string
	Group                  string
	Version                string
	Kind                   string
	Plural                 string
	Scope                  string
	ShortNames             []string
	Description            string
	XRDVersion             string
	Served                 bool
	Storage                bool
	Deprecated             bool
	ClaimNames             []string
	DefaultCompositionRef  *string
	EnforcedCompositionRef *string
	ValidationSchema       map[string]interface{}
	RequiredFields         []string
	FieldDescriptions      map[string]string
	SchemaDefaults         map[string]interface{}
}

func GetXRDSchema(ctx context.Context, client dynamic.Interface, kind string) (*XRDSchema, error) {
	xrds, err := client.Resource(xrdGVRForSchema).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing XRDs: %w", err)
	}

	for _, xrd := range xrds.Items {
		xrdKind := getNestedString(xrd.Object, "spec", "names", "kind")
		if xrdKind != kind {
			continue
		}

		result := &XRDSchema{
			Name:        xrd.GetName(),
			Kind:        xrdKind,
			Plural:      getNestedString(xrd.Object, "spec", "names", "plural"),
			ShortNames:  getStringSlice(xrd.Object, "spec", "names", "shortNames"),
			Description: getNestedString(xrd.Object, "spec", "description"),
			Scope:       getNestedString(xrd.Object, "spec", "scope"),
			ClaimNames:  getStringSlice(xrd.Object, "spec", "claimNames"),
		}

		specVersions := getSlice(xrd.Object, "spec", "versions")
		for _, v := range specVersions {
			ver, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			verName := getString(ver, "name")
			served, _ := ver["served"].(bool)
			storage, _ := ver["storage"].(bool)
			deprecated, _ := ver["deprecated"].(bool)

			if served {
				result.Served = true
				result.XRDVersion = verName
				specSchema := getMap(ver, "schema", "openAPIV3Schema", "properties", "spec")
				result.ValidationSchema = specSchema
				result.RequiredFields = getRequiredFields(specSchema)
				result.FieldDescriptions = getFieldDescriptions(specSchema, "")
				result.SchemaDefaults = extractDefaultValues(specSchema)
			}
			if storage {
				result.Version = verName
			}
			result.Deprecated = deprecated
		}

		if parts := strings.SplitN(result.Plural, ".", 2); len(parts) == 2 {
			result.Group = parts[1]
		} else {
			result.Group = getNestedString(xrd.Object, "spec", "group")
		}

		if compRef := getMap(xrd.Object, "spec", "defaultCompositionRef"); compRef != nil {
			if name := getString(compRef, "name"); name != "" {
				result.DefaultCompositionRef = &name
			}
		}

		if compRef := getMap(xrd.Object, "spec", "enforcedCompositionRef"); compRef != nil {
			if name := getString(compRef, "name"); name != "" {
				result.EnforcedCompositionRef = &name
			}
		}

		return result, nil
	}

	return nil, fmt.Errorf("XRD not found for kind: %s", kind)
}

func ListAllXRDSchemas(ctx context.Context, client dynamic.Interface) ([]XRDSchema, error) {
	xrds, err := client.Resource(xrdGVRForSchema).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listing XRDs: %w", err)
	}

	var result []XRDSchema
	for _, xrd := range xrds.Items {
		kind := getNestedString(xrd.Object, "spec", "names", "kind")
		xrdSchema, err := GetXRDSchema(ctx, client, kind)
		if err != nil {
			continue
		}
		result = append(result, *xrdSchema)
	}

	return result, nil
}

func ExplainXRDSchema(schema *XRDSchema) string {
	msg := fmt.Sprintf("XRD: %s (%s)\n", schema.Kind, schema.Plural)
	msg += fmt.Sprintf("Description: %s\n", schema.Description)
	msg += fmt.Sprintf("Scope: %s\n", schema.Scope)
	msg += fmt.Sprintf("API Version: %s/%s\n", schema.Group, schema.Version)
	msg += fmt.Sprintf("Default Composition: ")
	if schema.EnforcedCompositionRef != nil {
		msg += fmt.Sprintf("(enforced) %s\n", *schema.EnforcedCompositionRef)
	} else if schema.DefaultCompositionRef != nil {
		msg += fmt.Sprintf("%s\n", *schema.DefaultCompositionRef)
	} else {
		msg += "none\n"
	}

	if len(schema.RequiredFields) > 0 {
		msg += "\nRequired Fields:\n"
		for _, f := range schema.RequiredFields {
			desc := schema.FieldDescriptions[f]
			if desc != "" {
				msg += fmt.Sprintf("  - %s: %s\n", f, desc)
			} else {
				msg += fmt.Sprintf("  - %s\n", f)
			}
		}
	}

	if len(schema.SchemaDefaults) > 0 {
		msg += "\nDefault Values:\n"
		for f, v := range schema.SchemaDefaults {
			msg += fmt.Sprintf("  - %s: %v\n", f, v)
		}
	}

	return msg
}

func getRequiredFields(schema map[string]interface{}) []string {
	if schema == nil {
		return nil
	}

	var required []string
	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				required = append(required, s)
			}
		}
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, prop := range props {
			if propMap, ok := prop.(map[string]interface{}); ok {
				if nested := getRequiredFields(propMap); nested != nil {
					for _, field := range nested {
						required = append(required, name+"."+field)
					}
				}
			}
		}
	}

	return required
}

func getFieldDescriptions(schema map[string]interface{}, prefix string) map[string]string {
	descriptions := map[string]string{}
	if schema == nil {
		return descriptions
	}

	if desc, ok := schema["description"].(string); ok && prefix != "" {
		descriptions[prefix] = desc
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, prop := range props {
			fieldName := name
			if prefix != "" {
				fieldName = prefix + "." + name
			}

			if propMap, ok := prop.(map[string]interface{}); ok {
				if desc, ok := propMap["description"].(string); ok {
					descriptions[fieldName] = desc
				}
				if nested := getFieldDescriptions(propMap, fieldName); nested != nil {
					for k, v := range nested {
						descriptions[k] = v
					}
				}
			}
		}
	}

	return descriptions
}

func extractDefaultValues(schema map[string]interface{}) map[string]interface{} {
	defaults := map[string]interface{}{}
	if schema == nil {
		return defaults
	}

	extractDefaultsRecursive(schema, "", defaults)
	return defaults
}

func extractDefaultsRecursive(schema map[string]interface{}, prefix string, defaults map[string]interface{}) {
	if schema == nil {
		return
	}

	if defVal, ok := schema["default"]; ok && prefix != "" {
		defaults[prefix] = defVal
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		for name, prop := range props {
			fieldName := name
			if prefix != "" {
				fieldName = prefix + "." + name
			}

			if propMap, ok := prop.(map[string]interface{}); ok {
				if defVal, ok := propMap["default"]; ok {
					defaults[fieldName] = defVal
				}
				extractDefaultsRecursive(propMap, fieldName, defaults)
			}
		}
	}
}

func getSlice(obj map[string]interface{}, keys ...string) []interface{} {
	val := obj
	for _, key := range keys {
		if val == nil {
			return nil
		}
		next, ok := val[key]
		if !ok {
			return nil
		}
		val, ok = next.(map[string]interface{})
		if !ok {
			if slice, ok := next.([]interface{}); ok {
				return slice
			}
			return nil
		}
	}
	return nil
}

func getMap(obj map[string]interface{}, keys ...string) map[string]interface{} {
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
			return nil
		}
		val = mapVal
	}
	return val
}

func getStringSlice(obj map[string]interface{}, keys ...string) []string {
	slice := getSlice(obj, keys...)
	if slice == nil {
		return nil
	}

	var result []string
	for _, item := range slice {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
