package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

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

type VMManager struct {
	mu  sync.Mutex
	vms map[string]*qemu.QEMU
}

func NewVMManager() *VMManager {
	return &VMManager{
		vms: make(map[string]*qemu.QEMU),
	}
}

func (m *VMManager) Start(ctx context.Context, cfg qemu.Config) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.vms[cfg.Name]; exists {
		return os.ErrExist
	}

	vm := qemu.New()

	if err := vm.Start(ctx, cfg); err != nil {
		return err
	}

	m.vms[cfg.Name] = vm

	go func() {
		err := vm.Wait()

		m.mu.Lock()
		delete(m.vms, cfg.Name)
		m.mu.Unlock()

		if err != nil {
			log.Printf("VM %q exited: %v", cfg.Name, err)
		} else {
			log.Printf("VM %q exited", cfg.Name)
		}
	}()

	log.Printf("VM %q started", cfg.Name)

	return nil
}

func (m *VMManager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, vm := range m.vms {
		log.Printf("stopping VM %q", name)

		if err := vm.Stop(); err != nil {
			log.Printf("stop VM %q: %v", name, err)
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

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer cancel()

	manager := NewVMManager()

	go func() {
		<-ctx.Done()

		log.Println("hyved shutting down")

		// Stop accepting new connections.
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

	default:
		_ = json.NewEncoder(conn).Encode(Response{
			OK:    false,
			Error: "unknown command",
		})
	}
}