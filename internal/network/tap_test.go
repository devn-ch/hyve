//go:build linux

// go test ./internal/network -run TestCreateTAP -v
package network

import "testing"

func TestCreateTAP(t *testing.T) {
	tap, err := CreateTAP("hyve-test0")
	if err != nil {
		t.Fatalf("CreateTAP: %v", err)
	}

	if tap == nil {
		t.Fatal("CreateTAP returned nil TAP")
	}

	if tap.name != "hyve-test0" {
		t.Fatalf("TAP name = %q, want %q", tap.name, "hyve-test0")
	}

	t.Cleanup(func() {
		if err := tap.Close(); err != nil {
			t.Errorf("close TAP: %v", err)
		}
	})
}
