//go:build linux

// go test ./internal/network -run TestCreateBridge -v
package network

import (
	"testing"
)

func TestCreateBridge(t *testing.T) {
	bridge, err := CreateBridge("hyve-test0")
	if err != nil {
		t.Fatalf("CreateBridge: %v", err)
	}

	if bridge == nil {
		t.Fatal("CreateBridge returned nil bridge")
	}

	if bridge.name != "hyve-test0" {
		t.Fatalf("bridge name = %q, want %q", bridge.name, "hyve-test0")
	}

	if !bridge.created {
		t.Fatal("bridge.created = false, want true")
	}

	t.Cleanup(func() {
		if err := bridge.Close(); err != nil {
			t.Errorf("close bridge: %v", err)
		}
	})
}
