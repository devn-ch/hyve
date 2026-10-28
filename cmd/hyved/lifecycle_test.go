package main

// Start → DHCP-IP → Stop → TAP removed → Restart → DHCP-IP → Stop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/devn-ch/hyve/internal/qemu"
	"github.com/devn-ch/hyve/internal/vm"
)

const (
	lifecycleBridge    = "br0"
	lifecycleVMName    = "lifecycle-test"
	lifecycleDHCPIP    = "192.168.100.1"
	lifecycleDHCPRange = "192.168.100.100,192.168.100.200,255.255.255.0,1h"
	lifecycleTimeout   = 90 * time.Second
)

func TestVMManagerLifecycle(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("VM lifecycle integration test requires Linux")
	}
	if os.Geteuid() != 0 {
		t.Skip("VM lifecycle integration test requires root privileges")
	}

	if _, err := net.InterfaceByName(lifecycleBridge); err != nil {
		t.Skipf("bridge %q is unavailable: %v", lifecycleBridge, err)
	}

	sourceImage := lifecycleQGAImage(t)
	tempDir := t.TempDir()

	// Use a private copy: QEMU may modify the guest image.
	image := filepath.Join(tempDir, "guest.qcow2")
	copyFile(t, sourceImage, image)

	if err := startLifecycleDHCP(t, tempDir); err != nil {
		t.Fatalf("start DHCP server: %v", err)
	}

	manager := NewVMManager(nil)
	manager.qmpDir = filepath.Join(tempDir, "qmp")
	manager.qgaDir = filepath.Join(tempDir, "qga")
	manager.consoleDir = filepath.Join(tempDir, "console")

	ctx := context.Background()

	cfg := qemu.Config{
		Name:   lifecycleVMName,
		CPUs:   1,
		Memory: "512M",
		Drives: []qemu.Drive{
			{
				Type: qemu.DriveTypeDisk,
				Path: image,
			},
		},
		Network: qemu.NetworkConfig{
			Mode:      qemu.NetworkBridge,
			Interface: lifecycleBridge,
		},
	}

	// Cleanup must not leave a QEMU process or TAP interface behind.
	t.Cleanup(func() {
		manager.mu.Lock()
		entry := manager.vms[lifecycleVMName]
		manager.mu.Unlock()

		if entry != nil {
			switch entry.info.State {
			case vm.StateRunning:
				if err := manager.Stop(lifecycleVMName); err != nil {
					t.Logf("cleanup stop: %v", err)
					_ = entry.qemu.Stop()
				}
			case vm.StateStarting, vm.StateStopping:
				_ = entry.qemu.Stop()
			}

			_ = waitForVMState(manager, lifecycleVMName,
				[]vm.State{vm.StateStopped, vm.StateExited}, 15*time.Second)
		}
	})

	tapName := tapNameForVM(lifecycleVMName)

	for cycle := 1; cycle <= 2; cycle++ {
		t.Run(fmt.Sprintf("cycle_%d", cycle), func(t *testing.T) {
			t.Logf("starting VM (cycle %d)", cycle)

			if err := manager.Start(ctx, cfg); err != nil {
				t.Fatalf("start VM: %v", err)
			}

			ip := waitForGuestIPv4(t, manager, lifecycleVMName, lifecycleTimeout)
			t.Logf("guest obtained IPv4 address: %s", ip)

			tap := waitForInterface(t, tapName, true, 10*time.Second)
			if !tap {
				t.Fatalf("TAP interface %q was not created", tapName)
			}

			t.Log("stopping VM")
			if err := manager.Stop(lifecycleVMName); err != nil {
				t.Fatalf("stop VM: %v", err)
			}

			if err := waitForVMState(manager, lifecycleVMName,
				[]vm.State{vm.StateStopped}, lifecycleTimeout); err != nil {
				t.Fatalf("wait for VM to stop: %v", err)
			}

			if waitForInterface(t, tapName, false, 15*time.Second) {
				t.Fatalf("TAP interface %q still exists after VM stopped", tapName)
			}

			t.Logf("cycle %d completed: DHCP, stop and TAP cleanup succeeded", cycle)
		})
	}
}

func lifecycleQGAImage(t *testing.T) string {
	t.Helper()

	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test source location")
	}

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "../.."))
	image := filepath.Join(repoRoot, ".build", "qga", "debian-qga.qcow2")

	info, err := os.Stat(image)
	if err != nil {
		t.Fatalf("QGA test image not found at %s: %v", image, err)
	}
	if info.IsDir() {
		t.Fatalf("QGA image path is a directory: %s", image)
	}

	return image
}

