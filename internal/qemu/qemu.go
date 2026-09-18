package qemu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

type DriveType string

const (
	DriveTypeDisk  DriveType = "disk"
	DriveTypeCDROM DriveType = "cdrom"
)

type Drive struct {
	Type     DriveType
	Path     string
	Size     string
	ReadOnly bool
}

type Config struct {
	Name          string
	CPUs          int
	Memory        string
	Drives        []Drive
	BootFromCDROM bool
	QMPSocket     string
	ConsoleSocket string
}

type QEMU struct {
	cmd *exec.Cmd
}

func New() *QEMU {
	return &QEMU{}
}

func kvmAvailable() bool {
	_, err := os.Stat("/dev/kvm")
	return err == nil
}

func (q *QEMU) Start(ctx context.Context, cfg Config) error {
	if cfg.CPUs < 1 {
		cfg.CPUs = 1
	}

	if cfg.Memory == "" {
		cfg.Memory = "512M"
	}

	args := []string{
		"-name", cfg.Name,
		"-m", cfg.Memory,
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-cpu", "host",
		"-nographic",
		"-monitor", "none",
	}

	if cfg.BootFromCDROM {
		args = append(args,
			"-boot",
			"order=d",
		)
	}

	for _, drive := range cfg.Drives {
		switch drive.Type {
		case DriveTypeDisk:
			args = append(args,
				"-drive",
				fmt.Sprintf(
					"file=%s,format=qcow2,if=virtio",
					drive.Path,
				),
			)

		case DriveTypeCDROM:
			args = append(args,
				"-drive",
				fmt.Sprintf(
					"file=%s,media=cdrom,readonly=on",
					drive.Path,
				),
			)

		default:
			return fmt.Errorf("unsupported drive type %q", drive.Type)
		}
	}

	if cfg.QMPSocket != "" {
		args = append(args,
			"-qmp",
			fmt.Sprintf("unix:%s,server=on,wait=off", cfg.QMPSocket),
		)
	}

	if kvmAvailable() {
		args = append([]string{"-enable-kvm"}, args...)
	}

	q.cmd = exec.CommandContext(
		ctx,
		"qemu-system-x86_64",
		args...,
	)

	if err := q.cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	return nil
}

func (q *QEMU) Stop() error {
	if q.cmd == nil || q.cmd.Process == nil {
		return nil
	}

	// Graceful process termination.
	if err := q.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("stop qemu: %w", err)
	}

	return nil
}

func (q *QEMU) Wait() error {
	if q.cmd == nil {
		return fmt.Errorf("qemu is not running")
	}

	return q.cmd.Wait()
}
