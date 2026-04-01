package tools

import (
	"testing"
)

func TestGetNestedString(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		fields   []string
		expected string
	}{
		{
			name: "nested string found",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"name": "test",
				},
			},
			fields:   []string{"spec", "name"},
			expected: "test",
		},
		{
			name: "missing field",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			fields:   []string{"spec", "name"},
			expected: "unknown",
		},
		{
			name:     "empty object",
			obj:      map[string]interface{}{},
			fields:   []string{"spec", "name"},
			expected: "unknown",
		},
		{
			name: "single field",
			obj: map[string]interface{}{
				"name": "single",
			},
			fields:   []string{"name"},
			expected: "single",
		},
		{
			name: "non-string value",
			obj: map[string]interface{}{
				"value": 123,
			},
			fields:   []string{"value"},
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNestedString(tt.obj, tt.fields...)
			if result != tt.expected {
				t.Errorf("getNestedString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetNestedInt64(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		fields   []string
		expected int64
	}{
		{
			name: "int64 value",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"count": int64(42),
				},
			},
			fields:   []string{"spec", "count"},
			expected: 42,
		},
		{
			name: "float64 value",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"count": float64(100),
				},
			},
			fields:   []string{"spec", "count"},
			expected: 100,
		},
		{
			name: "missing field",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			fields:   []string{"spec", "count"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNestedInt64(tt.obj, tt.fields...)
			if result != tt.expected {
				t.Errorf("getNestedInt64() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		key      string
		expected string
	}{
		{
			name:     "string found",
			obj:      map[string]interface{}{"key": "value"},
			key:      "key",
			expected: "value",
		},
		{
			name:     "missing key",
			obj:      map[string]interface{}{},
			key:      "key",
			expected: "",
		},
		{
			name:     "non-string value",
			obj:      map[string]interface{}{"key": 123},
			key:      "key",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getString(tt.obj, tt.key)
			if result != tt.expected {
				t.Errorf("getString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		key      string
		expected []string
	}{
		{
			name:     "slice found",
			obj:      map[string]interface{}{"key": []interface{}{"a", "b", "c"}},
			key:      "key",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "missing key",
			obj:      map[string]interface{}{},
			key:      "key",
			expected: nil,
		},
		{
			name:     "non-slice value",
			obj:      map[string]interface{}{"key": "not a slice"},
			key:      "key",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringSlice(tt.obj, tt.key)
			if len(result) != len(tt.expected) {
				t.Errorf("getStringSlice() len = %v, want %v", len(result), len(tt.expected))
			}
		})
	}
}

func TestGetNestedSlice(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]interface{}
		fields   []string
		expected int
	}{
		{
			name: "slice found",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"items": []interface{}{1, 2, 3},
				},
			},
			fields:   []string{"spec", "items"},
			expected: 3,
		},
		{
			name: "missing slice",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			fields:   []string{"spec", "items"},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getNestedSlice(tt.obj, tt.fields...)
			if len(result) != tt.expected {
				t.Errorf("getNestedSlice() len = %v, want %v", len(result), tt.expected)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		list     []string
		item     string
		expected bool
	}{
		{
			name:     "item exists",
			list:     []string{"a", "b", "c"},
			item:     "b",
			expected: true,
		},
		{
			name:     "item not found",
			list:     []string{"a", "b", "c"},
			item:     "d",
			expected: false,
		},
		{
			name:     "empty list",
			list:     []string{},
			item:     "a",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.list, tt.item)
			if result != tt.expected {
				t.Errorf("contains() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{
			name: "string within limit",
			s:    "short",
			max:  10,
			want: "short",
		},
		{
			name: "string exceeds limit",
			s:    "this is a long string",
			max:  10,
			want: "this is a ...",
		},
		{
			name: "exact limit",
			s:    "exactly10",
			max:  10,
			want: "exactly10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.s, tt.max)
			if result != tt.want {
				t.Errorf("truncate() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestExtractImageTag(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{
			name:     "with tag",
			image:    "nginx:latest",
			expected: "latest",
		},
		{
			name:     "with version tag",
			image:    "nginx:v1.2.3",
			expected: "v1.2.3",
		},
		{
			name:     "no tag",
			image:    "nginx",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImageTag(tt.image)
			if result != tt.expected {
				t.Errorf("extractImageTag() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected int
	}{
		{
			name:     "v2 version",
			version:  "v2.0.1",
			expected: 2,
		},
		{
			name:     "v1 version",
			version:  "v1.20.0",
			expected: 1,
		},
		{
			name:     "short version",
			version:  "v",
			expected: 0,
		},
		{
			name:     "no prefix",
			version:  "2.0.0",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMajorVersion(tt.version)
			if result != tt.expected {
				t.Errorf("extractMajorVersion() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractFunctionName(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "with function name",
			message:  `error: "myFunction" failed`,
			expected: "myFunction",
		},
		{
			name:     "no function name",
			message:  "just a regular error",
			expected: "unknown",
		},
		{
			name:     "empty string",
			message:  "",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFunctionName(tt.message)
			if result != tt.expected {
				t.Errorf("extractFunctionName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSplitAPIVersion(t *testing.T) {
	tests := []struct {
		name          string
		apiVersion    string
		expectedGroup string
		expectedVer   string
	}{
		{
			name:          "with group",
			apiVersion:    "apps/v1",
			expectedGroup: "apps",
			expectedVer:   "v1",
		},
		{
			name:          "without group",
			apiVersion:    "v1",
			expectedGroup: "v1",
			expectedVer:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group, ver := splitAPIVersion(tt.apiVersion)
			if group != tt.expectedGroup || ver != tt.expectedVer {
				t.Errorf("splitAPIVersion() = (%v, %v), want (%v, %v)", group, ver, tt.expectedGroup, tt.expectedVer)
			}
		})
	}
}

func TestKindToPlural(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		expected string
	}{
		{
			name:     "simple kind",
			kind:     "Pod",
			expected: "pods",
		},
		{
			name:     "deployment",
			kind:     "Deployment",
			expected: "deployments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := kindToPlural(tt.kind)
			if result != tt.expected {
				t.Errorf("kindToPlural() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMrGroupToProviderConfigGroup(t *testing.T) {
	tests := []struct {
		name     string
		mrGroup  string
		expected string
	}{
		{
			name:     "with dot",
			mrGroup:  "database.aws.crossplane.io",
			expected: "aws.crossplane.io",
		},
		{
			name:     "without dot",
			mrGroup:  "noprefix",
			expected: "noprefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mrGroupToProviderConfigGroup(tt.mrGroup)
			if result != tt.expected {
				t.Errorf("mrGroupToProviderConfigGroup() = %v, want %v", result, tt.expected)
			}
		})
	}
}
