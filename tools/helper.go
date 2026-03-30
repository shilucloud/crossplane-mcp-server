package tools

import (
	"fmt"
	"strings"
)

func resolveProviderState(obj map[string]interface{}) string {
	conditions, ok := obj["status"].(map[string]interface{})["conditions"].([]interface{})
	if !ok {
		return "Unknown"
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condition["type"] == "Healthy" && condition["status"] == "True" {
			return "Healthy"
		}
		if condition["type"] == "Healthy" && condition["status"] == "False" {
			return "Unhealthy"
		}
	}
	return "Installing"
}

func extractImageTag(image string) string {
	for i := len(image) - 1; i >= 0; i-- {
		if image[i] == ':' {
			return image[i+1:]
		}
	}
	return "unknown"
}

func extractMajorVersion(version string) int {
	// e.g. "v2.0.1" -> 2, "v1.20.0" -> 1
	if len(version) < 2 {
		return 0
	}
	v := version
	if v[0] == 'v' {
		v = v[1:]
	}
	if len(v) > 0 && v[0] == '2' {
		return 2
	}
	return 1
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

// helpers
func getNestedMap(obj map[string]interface{}, fields ...string) map[string]interface{} {
	current := obj
	for _, field := range fields {
		next, ok := current[field].(map[string]interface{})
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func getStringSlice(obj map[string]interface{}, key string) []string {
	raw, ok := obj[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, v := range raw {
		if s, ok := v.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// getLatestNonDeprecatedVersion returns the newest non-deprecated served version
func getLatestNonDeprecatedVersion(obj map[string]interface{}) string {
	versions, ok := obj["spec"].(map[string]interface{})["versions"].([]interface{})
	if !ok {
		return ""
	}
	latest := ""
	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _ := ver["served"].(bool)
		deprecated, _ := ver["deprecated"].(bool)
		if served && !deprecated {
			name, _ := ver["name"].(string)
			latest = name // last one wins — versions are ordered newest last
		}
	}
	return latest
}

func getString(obj map[string]interface{}, key string) string {
	v, ok := obj[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func splitAPIVersion(apiVersion string) (string, string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return apiVersion, ""
}

func kindToPlural(kind string) string {
	return strings.ToLower(kind) + "s"
}

func getNestedSlice(obj map[string]interface{}, fields ...string) []interface{} {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return nil
		}
		if i == len(fields)-1 {
			slice, ok := val.([]interface{})
			if !ok {
				return nil
			}
			return slice
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return nil
		}
	}
	return nil
}

func mrGroupToProviderConfigGroup(mrGroup string) string {
	parts := strings.SplitN(mrGroup, ".", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return mrGroup
}

func getServedVersion(obj map[string]interface{}) string {
	versions, ok := obj["spec"].(map[string]interface{})["versions"].([]interface{})
	if !ok {
		return ""
	}
	for _, v := range versions {
		ver, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		served, _ := ver["served"].(bool)
		if served {
			name, _ := ver["name"].(string)
			return name
		}
	}
	return ""
}

func resolveConditionStatus(obj map[string]interface{}, condType string) string {
	status, ok := obj["status"].(map[string]interface{})
	if !ok {
		return "Unknown"
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return "Unknown"
	}
	for _, c := range conditions {
		condition, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		if condition["type"] == condType {
			s, _ := condition["status"].(string)
			msg, _ := condition["message"].(string)
			if msg != "" && s == "False" {
				return fmt.Sprintf("False (%s)", truncate(msg, 60))
			}
			return s
		}
	}
	return "Unknown"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func getNestedString(obj map[string]interface{}, fields ...string) string {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return "unknown"
		}
		if i == len(fields)-1 {
			str, ok := val.(string)
			if !ok {
				return "unknown"
			}
			return str
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return "unknown"
		}
	}
	return "unknown"
}

func getNestedInt64(obj map[string]interface{}, fields ...string) int64 {
	current := obj
	for i, field := range fields {
		val, ok := current[field]
		if !ok {
			return 0
		}
		if i == len(fields)-1 {
			switch v := val.(type) {
			case int64:
				return v
			case float64:
				return int64(v)
			}
			return 0
		}
		current, ok = val.(map[string]interface{})
		if !ok {
			return 0
		}
	}
	return 0
}

// Helper
func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
