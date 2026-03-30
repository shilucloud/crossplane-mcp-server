package tools

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func ValidateXR(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	group, version, resource, name, namespace string,
) (*ValidationResult, error) {
	result := &ValidationResult{
		XRName:      name,
		XRNamespace: namespace,
		Valid:       true,
	}

	// Step 1: get XR
	xrGVR := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	var xrObj map[string]interface{}
	if namespace != "" {
		o, err := dynamicClient.Resource(xrGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("XR not found: %w", err)
		}
		xrObj = o.Object
	} else {
		o, err := dynamicClient.Resource(xrGVR).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("XR not found: %w", err)
		}
		xrObj = o.Object
	}

	// Step 2: get composition name
	compositionName := getNestedString(xrObj, "spec", "crossplane", "compositionRef", "name")
	if compositionName == "unknown" || compositionName == "" {
		result.Warnings = append(result.Warnings, "No compositionRef set on XR")
		result.Summary = buildValidationSummary(result)
		return result, nil
	}
	result.Composition = compositionName

	// Step 3: get composition
	comp, err := dynamicClient.Resource(compositionGVR).Get(ctx, compositionName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("composition '%s' not found: %w", compositionName, err)
	}

	// Step 4: check required fields from XRD
	xrdName := resource + "." + group
	xrd, xrdErr := dynamicClient.Resource(XRDGVR).Get(ctx, xrdName, metav1.GetOptions{})
	if xrdErr == nil {
		versions, ok := xrd.Object["spec"].(map[string]interface{})["versions"].([]interface{})
		if ok {
			for _, v := range versions {
				ver, ok := v.(map[string]interface{})
				if !ok {
					continue
				}
				served, _ := ver["served"].(bool)
				if !served {
					continue
				}
				schema := getNestedMap(ver, "schema", "openAPIV3Schema",
					"properties", "spec", "properties", "parameters")
				if schema == nil {
					continue
				}
				required := getStringSlice(schema, "required")
				for _, r := range required {
					val := getNestedString(xrObj, "spec", "parameters", r)
					if val == "unknown" || val == "" {
						result.MissingFields = append(result.MissingFields,
							fmt.Sprintf("spec.parameters.%s", r))
						result.Valid = false
					}
				}
			}
		}
	}

	// Step 4.5: validate each XR parameter value against XRD schema
	xrParams := getNestedMap(xrObj, "spec", "parameters")
	if xrParams != nil {
		for paramName, paramVal := range xrParams {
			paramStr := fmt.Sprintf("%v", paramVal)
			fieldConflicts := validateFieldAgainstSchema(
				paramName, paramStr, xrdName, ctx, dynamicClient)
			for _, fc := range fieldConflicts {
				result.Conflicts = append(result.Conflicts, fc)
				if fc.ConflictType == "enum_violation" || fc.ConflictType == "type_mismatch" {
					result.Valid = false
				}
			}
		}
	}

	// Step 5: walk composition pipeline and find overrides + invalid values
	pipeline := getNestedSlice(comp.Object, "spec", "pipeline")
	for _, step := range pipeline {
		stepMap, ok := step.(map[string]interface{})
		if !ok {
			continue
		}

		input, ok := stepMap["input"].(map[string]interface{})
		if !ok {
			continue
		}

		resources, ok := input["resources"].([]interface{})
		if !ok {
			continue
		}

		for _, r := range resources {
			rMap, ok := r.(map[string]interface{})
			if !ok {
				continue
			}

			// get base defaults from composition
			base := getNestedMap(rMap, "base", "spec", "forProvider")

			// walk patches
			patches, ok := rMap["patches"].([]interface{})
			if !ok {
				continue
			}

			for _, p := range patches {
				pMap, ok := p.(map[string]interface{})
				if !ok {
					continue
				}

				patchType := getString(pMap, "type")
				fromField := getString(pMap, "fromFieldPath")
				toField := getString(pMap, "toFieldPath")

				if patchType != "FromCompositeFieldPath" {
					continue
				}

				// extract XR value for this patch
				xrValue := extractValueFromPath(xrObj, fromField)
				if xrValue == "" {
					continue
				}

				// extract composition default for toField
				compDefault := ""
				if base != nil {
					shortField := strings.TrimPrefix(toField, "spec.forProvider.")
					if v, ok := base[shortField]; ok {
						compDefault = fmt.Sprintf("%v", v)
					}
				}

				// check region validity
				if strings.Contains(toField, "region") {
					if !isValidAWSRegion(xrValue) {
						result.Conflicts = append(result.Conflicts, FieldConflict{
							XRField:            fromField,
							XRValue:            xrValue,
							CompositionField:   toField,
							CompositionDefault: compDefault,
							PatchType:          patchType,
							ConflictType:       "invalid_region",
							Warning:            fmt.Sprintf("'%s' is not a valid AWS region", xrValue),
						})
						result.Valid = false
						continue
					}
				}

				// check override — XR value differs from composition default
				if compDefault != "" && xrValue != compDefault {
					result.Conflicts = append(result.Conflicts, FieldConflict{
						XRField:            fromField,
						XRValue:            xrValue,
						CompositionField:   toField,
						CompositionDefault: compDefault,
						PatchType:          patchType,
						ConflictType:       "override",
						Warning: fmt.Sprintf("XR value '%s' overrides composition default '%s'",
							xrValue, compDefault),
					})
				}
			}
		}
	}

	result.Summary = buildValidationSummary(result)
	return result, nil
}

