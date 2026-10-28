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
	if t == nil {
		return nil
	}

	if t.file != nil {
		if err := t.file.Close(); err != nil {
			t.file = nil
			return fmt.Errorf("close TAP %q: %w", t.name, err)
		}
		t.file = nil
	}

	if t.name == "" {
		return nil
	}

	if err := deleteTAP(t.name); err != nil {
		return fmt.Errorf("delete TAP %q: %w", t.name, err)
	}

	t.name = ""
	return nil
}
