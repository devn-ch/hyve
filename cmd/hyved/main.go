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

const (
	socketPath = "/run/hyve/hyved.sock"
	qmpDir     = "/run/hyve/qmp"
	qgaDir     = "/run/hyve/qga"
	consoleDir = "/run/hyve/console"
)

type Request struct {
	Command string      `json:"command"`
	Config  qemu.Config `json:"config"`
}

type Response struct {
	OK            bool    `json:"ok"`
	Error         string  `json:"error,omitempty"`
	ConsoleSocket string  `json:"console_socket,omitempty"`
	Info          *VMInfo `json:"info,omitempty"`
}

type VMInfo struct {
	Name       string                       `json:"name"`
	State      vm.State                     `json:"state"`
	CPUs       int                          `json:"cpus"`
	Memory     string                       `json:"memory"`
	Network    qemu.NetworkConfig           `json:"network"`
	Interfaces []qemu.GuestNetworkInterface `json:"interfaces,omitempty"`
}

type ListResponse struct {
	OK    bool     `json:"ok"`
	Error string   `json:"error,omitempty"`
	VMs   []VMInfo `json:"vms,omitempty"`
}

type managedVM struct {
	info          vm.VM
	qemu          *qemu.QEMU
	qmpSocket     string
	qgaSocket     string
	consoleSocket string
}

type VMManager struct {
	mu    sync.Mutex
	vms   map[string]*managedVM
	store *vm.Store
}

func NewVMManager(store *vm.Store) *VMManager {
	return &VMManager{
		vms:   make(map[string]*managedVM),
		store: store,
	}
}

func (m *VMManager) Start(ctx context.Context, cfg qemu.Config) error {
	qmpSocket := filepath.Join(qmpDir, cfg.Name+".sock")
	qgaSocket := filepath.Join(qgaDir, cfg.Name+".sock")
	consoleSocket := filepath.Join(consoleDir, cfg.Name+".sock")

	// Clean up any stale sockets from previous runs of hyved.
	// This is important because if a socket file already exists, QEMU will fail to start.
	for _, socket := range []string{
		qmpSocket,
		qgaSocket,
		consoleSocket,
	} {
		if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale socket %s: %w", socket, err)
		}
	}

	// Create the directory for the sockets

	if err := os.MkdirAll(qmpDir, 0755); err != nil {
		return fmt.Errorf("create QMP directory: %w", err)
	}

	// create the directory for the console socket
	if err := os.MkdirAll(consoleDir, 0755); err != nil {
		return fmt.Errorf("create console directory: %w", err)
	}

	// Create the directory for the QGA socket
	if err := os.MkdirAll(qgaDir, 0755); err != nil {
		return fmt.Errorf("create QGA directory: %w", err)
	}

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
		qemu:          qemu.New(),
		qmpSocket:     qmpSocket,
		qgaSocket:     qgaSocket,
		consoleSocket: consoleSocket,
	}

	m.vms[cfg.Name] = entry
	m.mu.Unlock()

	cfg.QMPSocket = qmpSocket
	cfg.QGASocket = qgaSocket
	cfg.ConsoleSocket = consoleSocket

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

		_ = os.Remove(qmpSocket)
		_ = os.Remove(qgaSocket)
		_ = os.Remove(consoleSocket)

		m.mu.Lock()
		defer m.mu.Unlock()

		if err != nil {
			entry.info.State = vm.StateExited
			log.Printf("VM %q exited: %v", cfg.Name, err)
		} else {
			entry.info.State = vm.StateStopped
			log.Printf("VM %q stopped", cfg.Name)
		}
	}()

	return nil
}

func (m *VMManager) List() ([]VMInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	definitions, err := m.store.List()
	if err != nil {
		return nil, err
	}

	result := make([]VMInfo, 0, len(definitions))

	for _, def := range definitions {
		state := vm.StateStopped

		if entry, exists := m.vms[def.Name]; exists {
			state = entry.info.State
		}

		result = append(result, VMInfo{
			Name:   def.Name,
			State:  state,
			CPUs:   def.CPUs,
			Memory: def.Memory,
		})
	}

	return result, nil
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

func (m *VMManager) Stop(name string) error {
	m.mu.Lock()

	entry, exists := m.vms[name]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("VM %q is not running", name)
	}

	if entry.info.State != vm.StateRunning {
		state := entry.info.State
		m.mu.Unlock()

		return fmt.Errorf(
			"VM %q is not running (%s)",
			name,
			state,
		)
	}

	entry.info.State = vm.StateStopping

	qmpSocket := entry.qmpSocket

	m.mu.Unlock()

	log.Printf("stopping VM %q via QMP", name)

	client, err := qemu.ConnectQMP(qmpSocket)
	if err != nil {
		return fmt.Errorf("connect to QMP: %w", err)
	}

	defer client.Close()

	if err := client.SystemPowerdown(); err != nil {
		return fmt.Errorf(
			"QMP system_powerdown for VM %q: %w",
			name,
			err,
		)
	}

	return nil
}

