package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const DefaultStateDir = "/var/lib/hyve/vms"

type DriveType string

const (
	DriveTypeDisk  DriveType = "disk"
	DriveTypeCDROM DriveType = "cdrom"
)

type Drive struct {
	Type     DriveType `json:"type"`
	Path     string    `json:"path"`
	Size     string    `json:"size,omitempty"`
	ReadOnly bool      `json:"readonly,omitempty"`
}

type Definition struct {
	Name   string  `json:"name"`
	CPUs   int     `json:"cpus"`
	Memory string  `json:"memory"`
	Drives []Drive `json:"drives"`
}

type Store struct {
	BaseDir string
}

func NewStore(baseDir string) *Store {
	return &Store{
		BaseDir: baseDir,
	}
}

func (s *Store) Create(def Definition) error {
	if def.Name == "" {
		return fmt.Errorf("VM name is required")
	}

	if def.CPUs < 1 {
		def.CPUs = 1
	}

	if def.Memory == "" {
		def.Memory = "512M"
	}

	if err := validateDrives(def.Drives); err != nil {
		return err
	}

	vmDir := filepath.Join(s.BaseDir, def.Name)

	if _, err := os.Stat(vmDir); err == nil {
		return fmt.Errorf("VM %q already exists", def.Name)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("check VM %q: %w", def.Name, err)
	}

	if err := os.MkdirAll(vmDir, 0755); err != nil {
		return fmt.Errorf("create VM directory: %w", err)
	}

	if err := prepareDrives(vmDir, def.Drives); err != nil {
		_ = os.RemoveAll(vmDir)
		return err
	}

	data, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		_ = os.RemoveAll(vmDir)
		return fmt.Errorf("encode VM definition: %w", err)
	}

	path := filepath.Join(vmDir, "vm.json")

	if err := os.WriteFile(path, data, 0644); err != nil {
		_ = os.RemoveAll(vmDir)
		return fmt.Errorf("write VM definition: %w", err)
	}

	return nil
}

func validateDrives(drives []Drive) error {
	if len(drives) == 0 {
		return fmt.Errorf("at least one --drive is required")
	}

	hasDisk := false

	for _, drive := range drives {
		switch drive.Type {
		case DriveTypeDisk:
			if drive.Size == "" {
				return fmt.Errorf("disk drive requires a size")
			}

			hasDisk = true

		case DriveTypeCDROM:
			if drive.Path == "" {
				return fmt.Errorf("cdrom drive requires a path")
			}

		default:
			return fmt.Errorf("unsupported drive type %q", drive.Type)
		}
	}

	if !hasDisk {
		return fmt.Errorf("at least one disk drive is required")
	}

	return nil
}

func prepareDrives(vmDir string, drives []Drive) error {
	diskIndex := 0

	for i := range drives {
		drive := &drives[i]

		switch drive.Type {
		case DriveTypeDisk:
			filename := fmt.Sprintf("disk-%d.qcow2", diskIndex)
			diskIndex++

			drive.Path = filename

			diskPath := filepath.Join(vmDir, filename)

			if err := createDisk(diskPath, drive.Size); err != nil {
				return err
			}

		case DriveTypeCDROM:
			if _, err := os.Stat(drive.Path); err != nil {
				return fmt.Errorf(
					"cdrom %q: %w",
					drive.Path,
					err,
				)
			}

			drive.ReadOnly = true
		}
	}

	return nil
}

func createDisk(path string, size string) error {
	cmd := exec.Command(
		"qemu-img",
		"create",
		"-f", "qcow2",
		path,
		size,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf(
			"create disk %q: %w: %s",
			path,
			err,
			string(output),
		)
	}

	return nil
}

func (s *Store) Load(name string) (Definition, error) {
	path := filepath.Join(s.BaseDir, name, "vm.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("read VM %q: %w", name, err)
	}

	var def Definition

	if err := json.Unmarshal(data, &def); err != nil {
		return Definition{}, fmt.Errorf("decode VM %q: %w", name, err)
	}

	return def, nil
}

func (s *Store) List() ([]Definition, error) {
	entries, err := os.ReadDir(s.BaseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Definition{}, nil
		}

		return nil, fmt.Errorf("read VM directory: %w", err)
	}

	result := make([]Definition, 0)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		def, err := s.Load(entry.Name())
		if err != nil {
			return nil, err
		}

		result = append(result, def)
	}

	return result, nil
}

func (s *Store) Delete(name string) error {
	vmDir := filepath.Join(s.BaseDir, name)

	if _, err := os.Stat(vmDir); os.IsNotExist(err) {
		return fmt.Errorf("VM %q does not exist", name)
	}

	if err := os.RemoveAll(vmDir); err != nil {
		return fmt.Errorf("delete VM %q: %w", name, err)
	}

	return nil
}
