package tools

import (
	"testing"
)

func TestIsReady(t *testing.T) {
	ready = false
	if IsReady() {
		t.Error("IsReady() should return false when ready is false")
	}

	ready = true
	if !IsReady() {
		t.Error("IsReady() should return true when ready is true")
	}
}

func TestNewRESTMapper(t *testing.T) {
	// This test requires a k8s config, so we skip in unit tests
	// The actual test would need a mock or real k8s cluster
	t.Skip("NewRESTMapper requires a running Kubernetes cluster")
}
