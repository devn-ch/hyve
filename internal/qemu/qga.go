package qemu

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type qgaRequest struct {
	Execute string `json:"execute"`
}

type qgaResponse struct {
	Return json.RawMessage `json:"return"`
	Error  *qgaError       `json:"error,omitempty"`
}

type qgaError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

func (q *QEMU) GuestPing() error {
	if q.qga == "" {
		return fmt.Errorf("QGA socket is not configured")
	}

	conn, err := net.DialTimeout("unix", q.qga, 2*time.Second)
	if err != nil {
		return fmt.Errorf("connect to QGA: %w", err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	req := qgaRequest{
		Execute: "guest-ping",
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return fmt.Errorf("send QGA request: %w", err)
	}

	var resp qgaResponse

	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return fmt.Errorf("read QGA response: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf(
			"QGA guest-ping failed: %s: %s",
			resp.Error.Class,
			resp.Error.Desc,
		)
	}

	return nil
}
