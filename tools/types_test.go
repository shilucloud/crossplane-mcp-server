package tools

import (
	"testing"
)

func TestXRObjectInfo(t *testing.T) {
	obj := XRObjectInfo{
		Group:    "example.com",
		Version:  "v1alpha1",
		Resource: "myresources",
		Kind:     "MyResource",
		Scope:    "Namespaced",
	}

	if obj.Group != "example.com" {
		t.Errorf("XRObjectInfo.Group = %v, want example.com", obj.Group)
	}
	if obj.Scope != "Namespaced" {
		t.Errorf("XRObjectInfo.Scope = %v, want Namespaced", obj.Scope)
	}
}

func TestXRInfo(t *testing.T) {
	info := XRInfo{
		Name:           "test-xr",
		Namespace:      "default",
		Kind:           "MyResource",
		Ready:          "True",
		Synced:         "True",
		Age:            "1h",
		CompositionRef: "test-composition",
		Scope:          "Namespaced",
		Group:          "example.com",
	}

	if info.Name != "test-xr" {
		t.Errorf("XRInfo.Name = %v, want test-xr", info.Name)
	}
	if info.Ready != "True" {
		t.Errorf("XRInfo.Ready = %v, want True", info.Ready)
	}
}

func TestXRListResult(t *testing.T) {
	result := XRListResult{
		XRs: []XRInfo{
			{Name: "xr1"},
			{Name: "xr2"},
		},
		Warnings: []string{"warning 1", "warning 2"},
	}

	if len(result.XRs) != 2 {
		t.Errorf("XRListResult.XRs len = %v, want 2", len(result.XRs))
	}
	if len(result.Warnings) != 2 {
		t.Errorf("XRListResult.Warnings len = %v, want 2", len(result.Warnings))
	}
}

func TestCondition(t *testing.T) {
	cond := Condition{
		Type:               "Ready",
		Status:             "True",
		Reason:             "ResourcesAvailable",
		Message:            "Ready to use",
		LastTransitionTime: "2024-01-01T00:00:00Z",
		ObservedGeneration: 1,
	}

	if cond.Type != "Ready" {
		t.Errorf("Condition.Type = %v, want Ready", cond.Type)
	}
	if cond.Status != "True" {
		t.Errorf("Condition.Status = %v, want True", cond.Status)
	}
}

func TestProviderInfo(t *testing.T) {
	info := ProviderInfo{
		Name:      "provider-aws",
		Version:   "v1.0.0",
		Health:    true,
		Installed: true,
		State:     "Healthy",
	}

	if info.Health != true {
		t.Errorf("ProviderInfo.Health = %v, want true", info.Health)
	}
	if info.State != "Healthy" {
		t.Errorf("ProviderInfo.State = %v, want Healthy", info.State)
	}
}

func TestCrossplaneInfo(t *testing.T) {
	info := CrossplaneInfo{
		Version:          "v2.0.1",
		MajorVersion:     2,
		HasMRDs:          true,
		HasOperations:    true,
		HasNamespacedXRs: true,
		HasNamespacedMRs: true,
		TotalXRDs:        10,
		NumberOfProvider: 5,
	}

	if info.MajorVersion != 2 {
		t.Errorf("CrossplaneInfo.MajorVersion = %v, want 2", info.MajorVersion)
	}
	if !info.HasMRDs {
		t.Error("CrossplaneInfo.HasMRDs should be true")
	}
}

func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		XRName:      "test-xr",
		XRNamespace: "default",
		Composition: "test-comp",
		Valid:       true,
		Conflicts:   []FieldConflict{},
		Warnings:    []string{"Deprecated field used"},
		Summary:     "Validation passed",
	}

	if !result.Valid {
		t.Error("ValidationResult.Valid should be true")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("ValidationResult.Warnings len = %v, want 1", len(result.Warnings))
	}
}

func TestOperationResult(t *testing.T) {
	op := OperationResult{
		Name:           "test-op",
		Phase:          "Succeeded",
		StartTime:      "2024-01-01T00:00:00Z",
		CompletionTime: "2024-01-01T00:01:00Z",
		Duration:       "1m",
		Message:        "Operation completed",
	}

	if op.Phase != "Succeeded" {
		t.Errorf("OperationResult.Phase = %v, want Succeeded", op.Phase)
	}
}

func TestEvents(t *testing.T) {
	event := EventInfo{
		Type:    "Warning",
		Reason:  "Failed",
		Object:  "TestObject",
		Message: "Test failed",
		Count:   1,
	}

	if event.Type != "Warning" {
		t.Errorf("EventInfo.Type = %v, want Warning", event.Type)
	}
}
