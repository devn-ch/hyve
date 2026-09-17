package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/devn-ch/hyve/internal/qemu"
)

const socketPath = "/run/hyve/hyved.sock"

type Request struct {
	Command string     `json:"command"`
	Config qemu.Config `json:"config"`
}

type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "run":
		run()
	default:
		usage()
		os.Exit(1)
	}
}

func run() {
	name := "test"

	if len(os.Args) >= 3 {
		name = os.Args[2]
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyve: connect to hyved: %v\n", err)
		os.Exit(1)
	}
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

func usage() {
	fmt.Println("usage: hyve run [name]")
}