func validateFieldAgainstSchema(
	fieldName string,
	fieldValue string,
	xrdName string,
	ctx context.Context,
	dynamicClient dynamic.Interface,
) []FieldConflict {
	var conflicts []FieldConflict

	xrd, err := dynamicClient.Resource(XRDGVR).Get(ctx, xrdName, metav1.GetOptions{})
	if err != nil {
		return nil
	}

	versions, ok := xrd.Object["spec"].(map[string]interface{})["versions"].([]interface{})
	if !ok {
		return nil
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

		schema := getNestedMap(ver, "schema", "openAPIV3Schema",
			"properties", "spec", "properties", "parameters", "properties")
		if schema == nil {
			continue
		}

		fieldDef, ok := schema[fieldName].(map[string]interface{})
		if !ok {
			continue
		}

		// Case 1: enum violation
		if enums, ok := fieldDef["enum"].([]interface{}); ok {
			allowed := []string{}
			valid := false
			for _, e := range enums {
				s, ok := e.(string)
				if !ok {
					continue
				}
				allowed = append(allowed, s)
				if s == fieldValue {
					valid = true
				}
			}
			if !valid {
				conflicts = append(conflicts, FieldConflict{
					XRField:      fmt.Sprintf("spec.parameters.%s", fieldName),
					XRValue:      fieldValue,
					ConflictType: "enum_violation",
					Warning: fmt.Sprintf("'%s' is not allowed. Allowed values: [%s]",
						fieldValue, strings.Join(allowed, ", ")),
				})
			}
		}

		// Case 2: type mismatch
		expectedType := getString(fieldDef, "type")
		if expectedType != "" {
			actualType := inferType(fieldValue)
			if actualType != "" && expectedType != actualType {
				conflicts = append(conflicts, FieldConflict{
					XRField:      fmt.Sprintf("spec.parameters.%s", fieldName),
					XRValue:      fieldValue,
					ConflictType: "type_mismatch",
					Warning: fmt.Sprintf("expected type '%s' but got '%s'",
						expectedType, actualType),
				})
			}
		}
	}

	return conflicts
}

func extractValueFromPath(obj map[string]interface{}, path string) string {
	parts := strings.Split(path, ".")
	current := obj
	for i, part := range parts {
		if i == len(parts)-1 {
			if v, ok := current[part]; ok {
				return fmt.Sprintf("%v", v)
			}
			return ""
		}
		next, ok := current[part].(map[string]interface{})
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func inferType(value string) string {
	if value == "true" || value == "false" {
		return "boolean"
	}
	allDigits := true
	for _, c := range value {
		if c < '0' || c > '9' {
			allDigits = false
			break
		}
	}
	if allDigits && len(value) > 0 {
		return "integer"
	}
	return "string"
}

func isValidAWSRegion(region string) bool {
	validRegions := []string{
		"us-east-1", "us-east-2", "us-west-1", "us-west-2",
		"eu-west-1", "eu-west-2", "eu-west-3", "eu-central-1",
		"eu-north-1", "eu-south-1", "eu-south-2",
		"ap-south-1", "ap-south-2", "ap-southeast-1", "ap-southeast-2",
		"ap-northeast-1", "ap-northeast-2", "ap-northeast-3",
		"ap-east-1", "ca-central-1", "sa-east-1",
		"me-south-1", "me-central-1", "af-south-1",
		"us-gov-east-1", "us-gov-west-1",
	}
	for _, r := range validRegions {
		if r == region {
			return true
		}
	}
	return false
}

func buildValidationSummary(v *ValidationResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("XR: %s | Composition: %s\n", v.XRName, v.Composition))
	sb.WriteString(fmt.Sprintf("Valid: %v\n\n", v.Valid))

	if len(v.MissingFields) > 0 {
		sb.WriteString("Missing required fields:\n")
		for _, f := range v.MissingFields {
			sb.WriteString(fmt.Sprintf("  ❌ %s\n", f))
		}
		sb.WriteString("\n")
	}

	if len(v.Conflicts) > 0 {
		sb.WriteString("Field conflicts:\n")
		for _, c := range v.Conflicts {
			switch c.ConflictType {
			case "enum_violation":
				sb.WriteString(fmt.Sprintf("  ❌ ENUM VIOLATION: %s = '%s'\n",
					c.XRField, c.XRValue))
				sb.WriteString(fmt.Sprintf("     %s\n\n", c.Warning))
			case "type_mismatch":
				sb.WriteString(fmt.Sprintf("  ❌ TYPE MISMATCH: %s = '%s'\n",
					c.XRField, c.XRValue))
				sb.WriteString(fmt.Sprintf("     %s\n\n", c.Warning))
			case "invalid_region":
				sb.WriteString(fmt.Sprintf("  ⚠️  INVALID REGION: %s = '%s'\n",
					c.XRField, c.XRValue))
				sb.WriteString(fmt.Sprintf("     %s\n\n", c.Warning))
			default:
				sb.WriteString(fmt.Sprintf("  ℹ️  OVERRIDE: %s = '%s'\n",
					c.XRField, c.XRValue))
				sb.WriteString(fmt.Sprintf("     overrides composition default '%s' in '%s'\n\n",
					c.CompositionDefault, c.CompositionField))
			}
		}
	}

	if len(v.Warnings) > 0 {
		sb.WriteString("Warnings:\n")
		for _, w := range v.Warnings {
			sb.WriteString(fmt.Sprintf("  ⚠️  %s\n", w))
		}
	}

	if v.Valid && len(v.Conflicts) == 0 && len(v.MissingFields) == 0 {
		sb.WriteString("✅ XR is valid — all fields are correct\n")
	}

	return sb.String()
}
