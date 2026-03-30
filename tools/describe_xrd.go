package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
)

func DescribeXRD(ctx context.Context, dynamicClient dynamic.Interface, name string) (*XRDDescription, error) {
	xrd, err := dynamicClient.Resource(XRDGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("XRD '%s' not found: %w", name, err)
	}

	result := &XRDDescription{
		Name:  name,
		Kind:  getNestedString(xrd.Object, "spec", "names", "kind"),
		Group: getNestedString(xrd.Object, "spec", "group"),
		Scope: getNestedString(xrd.Object, "spec", "scope"),
	}

	// get served version
	result.Version = getServedVersion(xrd.Object)

	// check if established
	result.Established = resolveConditionStatus(xrd.Object, "Established") == "True"

	// find the schema for served version
	versions, ok := xrd.Object["spec"].(map[string]interface{})["versions"].([]interface{})
	if !ok {
		return result, nil
	}

	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _ := ver["served"].(bool)
		if !served {
			continue
		}

		// navigate to spec.parameters schema
		schema := getNestedMap(ver, "schema", "openAPIV3Schema",
			"properties", "spec", "properties", "parameters")
		if schema == nil {
			continue
		}

		// get required fields at parameters level
		required := getStringSlice(schema, "required")

		// parse properties
		properties, ok := schema["properties"].(map[string]interface{})
		if !ok {
			continue
		}

		for fieldName, fieldDef := range properties {
			fieldMap, ok := fieldDef.(map[string]interface{})
			if !ok {
				continue
			}

			fi := parseField(fieldName, fieldMap, required)
			result.AllFields = append(result.AllFields, fi)

			if fi.Required {
				result.RequiredFields = append(result.RequiredFields, fi)
			} else {
				result.OptionalFields = append(result.OptionalFields, fi)
			}
		}
	}

	result.Summary = buildXRDSummary(result)
	return result, nil
}

func parseField(name string, fieldMap map[string]interface{}, required []string) FieldInfo {
	fi := FieldInfo{
		Name:    name,
		Type:    getString(fieldMap, "type"),
		Default: fieldMap["default"],
	}

	// check if required
	for _, r := range required {
		if r == name {
			fi.Required = true
			break
		}
	}

	// check enum
	if enums, ok := fieldMap["enum"].([]interface{}); ok {
		for _, e := range enums {
			if s, ok := e.(string); ok {
				fi.Enum = append(fi.Enum, s)
			}
		}
	}

	// description
	fi.Description = getString(fieldMap, "description")

	// nested object
	if fi.Type == "object" {
		nestedRequired := getStringSlice(fieldMap, "required")
		if props, ok := fieldMap["properties"].(map[string]interface{}); ok {
			for childName, childDef := range props {
				childMap, ok := childDef.(map[string]interface{})
				if !ok {
					continue
				}
				fi.Children = append(fi.Children, parseField(childName, childMap, nestedRequired))
			}
		}
	}

	return fi
}

func buildXRDSummary(x *XRDDescription) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("XRD: %s\n", x.Name))
	sb.WriteString(fmt.Sprintf("Kind: %s | Group: %s | Version: %s | Scope: %s\n",
		x.Kind, x.Group, x.Version, x.Scope))
	sb.WriteString(fmt.Sprintf("Established: %v\n\n", x.Established))

	sb.WriteString("Required fields (spec.parameters):\n")
	if len(x.RequiredFields) == 0 {
		sb.WriteString("  none\n")
	}
	for _, f := range x.RequiredFields {
		sb.WriteString(fmt.Sprintf("  - %s (%s)", f.Name, f.Type))
		if len(f.Enum) > 0 {
			sb.WriteString(fmt.Sprintf(" [allowed: %s]", strings.Join(f.Enum, ", ")))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("\nOptional fields (spec.parameters):\n")
	if len(x.OptionalFields) == 0 {
		sb.WriteString("  none\n")
	}
	for _, f := range x.OptionalFields {
		sb.WriteString(fmt.Sprintf("  - %s (%s)", f.Name, f.Type))
		if f.Default != nil {
			sb.WriteString(fmt.Sprintf(" [default: %v]", f.Default))
		}
		if len(f.Enum) > 0 {
			sb.WriteString(fmt.Sprintf(" [allowed: %s]", strings.Join(f.Enum, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
