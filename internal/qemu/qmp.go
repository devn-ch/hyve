package qemu

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

type QMPClient struct {
	conn net.Conn
}

type qmpGreeting struct {
	QMPVersion struct {
		Major int `json:"major"`
		Minor int `json:"minor"`
		Micro int `json:"micro"`
	} `json:"QMP"`
}

type qmpCommand struct {
	Execute string `json:"execute"`
}

func ConnectQMP(socketPath string) (*QMPClient, error) {
	conn, err := net.DialTimeout(
		"unix",
		socketPath,
		2*time.Second,
	)
	if err != nil {
		return nil, fmt.Errorf("connect QMP: %w", err)
	}

	client := &QMPClient{
		conn: conn,
	}

	if err := client.readGreeting(); err != nil {
		conn.Close()
		return nil, err
	}

	if err := client.execute("qmp_capabilities"); err != nil {
		conn.Close()
		return nil, err
	}

	return client, nil
}

func (c *QMPClient) readGreeting() error {
	reader := bufio.NewReader(c.conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read QMP greeting: %w", err)
	}

	var greeting qmpGreeting

	if err := json.Unmarshal(line, &greeting); err != nil {
		return fmt.Errorf("decode QMP greeting: %w", err)
	}

	return nil
}

func (c *QMPClient) execute(command string) error {
	request := qmpCommand{
		Execute: command,
	}

	data, err := json.Marshal(request)
	if err != nil {
		return err
	}

	data = append(data, '\n')

	if _, err := c.conn.Write(data); err != nil {
		return fmt.Errorf("send QMP command: %w", err)
	}

	reader := bufio.NewReader(c.conn)

	line, err := reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read QMP response: %w", err)
	}

	var response map[string]interface{}

	if err := json.Unmarshal(line, &response); err != nil {
		return fmt.Errorf("decode QMP response: %w", err)
	}

	if _, ok := response["error"]; ok {
		return fmt.Errorf("QMP command %q failed", command)
	}

	return nil
}

func (c *QMPClient) SystemPowerdown() error {
	return c.execute("system_powerdown")
}

func (c *QMPClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}
