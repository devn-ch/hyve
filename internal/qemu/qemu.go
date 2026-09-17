package qemu

import (
	"context"
	"fmt"
	"os/exec"
)

type Config struct {
	Name   string
	CPUs   int
	Memory string
}

type QEMU struct {
	cmd *exec.Cmd
}

func New() *QEMU {
	return &QEMU{}
}

func (q *QEMU) Start(ctx context.Context, cfg Config) error {
	if cfg.CPUs < 1 {
		cfg.CPUs = 1
	}

	if cfg.Memory == "" {
		cfg.Memory = "512M"
	}

	args := []string{
		"-enable-kvm",
		"-name", cfg.Name,
		"-m", cfg.Memory,
		"-smp", fmt.Sprintf("%d", cfg.CPUs),
		"-nodefaults",
		"-nographic",
		"-serial", "stdio",
		"-monitor", "none",
	}

	q.cmd = exec.CommandContext(ctx, "qemu-system-x86_64", args...)

	q.cmd.Stdout = nil
	q.cmd.Stderr = nil

	if err := q.cmd.Start(); err != nil {
		return fmt.Errorf("start qemu: %w", err)
	}

	return nil
}

func (q *QEMU) Wait() error {
	if q.cmd == nil {
		return fmt.Errorf("qemu is not running")
	}

	return q.cmd.Wait()
}