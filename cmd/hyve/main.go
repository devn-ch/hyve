package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
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
	OK            bool   `json:"ok"`
	Error         string `json:"error,omitempty"`
	ConsoleSocket string `json:"console_socket,omitempty"`
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
		Network: qemu.NetworkConfig{
			Mode: qemu.NetworkNAT,
		},
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

		case arg == "--network":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf("--network requires a value")
			}

			mode, err := parseNetworkMode(args[i+1])
			if err != nil {
				return qemu.Config{}, err
			}

			cfg.Network.Mode = mode
			i++

		case strings.HasPrefix(arg, "--network="):
			value := strings.TrimPrefix(arg, "--network=")

			mode, err := parseNetworkMode(value)
			if err != nil {
				return qemu.Config{}, err
			}

			cfg.Network.Mode = mode

		case arg == "--interface":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf(
					"--interface requires a value",
				)
			}

			cfg.Network.Interface = args[i+1]
			i++

		case strings.HasPrefix(arg, "--interface="):
			value := strings.TrimPrefix(arg, "--interface=")

			if value == "" {
				return qemu.Config{}, fmt.Errorf(
					"--interface requires a value",
				)
			}

			cfg.Network.Interface = value

		case arg == "--mac":
			if i+1 >= len(args) {
				return qemu.Config{}, fmt.Errorf("--mac requires a value")
			}

			cfg.Network.MAC = args[i+1]
			i++

		case strings.HasPrefix(arg, "--mac="):
			value := strings.TrimPrefix(arg, "--mac=")

			if value == "" {
				return qemu.Config{}, fmt.Errorf(
					"--mac requires a value",
				)
			}

			cfg.Network.MAC = value

		default:
			return qemu.Config{}, fmt.Errorf(
				"unknown option %q",
				arg,
			)
		}
	}

	switch cfg.Network.Mode {
	case qemu.NetworkBridge, qemu.NetworkTAP:
		if cfg.Network.Interface == "" {
			return qemu.Config{}, fmt.Errorf(
				"--interface is required for network mode %q",
				cfg.Network.Mode,
			)
		}
	}

	return cfg, nil
}

func parseNetworkMode(value string) (qemu.NetworkMode, error) {
	switch value {
	case "nat":
		return qemu.NetworkNAT, nil

	case "bridge":
		return qemu.NetworkBridge, nil

	case "tap":
		return qemu.NetworkTAP, nil

	case "none":
		return qemu.NetworkNone, nil

	default:
		return "", fmt.Errorf(
			"unsupported network mode %q (expected nat, bridge, tap, or none)",
			value,
		)
	}
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

	case "console":
		console()

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

func proxyVNC(tcpConn net.Conn, consoleSocket string) {
	defer tcpConn.Close()

	unixConn, err := net.Dial("unix", consoleSocket)
	if err != nil {
		return
	}
	defer unixConn.Close()

	done := make(chan struct{}, 2)

	go func() {
		_, _ = io.Copy(unixConn, tcpConn)
		done <- struct{}{}
	}()

	go func() {
		_, _ = io.Copy(tcpConn, unixConn)
		done <- struct{}{}
	}()

	<-done

	_ = tcpConn.Close()
	_ = unixConn.Close()

	<-done
}

func console() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: hyve console <name>")
		os.Exit(1)
	}

	name := os.Args[2]

	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "console",
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

	if response.ConsoleSocket == "" {
		fmt.Fprintln(os.Stderr, "hyve: daemon returned no console socket")
		os.Exit(1)
	}

	if _, err := os.Stat(response.ConsoleSocket); err != nil {
		fmt.Fprintf(
			os.Stderr,
			"hyve: console socket %q is not available: %v\n",
			response.ConsoleSocket,
			err,
		)
		os.Exit(1)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyve: create local VNC listener: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	fmt.Printf(
		"Connecting to VM %q on local VNC port %d...\n",
		name,
		port,
	)

	viewer := exec.Command(
		"remote-viewer",
		fmt.Sprintf("vnc://127.0.0.1:%d", port),
	)

	if err := viewer.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "hyve: start remote-viewer: %v\n", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup

	acceptDone := make(chan struct{})

	wg.Add(1)

	go func() {
		defer wg.Done()
		defer close(acceptDone)

		for {
			tcpConn, err := listener.Accept()
			if err != nil {
				return
			}

			wg.Add(1)

			go func() {
				defer wg.Done()
				proxyVNC(tcpConn, response.ConsoleSocket)
			}()
		}
	}()

	// Wait until VNC viewer exits
	_ = viewer.Wait()

	// Stop accepting new VNC connections.
	_ = listener.Close()

	// Wait until the accept loop and all active proxies have finished.
	wg.Wait()

	fmt.Printf("Console for VM %q closed\n", name)
}

func usage() {
	fmt.Println("usage:")
	fmt.Println("  hyve create <name> [options]")
	fmt.Println("  hyve run [name]")
	fmt.Println("  hyve list")
	fmt.Println("  hyve stop <name>")
	fmt.Println("  hyve destroy <name>")
	fmt.Println("  hyve console <name>")
	fmt.Println()
	fmt.Println("create options:")
	fmt.Println("  --cpus <n>            Number of CPUs")
	fmt.Println("  --memory <size>       Memory size, e.g. 512M")
	fmt.Println("  --drive <type:value>  disk:SIZE or cdrom:PATH")
	fmt.Println("  --network <mode>      nat, bridge, tap, or none")
	fmt.Println("  --interface <name>    Bridge or TAP interface")
	fmt.Println("  --mac <address>       MAC address")
}
