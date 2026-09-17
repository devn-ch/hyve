package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/devn-ch/hyve/internal/qemu"
	"github.com/devn-ch/hyve/internal/vm"
)

const socketPath = "/run/hyve/hyved.sock"

type Request struct {
	Command string      `json:"command"`
	Config  qemu.Config `json:"config"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

type VMInfo struct {
	Name   string   `json:"name"`
	State  vm.State `json:"state"`
	CPUs   int      `json:"cpus"`
	Memory string   `json:"memory"`
}

type ListResponse struct {
	OK    bool     `json:"ok"`
	Error string   `json:"error,omitempty"`
	VMs   []VMInfo `json:"vms,omitempty"`
}

func parseCreateArgs(args []string) (qemu.Config, error) {
	if len(args) < 1 {
		return qemu.Config{}, fmt.Errorf("VM name is required")
	}

	cfg := qemu.Config{
		Name:   args[0],
		CPUs:   1,
		Memory: "512M",
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == "--cpus":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf("--cpus requires a value")
			}

			cpus, err := strconv.Atoi(args[i+1])
			if err != nil || cpus < 1 {
				return qemu.Config{}, fmt.Errorf(
					"invalid CPU count %q",
					args[i+1],
				)
			}

			cfg.CPUs = cpus
			i++

		case strings.HasPrefix(arg, "--cpus="):
			value := strings.TrimPrefix(arg, "--cpus=")

			cpus, err := strconv.Atoi(value)
			if err != nil || cpus < 1 {
				return qemu.Config{}, fmt.Errorf(
					"invalid CPU count %q",
					value,
				)
			}

			cfg.CPUs = cpus

		case arg == "--memory":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf("--memory requires a value")
			}

			cfg.Memory = args[i+1]
			i++

		case strings.HasPrefix(arg, "--memory="):
			cfg.Memory = strings.TrimPrefix(arg, "--memory=")

			if cfg.Memory == "" {
				return qemu.Config{}, fmt.Errorf(
					"--memory requires a value",
				)
			}

		case arg == "--drive":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf("--drive requires a value")
			}

			drive, err := parseDrive(args[i+1])
			if err != nil {
				return qemu.Config{}, err
			}

			cfg.Drives = append(cfg.Drives, drive)
			i++

		case strings.HasPrefix(arg, "--drive="):
			value := strings.TrimPrefix(arg, "--drive=")

			drive, err := parseDrive(value)
			if err != nil {
				return qemu.Config{}, err
			}

			cfg.Drives = append(cfg.Drives, drive)

		default:
			return qemu.Config{}, fmt.Errorf(
				"unknown option %q",
				arg,
			)
		}
	}

	return cfg, nil
}

func parseDrive(value string) (qemu.Drive, error) {
	parts := strings.SplitN(value, ":", 2)

	if len(parts) != 2 || parts[1] == "" {
		return qemu.Drive{}, fmt.Errorf(
			"invalid --drive %q, expected disk:SIZE or cdrom:PATH",
			value,
		)
	}

	switch parts[0] {
	case "disk":
		return qemu.Drive{
			Type: qemu.DriveTypeDisk,
			Size: parts[1],
		}, nil

	case "cdrom":
		return qemu.Drive{
			Type:     qemu.DriveTypeCDROM,
			Path:     parts[1],
			ReadOnly: true,
		}, nil

	default:
		return qemu.Drive{}, fmt.Errorf(
			"unsupported drive type %q",
			parts[0],
		)
	}
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		if len(os.Args) < 3 {
			usage()
			os.Exit(1)
		}

		cfg, err := parseCreateArgs(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "hyve: %v\n", err)
			os.Exit(1)
		}

		create(cfg)
	case "run":
		run()
	case "list":
		list()
	case "stop":
		stop()
	case "destroy":
		destroy()
	default:
		usage()
		os.Exit(1)
	}
}

func connect() net.Conn {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyve: connect to hyved: %v\n", err)
		os.Exit(1)
	}

	return conn
}

func run() {
	name := "test"

	if len(os.Args) >= 3 {
		name = os.Args[2]
	}

	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "run",
		Config: qemu.Config{
			Name:   name,
			CPUs:   2,
			Memory: "512M",
		},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: send request: %v\n", err)
		os.Exit(1)
	}

	var response Response

	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: read response: %v\n", err)
		os.Exit(1)
	}

	if !response.OK {
		fmt.Fprintf(os.Stderr, "hyve: %s\n", response.Error)
		os.Exit(1)
	}

	fmt.Printf("VM %q started\n", name)
}

func list() {
	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "list",
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: send request: %v\n", err)
		os.Exit(1)
	}

	var response ListResponse

	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: read response: %v\n", err)
		os.Exit(1)
	}

	if !response.OK {
		fmt.Fprintln(os.Stderr, "hyve: list failed")
		os.Exit(1)
	}

	if len(response.VMs) == 0 {
		fmt.Println("No VMs.")
		return
	}

	writer := tabwriter.NewWriter(
		os.Stdout,
		0,
		4,
		2,
		' ',
		0,
	)

	fmt.Fprintln(writer, "NAME\tSTATE\tCPU\tMEMORY")

	for _, v := range response.VMs {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%d\t%s\n",
			v.Name,
			v.State,
			v.CPUs,
			v.Memory,
		)
	}

	writer.Flush()
}

func create(cfg qemu.Config) {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyve: connect: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	request := Request{
		Command: "create",
		Config:  cfg,
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: send request: %v\n", err)
		os.Exit(1)
	}

	var response Response

	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: read response: %v\n", err)
		os.Exit(1)
	}

	if !response.OK {
		fmt.Fprintf(os.Stderr, "hyve: %s\n", response.Error)
		os.Exit(1)
	}

	fmt.Printf("VM %q created\n", cfg.Name)
}

func stop() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "hyve: VM name is required")
		os.Exit(1)
	}

	name := os.Args[2]

	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "stop",
		Config: qemu.Config{
			Name: name,
		},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: send request: %v\n", err)
		os.Exit(1)
	}

	var response Response

	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: read response: %v\n", err)
		os.Exit(1)
	}

	if !response.OK {
		fmt.Fprintf(os.Stderr, "hyve: %s\n", response.Error)
		os.Exit(1)
	}

	fmt.Printf("VM %q stopped\n", name)
}

func destroy() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "hyve: VM name is required")
		os.Exit(1)
	}

	name := os.Args[2]

	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "destroy",
		Config: qemu.Config{
			Name: name,
		},
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: send request: %v\n", err)
		os.Exit(1)
	}

	var response Response

	if err := json.NewDecoder(conn).Decode(&response); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: read response: %v\n", err)
		os.Exit(1)
	}

	if !response.OK {
		fmt.Fprintf(os.Stderr, "hyve: %s\n", response.Error)
		os.Exit(1)
	}

	fmt.Printf("VM %q destroyed\n", name)
}

func usage() {
	fmt.Println("usage:")
	fmt.Println("  hyve create <name>")
	fmt.Println("  hyve run [name]")
	fmt.Println("  hyve list")
	fmt.Println("  hyve stop <name>")
	fmt.Println("  hyve destroy <name>")
}
