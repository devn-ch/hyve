package qemu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
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
		"-nodefaults",
		"-nographic",
		"-serial", "stdio",
		"-monitor", "none",
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