func copyFile(t *testing.T, source, destination string) {
	t.Helper()

	in, err := os.Open(source)
	if err != nil {
		t.Fatalf("open source image: %v", err)
	}
	defer in.Close()

	out, err := os.Create(destination)
	if err != nil {
		t.Fatalf("create image copy: %v", err)
	}

	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		t.Fatalf("copy image: %v", err)
	}
	if err := out.Close(); err != nil {
		t.Fatalf("close image copy: %v", err)
	}
}

func startLifecycleDHCP(t *testing.T, tempDir string) error {
	t.Helper()

	leaseFile := filepath.Join(tempDir, "dnsmasq.leases")

	cmd := exec.Command(
		"dnsmasq",
		"--no-daemon",
		"--interface="+lifecycleBridge,
		"--bind-interfaces",
		"--except-interface=lo",
		"--port=0",
		"--dhcp-authoritative",
		"--dhcp-range="+lifecycleDHCPRange,
		"--dhcp-option=3,"+lifecycleDHCPIP,
		"--dhcp-option=6,"+lifecycleDHCPIP,
		"--dhcp-leasefile="+leaseFile,
		"--log-dhcp",
	)

	output, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start dnsmasq: %w", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Signal(os.Interrupt)
			done := make(chan struct{})
			go func() {
				_ = cmd.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		}
	})

	// dnsmasq runs in foreground. Reading its output also avoids
	// blocking if it writes enough data to the pipe.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := output.Read(buf); err != nil {
				return
			}
		}
	}()

	// Give dnsmasq a moment to report binding errors.
	time.Sleep(300 * time.Millisecond)
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		return fmt.Errorf("dnsmasq exited during startup")
	}

	return nil
}

func waitForGuestIPv4(
	t *testing.T,
	manager *VMManager,
	name string,
	timeout time.Duration,
) string {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var lastErr error

	for time.Now().Before(deadline) {
		manager.mu.Lock()
		entry := manager.vms[name]
		manager.mu.Unlock()

		if entry == nil {
			t.Fatalf("VM %q disappeared while waiting for DHCP", name)
		}

		interfaces, err := entry.qemu.GuestNetworkInterfaces()

		if err != nil {
			lastErr = err
		} else {
			data, marshalErr := json.Marshal(interfaces)
			if marshalErr == nil {
				if ip := findGuestIPv4(data); ip != "" {
					return ip
				}
			}
		}

		time.Sleep(1 * time.Second)
	}

	if lastErr != nil {
		t.Fatalf("guest did not obtain an IPv4 address within %s; last QGA error: %v",
			timeout, lastErr)
	}
	t.Fatalf("guest did not obtain an IPv4 address within %s", timeout)
	return ""
}

func findGuestIPv4(data []byte) string {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return ""
	}

	var walk func(any) string
	walk = func(v any) string {
		switch item := v.(type) {
		case map[string]any:
			// Prefer the explicit QGA address field.
			if address, ok := item["ip-address"].(string); ok {
				if ip := validGuestIPv4(address); ip != "" {
					return ip
				}
			}
			for key, child := range item {
				if strings.Contains(strings.ToLower(key), "ip-address") ||
					strings.Contains(strings.ToLower(key), "ip") {
					if found := walk(child); found != "" {
						return found
					}
				}
			}
			for _, child := range item {
				if found := walk(child); found != "" {
					return found
				}
			}
		case []any:
			for _, child := range item {
				if found := walk(child); found != "" {
					return found
				}
			}
		case string:
			return validGuestIPv4(item)
		}
		return ""
	}

	return walk(value)
}

func validGuestIPv4(address string) string {
	ip := net.ParseIP(address)
	if ip == nil || ip.To4() == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return ""
	}
	return ip.String()
}

func waitForVMState(
	manager *VMManager,
	name string,
	states []vm.State,
	timeout time.Duration,
) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		manager.mu.Lock()
		entry := manager.vms[name]
		if entry == nil {
			manager.mu.Unlock()
			return fmt.Errorf("VM %q no longer exists in manager", name)
		}
		state := entry.info.State
		manager.mu.Unlock()

		for _, expected := range states {
			if state == expected {
				return nil
			}
		}

		time.Sleep(200 * time.Millisecond)
	}

	return fmt.Errorf("VM %q did not reach states %v within %s",
		name, states, timeout)
}

func waitForInterface(
	t *testing.T,
	name string,
	wantPresent bool,
	timeout time.Duration,
) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		_, err := net.InterfaceByName(name)
		present := err == nil

		if present == wantPresent {
			return present
		}
		time.Sleep(100 * time.Millisecond)
	}

	_, err := net.InterfaceByName(name)
	return err == nil
}
