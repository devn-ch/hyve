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

type ConsoleType string

const (
	ConsoleVNC    ConsoleType = "vnc"
	ConsoleSerial ConsoleType = "serial"
)

type Config struct {
	Name          string
	CPUs          int
	Memory        string
	Drives        []Drive
	BootFromCDROM bool

	Network NetworkConfig

	QMPSocket     string
	QGASocket     string
	ConsoleType   ConsoleType
	ConsoleSocket string
}

type QEMU struct {
	cmd *exec.Cmd
	qga string
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
		"-display", "none",
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

	switch cfg.ConsoleType {
	case ConsoleVNC:
		if cfg.ConsoleSocket == "" {
			return fmt.Errorf("VNC console requires a console socket")
		}

		args = append(args,
			"-vnc",
			"unix:"+cfg.ConsoleSocket,
		)

	case ConsoleSerial:
		if cfg.ConsoleSocket == "" {
			return fmt.Errorf("serial console requires a console socket")
		}

		args = append(args,
			"-serial",
			"unix:"+cfg.ConsoleSocket+",server=on,wait=off",
		)

	case "":
		// No console requested.

	default:
		return fmt.Errorf("unsupported console type %q", cfg.ConsoleType)
	}

	q.qga = cfg.QGASocket
	if cfg.QGASocket != "" {
		args = append(args,
			"-chardev",
			fmt.Sprintf(
				"socket,id=qga0,path=%s,server=on,wait=off",
				cfg.QGASocket,
			),
			"-device",
			"virtio-serial",
			"-device",
			"virtserialport,chardev=qga0,name=org.qemu.guest_agent.0",
		)
	}

	network, err := networkArgs(cfg.Network)
	if err != nil {
		return err
	}

	args = append(args, network...)

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
