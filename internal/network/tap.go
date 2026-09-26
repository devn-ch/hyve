package network

import (
	"fmt"
	"os"
)

type TAP struct {
	name string
	file *os.File
}

func CreateTAP(name string) (*TAP, error) {
	if name == "" {
		return nil, fmt.Errorf("TAP name is empty")
	}

	file, actualName, err := createTAP(name)
	if err != nil {
		return nil, err
	}

	return &TAP{
		name: actualName,
		file: file,
	}, nil
}

func (t *TAP) Close() error {
	if t == nil || t.file == nil {
		return nil
	}

	err := t.file.Close()
	t.file = nil

	if err != nil {
		return fmt.Errorf("close TAP %q: %w", t.name, err)
	}

	return nil
}
