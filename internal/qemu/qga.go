package qemu

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type qgaRequest struct {
	Execute   string                 `json:"execute"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type qgaResponse struct {
	Return json.RawMessage `json:"return"`
	Error  *qgaError       `json:"error,omitempty"`
}

type qgaError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

type GuestNetworkInterface struct {
	Name            string           `json:"name"`
	HardwareAddress string           `json:"hardware-address"`
	IPAddresses     []GuestIPAddress `json:"ip-addresses"`
}

type GuestIPAddress struct {
	Type    string `json:"ip-address-type"`
	Address string `json:"ip-address"`
	Prefix  int    `json:"prefix"`
}

type guestExecResult struct {
	Exited   bool   `json:"exited"`
	ExitCode int    `json:"exitcode"`
	OutData  string `json:"out-data"`
	ErrData  string `json:"err-data"`
}

func (q *QEMU) GuestPing() error {
	if q.qga == "" {
		return fmt.Errorf("QGA socket is not configured")
	}

	_, err := qgaExecute(q.qga, "guest-ping", nil)
	return err
}

func (q *QEMU) GuestNetworkInterfaces() ([]GuestNetworkInterface, error) {
	if q.qga == "" {
		return nil, fmt.Errorf("QGA socket is not configured")
	}

	data, err := qgaExecute(
		q.qga,
		"guest-network-get-interfaces",
		nil,
	)
	if err != nil {
		return nil, err
	}

	var interfaces []GuestNetworkInterface

	if err := json.Unmarshal(data, &interfaces); err != nil {
		return nil, fmt.Errorf(
			"decode network interfaces: %w",
			err,
		)
	}

	return interfaces, nil
}

func (q *QEMU) GuestExec(command string, args ...string) (string, string, int, error) {
	if q.qga == "" {
		return "", "", -1, fmt.Errorf("QGA socket is not configured")
	}

	arguments := map[string]interface{}{
		"path":           command,
		"capture-output": true,
	}

	if len(args) > 0 {
		arguments["arg"] = args
	}

	data, err := qgaExecute(
		q.qga,
		"guest-exec",
		arguments,
	)
	if err != nil {
		return "", "", -1, err
	}

	var start struct {
		PID int `json:"pid"`
	}

	if err := json.Unmarshal(data, &start); err != nil {
		return "", "", -1, fmt.Errorf(
			"decode guest-exec response: %w",
			err,
		)
	}

	for i := 0; i < 50; i++ {
		data, err := qgaExecute(
			q.qga,
			"guest-exec-status",
			map[string]interface{}{
				"pid": start.PID,
			},
		)
		if err != nil {
			return "", "", -1, err
		}

		var result guestExecResult

		if err := json.Unmarshal(data, &result); err != nil {
			return "", "", -1, fmt.Errorf(
				"decode guest-exec-status response: %w",
				err,
			)
		}

		if result.Exited {
			return result.OutData, result.ErrData, result.ExitCode, nil
		}

		time.Sleep(100 * time.Millisecond)
	}

	return "", "", -1, fmt.Errorf(
		"guest-exec did not finish within timeout",
	)
}

func qgaExecute(
	socket string,
	command string,
	arguments map[string]interface{},
) (json.RawMessage, error) {
	conn, err := net.DialTimeout(
		"unix",
		socket,
		2*time.Second,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"connect to QGA: %w",
			err,
		)
	}
	defer conn.Close()

	if err := conn.SetDeadline(
		time.Now().Add(2 * time.Second),
	); err != nil {
		return nil, fmt.Errorf(
			"set QGA deadline: %w",
			err,
		)
	}

	request := qgaRequest{
		Execute:   command,
		Arguments: arguments,
	}

	if err := json.NewEncoder(conn).Encode(request); err != nil {
		return nil, fmt.Errorf(
			"send QGA request: %w",
			err,
		)
	}

	var response qgaResponse

	decoder := json.NewDecoder(conn)

	if err := decoder.Decode(&response); err != nil {
		return nil, fmt.Errorf(
			"read QGA response: %w",
			err,
		)
	}

	if response.Error != nil {
		return nil, fmt.Errorf(
			"QGA command %s failed: %s: %s",
			command,
			response.Error.Class,
			response.Error.Desc,
		)
	}

	return response.Return, nil
}
