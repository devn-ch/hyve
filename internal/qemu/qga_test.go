package qemu

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestGuestPing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("QGA integration test requires Linux")
	}

	image := qgaTestImage(t)
	socket := filepath.Join(t.TempDir(), "qga.sock")

	cmd := exec.Command(
		"qemu-system-x86_64",
		"-machine", "q35",
		"-m", "512M",
		"-smp", "1",
		"-drive",
		"file="+image+",if=virtio,format=qcow2",
		"-chardev",
		"socket,id=qga,path="+socket+",server=on,wait=off",
		"-device", "virtio-serial",
		"-device",
		"virtserialport,chardev=qga,name=org.qemu.guest_agent.0",
		"-display", "none",
		"-serial", "none",
		"-monitor", "none",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	t.Logf("starting QEMU")
	t.Logf("image: %s", image)
	t.Logf("QGA socket: %s", socket)

	if err := cmd.Start(); err != nil {
		t.Fatalf("start QEMU: %v", err)
	}

	t.Logf("QEMU started with pid %d", cmd.Process.Pid)

	qemuDone := make(chan error, 1)

	go func() {
		qemuDone <- cmd.Wait()
	}()

	t.Cleanup(func() {
		select {
		case err := <-qemuDone:
			t.Logf("QEMU already exited: %v", err)

		default:
			t.Logf("stopping QEMU (pid %d)", cmd.Process.Pid)

			_ = cmd.Process.Kill()

			err := <-qemuDone
			t.Logf("QEMU stopped: %v", err)
		}
	})

	q := &QEMU{
		cmd: cmd,
		qga: socket,
	}

	waitForQGA(t, q, qemuDone)

	t.Log("QGA is ready")

	if err := q.GuestPing(); err != nil {
		t.Fatalf("GuestPing failed: %v", err)
	}

	t.Log("GuestPing successful")
}

func qgaTestImage(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test source path")
	}

	root := filepath.Clean(
		filepath.Join(filepath.Dir(filename), "..", ".."),
	)

	image := filepath.Join(
		root,
		".build",
		"qga",
		"debian-qga.qcow2",
	)

	if _, err := os.Stat(image); err != nil {
		t.Skipf(
			"QGA test image not found: %s (run testdata/qga/build.sh first)",
			image,
		)
	}

	return image
}

func waitForQGA(
	t *testing.T,
	q *QEMU,
	qemuDone <-chan error,
) {
	t.Helper()

	const (
		timeout       = 60 * time.Second
		retryInterval = 500 * time.Millisecond
	)

	deadline := time.Now().Add(timeout)

	var lastErr error

	for time.Now().Before(deadline) {
		select {
		case err := <-qemuDone:
			t.Fatalf(
				"QEMU exited while waiting for QGA: %v",
				err,
			)

		default:
		}

		if _, err := os.Stat(q.qga); err == nil {
			if err := q.GuestPing(); err == nil {
				return
			} else {
				lastErr = err
			}
		}

		time.Sleep(retryInterval)
	}

	t.Fatalf(
		"timeout waiting for QGA after %s: last error: %v",
		timeout,
		lastErr,
	)
}
