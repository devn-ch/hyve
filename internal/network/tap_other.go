//go:build !linux

package network

import (
	"fmt"
	"os"
)

func createTAP(name string) (*os.File, string, error) {
	return nil, "", fmt.Errorf("TAP networking is not supported on this platform")
}

func deleteTAP(name string) error {
	return fmt.Errorf("TAP networking is unsupported on this platform")
}