func (m *VMManager) Destroy(name string) error {
	m.mu.Lock()

	if entry, exists := m.vms[name]; exists {
		switch entry.info.State {
		case vm.StateRunning, vm.StateStarting, vm.StateStopping:
			m.mu.Unlock()
			return fmt.Errorf(
				"VM %q is still running (%s)",
				name,
				entry.info.State,
			)
		}

		delete(m.vms, name)
	}

	m.mu.Unlock()

	if err := m.store.Delete(name); err != nil {
		return err
	}

	log.Printf("destroyed VM %q", name)

	return nil
}

func (m *VMManager) LoadDefinition(name string) (vm.Definition, error) {
	return m.store.Load(name)
}

func (m *VMManager) Create(def vm.Definition) error {
	return m.store.Create(def)
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

	store := vm.NewStore(vm.DefaultStateDir)
	manager := NewVMManager(store)

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
	case "create":
		drives := make([]vm.Drive, 0, len(request.Config.Drives))

		for _, drive := range request.Config.Drives {
			drives = append(drives, vm.Drive{
				Type:     vm.DriveType(drive.Type),
				Path:     drive.Path,
				Size:     drive.Size,
				ReadOnly: drive.ReadOnly,
			})
		}

		err := manager.Create(vm.Definition{
			Name:   request.Config.Name,
			CPUs:   request.Config.CPUs,
			Memory: request.Config.Memory,
			Drives: drives,
			Network: vm.NetworkConfig{
				Mode:      vm.NetworkMode(request.Config.Network.Mode),
				Interface: request.Config.Network.Interface,
				MAC:       request.Config.Network.MAC,
			},
		})

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

	case "run":
		def, err := manager.LoadDefinition(request.Config.Name)

		if err != nil {
			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		drives := make([]qemu.Drive, 0, len(def.Drives))

		vmDir := filepath.Join(manager.store.BaseDir, def.Name)

		for _, drive := range def.Drives {
			path := drive.Path

			// internal VM-files are relative to the VM directory.
			if !filepath.IsAbs(path) {
				path = filepath.Join(vmDir, path)
			}

			drives = append(drives, qemu.Drive{
				Type:     qemu.DriveType(drive.Type),
				Path:     path,
				Size:     drive.Size,
				ReadOnly: drive.ReadOnly,
			})
		}

		err = manager.Start(ctx, qemu.Config{
			Name:   def.Name,
			CPUs:   def.CPUs,
			Memory: def.Memory,
			Drives: drives,
			Network: qemu.NetworkConfig{
				Mode:      qemu.NetworkMode(def.Network.Mode),
				Interface: def.Network.Interface,
				MAC:       def.Network.MAC,
			},
			BootFromCDROM: true,
			ConsoleType:   qemu.ConsoleVNC,
		})

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

	case "stop":
		err := manager.Stop(request.Config.Name)

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

	case "destroy":
		err := manager.Destroy(request.Config.Name)

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
		vms, err := manager.List()
		if err != nil {
			_ = json.NewEncoder(conn).Encode(ListResponse{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		_ = json.NewEncoder(conn).Encode(ListResponse{
			OK:  true,
			VMs: vms,
		})

	case "console":
		manager.mu.Lock()
		entry, ok := manager.vms[request.Config.Name]

		if !ok {
			manager.mu.Unlock()

			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: fmt.Sprintf("VM %q is not running", request.Config.Name),
			})
			return
		}

		if entry.info.State != vm.StateRunning {
			state := entry.info.State
			manager.mu.Unlock()

			_ = json.NewEncoder(conn).Encode(Response{
				OK: false,
				Error: fmt.Sprintf(
					"VM %q is not running (%s)",
					request.Config.Name,
					state,
				),
			})
			return
		}

		consoleSocket := entry.consoleSocket
		manager.mu.Unlock()

		if consoleSocket == "" {
			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: fmt.Sprintf("VM %q has no console socket", request.Config.Name),
			})
			return
		}

		_ = json.NewEncoder(conn).Encode(Response{
			OK:            true,
			ConsoleSocket: consoleSocket,
		})

	case "info":
		def, err := manager.LoadDefinition(request.Config.Name)
		if err != nil {
			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		manager.mu.Lock()

		entry, running := manager.vms[request.Config.Name]

		var state vm.State
		var qemuVM *qemu.QEMU

		if running {
			state = entry.info.State
			qemuVM = entry.qemu
		} else {
			state = vm.StateStopped
		}

		manager.mu.Unlock()

		info := VMInfo{
			Name:   def.Name,
			State:  state,
			CPUs:   def.CPUs,
			Memory: def.Memory,
			Network: qemu.NetworkConfig{
				Mode:      qemu.NetworkMode(def.Network.Mode),
				Interface: def.Network.Interface,
				MAC:       def.Network.MAC,
			},
		}

		if running && state == vm.StateRunning {
			interfaces, err := qemuVM.GuestNetworkInterfaces()
			if err != nil {
				_ = json.NewEncoder(conn).Encode(Response{
					OK:    false,
					Error: fmt.Sprintf("guest network info: %v", err),
				})
				return
			}

			info.Interfaces = interfaces
		}

		_ = json.NewEncoder(conn).Encode(Response{
			OK:   true,
			Info: &info,
		})

	default:
		_ = json.NewEncoder(conn).Encode(Response{
			OK:    false,
			Error: "unknown command",
		})
	}
}
