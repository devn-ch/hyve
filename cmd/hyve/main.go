package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
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

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "create":
		create()
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

func create() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hyve create <name>")
		os.Exit(1)
	}

	name := os.Args[2]

	conn := connect()
	defer conn.Close()

	request := Request{
		Command: "create",
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

	fmt.Printf("VM %q created\n", name)
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
