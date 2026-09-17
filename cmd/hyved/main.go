package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/devn-ch/hyve/internal/qemu"
	"github.com/devn-ch/hyve/internal/vm"
)

const socketPath = "/run/hyve/hyved.sock"

type Request struct {
	Command string     `json:"command"`
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
	OK  bool     `json:"ok"`
	VMs []VMInfo `json:"vms,omitempty"`
}

type managedVM struct {
	info vm.VM
	qemu *qemu.QEMU
}

type VMManager struct {
	mu  sync.Mutex
	vms map[string]*managedVM
}

func NewVMManager() *VMManager {
	return &VMManager{
		vms: make(map[string]*managedVM),
	}
}

func (m *VMManager) Start(ctx context.Context, cfg qemu.Config) error {
	m.mu.Lock()

	if existing, exists := m.vms[cfg.Name]; exists {
		state := existing.info.State
		m.mu.Unlock()

		return fmt.Errorf("VM %q already exists (%s)", cfg.Name, state)
	}

	entry := &managedVM{
		info: vm.VM{
			Name:   cfg.Name,
			State:  vm.StateStarting,
			CPUs:   cfg.CPUs,
			Memory: cfg.Memory,
		},
		qemu: qemu.New(),
	}

	m.vms[cfg.Name] = entry
	m.mu.Unlock()

	if err := entry.qemu.Start(ctx, cfg); err != nil {
		m.mu.Lock()
		delete(m.vms, cfg.Name)
		m.mu.Unlock()

		return err
	}

	m.mu.Lock()
	entry.info.State = vm.StateRunning
	m.mu.Unlock()

	log.Printf("VM %q is running", cfg.Name)

	go func() {
		err := entry.qemu.Wait()

		m.mu.Lock()

		if err != nil {
			entry.info.State = vm.StateExited
		} else {
			entry.info.State = vm.StateStopped
		}

		m.mu.Unlock()

		if err != nil {
			log.Printf("VM %q exited: %v", cfg.Name, err)
		} else {
			log.Printf("VM %q stopped", cfg.Name)
		}
	}()

	return nil
}

func (m *VMManager) List() []VMInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]VMInfo, 0, len(m.vms))

	for _, entry := range m.vms {
		result = append(result, VMInfo{
			Name:   entry.info.Name,
			State:  entry.info.State,
			CPUs:   entry.info.CPUs,
			Memory: entry.info.Memory,
		})
	}

	return result
}

func (m *VMManager) StopAll() {
	m.mu.Lock()

	vms := make([]*managedVM, 0, len(m.vms))

	for _, entry := range m.vms {
		if entry.info.State == vm.StateRunning {
			entry.info.State = vm.StateStopping
			vms = append(vms, entry)
		}
	}

	m.mu.Unlock()

	for _, entry := range vms {
		log.Printf("stopping VM %q", entry.info.Name)

		if err := entry.qemu.Stop(); err != nil {
			log.Printf("stop VM %q: %v", entry.info.Name, err)
		}
	}
}

func main() {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		log.Fatal(err)
	}

	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		listener.Close()
		os.Remove(socketPath)
	}()

	if err := os.Chmod(socketPath, 0660); err != nil {
		log.Fatal(err)
	}

	log.Printf("hyved listening on %s", socketPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := NewVMManager()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	go func() {
		<-signalChan

		log.Println("hyved shutting down")

		// Signal the accept loop first so it exits cleanly.
		cancel()

		// Close the socket so no new connections are accepted.
		listener.Close()

		// Stop all VMs owned by hyved.
		manager.StopAll()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				log.Printf("accept: %v", err)
				continue
			}
		}

		go handleConnection(ctx, manager, conn)
	}
}

func handleConnection(
	ctx context.Context,
	manager *VMManager,
	conn net.Conn,
) {
	defer conn.Close()

	var request Request

	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		_ = json.NewEncoder(conn).Encode(Response{
			OK:    false,
			Error: err.Error(),
		})
		return
	}

	switch request.Command {
	case "run":
		err := manager.Start(ctx, request.Config)

		if err != nil {
			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		_ = json.NewEncoder(conn).Encode(Response{
			OK: true,
		})

	case "list":
		_ = json.NewEncoder(conn).Encode(ListResponse{
			OK:  true,
			VMs: manager.List(),
		})

	default:
		_ = json.NewEncoder(conn).Encode(Response{
			OK:    false,
			Error: "unknown command",
		})
	}
}