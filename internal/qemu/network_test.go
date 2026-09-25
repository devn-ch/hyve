package qemu

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"
	"time"
)

func TestNetworkArgs(t *testing.T) {
	tests := []struct {
		name string
		cfg  NetworkConfig
		want []string
	}{
		{
			name: "nat",
			cfg: NetworkConfig{
				Mode: NetworkNAT,
			},
			want: []string{
				"-netdev",
				"user,id=net0",
				"-device",
				"virtio-net-pci,netdev=net0",
			},
		},
		{
			name: "nat with mac",
			cfg: NetworkConfig{
				Mode: NetworkNAT,
				MAC:  "52:54:00:12:34:56",
			},
			want: []string{
				"-netdev",
				"user,id=net0",
				"-device",
				"virtio-net-pci,netdev=net0,mac=52:54:00:12:34:56",
			},
		},
		{
			name: "bridge",
			cfg: NetworkConfig{
				Mode:      NetworkBridge,
				Interface: "br0",
			},
			want: []string{
				"-netdev",
				"bridge,id=net0,br=br0",
				"-device",
				"virtio-net-pci,netdev=net0",
			},
		},
		{
			name: "bridge with mac",
			cfg: NetworkConfig{
				Mode:      NetworkBridge,
				Interface: "br0",
				MAC:       "52:54:00:12:34:56",
			},
			want: []string{
				"-netdev",
				"bridge,id=net0,br=br0",
				"-device",
				"virtio-net-pci,netdev=net0,mac=52:54:00:12:34:56",
			},
		},
		{
			name: "tap",
			cfg: NetworkConfig{
				Mode:      NetworkTAP,
				Interface: "tap0",
			},
			want: []string{
				"-netdev",
				"tap,id=net0,ifname=tap0,script=no,downscript=no",
				"-device",
				"virtio-net-pci,netdev=net0",
			},
		},
		{
			name: "none",
			cfg: NetworkConfig{
				Mode: NetworkNone,
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := networkArgs(tt.cfg)
			if err != nil {
				t.Fatalf("networkArgs() error = %v", err)
			}

			if !slices.Equal(got, tt.want) {
				t.Fatalf("networkArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func requireBridge(t *testing.T, name string) {
	t.Helper()

	if runtime.GOOS != "linux" {
		t.Skip("bridge networking requires Linux")
	}

	if _, err := net.InterfaceByName(name); err != nil {
		t.Skipf("network bridge %q not available: %v. For dev container: sudo ./testdata/host/network-test.sh setup", name, err)
	}
}

func startTestDHCP(t *testing.T) func() {
	t.Helper()

	dir := t.TempDir()

	// pidFile := filepath.Join(dir, "dnsmasq.pid")
	leaseFile := filepath.Join(dir, "dnsmasq.leases")

	cmd := exec.Command(
		"sudo",
		"dnsmasq",
		"--no-daemon",
		"--interface=br0",
		"--bind-interfaces",
		"--except-interface=lo",
		"--dhcp-range=192.168.100.100,192.168.100.200,255.255.255.0,1h",
		"--dhcp-option=3,192.168.100.1",
		"--dhcp-option=6,192.168.100.1",
		// "--pid-file="+pidFile,
		"--dhcp-leasefile="+leaseFile,
		"--log-dhcp",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	t.Log("starting test DHCP server")

	if err := cmd.Start(); err != nil {
		t.Fatalf("start dnsmasq: %v", err)
	}

	t.Cleanup(func() {
		t.Log("stopping test DHCP server")

		if cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}

		_ = cmd.Wait()
	})

	return func() {}
}

func TestGuestDHCP(t *testing.T) {
	requireBridge(t, "br0")

	image := qgaTestImage(t)
	socket := filepath.Join(t.TempDir(), "qga.sock")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := Config{
		Name:   "network-test",
		CPUs:   1,
		Memory: "512M",
		Drives: []Drive{
			{
				Type: DriveTypeDisk,
				Path: image,
			},
		},
		Network: NetworkConfig{
			Mode:      NetworkTAP,
			Interface: "hyve-tap0",
		},
		QGASocket: socket,
	}

	q := New()

	startTestDHCP(t)

	t.Logf("starting QEMU")
	t.Logf("image: %s", image)
	t.Logf("QGA socket: %s", socket)
	t.Logf("network: tap hyve-tap0 -> br0")

	if err := q.Start(ctx, cfg); err != nil {
		t.Fatalf("start QEMU: %v", err)
	}

	defer func() {
		if err := q.Stop(); err != nil {
			t.Logf("stop QEMU: %v", err)
		}
	}()

	done := make(chan error, 1)

	go func() {
		done <- q.Wait()
	}()

	waitForQGA(t, q, done)

	t.Log("QGA is ready")

	deadline := time.Now().Add(60 * time.Second)

	for time.Now().Before(deadline) {
		interfaces, err := q.GuestNetworkInterfaces()
		if err != nil {
			t.Logf("get guest network interfaces: %v", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for _, iface := range interfaces {
			t.Logf(
				"interface %s (%s): %+v",
				iface.Name,
				iface.HardwareAddress,
				iface.IPAddresses,
			)

			if iface.Name == "lo" {
				continue
			}

			for _, ip := range iface.IPAddresses {
				if ip.Type != "ipv4" {
					continue
				}

				t.Logf(
					"guest received IPv4 address: %s/%d on %s",
					ip.Address,
					ip.Prefix,
					iface.Name,
				)

				t.Log("bridge DHCP test successful")
				return
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	t.Fatal("guest did not receive an IPv4 address")
}
