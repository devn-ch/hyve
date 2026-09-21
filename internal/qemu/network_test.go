package qemu

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestGuestDHCPBridge(t *testing.T) {
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
			Mode:      NetworkBridge,
			Interface: "br0",
		},
		QGASocket: socket,
	}

	q := New()

	t.Logf("starting QEMU")
	t.Logf("image: %s", image)
	t.Logf("QGA socket: %s", socket)
	t.Logf("network: bridge br0")

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
