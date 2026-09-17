
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
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

func main() {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0755); err != nil {
		log.Fatal(err)
	}

	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

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

	go func() {
		<-ctx.Done()
		listener.Close()
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

		go handleConnection(ctx, conn)
	}
}

func handleConnection(ctx context.Context, conn net.Conn) {
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
		q := qemu.New()

		if err := q.Start(ctx, request.Config); err != nil {
			_ = json.NewEncoder(conn).Encode(Response{
				OK:    false,
				Error: err.Error(),
			})
			return
		}

		_ = json.NewEncoder(conn).Encode(Response{
			OK: true,
		})

		log.Printf("VM %q started", request.Config.Name)

		go func() {
			if err := q.Wait(); err != nil {
				log.Printf("VM %q exited: %v", request.Config.Name, err)
			}
		}()

	default:
		_ = json.NewEncoder(conn).Encode(Response{
			OK:    false,
			Error: "unknown command",
		})
	}
}