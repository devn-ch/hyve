//go:build linux

package network

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	tunDevice = "/dev/net/tun"

	iffTap  = 0x0002
	iffNoPI = 0x1000
)

type ifreq struct {
	Name  [unix.IFNAMSIZ]byte
	Flags uint16
	_     [unix.IFNAMSIZ - 2 - 2]byte
}

func createTAP(name string) (*os.File, string, error) {
	file, err := os.OpenFile(tunDevice, os.O_RDWR, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open %s: %w", tunDevice, err)
	}

	var req ifreq

	copy(req.Name[:], name)
	req.Flags = iffTap | iffNoPI

	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		file.Fd(),
		uintptr(unix.TUNSETIFF),
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		_ = file.Close()
		return nil, "", fmt.Errorf("TUNSETIFF %q: %w", name, errno)
	}

	actualName := unix.ByteSliceToString(req.Name[:])

	return file, actualName, nil
}
